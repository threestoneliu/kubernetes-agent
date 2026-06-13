package llm

import "errors"

// Client is the unified LLM client interface used by the agent loop.
// The actual concrete type is provided by github.com/charmbracelet/fantasy
// in Task 8. For now, this interface is empty so adapter stubs compile.
type Client interface {
	// Task 8 will define the methods, e.g.:
	// StreamChat(ctx context.Context, req StreamRequest) (Stream, error)
}

var errNotImplemented = errors.New("llm client: not implemented (wired in Task 8)")
