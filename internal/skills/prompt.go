package skills

import (
	"fmt"
	"strings"
)

// PromptBuilder builds system prompts with skills.
type PromptBuilder struct {
	skills []*SkillEntry
}

// NewPromptBuilder creates a new prompt builder.
func NewPromptBuilder(skills []*SkillEntry) *PromptBuilder {
	return &PromptBuilder{skills: skills}
}

// FormatSkillsForPrompt generates the <available_skills> XML section.
func (pb *PromptBuilder) FormatSkillsForPrompt() string {
	if len(pb.skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n<available_skills>\n")

	for _, entry := range pb.skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", xmlEscape(entry.Skill.Name)))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", xmlEscape(entry.Skill.Description)))
		sb.WriteString(fmt.Sprintf("    <location>%s</location>\n", xmlEscape(entry.Skill.FilePath)))
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</available_skills>\n")
	return sb.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
