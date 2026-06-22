package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFSRead_WithinAllowedDir(t *testing.T) {
	// Create temp ~/.kubernetes-agent structure
	home := t.TempDir()
	skillDir := filepath.Join(home, ".kubernetes-agent", "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	testFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(testFile, []byte("# Test Skill Content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	tool := NewFSReadTool(filepath.Join(home, ".kubernetes-agent"))

	input := map[string]string{"path": filepath.Join(skillDir, "SKILL.md")}
	inputBytes, _ := json.Marshal(input)

	result, err := tool.Handle(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output fsReadOutput
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if output.Content != "# Test Skill Content" {
		t.Errorf("expected content '# Test Skill Content', got '%s'", output.Content)
	}
}

func TestFSRead_OutsideAllowedDir(t *testing.T) {
	home := t.TempDir()
	tool := NewFSReadTool(filepath.Join(home, ".kubernetes-agent"))

	// Try to read /etc/passwd
	input := map[string]string{"path": "/etc/passwd"}
	inputBytes, _ := json.Marshal(input)

	result, err := tool.Handle(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var errOutput fsReadError
	if err := json.Unmarshal(result, &errOutput); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if errOutput.Error == "" {
		t.Error("expected error for path outside allowed directory")
	}
}

func TestFSRead_FileNotFound(t *testing.T) {
	home := t.TempDir()
	tool := NewFSReadTool(filepath.Join(home, ".kubernetes-agent"))

	input := map[string]string{"path": filepath.Join(home, ".kubernetes-agent", "nonexistent", "SKILL.md")}
	inputBytes, _ := json.Marshal(input)

	result, err := tool.Handle(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var errOutput fsReadError
	if err := json.Unmarshal(result, &errOutput); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if errOutput.Error != "file not found" {
		t.Errorf("expected 'file not found', got '%s'", errOutput.Error)
	}
}

func TestFSRead_PathTraversal(t *testing.T) {
	home := t.TempDir()
	tool := NewFSReadTool(filepath.Join(home, ".kubernetes-agent"))

	// Try path traversal attempt
	input := map[string]string{"path": filepath.Join(home, ".kubernetes-agent", "..", "etc", "passwd")}
	inputBytes, _ := json.Marshal(input)

	result, err := tool.Handle(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var errOutput fsReadError
	if err := json.Unmarshal(result, &errOutput); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if errOutput.Error == "" {
		t.Error("expected error for path traversal attempt")
	}
}

func TestFSRead_MissingPath(t *testing.T) {
	home := t.TempDir()
	tool := NewFSReadTool(filepath.Join(home, ".kubernetes-agent"))

	input := map[string]string{"path": ""}
	inputBytes, _ := json.Marshal(input)

	result, err := tool.Handle(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var errOutput fsReadError
	if err := json.Unmarshal(result, &errOutput); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if errOutput.Error != "path is required" {
		t.Errorf("expected 'path is required', got '%s'", errOutput.Error)
	}
}
