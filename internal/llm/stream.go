package llm

import (
	"context"
	"encoding/json"
	"io"
)

// EventType is the discriminator for the type of streaming event emitted
// from an LLM. The agent loop in internal/agent maps these to SSE
// payloads that the frontend renders.
type EventType string

const (
	// EventReasoning is an incremental reasoning chunk from the model
	// (Anthropic extended thinking, OpenAI o-series, etc.). The agent
	// loop may forward it as a `reasoning` SSE event.
	EventReasoning EventType = "reasoning"

	// EventToken is an incremental text chunk from the model — the
	// final assistant answer streaming in.
	EventToken EventType = "token"

	// EventToolCall signals the model wants to invoke a registered
	// tool. The agent loop must call the handler, emit a ToolResult
	// event, and feed the result back into the next LLM turn.
	EventToolCall EventType = "tool_call"

	// EventPlanReady is emitted when a tool handler (typically
	// k8s_plan_write) has produced a plan that is awaiting user
	// confirmation. The agent loop must surface the plan and block
	// until ConfirmPlan / CancelPlan is called.
	EventPlanReady EventType = "plan_ready"

	// EventPlanAwaitingConfirm is the blocking signal: the agent loop
	// has emitted the plan to the client and is now waiting for the
	// user to confirm or cancel.
	EventPlanAwaitingConfirm EventType = "plan_awaiting_confirm"

	// EventAskUser signals the model wants to ask the user a
	// question. The agent loop emits AskUserPayload to the client and
	// blocks until AnswerAsk is called.
	EventAskUser EventType = "ask_user"

	// EventClusterSwitch is emitted when the session's active cluster
	// changes (e.g. the user said "switch to staging").
	EventClusterSwitch EventType = "cluster_switch"

	// EventCancelled signals the user cancelled the in-flight plan /
	// ask / turn. The agent loop must unwind promptly.
	EventCancelled EventType = "cancelled"

	// EventMessageEnd marks the end of one assistant turn. The agent
	// loop should batch-insert the accumulated messages and return
	// from Run.
	EventMessageEnd EventType = "message_end"

	// EventError is a stream-level error. The agent loop should
	// classify (auth/429/5xx) and emit an ErrorPayload SSE event.
	// The Stream itself terminates after emitting EventError.
	EventError EventType = "error"
)

// ToolCall is the model-issued request to invoke a registered tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// PlanSummary describes a plan that's ready for user confirmation. It's
// emitted on the plan_ready event so the SSE layer can render the diff
// preview before the agent loop blocks on the user.
type PlanSummary struct {
	PlanID  string          `json:"plan_id"`
	Summary string          `json:"summary"`
	Diffs   json.RawMessage `json:"diffs,omitempty"`
	Denied  json.RawMessage `json:"denied,omitempty"`
}

// AskUser is the question the model wants to ask the user.
type AskUser struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select"`
}

// Event is a single unit emitted by an LLM Stream. Exactly one of the
// payload fields is meaningful for each Type.
type Event struct {
	Type     EventType
	Text     string     // reasoning / token
	Reason   string     // error
	Call     ToolCall   // tool_call
	PlanID   string     // plan_awaiting_confirm
	Plan     PlanSummary // plan_ready
	Question string     // ask_user
	Options  []string   // ask_user
	Multi    bool       // ask_user
	Cluster  string     // cluster_switch
	In       int64      // message_end (input tokens)
	Out      int64      // message_end (output tokens)
}

// Stream is the iterator surface for an in-flight LLM completion. The
// agent loop calls Next until it returns io.EOF (or another error).
type Stream interface {
	// Next blocks until the next event is available, returns io.EOF
	// when the stream is complete, or a non-nil error on failure.
	// EventError is delivered as a regular Event with Type=EventError
	// and Reason set — the next call to Next returns io.EOF.
	Next(ctx context.Context) (Event, error)
	// Close releases the underlying resources. Safe to call multiple
	// times.
	Close() error
}

// ErrStreamClosed is returned by Next after Close has been called.
var ErrStreamClosed = io.ErrClosedPipe

// drainErrEOF is a small helper for adapters: a non-nil error that is
// just io.EOF means the stream ended cleanly.
func isEOF(err error) bool { return err == io.EOF }
