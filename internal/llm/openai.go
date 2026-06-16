package llm

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	openaiprovider "charm.land/fantasy/providers/openai"
)

// openaiClient adapts the charm.land/fantasy OpenAI provider to our
// llm.Client interface.
type openaiClient struct {
	provider fantasy.Provider
	modelID  string
}

// NewOpenAIClient constructs a Client backed by OpenAI's Chat
// Completions API (or Responses API if the provider is configured for
// it) via charm.land/fantasy.
func NewOpenAIClient(p Provider) (Client, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("openai: apiKey is required")
	}
	if p.Model == "" {
		return nil, fmt.Errorf("openai: model is required")
	}
	opts := []openaiprovider.Option{openaiprovider.WithAPIKey(p.APIKey)}
	if p.BaseURL != "" {
		opts = append(opts, openaiprovider.WithBaseURL(p.BaseURL))
	}
	if p.Name != "" {
		opts = append(opts, openaiprovider.WithName(p.Name))
	}
	prov, err := openaiprovider.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("openai provider: %w", err)
	}
	return &openaiClient{provider: prov, modelID: p.Model}, nil
}

// Chat starts a streaming completion. Behaviour mirrors anthropicClient.
func (c *openaiClient) Chat(ctx context.Context, messages []Message, tools []Tool) (Stream, error) {
	prompt, _, err := toFantasyPrompt(messages)
	if err != nil {
		return nil, err
	}
	model, err := c.provider.LanguageModel(ctx, c.modelID)
	if err != nil {
		return nil, fmt.Errorf("openai: resolve model %q: %w", c.modelID, err)
	}
	fs := &fantasyStream{model: model, tools: tools, prompt: prompt}
	fs.start(ctx)
	return fs, nil
}
