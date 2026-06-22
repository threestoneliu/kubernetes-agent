# Skill System Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development
> to implement this plan task-by-task.

**Goal:** Add a Skill system to kubernetes-agent that allows LLM to match user intent and execute structured workflows via `<available_skills>` XML injection.

**Architecture:** Skill system consists of three components: (1) `fs_read` tool for reading Skill files, (2) `internal/skills/` package for loading and managing Skills, (3) System Prompt injection of `<available_skills>` XML. LLM matches user intent to Skills and calls `fs_read` to load Skill content.

**Tech Stack:** Go (kubernetes-agent), YAML frontmatter parsing, filesystem access restricted to `~/.kubernetes-agent/`

---

## Task 1: fs_read Tool

**Files:**
- Create: `internal/agent/fs_tool.go`
- Modify: `internal/agent/tools.go` (register tool)
- Test: `internal/agent/fs_tool_test.go`

### Steps

- [ ] **Step 1: Write the failing test**

```go
// internal/agent/fs_tool_test.go
package agent

import (
    "os"
    "path/filepath"
    "testing"
)

func TestFSRead_WithinAllowedDir(t *testing.T) {
    // Create temp ~/.kubernetes-agent structure
    home := t.TempDir()
    os.Setenv("HOME", home)
    
    skillDir := filepath.Join(home, ".kubernetes-agent", "skills", "test-skill")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test"), 0644)
    
    tool := newFSReadTool(skillDir)
    result, err := tool.Handle(context.Background(), llm.ToolCall{
        Input: json.RawMessage(`{"path": "~/.kubernetes-agent/skills/test-skill/SKILL.md"}`),
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.Contains(string(result), "Test") {
        t.Error("expected content to contain 'Test'")
    }
}

func TestFSRead_OutsideAllowedDir(t *testing.T) {
    tool := newFSReadTool("~/.kubernetes-agent")
    _, err := tool.Handle(context.Background(), llm.ToolCall{
        Input: json.RawMessage(`{"path": "/etc/passwd"}`),
    })
    if err == nil {
        t.Error("expected error for path outside allowed directory")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/... -run TestFSRead -v`
Expected: FAIL - fs_read tool not yet implemented

- [ ] **Step 3: Write fs_tool.go implementation**

```go
// internal/agent/fs_tool.go
package agent

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// FSReadTool handles reading files from the filesystem.
type FSReadTool struct {
    allowedDir string // ~/.kubernetes-agent
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

// newFSReadTool creates a new fs_read tool that restricts access to allowedDir.
func newFSReadTool(allowedDir string) *FSReadTool {
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
func (t *FSReadTool) Handle(ctx context.Context, call llm.ToolCall) ([]byte, error) {
    var input fsReadInput
    if err := json.Unmarshal(call.Input, &input); err != nil {
        return json.Marshal(fsReadError{Error: "invalid input: " + err.Error()})
    }
    
    if input.Path == "" {
        return json.Marshal(fsReadError{Error: "path is required"})
    }
    
    // Expand ~ to home directory
    if strings.HasPrefix(input.Path, "~/") {
        home, err := os.UserHomeDir()
        if err != nil {
            return json.Marshal(fsReadError{Error: "cannot determine home directory"})
        }
        input.Path = filepath.Join(home, input.Path[2:])
    }
    
    // Resolve to absolute path to prevent path traversal
    absPath, err := filepath.Abs(input.Path)
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/... -run TestFSRead -v`
Expected: PASS

- [ ] **Step 5: Register fs_read in tools.go**

```go
// In internal/agent/tools.go, add to RegisterK8sTools:

fsReadTool := newFSReadTool(expandHome("~/.kubernetes-agent"))

return []llm.Tool{
    // ... existing k8s tools ...
    {
        Name:        "fs_read",
        Description: fsReadTool.Description(),
        InputSchema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "path": map[string]any{"type": "string", "description": "Path to file to read"},
            },
            "required": []string{"path"},
        },
        Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
            return fsReadTool.Handle(ctx, call)
        },
    },
}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/agent/... -v`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/agent/fs_tool.go internal/agent/fs_tool_test.go internal/agent/tools.go
git commit -m "feat(agent): add fs_read tool for reading Skill files"
```

---

## Task 2: Skill System Core

**Files:**
- Create: `internal/skills/types.go`
- Create: `internal/skills/loader.go`
- Create: `internal/skills/prompt.go`
- Modify: `internal/config/config.go` (add Skills config)
- Modify: `cmd/server/main.go` (integrate skill loading)

### Steps

- [ ] **Step 1: Write types.go**

```go
// internal/skills/types.go
package skills

// Skill represents a loaded skill.
type Skill struct {
    Name        string
    Description string
    FilePath    string // absolute path to SKILL.md
    BaseDir     string
    Source      string // "project" or "user"
    AlwaysInject bool
    Priority    int
    content     string // SKILL.md raw content
}

// SkillEntry is a skill with its parsed metadata.
type SkillEntry struct {
    Skill       Skill
    Frontmatter map[string]string
    Metadata    *SkillMetadata
}

// SkillMetadata contains optional skill metadata.
type SkillMetadata struct {
    Emoji    string
    Homepage  string
    OS       []string
    Requires  *Requires
}

// Requires lists skill dependencies.
type Requires struct {
    Bins   []string
    Env    []string
    Config []string
}
```

- [ ] **Step 2: Write loader.go with frontmatter parsing**

```go
// internal/skills/loader.go
package skills

import (
    "embed"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    
    "gopkg.in/yaml.v3"
)

// Loader loads skills from directories.
type Loader struct {
    dirs []string
}

// NewLoader creates a new skill loader.
func NewLoader(dirs ...string) *Loader {
    return &Loader{dirs: dirs}
}

// LoadAll loads all skills from configured directories.
func (l *Loader) LoadAll() ([]*SkillEntry, error) {
    var entries []*SkillEntry
    for _, dir := range l.dirs {
        loaded, err := l.loadFromDir(dir)
        if err != nil {
            continue // fail-safe: log and continue
        }
        entries = append(entries, loaded...)
    }
    return entries, nil
}

func (l *Loader) loadFromDir(dir string) ([]*SkillEntry, error) {
    dir = expandHome(dir)
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }
    
    var result []*SkillEntry
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
        if _, err := os.Stat(skillPath); os.IsNotExist(err) {
            continue
        }
        
        entry, err := l.loadSkillFile(skillPath, dir)
        if err != nil {
            continue // fail-safe
        }
        result = append(result, entry)
    }
    return result, nil
}

func (l *Loader) loadSkillFile(skillPath, baseDir string) (*SkillEntry, error) {
    content, err := os.ReadFile(skillPath)
    if err != nil {
        return nil, err
    }
    
    frontmatter, body, err := parseFrontmatter(string(content))
    if err != nil {
        return nil, err
    }
    
    entry := &SkillEntry{
        Frontmatter: frontmatter,
        Skill: Skill{
            Name:        frontmatter["name"],
            Description: frontmatter["description"],
            FilePath:    skillPath,
            BaseDir:     baseDir,
            Source:      "user",
            content:     body,
        },
    }
    
    return entry, nil
}

// parseFrontmatter extracts YAML frontmatter from markdown content.
func parseFrontmatter(content string) (map[string]string, string, error) {
    if !strings.HasPrefix(content, "---") {
        return nil, content, nil
    }
    
    lines := strings.Split(content, "\n")
    var frontmatterLines []string
    var bodyLines []string
    inFrontmatter := false
    
    for i, line := range lines {
        if i == 0 && strings.HasPrefix(line, "---") {
            inFrontmatter = true
            continue
        }
        if inFrontmatter && strings.HasPrefix(line, "---") {
            bodyLines = lines[i+1:]
            break
        }
        if inFrontmatter {
            frontmatterLines = append(frontmatterLines, line)
        }
    }
    
    var frontmatter map[string]string
    if len(frontmatterLines) > 0 {
        if err := yaml.Unmarshal([]byte(strings.Join(frontmatterLines, "\n")), &frontmatter); err != nil {
            return nil, "", err
        }
    }
    
    return frontmatter, strings.Join(bodyLines, "\n"), nil
}

// expandHome expands ~ to user's home directory.
func expandHome(p string) string {
    if !strings.HasPrefix(p, "~") {
        return p
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return p
    }
    if p == "~" {
        return home
    }
    if strings.HasPrefix(p, "~/") {
        return filepath.Join(home, p[2:])
    }
    return p
}
```

- [ ] **Step 3: Write prompt.go for `<available_skills>` XML generation**

```go
// internal/skills/prompt.go
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
```

- [ ] **Step 4: Add Skills config to config.go**

```go
// In internal/config/config.go:

type Skills struct {
    Dir    string `yaml:"dir"`
    Enabled bool  `yaml:"enabled"`
}

type Config struct {
    Server  Server  `yaml:"server"`
    Storage Storage `yaml:"storage"`
    LLM     LLM     `yaml:"llm"`
    Logging Logging `yaml:"logging"`
    Skills  Skills  `yaml:"skills"`
}

// Set defaults
if c.Skills.Dir == "" {
    c.Skills.Dir = "~/.kubernetes-agent/skills"
}
if c.Skills.Enabled == false {
    c.Skills.Enabled = true
}
```

- [ ] **Step 5: Integrate skill loading in cmd/server/main.go**

```go
// In buildDeps function, add skills:

func buildDeps(cfg *config.Config, db *store.DB, aead *crypto.AEAD) server.Deps {
    // ... existing code ...
    
    // Load skills
    skillsDir := expandHome(cfg.Skills.Dir)
    skillLoader := skills.NewLoader(skillsDir)
    skillEntries, _ := skillLoader.LoadAll() // fail-safe
    skillPromptBuilder := skills.NewPromptBuilder(skillEntries)
    
    rf := newRunnerFactory(registry, db, factory, engine, cfg.LLM.Default, skillPromptBuilder)
    // ... rest
}
```

- [ ] **Step 6: Run tests**

Run: `go build ./... && go test ./internal/skills/...`
Expected: Build succeeds, no tests yet

- [ ] **Step 7: Commit**

```bash
git add internal/skills/ internal/config/config.go cmd/server/main.go
git commit -m "feat(skills): add skill system core (types, loader, prompt)"
```

---

## Task 3: Skill System - Agent Integration

**Files:**
- Modify: `cmd/server/main.go` (pass PromptBuilder to RunnerFactory)
- Modify: `internal/agent/runner.go` (use PromptBuilder in system prompt)
- Test: `internal/agent/agent_test.go`

### Steps

- [ ] **Step 1: Modify RunnerFactory to accept PromptBuilder**

```go
// In cmd/server/main.go, modify runnerFactory struct and newRunnerFactory:

type runnerFactory struct {
    // ... existing fields ...
    skillPromptBuilder *skills.PromptBuilder
}

func newRunnerFactory(...) *runnerFactory {
    // ... existing code ...
    return &runnerFactory{
        // ... existing fields ...
        skillPromptBuilder: skillPromptBuilder,
    }
}

func (rf *runnerFactory) NewRunner(...) *agent.Runner {
    r := &agent.Runner{
        Client: cli, 
        Store: rf.db,
        SkillsPrompt: rf.skillPromptBuilder.FormatSkillsForPrompt(),
    }
    // ...
}
```

- [ ] **Step 2: Modify Runner to include skills in system prompt**

```go
// In internal/agent/agent.go, add SkillsPrompt field to Runner:

type Runner struct {
    Client llm.Client
    Tools  []llm.Tool
    Store  MessageStore
    Events chan<- Event
    Session *Session
    Deps   ToolDeps
    SystemPrompt string // existing
    SkillsPrompt string // NEW: <available_skills> XML
    // ...
}
```

- [ ] **Step 3: Build system prompt with skills in Run()**

```go
// In internal/agent/agent.go, modify Run():

systemPrompt := r.systemPrompt()
if r.SkillsPrompt != "" {
    systemPrompt += r.SkillsPrompt
}

msgs := []transcriptMessage{
    {Role: llm.RoleSystem, Parts: []llm.ContentPart{{Type: "text", Text: systemPrompt}}},
}
```

- [ ] **Step 4: Run tests**

Run: `go build ./... && go test ./internal/agent/...`
Expected: Build succeeds, tests pass

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go internal/agent/agent.go
git commit -m "feat(agent): integrate skills prompt into system prompt"
```

---

## Task 4: Initial Skills Creation

**Files:**
- Create: `~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md`
- Create: `~/.kubernetes-agent/skills/k8s-debug-pod/REFERENCE.md`
- Create: `~/.kubernetes-agent/skills/k8s-debug-pod/EXAMPLES.md`
- Create: `~/.kubernetes-agent/skills/k8s-deploy-app/SKILL.md`
- Create: `~/.kubernetes-agent/skills/k8s-deploy-app/REFERENCE.md`
- Create: `~/.kubernetes-agent/skills/k8s-deploy-app/EXAMPLES.md`
- Create: `~/.kubernetes-agent/skills/k8s-scale-app/SKILL.md`
- Create: `~/.kubernetes-agent/skills/k8s-scale-app/REFERENCE.md`
- Create: `~/.kubernetes-agent/skills/k8s-scale-app/EXAMPLES.md`
- Create: `~/.kubernetes-agent/skills/k8s-check-health/SKILL.md`
- Create: `~/.kubernetes-agent/skills/k8s-check-health/REFERENCE.md`
- Create: `~/.kubernetes-agent/skills/k8s-check-health/EXAMPLES.md`
- Create: `~/.kubernetes-agent/skills/k8s-cluster-inspect/SKILL.md`
- Create: `~/.kubernetes-agent/skills/k8s-cluster-inspect/REFERENCE.md`
- Create: `~/.kubernetes-agent/skills/k8s-cluster-inspect/EXAMPLES.md`

### Steps

- [ ] **Step 1: Create k8s-debug-pod Skill**

SKILL.md:
```markdown
---
name: k8s-debug-pod
description: Debug Kubernetes pod issues. Use when user wants to debug, troubleshoot, or diagnose a pod problem.
---

# k8s-debug-pod

Debug a Kubernetes pod systematically.

## Workflow

### Phase 1: Gather Information
1. Use k8s_describe to get pod details and diagnosis_hints
2. Check events for error messages

### Phase 2: Analyze
Interpret diagnosis_hints:
- "ImagePullBackOff" → Check image name and imagePullSecrets
- "CrashLoopBackOff" → Get logs and check startup configuration
- "容器反复崩溃" → Check application logs

### Phase 3: Take Action
Recommend and optionally execute fixes.

## Output Format

Provide a structured diagnosis report.
```

- [ ] **Step 2: Create remaining 4 Skills** (k8s-deploy-app, k8s-scale-app, k8s-check-health, k8s-cluster-inspect)

Each skill should follow the same pattern with appropriate workflows.

- [ ] **Step 3: Verify skill loading**

Run server and check logs for skill loading messages.

- [ ] **Step 4: Commit skills**

```bash
mkdir -p ~/.kubernetes-agent/skills/
git add ~/.kubernetes-agent/skills/
git commit -m "feat(skills): add initial 5 Skills (debug-pod, deploy-app, scale-app, check-health, cluster-inspect)"
```

---

## Task 5: Testing

**Files:**
- Modify: `internal/skills/loader_test.go`
- Modify: `internal/agent/fs_tool_test.go`

### Steps

- [ ] **Step 1: Write loader tests**

```go
// internal/skills/loader_test.go
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
    
    if !strings.Contains(body, "# Content") {
        t.Error("expected body to contain '# Content'")
    }
}

func TestLoader_LoadFromDir(t *testing.T) {
    tmp := t.TempDir()
    skillDir := filepath.Join(tmp, "skills", "test")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test
description: Test.
---
# Test`), 0644)
    
    loader := NewLoader(filepath.Join(tmp, "skills"))
    entries, err := loader.LoadAll()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if len(entries) != 1 {
        t.Errorf("expected 1 skill, got %d", len(entries))
    }
}
```

- [ ] **Step 2: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 3: Final build verification**

Run: `make build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add internal/skills/loader_test.go
git commit -m "test(skills): add loader and fs_tool tests"
```

---

## Task 6: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

- [ ] **Step 2: Verify skill loading at startup**

Check server logs show skill loading.

- [ ] **Step 3: Verify `<available_skills>` in system prompt**

Add debug log to verify XML is injected.
