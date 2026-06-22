package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"charm.land/fantasy"
)

// toFantasyPrompt converts our []Message into a fantasy.Prompt (a slice
// of fantasy.Message). System messages ARE KEPT in the prompt with
// fantasy.MessageRoleSystem — fantasy's provider code (e.g. anthropic's
// toPrompt) extracts system-role messages and routes them into the
// provider's native "system" parameter. If we drop them here, they
// never reach the LLM and skills / persona instructions vanish.
func toFantasyPrompt(msgs []Message) (fantasy.Prompt, string, error) {
	var (
		prompt fantasy.Prompt
		system string
	)
	for i, m := range msgs {
		if m.Role == RoleSystem {
			// Keep the system message in the prompt as a
			// fantasy.MessageRoleSystem part. Fantasy's
			// toPrompt will group consecutive system messages
			// and forward them to the provider's system param.
			var sysText string
			for _, p := range m.Content {
				if p.Type == "text" {
					if sysText != "" {
						sysText += "\n"
					}
					sysText += p.Text
				}
			}
			if sysText != "" {
				if system != "" {
					system += "\n"
				}
				system += sysText
				prompt = append(prompt, fantasy.Message{
					Role: fantasy.MessageRoleSystem,
					Content: []fantasy.MessagePart{
						fantasy.TextPart{Text: sysText},
					},
				})
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

// toFantasyTools converts our []Tool into fantasy's []Tool (function
// tool definitions). Handlers are NOT converted; fantasy's high-level
// agent runs handlers, but we drive the loop ourselves so the agent
// loop can block on plan confirmation / ask_user.
func toFantasyTools(tools []Tool) []fantasy.Tool {
	out := make([]fantasy.Tool, 0, len(tools))
	for _, t := range tools {
		// Extract the actual `properties` and `required` from the
		// tool's JSON-schema-shaped InputSchema. Previously the
		// whole InputSchema map was assigned to the top-level
		// `properties` key, which produced a nested schema the LLM
		// could not parse — every tool call arrived with `{}`.
		props, _ := t.InputSchema["properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
		}
		schema := map[string]any{
			"type":       "object",
			"properties": props,
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

// thinkingTagRE matches reasoning-block open / close tags produced by
// various model families that embed thinking in the regular text
// stream instead of using a structured reasoning channel:
//
//   - <think>…</think>, <thinking>…</thinking>, <thought>…</thought>
//   - Anthropic XML namespace variants: antml:think, antml:thinking,
//     antml:thought
//   - The vendor-specific "antthinking" wrapper some OpenAI-compatible
//     gateways emit
//
// We match both open and close tags with a single regex; the splitter
// below uses the matched text to decide which kind to emit.
var thinkingTagRE = regexp.MustCompile(`(?i)</?(?:(?:antml:)?(?:think(?:ing)?|thought)|antthinking)>`)

// thinkSplitter scans text deltas and splits them into a stream of
// (kind, text) pairs so the agent loop can route reasoning content
// to EventReasoning and user-visible content to EventToken. Models
// that emit <think>…</think> inside the regular text channel — which
// is most OpenAI-compatible reasoning models today — need this
// treatment because the upstream protocol has no separate reasoning
// channel for them.
//
// The splitter is stateful across calls: a tag may be split between
// two adjacent deltas ("<th" + "ink>…</think>") and the buffer
// remembers the pending bytes until the regex can decide.
type thinkSplitter struct {
	inside      bool // currently between an open and close tag
	buf         strings.Builder
	reasoning   strings.Builder // accumulates text inside <think>…
}

// maxTagPrefixLen is the longest prefix that could still grow into
// one of the recognised tag names. The buffer only holds bytes
// from the last `<` onward — content with no `<` is plain user-
// visible text and gets flushed immediately.
const maxTagPrefixLen = len("<antml:thinking>")

// feed takes one streaming text delta and returns zero or more
// Events to emit downstream. Text outside reasoning tags is flushed
// eagerly so the user sees incremental tokens; reasoning content
// is buffered until the close tag (or stream end) so a multi-delta
// think block surfaces as a single EventReasoning.
func (s *thinkSplitter) feed(delta string) []Event {
	s.buf.WriteString(delta)
	raw := s.buf.String()
	s.buf.Reset()

	var out []Event
	emitToken := func(text string) {
		if text != "" {
			out = append(out, Event{Type: EventToken, Text: text})
		}
	}
	flushReasoning := func() {
		if s.reasoning.Len() == 0 {
			return
		}
		out = append(out, Event{Type: EventReasoning, Text: s.reasoning.String()})
		s.reasoning.Reset()
	}

	for {
		loc := thinkingTagRE.FindStringIndex(raw)
		if loc == nil {
			// No complete tag. If we are inside a think block,
			// buffer everything except the last maxTagPrefixLen
			// bytes (those could grow into a tag). Otherwise,
			// user-visible text after the last '<' is the only
			// part that could become a tag — flush the rest.
			cut := strings.LastIndex(raw, "<")
			if s.inside {
				if cut < 0 || len(raw)-cut > maxTagPrefixLen {
					s.reasoning.WriteString(raw)
					return out
				}
				s.reasoning.WriteString(raw[:cut])
				s.buf.WriteString(raw[cut:])
				return out
			}
			if cut < 0 || len(raw)-cut > maxTagPrefixLen {
				emitToken(raw)
				return out
			}
			emitToken(raw[:cut])
			s.buf.WriteString(raw[cut:])
			return out
		}
		tag := raw[loc[0]:loc[1]]
		prefix := raw[:loc[0]]
		raw = raw[loc[1]:]
		if isOpenTag(tag) {
			// The text before the open tag is user-visible.
			emitToken(prefix)
			s.inside = true
		} else {
			// Close tag: prefix belongs to the think block.
			s.reasoning.WriteString(prefix)
			flushReasoning()
			s.inside = false
		}
	}
}

// flush emits any remaining buffered bytes when the stream ends.
// Anything held inside an unclosed think block is emitted as
// reasoning; anything outside is emitted as text.
func (s *thinkSplitter) flush() (Event, bool) {
	tail := s.buf.String()
	s.buf.Reset()
	combined := s.reasoning.String() + tail
	s.reasoning.Reset()
	if combined == "" {
		return Event{}, false
	}
	if s.inside {
		return Event{Type: EventReasoning, Text: combined}, true
	}
	return Event{Type: EventToken, Text: combined}, true
}

func isOpenTag(tag string) bool {
	// Close tags start with "</"; everything else is an open tag
	// (including self-closing forms like <think/>, which we treat
	// as a no-op since there is no enclosed content to flush).
	return !strings.HasPrefix(tag, "</")
}
