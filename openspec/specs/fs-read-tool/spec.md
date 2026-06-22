# fs_read Tool

## Purpose

**TBD**

## Requirements

### Requirement: fs_read Tool Registration

The Agent loop SHALL register an `fs_read` tool that allows LLM to read local filesystem files. This tool SHALL be available alongside the existing K8s tools (k8s_get, k8s_list, k8s_describe, k8s_plan_write, k8s_execute_plan, k8s_ask_user).

#### Scenario: fs_read tool is registered
- **WHEN** The Agent initializes its tool set
- **THEN** The `fs_read` tool SHALL be included in the list of available tools passed to the LLM

---

### Requirement: fs_read Input Schema

The `fs_read` tool SHALL accept a JSON input with a `path` field representing the file path to read.

#### Scenario: Valid input with path
- **WHEN** The LLM calls `fs_read` with `{"path": "~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md"}`
- **THEN** The tool SHALL attempt to read the file at the specified path

#### Scenario: Missing path field
- **WHEN** The LLM calls `fs_read` with `{}`
- **THEN** The tool SHALL return an error indicating the path field is required

---

### Requirement: Directory Access Restriction

The `fs_read` tool SHALL restrict file access to files within the `~/.kubernetes-agent/` directory tree. Any file path that resolves to a location outside this directory SHALL be rejected with an error.

#### Scenario: Access within allowed directory
- **WHEN** The LLM calls `fs_read` with `{"path": "~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md"}`
- **THEN** The tool SHALL successfully read the file

#### Scenario: Access outside allowed directory
- **WHEN** The LLM calls `fs_read` with `{"path": "/etc/passwd"}`
- **THEN** The tool SHALL return an error indicating access is denied

#### Scenario: Path traversal attempt
- **WHEN** The LLM calls `fs_read` with `{"path": "~/.kubernetes-agent/../etc/passwd"}`
- **THEN** The tool SHALL resolve the path and reject access outside `~/.kubernetes-agent/`

---

### Requirement: fs_read Output Format

The `fs_read` tool SHALL return the file contents as a JSON object with a `content` field containing the file text. On error, it SHALL return a JSON object with an `error` field.

#### Scenario: Successful file read
- **WHEN** The LLM calls `fs_read` with a valid path to an existing file containing "Hello World"
- **THEN** The tool SHALL return `{"content": "Hello World"}`

#### Scenario: File not found
- **WHEN** The LLM calls `fs_read` with a path to a non-existent file
- **THEN** The tool SHALL return `{"error": "file not found"}`

#### Scenario: Permission denied
- **WHEN** The LLM calls `fs_read` with a path to a file without read permissions
- **THEN** The tool SHALL return `{"error": "permission denied"}`
