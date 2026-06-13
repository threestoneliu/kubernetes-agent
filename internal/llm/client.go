package llm

import (
	"context"
	"encoding/json"
	"errors"
)

// Role is the role of a chat message. Mirrors the values used by every
// major provider ("system" / "user" / "assistant" / "tool").
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ContentPart is one block of a message's content. A message may have
// multiple parts (e.g. text + tool calls + tool result).
type ContentPart struct {
	// Type is "text" | "tool_call" | "tool_result" | "reasoning".
	Type string `json:"type"`
	// Text is the textual content (for Type == "text" or "reasoning").
	Text string `json:"text,omitempty"`
	// ToolCallID references the model-issued ToolCall (for
	// Type == "tool_call" or "tool_result").
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName is the name of the tool (Type == "tool_call" or
	// "tool_result"). The agent loop uses it to route results back
	// into the LLM's conversation context.
	ToolName string `json:"tool_name,omitempty"`
	// Input is the raw JSON of the tool's arguments (Type == "tool_call").
	Input json.RawMessage `json:"input,omitempty"`
	// Output is the tool's result content (Type == "tool_result"). For
	// errors this is set to a human-readable string; the IsError flag
	// tells the LLM the call failed.
	Output string `json:"output,omitempty"`
	// IsError marks a tool_result part as a failure.
	IsError bool `json:"is_error,omitempty"`
}

// Message is one entry in the chat transcript. The agent loop appends
// user messages, then for each LLM turn: an assistant message (with
// its tool calls) and zero or more tool result messages.
type Message struct {
	Role       Role          `json:"role"`
	Content    []ContentPart `json:"content,omitempty"`
	// ToolCallID is the legacy single-tool-call form used by older
	// OpenAI-style APIs. For multi-tool responses prefer the
	// Content[].ToolCallID form. Set automatically by the agent loop
	// when converting a single tool result.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Tool is the registration record for a function the LLM may call. The
// agent loop holds a slice of Tools and passes them into Client.Chat.
type Tool struct {
	// Name is the function name the LLM will emit (e.g. "k8s_get").
	Name string
	// Description helps the model decide when to call the tool.
	Description string
	// InputSchema is a JSON Schema object describing the tool's
	// parameters. The adapter passes it through to the provider.
	InputSchema map[string]any
	// Handler is invoked by the agent loop when the LLM calls the
	// tool. The returned (output, err) becomes a ToolResult event;
	// err != nil marks the call as failed (IsError=true).
	Handler func(ctx context.Context, call ToolCall) (output []byte, err error)
}

// Client is the unified LLM client interface used by the agent loop.
// The actual concrete type is provided by charm.land/fantasy in
// adapters internal/llm/{anthropic,openai,openai_compat}.go.
type Client interface {
	// Chat starts a streaming completion. The returned Stream must
	// be Closed by the caller (typically via defer). The Messages
	// slice must contain at least one message; the first system
	// message is the agent's system prompt.
	//
	// Tools are the function definitions the model may invoke. The
	// agent loop is responsible for routing tool calls to handlers
	// and feeding the results back into the next turn.
	Chat(ctx context.Context, messages []Message, tools []Tool) (Stream, error)
}

// errNotImplemented is returned by adapter stubs whose provider has
// not yet been wired. Callers should surface it as a stream error
// event so the agent loop emits an ErrorPayload.
var errNotImplemented = errors.New("llm client: not implemented for this provider type")
