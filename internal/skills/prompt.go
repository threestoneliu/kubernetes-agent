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
	sb.WriteString("\n## Skills (mandatory)")
	sb.WriteString("\nBefore replying: scan <available_skills> <description> entries.")
	sb.WriteString("\n- If exactly one skill clearly applies: call `load_skill` with its name (the <name> tag) BEFORE taking any other action, then follow the returned instructions.")
	sb.WriteString("\n- If multiple could apply: choose the most specific one, then load and follow it.")
	sb.WriteString("\n- If none clearly apply: proceed without loading a skill.")
	sb.WriteString("\nConstraint: load only one skill at a time, only after you have selected it.")
	sb.WriteString("\nDo NOT construct file paths yourself — `load_skill` takes the skill name only.")

	sb.WriteString("\n\n<available_skills>")
	for _, entry := range pb.skills {
		sb.WriteString("\n  <skill>")
		sb.WriteString(fmt.Sprintf("\n    <name>%s</name>", xmlEscape(entry.Skill.Name)))
		sb.WriteString(fmt.Sprintf("\n    <description>%s</description>", xmlEscape(entry.Skill.Description)))
		sb.WriteString("\n  </skill>")
	}
	sb.WriteString("\n</available_skills>")

	return sb.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
