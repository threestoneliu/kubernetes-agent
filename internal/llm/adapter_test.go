package llm

import (
	"context"
	"errors"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnthropicClient_MissingAPIKey(t *testing.T) {
	_, err := NewAnthropicClient(Provider{Model: "claude-sonnet-4-5"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiKey")
}

func TestNewAnthropicClient_MissingModel(t *testing.T) {
	_, err := NewAnthropicClient(Provider{APIKey: "k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestNewOpenAIClient_MissingAPIKey(t *testing.T) {
	_, err := NewOpenAIClient(Provider{Model: "gpt-4"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiKey")
}

func TestNewOpenAIClient_MissingModel(t *testing.T) {
	_, err := NewOpenAIClient(Provider{APIKey: "k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestNewOpenAICompatClient_MissingBaseURL(t *testing.T) {
	_, err := NewOpenAICompatClient(Provider{Model: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseURL")
}

func TestNewOpenAICompatClient_MissingModel(t *testing.T) {
	_, err := NewOpenAICompatClient(Provider{BaseURL: "http://x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestToFantasyPrompt_SkipsSystemAndEmpty(t *testing.T) {
	msgs := []Message{
		{Role: RoleSystem, Content: []ContentPart{{Type: "text", Text: "sys"}}},
		{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "hi"}}},
		{Role: RoleAssistant, Content: nil},
	}
	prompt, system, err := toFantasyPrompt(msgs)
	require.NoError(t, err)
	assert.Equal(t, "sys", system)
	// System is dropped (returned separately); user and assistant stay.
	require.Len(t, prompt, 2)
}

func TestToFantasyPrompt_ConcatsSystem(t *testing.T) {
	msgs := []Message{
		{Role: RoleSystem, Content: []ContentPart{{Type: "text", Text: "first"}}},
		{Role: RoleSystem, Content: []ContentPart{{Type: "text", Text: "second"}}},
	}
	_, system, err := toFantasyPrompt(msgs)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond", system)
}

func TestToFantasyTools_EmptyAndPopulated(t *testing.T) {
	empty := toFantasyTools(nil)
	assert.Empty(t, empty)

	pop := toFantasyTools([]Tool{{
		Name:        "k8s_list",
		Description: "list pods",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"resource": map[string]any{"type": "string"}},
			"required":   []string{"resource"},
		},
	}})
	require.Len(t, pop, 1)
}

func TestToFantasyTools_NoRequired(t *testing.T) {
	pop := toFantasyTools([]Tool{{
		Name:        "t",
		Description: "d",
		InputSchema: map[string]any{"type": "object"},
	}})
	require.Len(t, pop, 1)
}

func TestConvertStreamPart_AllTypes(t *testing.T) {
	cases := []struct {
		in       fantasy.StreamPart
		wantType EventType
		wantText string
	}{
		{fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "hi"}, EventToken, "hi"},
		{fantasy.StreamPart{Type: fantasy.StreamPartTypeReasoningDelta, Delta: "thinking"}, EventReasoning, "thinking"},
		{fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeToolCall,
			ID:           "tc1",
			ToolCallName: "k8s_list",
			ToolCallInput: `{"resource":"pods"}`,
		}, EventToolCall, ""},
		{fantasy.StreamPart{
			Type:  fantasy.StreamPartTypeFinish,
			Usage: fantasy.Usage{InputTokens: 11, OutputTokens: 22},
		}, EventMessageEnd, ""},
		{fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: errors.New("boom")}, EventError, ""},
		{fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd}, "", ""}, // not surfaced
	}
	for i, tc := range cases {
		ev, ok := convertStreamPart(tc.in)
		if tc.wantType == "" {
			assert.Falsef(t, ok, "case %d should not be converted", i)
			continue
		}
		require.Truef(t, ok, "case %d should convert", i)
		assert.Equalf(t, tc.wantType, ev.Type, "case %d type", i)
		if tc.wantText != "" {
			assert.Equalf(t, tc.wantText, ev.Text, "case %d text", i)
		}
	}
}

func TestConvertStreamPart_ErrorWithNilErr(t *testing.T) {
	ev, ok := convertStreamPart(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: nil})
	require.True(t, ok)
	assert.Equal(t, EventError, ev.Type)
	assert.Empty(t, ev.Reason)
}

func TestIsEOF(t *testing.T) {
	assert.True(t, isEOF(io.EOF))
	assert.False(t, isEOF(errors.New("other")))
}

func TestAnthropicChat_NilMessages(t *testing.T) {
	// Calling Chat on a real anthropicClient requires a provider
	// connection; we exercise the input-validation path via
	// toFantasyPrompt which Chat also calls. Here we just ensure
	// the helper doesn't panic on empty input.
	prompt, sys, err := toFantasyPrompt(nil)
	require.NoError(t, err)
	assert.Empty(t, prompt)
	assert.Empty(t, sys)
}

// Stub stream that emits a single error then closes — exercises
// fantasyStream.Next and Close without a real provider.
type stubStream struct {
	ch   chan Event
	done chan struct{}
	err  error
}

func newStubStream(ev Event, err error) *stubStream {
	ch := make(chan Event, 1)
	if ev.Type != "" {
		ch <- ev
	}
	done := make(chan struct{})
	close(done)
	return &stubStream{ch: ch, done: done, err: err}
}

func (s *stubStream) Next(ctx context.Context) (Event, error) {
	select {
	case ev, ok := <-s.ch:
		if !ok {
			if s.err != nil {
				return Event{}, s.err
			}
			return Event{}, errEOFish
		}
		return ev, nil
	case <-ctx.Done():
		return Event{}, ctx.Err()
	}
}

func (s *stubStream) Close() error {
	return nil
}

var errEOFish = errors.New("eof")

// fakeProvider and fakeModel implement the fantasy.Provider /
// fantasy.LanguageModel interfaces so we can exercise anthropicClient.Chat
// (and the fantasyStream adapter) without touching the network.

type fakeProvider struct{ model *fakeModel }

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) LanguageModel(ctx context.Context, id string) (fantasy.LanguageModel, error) {
	return p.model, nil
}

type fakeModel struct {
	parts   []fantasy.StreamPart
	streamErr error
}

func (m *fakeModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}
func (m *fakeModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		for _, p := range m.parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}
func (m *fakeModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}
func (m *fakeModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}
func (m *fakeModel) Provider() string { return "fake" }
func (m *fakeModel) Model() string    { return "m" }

func TestAnthropicClient_Chat_StreamError(t *testing.T) {
	m := &fakeModel{streamErr: errors.New("net down")}
	c := &anthropicClient{provider: &fakeProvider{model: m}, modelID: "m"}
	s, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "hi"}}}}, nil)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	_, err = s.Next(context.Background())
	require.Error(t, err)
}

func TestAnthropicClient_Chat_StreamParts(t *testing.T) {
	m := &fakeModel{parts: []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeTextDelta, Delta: "hi"},
		{Type: fantasy.StreamPartTypeFinish, Usage: fantasy.Usage{InputTokens: 1, OutputTokens: 2}},
	}}
	c := &anthropicClient{provider: &fakeProvider{model: m}, modelID: "m"}
	s, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "q"}}}}, nil)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	ev, err := s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, EventToken, ev.Type)
	assert.Equal(t, "hi", ev.Text)
	ev, err = s.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, EventMessageEnd, ev.Type)
	_, err = s.Next(context.Background())
	assert.Equal(t, io.EOF, err)
}

func TestOpenAIClient_Chat_StreamError(t *testing.T) {
	m := &fakeModel{streamErr: errors.New("net down")}
	c := &openaiClient{provider: &fakeProvider{model: m}, modelID: "m"}
	s, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "hi"}}}}, nil)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	_, err = s.Next(context.Background())
	require.Error(t, err)
}

func TestOpenAICompatClient_Chat_StreamError(t *testing.T) {
	m := &fakeModel{streamErr: errors.New("net down")}
	c := &openaiCompatClient{provider: &fakeProvider{model: m}, modelID: "m"}
	s, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "hi"}}}}, nil)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	_, err = s.Next(context.Background())
	require.Error(t, err)
}

func TestToFantasyPrompt_UnknownPart(t *testing.T) {
	_, _, err := toFantasyPrompt([]Message{{Role: RoleUser, Content: []ContentPart{{Type: "bogus"}}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestToFantasyPrompt_ToolCallEmptyInput(t *testing.T) {
	msgs := []Message{{Role: RoleAssistant, Content: []ContentPart{{
		Type: "tool_call", ToolCallID: "tc", ToolName: "t",
	}}}}
	prompt, _, err := toFantasyPrompt(msgs)
	require.NoError(t, err)
	require.Len(t, prompt, 1)
}

func TestToFantasyPrompt_ToolResultError(t *testing.T) {
	msgs := []Message{{Role: RoleTool, Content: []ContentPart{{
		Type: "tool_result", ToolCallID: "tc", Output: "boom", IsError: true,
	}}}}
	prompt, _, err := toFantasyPrompt(msgs)
	require.NoError(t, err)
	require.Len(t, prompt, 1)
}

func TestPingAll_AllProviders(t *testing.T) {
	// PingAll runs concurrent pings; use an httptest server for one
	// and an unreachable URL for the other so both paths are
	// exercised.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	res := PingAll(context.Background(), []Provider{
		{Name: "good", BaseURL: srv.URL, Type: "openai-compatible"},
		{Name: "bad", BaseURL: "http://127.0.0.1:1", Type: "openai-compatible"},
	}, 2)
	require.Contains(t, res, "good")
	require.Contains(t, res, "bad")
	assert.True(t, res["good"].OK)
	assert.False(t, res["bad"].OK)
}