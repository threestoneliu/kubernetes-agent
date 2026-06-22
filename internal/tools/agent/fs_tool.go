package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// FSReadTool handles reading files from the filesystem, restricted to
// a specific directory tree. Used by the agent loop to give the LLM
// read access to files (e.g. skill content) without unrestricted
// filesystem access.
type FSReadTool struct {
	allowedDir string // e.g. ~/.kubernetes-agent
}

type fsReadInput struct {
	Path string `json:"path"`
}

type fsReadOutput struct {
	Content string `json:"content"`
}

type fsReadError struct {
	Error string `json:"error"`
}

// NewFSReadTool creates a new fs_read tool that restricts access to
// files under allowedDir.
func NewFSReadTool(allowedDir string) *FSReadTool {
	return &FSReadTool{allowedDir: allowedDir}
}

// Name returns the tool name.
func (t *FSReadTool) Name() string {
	return "fs_read"
}

// Description returns the tool description.
func (t *FSReadTool) Description() string {
	return "Read a file from the local filesystem. Access is restricted to ~/.kubernetes-agent/ directory."
}

// Handle reads a file from the allowed directory.
func (t *FSReadTool) Handle(ctx context.Context, call interface{}) ([]byte, error) {
	var input fsReadInput

	switch v := call.(type) {
	case []byte:
		if err := json.Unmarshal(v, &input); err != nil {
			return json.Marshal(fsReadError{Error: "invalid input: " + err.Error()})
		}
	case string:
		if err := json.Unmarshal([]byte(v), &input); err != nil {
			return json.Marshal(fsReadError{Error: "invalid input: " + err.Error()})
		}
	default:
		inputBytes, err := json.Marshal(call)
		if err != nil {
			return json.Marshal(fsReadError{Error: "invalid input"})
		}
		if err := json.Unmarshal(inputBytes, &input); err != nil {
			return json.Marshal(fsReadError{Error: "invalid input: " + err.Error()})
		}
	}

	if input.Path == "" {
		return json.Marshal(fsReadError{Error: "path is required"})
	}

	// Expand ~ to home directory
	path := input.Path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return json.Marshal(fsReadError{Error: "cannot determine home directory"})
		}
		path = filepath.Join(home, path[2:])
	}

	// Resolve to absolute path to prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		return json.Marshal(fsReadError{Error: "invalid path"})
	}

	// Verify path is within allowed directory
	if !strings.HasPrefix(absPath, t.allowedDir) {
		return json.Marshal(fsReadError{Error: "access denied: path outside allowed directory"})
	}

	// Read the file
	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return json.Marshal(fsReadError{Error: "file not found"})
		}
		if os.IsPermission(err) {
			return json.Marshal(fsReadError{Error: "permission denied"})
		}
		return json.Marshal(fsReadError{Error: err.Error()})
	}

	return json.Marshal(fsReadOutput{Content: string(content)})
}

// fsReadSchema defines the input schema for the fs_read tool.
var FSReadSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Absolute path to the file to read, or a path beginning with ~/. The path must be inside ~/.kubernetes-agent/.",
		},
	},
	"required": []string{"path"},
}

// fsReadHandler creates the handler function for fs_read tool registration.
func fsReadHandler(tool *FSReadTool) func(ctx context.Context, call interface{}) ([]byte, error) {
	return func(ctx context.Context, call interface{}) ([]byte, error) {
		return tool.Handle(ctx, call)
	}
}
