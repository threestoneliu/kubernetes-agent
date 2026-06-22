package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/threestoneliu/kubernetes-agent/internal/skills"
)

// SkillTool handles loading a SKILL.md file by its name. The LLM
// calls this when a user task matches a skill description from the
// system prompt. Unlike fs_read (which requires the LLM to construct
// a file path), load_skill is name-based so the LLM only needs to
// know the skill's name tag.
type SkillTool struct {
	skills []*skills.SkillEntry
}

// NewSkillTool creates a skill tool backed by the given skill entries.
func NewSkillTool(entries []*skills.SkillEntry) *SkillTool {
	return &SkillTool{skills: entries}
}

// Name returns the tool name.
func (t *SkillTool) Name() string {
	return "load_skill"
}

// Description returns the tool description.
func (t *SkillTool) Description() string {
	return "Load a SKILL.md file by its name. REQUIRED: the name field must always be provided — never call this tool without a skill name. Use when the user's task matches a skill description from the system prompt. Pass the skill name exactly as it appears in the <name> tag (e.g. \"k8s-debug-pod\"). Returns the skill's workflow instructions."
}

// Handle looks up the skill by name and returns its full content.
func (t *SkillTool) Handle(ctx context.Context, call interface{}) ([]byte, error) {
	var input loadSkillInput

	switch v := call.(type) {
	case []byte:
		if err := json.Unmarshal(v, &input); err != nil {
			return json.Marshal(LoadSkillOutput{Error: "invalid input: " + err.Error()})
		}
	case string:
		if err := json.Unmarshal([]byte(v), &input); err != nil {
			return json.Marshal(LoadSkillOutput{Error: "invalid input: " + err.Error()})
		}
	default:
		inputBytes, err := json.Marshal(call)
		if err != nil {
			return json.Marshal(LoadSkillOutput{Error: "invalid input"})
		}
		if err := json.Unmarshal(inputBytes, &input); err != nil {
			return json.Marshal(LoadSkillOutput{Error: "invalid input: " + err.Error()})
		}
	}

	if input.Name == "" {
		return json.Marshal(LoadSkillOutput{Error: "name is required"})
	}

	slog.Debug("load_skill: called", "name", input.Name)
	for _, entry := range t.skills {
		if entry.Skill.Name == input.Name {
			content := entry.Content()
			if content == "" {
				// Fallback: re-read from disk if not cached
				data, err := os.ReadFile(entry.Skill.FilePath)
				if err != nil {
					return json.Marshal(LoadSkillOutput{Error: "failed to read SKILL.md: " + err.Error()})
				}
				content = string(data)
			}
			return json.Marshal(LoadSkillOutput{
				Name:        entry.Skill.Name,
				Description: entry.Skill.Description,
				Content:     content,
			})
		}
	}
	return json.Marshal(LoadSkillOutput{Error: fmt.Sprintf("skill %q not found", input.Name)})
}

type loadSkillInput struct {
	Name string `json:"name"`
}

type LoadSkillOutput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	Error       string `json:"error,omitempty"`
}

var LoadSkillSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "REQUIRED. Skill name exactly as it appears in the <name> tag of <available_skills> (e.g. \"k8s-debug-pod\"). Never omit or empty this field.",
		},
	},
	"required": []string{"name"},
}

// skillHandler creates the handler function for load_skill tool registration.
func skillHandler(tool *SkillTool) func(ctx context.Context, call interface{}) ([]byte, error) {
	return func(ctx context.Context, call interface{}) ([]byte, error) {
		return tool.Handle(ctx, call)
	}
}
