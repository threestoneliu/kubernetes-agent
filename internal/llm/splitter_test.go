package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collect(s *thinkSplitter, deltas ...string) []Event {
	var out []Event
	for _, d := range deltas {
		out = append(out, s.feed(d)...)
	}
	if last, ok := s.flush(); ok {
		out = append(out, last)
	}
	return out
}

func TestThinkSplitter_PlainText(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "hello world")
	require.Len(t, events, 1)
	assert.Equal(t, EventToken, events[0].Type)
	assert.Equal(t, "hello world", events[0].Text)
}

func TestThinkSplitter_BasicThinkTag(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "<think>reasoning here</think>final answer")
	require.Len(t, events, 2)
	assert.Equal(t, EventReasoning, events[0].Type)
	assert.Equal(t, "reasoning here", events[0].Text)
	assert.Equal(t, EventToken, events[1].Type)
	assert.Equal(t, "final answer", events[1].Text)
}

func TestThinkSplitter_PrefixBeforeThinkTag(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "Hello. <think>let me think</think>answer")
	require.Len(t, events, 3)
	assert.Equal(t, EventToken, events[0].Type)
	assert.Equal(t, "Hello. ", events[0].Text)
	assert.Equal(t, EventReasoning, events[1].Type)
	assert.Equal(t, "let me think", events[1].Text)
	assert.Equal(t, EventToken, events[2].Type)
	assert.Equal(t, "answer", events[2].Text)
}

func TestThinkSplitter_TagSplitAcrossDeltas(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "<think>reas", "oning</think>final")
	require.Len(t, events, 2)
	assert.Equal(t, EventReasoning, events[0].Type)
	assert.Equal(t, "reasoning", events[0].Text)
	assert.Equal(t, EventToken, events[1].Type)
	assert.Equal(t, "final", events[1].Text)
}

func TestThinkSplitter_AntThinkingVariant(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "<antml:thinking>deep thoughts</antml:thinking>visible")
	require.Len(t, events, 2)
	assert.Equal(t, EventReasoning, events[0].Type)
	assert.Equal(t, "deep thoughts", events[0].Text)
	assert.Equal(t, EventToken, events[1].Type)
	assert.Equal(t, "visible", events[1].Text)
}

func TestThinkSplitter_ThoughtVariant(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "<thought>hidden</thought>shown")
	require.Len(t, events, 2)
	assert.Equal(t, EventReasoning, events[0].Type)
	assert.Equal(t, "hidden", events[0].Text)
	assert.Equal(t, EventToken, events[1].Type)
	assert.Equal(t, "shown", events[1].Text)
}

func TestThinkSplitter_MultipleThinkBlocks(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "<think>a</think>x<think>b</think>y")
	require.Len(t, events, 4)
	assert.Equal(t, EventReasoning, events[0].Type)
	assert.Equal(t, "a", events[0].Text)
	assert.Equal(t, EventToken, events[1].Type)
	assert.Equal(t, "x", events[1].Text)
	assert.Equal(t, EventReasoning, events[2].Type)
	assert.Equal(t, "b", events[2].Text)
	assert.Equal(t, EventToken, events[3].Type)
	assert.Equal(t, "y", events[3].Text)
}

func TestThinkSplitter_NoCloseTag(t *testing.T) {
	s := &thinkSplitter{}
	events := collect(s, "<think>orphan content with no closer")
	// No close tag — flush emits the buffered content as reasoning.
	// The literal <think> tag itself is stripped; only the
	// in-between text becomes the reasoning payload.
	require.Len(t, events, 1)
	assert.Equal(t, EventReasoning, events[0].Type)
	assert.Equal(t, "orphan content with no closer", events[0].Text)
}