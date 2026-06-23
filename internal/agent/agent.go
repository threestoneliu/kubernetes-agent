package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// MessageStore is the slice of the SQLite store.Message repo the
// agent loop needs. Defined as an interface so tests can swap in a
// fake without dragging in the database.
type MessageStore interface {
	BatchInsertMessages(ctx context.Context, msgs []store.Message) error
}

// Runner is one turn of the agent loop. The HTTP layer constructs a
// fresh Runner per request: it carries the LLM client, the
// registered tools, the store handle, the SSE event channel, the
// session, and the tool handler dependencies.
//
// MaxRetries bounds the count of retries on transient (429 / 5xx)
// stream errors. A value <= 0 disables retry.
type Runner struct {
	Client llm.Client
	Tools  []llm.Tool
	Store  MessageStore
	Events chan<- Event
	// Session is the per-conversation state. Plan/ask confirmations
	// block via Session.WaitPlan / Session.WaitAsk.
	Session *Session
	// Deps is the shared tool handler context (factory, engine,
	// store, session, emit). It is used to build the per-call
	// emit callback passed to the tool handlers.
	Deps ToolDeps
	// SystemPrompt overrides llm.SystemPrompt when non-empty.
	SystemPrompt string
	// SkillsPrompt contains the <available_skills> XML injected
	// into the system prompt for LLM skill matching.
	SkillsPrompt string
	// MaxRetries bounds retry attempts on transient stream errors.
	// Defaults to 1 when zero.
	MaxRetries int
	// ModelContextWindow is the model's context window in tokens.
	// When the transcript exceeds 80% of this, the runner drops
	// the oldest non-system messages. Defaults to 128000 (Claude
	// 3-class default) when zero.
	ModelContextWindow int
	// Now is injectable for tests. Defaults to time.Now.
	Now func() time.Time
}

// SetSession sets the Session and Events channel needed by the
// scheduler's trigger path. It is separate from Runner.Run so the
// scheduler (which cannot import the agent package) can inject a
// session without going through the chat handler that normally wires
// both fields.
func (r *Runner) SetSession(sessionID, clusterID string) {
	r.Session = &Session{
		ID:        sessionID,
		ClusterID: clusterID,
	}
	r.Events = make(chan Event, 64)
}

// transcriptMessage is the agent loop's view of one entry in the chat
// history. We keep the full ContentPart slice (text, tool_call,
// tool_result) so the runner can reconstruct the full Message that
// the LLM provider expects.
type transcriptMessage struct {
	Role    llm.Role
	Parts   []llm.ContentPart
}

// Run drives one assistant turn in response to userMessage. It
// streams events to r.Events and returns nil on clean completion, or
// a non-nil error if the loop is unable to make progress (e.g. a
// non-retryable stream error, context cancellation, or store
// failure).
//
// The outer loop calls Client.Chat repeatedly: the LLM may respond
// with a tool call, the runner dispatches the handler synchronously
// (plan/ask block inside the handler), the tool result is appended
// to the transcript, and Chat is called again. The loop terminates
// when the LLM produces a final message (no tool calls) or when an
// error is encountered.
//
// On clean completion, all messages generated during the turn
// (assistant + tool) are batch-inserted to the store as a single
// transaction, keyed by the session id.
func (r *Runner) Run(ctx context.Context, userMessage string) error {
	if r.Session == nil {
		return errors.New("agent.Run: Session is nil")
	}
	if r.Events == nil {
		return errors.New("agent.Run: Events channel is nil")
	}
	if r.Deps.Emit == nil {
		// Wire the events channel as the default emit sink so
		// tool handlers (plan / ask) can surface their events.
		r.Deps.Emit = func(e Event) { r.Events <- e }
	}
	if r.Deps.Session == nil {
		r.Deps.Session = r.Session
	}

	msgs := []transcriptMessage{
		{Role: llm.RoleSystem, Parts: []llm.ContentPart{{Type: "text", Text: r.systemPrompt()}}},
	}
	// Load session history from store, oldest first.
	if r.Deps.Store != nil {
		history, err := r.Deps.Store.ListMessagesBySession(ctx, r.Session.ID)
		if err == nil && len(history) > 0 {
			for _, m := range history {
				parts := contentToParts(m)
				if len(parts) > 0 {
					msgs = append(msgs, transcriptMessage{Role: llm.Role(m.Role), Parts: parts})
				}
			}
		}
	}
	// Current user turn goes last.
	msgs = append(msgs, transcriptMessage{Role: llm.RoleUser, Parts: []llm.ContentPart{{Type: "text", Text: userMessage}}})

	// Debug: log the system prompt with available skills
	if r.SkillsPrompt != "" {
		slog.Debug("agent: system prompt with skills",
			"session_id", r.Session.ID,
			"system_prompt_length", len(r.systemPrompt()),
			"skills_prompt_length", len(r.SkillsPrompt),
			"history_messages", len(msgs)-2, // exclude system and current user
			"skills_prompt", r.SkillsPrompt)
	}

	maxRetries := r.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries == 0 {
		maxRetries = 1
	}

	// outerStep guards against pathological infinite loops where the
	// LLM keeps calling tools without ever producing a final
	// message. 32 steps is well above any realistic agent run.
	const outerStepLimit = 32
	// msgs accumulates the full in-memory transcript (for LLM context).
	// Each LLM API call is persisted at EventMessageEnd so that on reload
	// the frontend sees the exact same sequence as during streaming.
	for step := 0; step < outerStepLimit; step++ {
		var pendingAssistant []llm.ContentPart
		var pendingToolResults []llm.ContentPart
		var streamErr error
		var hadToolCall bool

		for attempt := 0; attempt <= maxRetries; attempt++ {
			msgs = r.truncate(msgs)
			stream, err := r.Client.Chat(ctx, toLLMMessages(msgs), r.Tools)
			if err != nil {
				if isRetryable(err) && attempt < maxRetries {
					time.Sleep(retryBackoff(attempt))
					continue
				}
				_ = r.emitError("llm_error", err.Error(), isRetryable(err))
				return err
			}
			hadToolCall, streamErr = r.consumeStream(ctx, stream, &pendingAssistant, &pendingToolResults)
			_ = stream.Close()
			break
		}

		if streamErr != nil {
			if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
				return streamErr
			}
			_ = r.emitError("llm_error", streamErr.Error(), isRetryable(streamErr))
			return streamErr
		}

		// Append to in-memory transcript so the LLM sees it on the next call.
		if len(pendingAssistant) > 0 {
			msgs = append(msgs, transcriptMessage{Role: llm.RoleAssistant, Parts: pendingAssistant})
		}
		for _, p := range pendingToolResults {
			msgs = append(msgs, transcriptMessage{Role: llm.RoleTool, Parts: []llm.ContentPart{p}})
		}

		// Persist this LLM call's output at EventMessageEnd.
		// hadToolCall only controls whether we loop for another call,
		// not whether we persist this call's content.
		if r.Store != nil {
			if err := r.persistTurn(ctx, pendingAssistant, pendingToolResults); err != nil {
				_ = r.emitError("store_error", err.Error(), true)
				return err
			}
		}

		if hadToolCall {
			continue
		}

		return nil
	}
	return errors.New("agent.Run: outer step limit exceeded")
}

// consumeStream pulls events from stream until io.EOF, a stream
// error, or a message_end. It accumulates the assistant turn's
// content into pendingAssistant and tool results into
// pendingToolResults so the caller can persist them in one batch.
//
// Returns (hadToolCall=true, err=nil) if any tool was dispatched in
// this stream (the caller will loop back and call Chat again with
// the tool result in the transcript). Returns (false, nil) on a
// clean message_end with no tool calls (final assistant turn).
// Returns (false, err) on any error.
func (r *Runner) consumeStream(
	ctx context.Context,
	stream llm.Stream,
	pendingAssistant *[]llm.ContentPart,
	pendingToolResults *[]llm.ContentPart,
) (bool, error) {
	hadToolCall := false
	for {
		ev, err := stream.Next(ctx)
		if errors.Is(err, io.EOF) {
			return hadToolCall, nil
		}
		if err != nil {
			return hadToolCall, err
		}
		switch ev.Type {
		case llm.EventToken:
			r.emit(EventToken, Token{Text: ev.Text})
			*pendingAssistant = append(*pendingAssistant, llm.ContentPart{Type: "text", Text: ev.Text})
		case llm.EventReasoning:
			r.emit(EventReasoning, Reasoning{Text: ev.Text})
			*pendingAssistant = append(*pendingAssistant, llm.ContentPart{Type: "reasoning", Text: ev.Text})
		case llm.EventToolCall:
			hadToolCall = true
			// Emit the tool_call event first so the frontend can
			// render the in-flight call, then dispatch the handler.
			r.emit(EventToolCall, ToolCall{ID: ev.Call.ID, Name: ev.Call.Name, Input: ev.Call.Input})
			*pendingAssistant = append(*pendingAssistant, llm.ContentPart{
				Type:       "tool_call",
				ToolCallID: ev.Call.ID,
				ToolName:   ev.Call.Name,
				Input:      ev.Call.Input,
			})
			output, callErr := r.dispatch(ctx, ev.Call)
			if callErr != nil {
				errStr := callErr.Error()
				r.emit(EventToolResult, ToolResult{ID: ev.Call.ID, Error: errStr})
				*pendingToolResults = append(*pendingToolResults, llm.ContentPart{
					Type:       "tool_result",
					ToolCallID: ev.Call.ID,
					ToolName:   ev.Call.Name,
					Output:     errStr,
					IsError:    true,
				})
				continue
			}
			// output may be nil for tools that don't return
			// content (rare). Treat as empty object.
			out := string(output)
			if out == "" {
				out = "{}"
			}
			raw := json.RawMessage(out)
			if !json.Valid(raw) {
				raw = json.RawMessage(`"` + jsonEscape(out) + `"`)
			}
			r.emit(EventToolResult, ToolResult{ID: ev.Call.ID, Output: raw})
			*pendingToolResults = append(*pendingToolResults, llm.ContentPart{
				Type:       "tool_result",
				ToolCallID: ev.Call.ID,
				ToolName:   ev.Call.Name,
				Output:     out,
			})
		case llm.EventMessageEnd:
			r.emit(EventMessageEnd, MessageEnd{InputTokens: ev.In, OutputTokens: ev.Out})
			return hadToolCall, nil
		case llm.EventError:
			// The outer Run loop will emit a single classified
			// ErrorPayload (with Retryable set) after seeing
			// the returned error. Don't double-emit here.
			return hadToolCall, errors.New(ev.Reason)
		default:
			// Unknown event type: ignore (forward-compatible with
			// future additions).
		}
	}
}

// dispatch routes a tool call to the registered handler. The tool
// name lookup is a linear scan — for the MVP with 6 tools this is
// fine.
func (r *Runner) dispatch(ctx context.Context, call llm.ToolCall) ([]byte, error) {
	for _, t := range r.Tools {
		if t.Name == call.Name {
			return t.Handler(ctx, call)
		}
	}
	return nil, fmt.Errorf("agent: tool %q not registered", call.Name)
}

// emit serialises an event and writes it to the channel. If the
// channel is full or closed, the event is dropped (we don't block
// the agent loop on a slow SSE consumer). The HTTP layer is
// responsible for backpressure.
func (r *Runner) emit(t string, payload any) {
	e, err := NewEvent(t, payload)
	if err != nil {
		return
	}
	select {
	case r.Events <- e:
	default:
		// drop on full channel
	}
}

// emitError is a convenience wrapper that emits an ErrorPayload event.
func (r *Runner) emitError(code, msg string, retryable bool) error {
	r.emit(EventError, ErrorPayload{Code: code, Message: msg, Retryable: retryable})
	return fmt.Errorf("%s: %s", code, msg)
}

// systemPrompt returns the configured system prompt or the package
// default, plus any injected skills prompt. Skills prompt is placed
// FIRST so it has higher attention weight — the LLM is more likely
// to follow the "load skill before acting" rule when it appears at
// the top of the system message.
func (r *Runner) systemPrompt() string {
	base := r.SystemPrompt
	if base == "" {
		base = llm.SystemPrompt
	}
	if r.SkillsPrompt != "" {
		return r.SkillsPrompt + "\n\n" + base
	}
	return base
}

// truncate drops oldest non-system messages until the transcript's
// estimated token count is below 80% of the context window. Token
// estimation uses the 4-chars-per-token heuristic (good enough for
// the MVP — exact BPE counts require a tokenizer dependency).
func (r *Runner) truncate(msgs []transcriptMessage) []transcriptMessage {
	window := r.ModelContextWindow
	if window <= 0 {
		window = 128000
	}
	limit := int64(float64(window) * 0.8)
	chars := estimateChars(msgs)
	tokens := chars / 4
	if tokens <= limit {
		return msgs
	}
	// Drop oldest non-system messages one at a time until under
	// limit. We never drop the leading system message (msgs[0])
	// and we keep the trailing user message (msgs[last]).
	if len(msgs) <= 2 {
		return msgs
	}
	out := []transcriptMessage{msgs[0]}
	drop := 1 // index of the next candidate to consider adding
	for drop < len(msgs)-1 {
		// Subtract the message we are dropping from the running
		// char count.
		chars -= estimateChars([]transcriptMessage{msgs[drop]})
		drop++
		if chars/4 <= limit {
			break
		}
	}
	// Append the surviving middle messages.
	for i := drop; i < len(msgs)-1; i++ {
		out = append(out, msgs[i])
	}
	// Always keep the last message (current user turn).
	out = append(out, msgs[len(msgs)-1])
	return out
}

// estimateChars sums the text length of every content part in msgs.
// Tool calls and tool results contribute their serialised JSON.
func estimateChars(msgs []transcriptMessage) int64 {
	var n int64
	for _, m := range msgs {
		for _, p := range m.Parts {
			if p.Text != "" {
				n += int64(len(p.Text))
			}
			if p.Input != nil {
				n += int64(len(p.Input))
			}
			if p.Output != "" {
				n += int64(len(p.Output))
			}
		}
	}
	return n
}

// toLLMMessages converts the agent loop's transcript view into the
// []llm.Message the LLM client expects.
func toLLMMessages(msgs []transcriptMessage) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		// Skip empty messages (a tool call with no output can
		// produce an assistant part with no text).
		hasContent := false
		for _, p := range m.Parts {
			if p.Text != "" || len(p.Input) > 0 || p.Output != "" {
				hasContent = true
				break
			}
		}
		if !hasContent {
			continue
		}
		out = append(out, llm.Message{Role: m.Role, Content: m.Parts})
	}
	return out
}

// persistTurn writes one assistant message row per LLM streaming response,
// storing only the fields that were present in that response (reasoning,
// content, tool_calls). Tool results are stored as separate rows.
func (r *Runner) persistTurn(ctx context.Context, assistant []llm.ContentPart, toolResults []llm.ContentPart) error {
	now := r.now().Unix()
	stMsgs := make([]store.Message, 0, 1+len(toolResults))

	// Merge this streaming response's parts into one assistant message.
	// Only non-empty fields are set.
	var textContent strings.Builder
	var reasoningContent strings.Builder
	var toolCalls []map[string]any
	for _, p := range assistant {
		switch p.Type {
		case "text":
			textContent.WriteString(p.Text)
		case "reasoning":
			reasoningContent.WriteString(p.Text)
		case "tool_call":
			toolCalls = append(toolCalls, map[string]any{
				"id":    p.ToolCallID,
				"name":  p.ToolName,
				"input": json.RawMessage(p.Input),
			})
		}
	}

	tc := textContent.String()
	reasoning := reasoningContent.String()
	asstMsg := store.Message{
		ID:        uuid.NewString(),
		SessionID: r.Session.ID,
		Role:      string(llm.RoleAssistant),
		CreatedAt: now,
	}
	if tc != "" {
		asstMsg.Content = &tc
	}
	if reasoning != "" {
		asstMsg.Reasoning = &reasoning
	}
	if len(toolCalls) > 0 {
		b, _ := json.Marshal(toolCalls)
		s := string(b)
		asstMsg.ToolCalls = &s
	}
	stMsgs = append(stMsgs, asstMsg)

	// Tool results: one row per tool call.
	for _, p := range toolResults {
		tr := map[string]any{
			"tool_call_id": p.ToolCallID,
			"name":         p.ToolName,
			"output":       p.Output,
			"is_error":     p.IsError,
		}
		b, _ := json.Marshal(tr)
		out := string(b)
		tcid := p.ToolCallID
		stMsgs = append(stMsgs, store.Message{
			ID:         uuid.NewString(),
			SessionID:  r.Session.ID,
			Role:       string(llm.RoleTool),
			Content:    &out,
			ToolCallID: &tcid,
			CreatedAt:  now,
		})
	}

	return r.Store.BatchInsertMessages(ctx, stMsgs)
}

// now returns the current time, falling back to time.Now.
func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// isRetryable classifies a stream / chat error as transient (worth
// retrying) or not. We rely on the HTTP status code embedded in the
// error message — fantasy / the underlying SDK includes it.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 401/403: auth/permission — not retryable.
	if strings.Contains(msg, "status 401") || strings.Contains(msg, "status 403") {
		return false
	}
	// 429: rate-limited — retryable.
	if strings.Contains(msg, "status 429") {
		return true
	}
	// 5xx: server error — retryable.
	if strings.Contains(msg, "status 5") {
		return true
	}
	// Connection / network errors are typically retryable.
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "EOF") {
		return true
	}
	// Default: non-retryable. Avoid hot-looping on every error.
	return false
}

// retryBackoff returns the wait before retry attempt N. Currently
// 1s flat per design D12; a future improvement can switch to
// exponential backoff with jitter.
func retryBackoff(attempt int) time.Duration {
	return time.Second
}

// jsonEscape escapes s so it can be embedded inside a JSON string
// literal. We use it when a tool handler returns non-JSON output.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// contentToParts converts a store.Message into the []llm.ContentPart
// form used by the agent loop's transcript. The reverse direction
// (transcript → store) is handled by persistTurn.
func contentToParts(m store.Message) []llm.ContentPart {
	var parts []llm.ContentPart
	// Text content.
	if m.Content != nil && *m.Content != "" {
		parts = append(parts, llm.ContentPart{Type: "text", Text: *m.Content})
	}
	// Reasoning content.
	if m.Reasoning != nil && *m.Reasoning != "" {
		parts = append(parts, llm.ContentPart{Type: "reasoning", Text: *m.Reasoning})
	}
	// Tool calls: stored as JSON array of {id, name, input}.
	if m.ToolCalls != nil && *m.ToolCalls != "" {
		var calls []map[string]any
		if err := json.Unmarshal([]byte(*m.ToolCalls), &calls); err == nil {
			for _, c := range calls {
				id, _ := c["id"].(string)
				name, _ := c["name"].(string)
				inputRaw, _ := c["input"].(json.RawMessage)
				parts = append(parts, llm.ContentPart{
					Type:       "tool_call",
					ToolCallID: id,
					ToolName:   name,
					Input:      inputRaw,
				})
			}
		}
	}
	// Tool result: stored in Content as JSON {tool_call_id, name, output, is_error}.
	// Only present for role=tool messages.
	if m.Role == string(llm.RoleTool) && m.Content != nil && *m.Content != "" {
		var result map[string]any
		if err := json.Unmarshal([]byte(*m.Content), &result); err == nil {
			tcid, _ := result["tool_call_id"].(string)
			name, _ := result["name"].(string)
			output, _ := result["output"].(string)
			isErr, _ := result["is_error"].(bool)
			parts = append(parts, llm.ContentPart{
				Type:       "tool_result",
				ToolCallID: tcid,
				ToolName:   name,
				Output:     output,
				IsError:    isErr,
			})
		}
	}
	return parts
}

// Compile-time check: http.Status* is referenced indirectly by the
// retry classifier. Touching it here keeps the import set honest
// when we add direct status checks later.
var _ = http.StatusTooManyRequests
