package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// renderSessionMarkdown turns a session and its messages into a
// Markdown document for browser download. Reasoning content is
// folded inside a <details> block so the document stays compact;
// tool calls are emitted as fenced JSON code blocks next to the
// assistant turn that produced them.
//
// The export is intentionally a one-way shape — we don't try to
// round-trip Markdown back into the store; it exists for backups,
// sharing, and reading outside the app.
func renderSessionMarkdown(s store.Session, msgs []store.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 会话: %s\n\n", s.Title)
	fmt.Fprintf(&b, "- session_id: %s\n", s.ID)
	if s.ClusterID != nil {
		fmt.Fprintf(&b, "- cluster_id: %s\n", *s.ClusterID)
	}
	fmt.Fprintf(&b, "- created_at: %s\n", s.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- updated_at: %s\n", s.UpdatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- message_count: %d\n\n", len(msgs))
	b.WriteString("---\n\n")

	for _, m := range msgs {
		role := m.Role
		if role == "" {
			role = "system"
		}
		fmt.Fprintf(&b, "## %s\n\n", role)
		if m.Reasoning != nil && *m.Reasoning != "" {
			fmt.Fprintf(&b, "<details><summary>思考过程</summary>\n\n%s\n\n</details>\n\n", *m.Reasoning)
		}
		if m.Content != nil && *m.Content != "" {
			b.WriteString(*m.Content)
			b.WriteString("\n\n")
		}
		if m.ToolCalls != nil && *m.ToolCalls != "" {
			b.WriteString(renderToolCalls(*m.ToolCalls))
		}
		if m.ToolCallID != nil && *m.ToolCallID != "" {
			fmt.Fprintf(&b, "> _对应工具调用 ID: `%s`_\n\n", *m.ToolCallID)
		}
	}

	return b.String()
}

// renderToolCalls parses the stored JSON tool-call array and
// renders each call as a fenced code block. We try to pretty-print;
// if parsing fails, the raw string is emitted unchanged.
func renderToolCalls(raw string) string {
	var calls []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &calls); err == nil && len(calls) > 0 {
		var out strings.Builder
		for _, c := range calls {
			// Best-effort: pretty print if it's an object.
			var obj map[string]any
			if err := json.Unmarshal(c, &obj); err == nil {
				name, _ := obj["name"].(string)
				if name == "" {
					name = "tool"
				}
				fmt.Fprintf(&out, "🔧 **%s**\n\n```json\n", name)
				pretty, _ := json.MarshalIndent(obj, "", "  ")
				out.Write(pretty)
				out.WriteString("\n```\n\n")
				continue
			}
			out.WriteString("```json\n")
			out.WriteString(string(c))
			out.WriteString("\n```\n\n")
		}
		return out.String()
	}
	return "```json\n" + raw + "\n```\n\n"
}