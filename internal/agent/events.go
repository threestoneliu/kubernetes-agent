// Package agent implements the agent loop, SSE event types, and the
// per-session state machine that drives a single chat turn.
//
// The package bridges two interfaces:
//
//   - llm.Stream (provider-agnostic streaming events) -> []Event
//     (SSE payloads consumed by internal/http).
//   - plan/ask confirmations come back as ResumePlan / ResumeAsk
//     channel closes on the Session, which the runner blocks on.
package agent

import (
	"encoding/json"

	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// Event is a single SSE frame: a type discriminator + a marshalled
// JSON payload. The HTTP layer in internal/http serialises Event
// directly to the SSE wire format.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewEvent marshals v to JSON and wraps it as an Event. The HTTP layer
// relies on json.RawMessage so the encoded payload is preserved
// verbatim (no double-marshalling).
func NewEvent(t string, v any) (Event, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Event{}, err
	}
	return Event{Type: t, Payload: b}, nil
}

// MustNewEvent panics on marshal failure. Use only for compile-time
// constants where the payload type is known to be JSON-serialisable.
func MustNewEvent(t string, v any) Event {
	e, err := NewEvent(t, v)
	if err != nil {
		panic(err)
	}
	return e
}

// --- Payload types — the 12 SSE event types per design D6. ---

// SessionMeta is emitted once per turn as the first event, before any
// content. The frontend uses session_id / cluster_id to label the
// conversation.
type SessionMeta struct {
	SessionID string `json:"session_id"`
	ClusterID string `json:"cluster_id,omitempty"`
}

// Reasoning carries an extended-thinking chunk (Anthropic) or o-series
// reasoning (OpenAI). Frontend collapses / folds it.
type Reasoning struct {
	Text string `json:"text"`
}

// Token is a single assistant text delta. The full assistant message
// is reassembled client-side.
type Token struct {
	Text string `json:"text"`
}

// ToolCall announces a model-issued tool call. The runner will follow
// up with a ToolResult event once the handler returns.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult is the outcome of a tool call. Error is set when the
// handler returned a non-nil error; Output is the JSON-encoded
// payload on success.
type ToolResult struct {
	ID     string          `json:"id"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// PlanReady is the preview of a plan that is about to be presented to
// the user. The frontend renders the diffs in a modal; the next event
// will be PlanAwaitingConfirm which actually blocks the loop.
type PlanReady struct {
	PlanID  string         `json:"plan_id"`
	Summary string         `json:"summary"`
	Diffs   []k8s.Diff     `json:"diffs"`
	Denied  []k8s.DeniedOp `json:"denied,omitempty"`
}

// PlanAwaitingConfirm signals that the agent loop is now blocked
// waiting for the user to confirm or cancel the plan. The HTTP layer
// delivers the plan_id back to the frontend.
type PlanAwaitingConfirm struct {
	PlanID string `json:"plan_id"`
}

// AskUserPayload carries the question + optional enumerated options
// for a model-issued ask_user. The frontend renders an input form.
type AskUserPayload struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select"`
}

// ClusterSwitch notifies the frontend that the active cluster has
// changed. (The HTTP layer normally handles this at request time, so
// the agent only emits it on an explicit /cluster switch tool call.)
type ClusterSwitch struct {
	ClusterID string `json:"cluster_id"`
}

// Cancelled is emitted when the user cancels an in-flight plan, ask,
// or turn. The runner treats this as a graceful exit.
type Cancelled struct{}

// ErrorPayload is emitted on a stream / tool error. Retryable tells
// the frontend whether a "retry" button should be shown (transient
// errors) vs a hard "report" button (auth / 4xx).
type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// MessageEnd marks the end of one assistant turn. The HTTP layer
// closes the SSE stream after emitting it. The token counts are the
// model's reported usage for the turn.
type MessageEnd struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// Event type strings. Exported so tests and other packages can match
// on them without re-typing the literal.
const (
	EventSessionMeta           = "session_meta"
	EventReasoning             = "reasoning"
	EventToken                 = "token"
	EventToolCall              = "tool_call"
	EventToolResult            = "tool_result"
	EventPlanReady             = "plan_ready"
	EventPlanAwaitingConfirm   = "plan_awaiting_confirm"
	EventAskUser               = "ask_user"
	EventClusterSwitch         = "cluster_switch"
	EventCancelled             = "cancelled"
	EventError                 = "error"
	EventMessageEnd            = "message_end"
)
