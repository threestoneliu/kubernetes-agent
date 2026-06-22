package llm

import (
	"context"
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

		// thinkSplitter routes <think>…</think> blocks to
		// text channel into EventReasoning so models that embed
		// thinking in plain text (most OpenAI-compatible reasoning
		// models today) get the same surface as Anthropic's native
		// reasoning channel.
		var splitTextDeltas thinkSplitter
		for part := range stream {
			ev, ok := convertStreamPart(part)
			if !ok {
				continue
			}
			events := []Event{ev}
			if ev.Type == EventToken {
				events = splitTextDeltas.feed(ev.Text)
			}
			for _, out := range events {
				select {
				case s.ch <- out:
				case <-ctx.Done():
					return
				}
			}
		}
		if last, ok := splitTextDeltas.flush(); ok {
			select {
			case s.ch <- last:
			case <-ctx.Done():
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
