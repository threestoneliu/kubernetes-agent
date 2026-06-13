package llm

type Provider struct {
	Name    string
	Type    string // anthropic | openai | openai-compatible
	APIKey  string
	BaseURL string
	Model   string
}

type PingStatus struct {
	Name   string
	OK     bool
	Reason string
}
