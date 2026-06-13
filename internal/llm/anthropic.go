package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"charm.land/fantasy"
	anthropicprovider "charm.land/fantasy/providers/anthropic"
)

// anthropicClient adapts the charm.land/fantasy Anthropic provider to our
// llm.Client interface. It is safe to use Chat concurrently from
// different goroutines (fantasy builds an http.Client per provider).
type anthropicClient struct {
	provider fantasy.Provider
	modelID  string
}

// NewAnthropicClient constructs a Client backed by the Anthropic Messages
// API via charm.land/fantasy. The model is selected by Provider.Model.
func NewAnthropicClient(p Provider) (Client, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("anthropic: apiKey is required")
	}
	if p.Model == "" {
		return nil, fmt.Errorf("anthropic: model is required")
	}
	opts := []anthropicprovider.Option{anthropicprovider.WithAPIKey(p.APIKey)}
	if p.BaseURL != "" {
		opts = append(opts, anthropicprovider.WithBaseURL(p.BaseURL))
	}
	if p.Name != "" {
		opts = append(opts, anthropicprovider.WithName(p.Name))
	}
	prov, err := anthropicprovider.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("anthropic provider: %w", err)
	}
	return &anthropicClient{provider: prov, modelID: p.Model}, nil
}

// Chat starts a streaming completion. The first system message in
// messages is the system prompt. Tools are passed through as function
// definitions; their Handlers are NOT registered with fantasy (we
// invoke them ourselves so the agent loop can block on plan / ask).
func (c *anthropicClient) Chat(ctx context.Context, messages []Message, tools []Tool) (Stream, error) {
	prompt, _, err := toFantasyPrompt(messages)
	if err != nil {
		return nil, err
	}
	model, err := c.provider.LanguageModel(ctx, c.modelID)
	if err != nil {
		return nil, fmt.Errorf("anthropic: resolve model %q: %w", c.modelID, err)
	}
	fs := &fantasyStream{model: model, tools: tools, prompt: prompt}
	fs.start(ctx)
	return fs, nil
}

// toFantasyPrompt converts our []Message into a fantasy.Prompt (a slice
// of fantasy.Message). System messages are dropped here — fantasy's
// Call API takes the system prompt at the call level. The first
// system message is returned as the call's system prompt; the rest are
// concatenated. Empty content parts are skipped.
func toFantasyPrompt(msgs []Message) (fantasy.Prompt, string, error) {
	var (
		prompt fantasy.Prompt
		system string
	)
	for i, m := range msgs {
		if m.Role == RoleSystem {
			for _, p := range m.Content {
				if p.Type == "text" {
					if system != "" {
						system += "\n"
					}
					system += p.Text
				}
			}
			continue
		}
		parts := make([]fantasy.MessagePart, 0, len(m.Content))
		for _, p := range m.Content {
			switch p.Type {
			case "text":
				if p.Text != "" {
					parts = append(parts, fantasy.TextPart{Text: p.Text})
				}
			case "tool_call":
				input := string(p.Input)
				if input == "" {
					input = "{}"
				}
				parts = append(parts, fantasy.ToolCallPart{
					ToolCallID: p.ToolCallID,
					ToolName:   p.ToolName,
					Input:      input,
				})
			case "tool_result":
				text := p.Output
				if p.IsError {
					text = "ERROR: " + p.Output
				}
				parts = append(parts, fantasy.ToolResultPart{
					ToolCallID: p.ToolCallID,
					Output:     fantasy.ToolResultOutputContentText{Text: text},
				})
			case "reasoning":
				if p.Text != "" {
					parts = append(parts, fantasy.ReasoningPart{Text: p.Text})
				}
			default:
				return nil, "", fmt.Errorf("message %d: unknown content part type %q", i, p.Type)
			}
		}
		prompt = append(prompt, fantasy.Message{
			Role:    fantasy.MessageRole(m.Role),
			Content: parts,
		})
	}
	return prompt, system, nil
}

// fantasyStream adapts a fantasy iter.Seq[StreamPart] into our Stream
// interface. Parts are buffered in a channel that the consumer drains
// via Next. Close stops the underlying fantasy stream by cancelling
// its context.
type fantasyStream struct {
	model  fantasy.LanguageModel
	tools  []Tool
	prompt fantasy.Prompt

	once   sync.Once
	ch     chan Event
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func (s *fantasyStream) start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.ch = make(chan Event, 32)
	s.done = make(chan struct{})

	call := fantasy.Call{
		Prompt: s.prompt,
		Tools:  toFantasyTools(s.tools),
	}
	stream, err := s.model.Stream(ctx, call)
	if err != nil {
		s.err = err
		close(s.done)
		close(s.ch)
		return
	}
	go func() {
		defer close(s.ch)
		defer close(s.done)
		for part := range stream {
			ev, ok := convertStreamPart(part)
			if !ok {
				continue
			}
			select {
			case s.ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *fantasyStream) Next(ctx context.Context) (Event, error) {
	s.once.Do(func() {})
	select {
	case ev, ok := <-s.ch:
		if !ok {
			if s.err != nil {
				return Event{}, s.err
			}
			return Event{}, io.EOF
		}
		return ev, nil
	case <-ctx.Done():
		return Event{}, ctx.Err()
	}
}

func (s *fantasyStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
	return nil
}

// toFantasyTools converts our []Tool into fantasy's []Tool (function
// tool definitions). Handlers are NOT converted; fantasy's high-level
// agent runs handlers, but we drive the loop ourselves so the agent
// loop can block on plan confirmation / ask_user.
func toFantasyTools(tools []Tool) []fantasy.Tool {
	out := make([]fantasy.Tool, 0, len(tools))
	for _, t := range tools {
		schema := map[string]any{
			"type":       "object",
			"properties": t.InputSchema,
		}
		if required, ok := t.InputSchema["required"]; ok {
			schema["required"] = required
		}
		out = append(out, fantasy.FunctionTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return out
}

// convertStreamPart maps a fantasy StreamPart to our Event. Returns
// (zero, false) if the part doesn't carry a payload we surface.
func convertStreamPart(p fantasy.StreamPart) (Event, bool) {
	switch p.Type {
	case fantasy.StreamPartTypeTextDelta:
		return Event{Type: EventToken, Text: p.Delta}, true
	case fantasy.StreamPartTypeReasoningDelta:
		return Event{Type: EventReasoning, Text: p.Delta}, true
	case fantasy.StreamPartTypeToolCall:
		return Event{Type: EventToolCall, Call: ToolCall{
			ID:    p.ID,
			Name:  p.ToolCallName,
			Input: json.RawMessage(p.ToolCallInput),
		}}, true
	case fantasy.StreamPartTypeFinish:
		return Event{Type: EventMessageEnd, In: p.Usage.InputTokens, Out: p.Usage.OutputTokens}, true
	case fantasy.StreamPartTypeError:
		reason := ""
		if p.Error != nil {
			reason = p.Error.Error()
		}
		return Event{Type: EventError, Reason: reason}, true
	default:
		return Event{}, false
	}
}
