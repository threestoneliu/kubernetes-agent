package llm

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	openaicompatprovider "charm.land/fantasy/providers/openaicompat"
)

// openaiCompatClient adapts the charm.land/fantasy openai-compat
// provider (Ollama, vLLM, local llama.cpp server, etc.) to our
// llm.Client interface.
type openaiCompatClient struct {
	provider fantasy.Provider
	modelID  string
}

// NewOpenAICompatClient constructs a Client for any OpenAI-protocol-
// compatible backend. BaseURL is required (no default in fantasy).
func NewOpenAICompatClient(p Provider) (Client, error) {
	if p.BaseURL == "" {
		return nil, fmt.Errorf("openai-compat: baseURL is required")
	}
	if p.Model == "" {
		return nil, fmt.Errorf("openai-compat: model is required")
	}
	opts := []openaicompatprovider.Option{
		openaicompatprovider.WithBaseURL(p.BaseURL),
	}
	if p.APIKey != "" {
		opts = append(opts, openaicompatprovider.WithAPIKey(p.APIKey))
	}
	if p.Name != "" {
		opts = append(opts, openaicompatprovider.WithName(p.Name))
	}
	prov, err := openaicompatprovider.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("openai-compat provider: %w", err)
	}
	return &openaiCompatClient{provider: prov, modelID: p.Model}, nil
}

// Chat starts a streaming completion. Behaviour mirrors anthropicClient.
func (c *openaiCompatClient) Chat(ctx context.Context, messages []Message, tools []Tool) (Stream, error) {
	prompt, _, err := toFantasyPrompt(messages)
	if err != nil {
		return nil, err
	}
	model, err := c.provider.LanguageModel(ctx, c.modelID)
	if err != nil {
		return nil, fmt.Errorf("openai-compat: resolve model %q: %w", c.modelID, err)
	}
	fs := &fantasyStream{model: model, tools: tools, prompt: prompt}
	fs.start(ctx)
	return fs, nil
}
