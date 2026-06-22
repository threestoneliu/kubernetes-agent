package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_ParseFrontmatter(t *testing.T) {
	content := `---
name: test-skill
description: Test description.
---
# Content`

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm["name"] != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", fm["name"])
	}

	if fm["description"] != "Test description." {
		t.Errorf("expected description 'Test description.', got '%s'", fm["description"])
	}

	if body == "" {
		t.Error("expected body to be non-empty")
	}
}

func TestLoader_ParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just Content`

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}

	if body != "# Just Content" {
		t.Errorf("expected body '# Just Content', got '%s'", body)
	}
}

func TestLoader_LoadFromDir(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "test")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	skillContent := `---
name: test
description: Test skill.
---
# Test Skill Content`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	loader := NewLoader(filepath.Join(tmp, "skills"))
	entries, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 skill, got %d", len(entries))
	}

	if entries[0].Skill.Name != "test" {
		t.Errorf("expected skill name 'test', got '%s'", entries[0].Skill.Name)
	}
}

func TestLoader_LoadFromDir_NoSKILL(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "test")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Don't write SKILL.md

	loader := NewLoader(filepath.Join(tmp, "skills"))
	entries, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 skills, got %d", len(entries))
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	result := expandHome("~/.kubernetes-agent/skills")
	expected := filepath.Join(home, ".kubernetes-agent/skills")
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}

	result = expandHome("~")
	if result != home {
		t.Errorf("expected '%s', got '%s'", home, result)
	}

	result = expandHome("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("expected '/absolute/path', got '%s'", result)
	}
}
