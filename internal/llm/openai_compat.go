package llm

// NewOpenAICompatClient is a placeholder for any OpenAI-protocol-compatible
// backend (Ollama, vLLM, local llama.cpp server, etc.). Wired in Task 8.
func NewOpenAICompatClient(p Provider) (Client, error) {
	return nil, errNotImplemented
}
