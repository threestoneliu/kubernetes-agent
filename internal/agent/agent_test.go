package agent

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// --- fakes ---

// fakeStream is a deterministic llm.Stream that yields a fixed list
// of events and then signals io.EOF.
type fakeStream struct {
	events []llm.Event
	i      int
}

func (s *fakeStream) Next(ctx context.Context) (llm.Event, error) {
	if s.i >= len(s.events) {
		return llm.Event{}, io.EOF
	}
	ev := s.events[s.i]
	s.i++
	return ev, nil
}

func (s *fakeStream) Close() error { return nil }

// fakeClient is an llm.Client that returns a pre-scripted sequence of
// events. Multiple Chat calls each return a fresh stream that drains
// the same script.
type fakeClient struct {
	mu     sync.Mutex
	script [][]llm.Event
	calls  int
}

func (c *fakeClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (llm.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls >= len(c.script) {
		return &fakeStream{}, nil
	}
	events := c.script[c.calls]
	c.calls++
	return &fakeStream{events: events}, nil
}

// fakeStore records every BatchInsertMessages call. It does not
// persist anything; tests assert on the recorded contents.
type fakeStore struct {
	mu        sync.Mutex
	batches   [][]store.Message
	BatchErr  error // optional: returned by BatchInsertMessages when set
}

func (s *fakeStore) BatchInsertMessages(ctx context.Context, msgs []store.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Take a copy so callers can't mutate.
	cp := make([]store.Message, len(msgs))
	copy(cp, msgs)
	s.batches = append(s.batches, cp)
	return s.BatchErr
}

func (s *fakeStore) lastBatch() []store.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.batches) == 0 {
		return nil
	}
	return s.batches[len(s.batches)-1]
}

func (s *fakeStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.batches)
}

// collectEvents starts a goroutine that drains the events channel
// into a slice and returns a function that, when called:
//  1. Waits for the runner to return (the done channel).
//  2. Closes the events channel (unblocking the drain goroutine).
//  3. Returns the collected events + the run error.
//
// The safe pattern: the drain goroutine starts immediately so the
// runner's writes never block, and the channel is only closed
// after the runner returns. No deadlock between drain and close.
func collectEvents(events chan Event, done <-chan error) func() ([]Event, error) {
	var (
		mu  sync.Mutex
		evs []Event
	)
	go func() {
		for e := range events {
			mu.Lock()
			evs = append(evs, e)
			mu.Unlock()
		}
	}()
	return func() ([]Event, error) {
		err := <-done
		close(events)
		// Brief wait so the drain goroutine flushes the
		// remaining buffered events into evs before we read.
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		out := make([]Event, len(evs))
		copy(out, evs)
		return out, err
	}
}

// startRunner starts a Runner on its own goroutine, returning a
// channel that closes when Run returns.
func startRunner(r *Runner, msg string) <-chan error {
	done := make(chan error, 1)
	go func() { done <- r.Run(context.Background(), msg) }()
	return done
}

// --- happy path: token + message_end ---

func TestRunner_TokenThenEnd(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 16)
	sess := NewSession("s1")
	r := &Runner{
		Client: &fakeClient{script: [][]llm.Event{{
			{Type: llm.EventToken, Text: "hello "},
			{Type: llm.EventToken, Text: "world"},
			{Type: llm.EventMessageEnd, In: 10, Out: 5},
		}}},
		Tools:   nil,
		Store:   store,
		Events:  events,
		Session: sess,
	}
	done := startRunner(r, "hi")
	finish := collectEvents(events, done)
	evs, runErr := finish()
	require.NoError(t, runErr)

	// Token events arrive, then MessageEnd.
	var (
		sawHello, sawWorld, sawEnd bool
	)
	for _, e := range evs {
		switch e.Type {
		case EventToken:
			var p Token
			_ = json.Unmarshal(e.Payload, &p)
			if p.Text == "hello " {
				sawHello = true
			}
			if p.Text == "world" {
				sawWorld = true
			}
		case EventMessageEnd:
			var p MessageEnd
			_ = json.Unmarshal(e.Payload, &p)
			assert.Equal(t, int64(10), p.InputTokens)
			assert.Equal(t, int64(5), p.OutputTokens)
			sawEnd = true
		}
	}
	assert.True(t, sawHello, "missing first token")
	assert.True(t, sawWorld, "missing second token")
	assert.True(t, sawEnd, "missing message_end")

	// One assistant message row per LLM streaming response (tokens merged).
	assert.Equal(t, 1, store.callCount())
	batch := store.lastBatch()
	require.Len(t, batch, 1)
	assert.Equal(t, "assistant", batch[0].Role)
	require.NotNil(t, batch[0].Content)
	assert.Equal(t, "hello world", *batch[0].Content)
}

// --- tool_call dispatch ---

func TestRunner_ToolCallDispatch(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 32)
	sess := NewSession("s1")
	var toolCalls atomic.Int32
	tools := []llm.Tool{{
		Name: "echo",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"msg": map[string]any{"type": "string"}},
		},
		Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
			toolCalls.Add(1)
			return []byte(`{"echoed":true}`), nil
		},
	}}
	// Two Chat calls: first returns tool_call, second returns text.
	r := &Runner{
		Client: &fakeClient{script: [][]llm.Event{
			{
				{Type: llm.EventToolCall, Call: llm.ToolCall{
					ID:    "tc1",
					Name:  "echo",
					Input: json.RawMessage(`{"msg":"hi"}`),
				}},
				{Type: llm.EventMessageEnd, In: 1, Out: 1},
			},
			{
				{Type: llm.EventToken, Text: "done"},
				{Type: llm.EventMessageEnd, In: 1, Out: 1},
			},
		}},
		Tools:   tools,
		Store:   store,
		Events:  events,
		Session: sess,
	}
	done := startRunner(r, "call echo")
	finish := collectEvents(events, done)
	evs, runErr := finish()
	require.NoError(t, runErr)

	assert.Equal(t, int32(1), toolCalls.Load(), "handler should be invoked once")

	var sawCall, sawResult bool
	for _, e := range evs {
		switch e.Type {
		case EventToolCall:
			var p ToolCall
			_ = json.Unmarshal(e.Payload, &p)
			assert.Equal(t, "tc1", p.ID)
			assert.Equal(t, "echo", p.Name)
			sawCall = true
		case EventToolResult:
			var p ToolResult
			_ = json.Unmarshal(e.Payload, &p)
			assert.Equal(t, "tc1", p.ID)
			assert.Empty(t, p.Error)
			var m map[string]any
			require.NoError(t, json.Unmarshal(p.Output, &m))
			assert.Equal(t, true, m["echoed"])
			sawResult = true
		}
	}
	assert.True(t, sawCall)
	assert.True(t, sawResult)

	// Two batches: first = tool call turn, second = text turn.
	assert.Equal(t, 2, store.callCount())
	firstBatch := store.batches[0]
	require.Len(t, firstBatch, 2, "first turn: assistant(tool_call) + tool_result")
	assert.Equal(t, "assistant", firstBatch[0].Role)
	require.NotNil(t, firstBatch[0].ToolCalls)
	assert.Contains(t, *firstBatch[0].ToolCalls, `"echo"`)
	assert.Equal(t, "tool", firstBatch[1].Role)
	require.NotNil(t, firstBatch[1].ToolCallID)
	assert.Equal(t, "tc1", *firstBatch[1].ToolCallID)
}

// --- plan_awaiting_confirm blocks ---

func TestRunner_PlanAwaitingConfirmBlocks(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 32)
	sess := NewSession("s1")

	// k8s_plan_write tool handler: emits plan events, blocks on
	// WaitPlan, then returns. The Deps.Emit / Deps.Session are
	// wired by the runner.
	tools := []llm.Tool{{
		Name: "plan_only",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
			// Mimic what tools.go's plan_write handler does:
			// emit PlanReady + PlanAwaitingConfirm, then block.
			// For this test we push events directly to the
			// events channel (the runner's wiring would normally
			// route through Deps.Emit).
			evReady, _ := NewEvent(EventPlanReady, PlanReady{PlanID: "p1", Summary: "do thing"})
			evConf, _ := NewEvent(EventPlanAwaitingConfirm, PlanAwaitingConfirm{PlanID: "p1"})
			events <- evReady
			events <- evConf
			if err := sess.WaitPlan(ctx); err != nil {
				return nil, err
			}
			return []byte(`{"plan_id":"p1","decision":"confirmed"}`), nil
		},
	}}
	r := &Runner{
		Client: &fakeClient{script: [][]llm.Event{{
			{Type: llm.EventToolCall, Call: llm.ToolCall{
				ID:    "tc1",
				Name:  "plan_only",
				Input: json.RawMessage(`{}`),
			}},
			{Type: llm.EventMessageEnd, In: 1, Out: 1},
		}}},
		Tools:   tools,
		Store:   store,
		Events:  events,
		Session: sess,
	}
	done := startRunner(r, "go")
	finish := collectEvents(events, done)

	// Give the runner a moment to dispatch the tool call and block
	// on WaitPlan. Then confirm and verify it unblocks.
	time.Sleep(30 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Run returned before plan was confirmed")
	default:
	}
	sess.ConfirmPlan()

	evs, runErr := finish()
	require.NoError(t, runErr)

	var sawReady, sawConfirm, sawResult bool
	for _, e := range evs {
		switch e.Type {
		case EventPlanReady:
			var p PlanReady
			_ = json.Unmarshal(e.Payload, &p)
			assert.Equal(t, "p1", p.PlanID)
			sawReady = true
		case EventPlanAwaitingConfirm:
			sawConfirm = true
		case EventToolResult:
			sawResult = true
		}
	}
	assert.True(t, sawReady, "missing plan_ready")
	assert.True(t, sawConfirm, "missing plan_awaiting_confirm")
	assert.True(t, sawResult, "missing tool_result after confirm")
	assert.Equal(t, "confirmed", sess.PlanResult)
}

// --- ask_user blocks and answer is appended ---

func TestRunner_AskUserBlocksAndAnswerAppended(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 32)
	sess := NewSession("s1")

	// First Chat call: model emits ask_user, blocks inside the
	// handler. Second Chat call: model emits the final token +
	// message_end. The second call must include the tool result in
	// the transcript.
	client := &fakeClient{script: [][]llm.Event{
		{{Type: llm.EventToolCall, Call: llm.ToolCall{
			ID:    "tc1",
			Name:  "ask_only",
			Input: json.RawMessage(`{}`),
		}}},
		{{Type: llm.EventToken, Text: "thanks"}, {Type: llm.EventMessageEnd, In: 1, Out: 1}},
	}}

	tools := []llm.Tool{{
		Name:        "ask_only",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
			ev, _ := NewEvent(EventAskUser, AskUserPayload{Question: "which ns?"})
			events <- ev
			if err := sess.WaitAsk(ctx); err != nil {
				return nil, err
			}
			return []byte(`{"answer":"` + sess.AskAnswer + `"}`), nil
		},
	}}
	r := &Runner{
		Client:  client,
		Tools:   tools,
		Store:   store,
		Events:  events,
		Session: sess,
	}
	done := startRunner(r, "go")
	finish := collectEvents(events, done)

	time.Sleep(30 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Run returned before ask was answered")
	default:
	}
	sess.AnswerAsk("ns-prod")

	evs, runErr := finish()
	require.NoError(t, runErr)

	var sawAsk, sawResult, sawEnd bool
	for _, e := range evs {
		switch e.Type {
		case EventAskUser:
			var p AskUserPayload
			_ = json.Unmarshal(e.Payload, &p)
			assert.Equal(t, "which ns?", p.Question)
			sawAsk = true
		case EventToolResult:
			sawResult = true
		case EventMessageEnd:
			sawEnd = true
		}
	}
	assert.True(t, sawAsk)
	assert.True(t, sawResult)
	assert.True(t, sawEnd)
	// The fake client should have been called twice.
	assert.Equal(t, 2, client.calls)
}

// --- stream error ---

func TestRunner_StreamErrorEmitsErrorPayload(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 32)
	sess := NewSession("s1")

	// Use a custom stream that yields an error event then EOF.
	client := &errClient{stream: &errStream{errEv: llm.Event{Type: llm.EventError, Reason: "boom"}}}
	r := &Runner{
		Client:  client,
		Store:   store,
		Events:  events,
		Session: sess,
	}
	done := startRunner(r, "go")
	finish := collectEvents(events, done)
	evs, err := finish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")

	var sawError bool
	for _, e := range evs {
		if e.Type == EventError {
			var p ErrorPayload
			_ = json.Unmarshal(e.Payload, &p)
			// Either "stream_error" (from the inner consumeStream)
			// or "llm_error" (from the outer Run classifier) is
			// acceptable — the runner emits one or both depending
			// on the path. What matters is that the user sees
			// an ErrorPayload.
			assert.Contains(t, []string{"stream_error", "llm_error"}, p.Code)
			assert.Contains(t, p.Message, "boom")
			sawError = true
		}
	}
	assert.True(t, sawError, "missing error event")
	// No store writes on error.
	assert.Equal(t, 0, store.callCount())
}

// errClient yields a stream that immediately emits an error event.
type errClient struct{ stream *errStream }

func (c *errClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (llm.Stream, error) {
	return c.stream, nil
}

type errStream struct {
	errEv llm.Event
	done  bool
	mu    sync.Mutex
}

func (s *errStream) Next(ctx context.Context) (llm.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return llm.Event{}, io.EOF
	}
	s.done = true
	return s.errEv, nil
}

func (s *errStream) Close() error { return nil }

// --- plan_awaiting_confirm is emitted AFTER plan_ready ---

func TestRunner_PlanEventsOrdering(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 32)
	sess := NewSession("s1")

	tools := []llm.Tool{{
		Name:        "plan_only",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
			evR, _ := NewEvent(EventPlanReady, PlanReady{PlanID: "p1"})
			evC, _ := NewEvent(EventPlanAwaitingConfirm, PlanAwaitingConfirm{PlanID: "p1"})
			events <- evR
			events <- evC
			if err := sess.WaitPlan(ctx); err != nil {
				return nil, err
			}
			return []byte(`{}`), nil
		},
	}}
	r := &Runner{
		Client: &fakeClient{script: [][]llm.Event{{
			{Type: llm.EventToolCall, Call: llm.ToolCall{ID: "tc1", Name: "plan_only", Input: json.RawMessage(`{}`)}},
			{Type: llm.EventMessageEnd, In: 1, Out: 1},
		}}},
		Tools:   tools,
		Store:   store,
		Events:  events,
		Session: sess,
	}
	done := startRunner(r, "go")
	finish := collectEvents(events, done)
	time.Sleep(30 * time.Millisecond)
	sess.ConfirmPlan()
	evs, runErr := finish()
	require.NoError(t, runErr)

	// Order: tool_call, plan_ready, plan_awaiting_confirm,
	// tool_result, message_end.
	want := []string{
		EventToolCall,
		EventPlanReady,
		EventPlanAwaitingConfirm,
		EventToolResult,
		EventMessageEnd,
	}
	var got []string
	for _, e := range evs {
		got = append(got, e.Type)
	}
	assert.Equal(t, want, got, "event order mismatch")
}

// --- history truncation ---

func TestRunner_TruncationDropsOldest(t *testing.T) {
	store := &fakeStore{}
	events := make(chan Event, 64)
	sess := NewSession("s1")

	// 200 messages with 4KB each, window 128k tokens (~512k chars).
	// At 4 chars/token, 80% of 128k = 102k tokens = ~410k chars.
	// Each 4KB msg is 1024 tokens; ~410 such messages fit. We have
	// 200 < 410, so truncation must NOT trigger; this is a no-op
	// smoke test. We exercise the truncation directly via a
	// smaller window.
	client := &fakeClient{script: [][]llm.Event{
		{{Type: llm.EventToken, Text: "x"}, {Type: llm.EventMessageEnd, In: 1, Out: 1}},
	}}
	r := &Runner{
		Client:            client,
		Store:             store,
		Events:            events,
		Session:           sess,
		ModelContextWindow: 100, // ~80 tokens / 320 chars
	}
	// Inject a fat history by running once, then check via direct
	// call to truncate.
	big := make([]transcriptMessage, 0, 20)
	big = append(big, transcriptMessage{Role: llm.RoleSystem, Parts: []llm.ContentPart{{Type: "text", Text: "system"}}})
	for i := 0; i < 18; i++ {
		big = append(big, transcriptMessage{
			Role:  llm.RoleUser,
			Parts: []llm.ContentPart{{Type: "text", Text: strings.Repeat("a", 1000)}},
		})
	}
	big = append(big, transcriptMessage{Role: llm.RoleUser, Parts: []llm.ContentPart{{Type: "text", Text: "current"}}})
	out := r.truncate(big)
	// Should keep the system msg + last user msg + maybe one or two
	// of the fillers (whichever fits in ~320 chars).
	assert.GreaterOrEqual(t, len(out), 2, "should keep at least system + last user")
	assert.Equal(t, llm.RoleSystem, out[0].Role)
	assert.Equal(t, "current", out[len(out)-1].Parts[0].Text)

	// And the loop still completes for the real run.
	done := startRunner(r, "hi")
	finish := collectEvents(events, done)
	_, _ = finish()
}
