# Skill System Design

## Overview

Design a Skill system for kubernetes-agent that allows LLM to match user intent and execute specific workflows.

## Architecture

### Core Mechanism

1. **Trigger**: LLM intent matching based on `<available_skills>` in system prompt
2. **Skill Loading**: LLM calls Read Tool to read `~/.kubernetes-agent/skills/<name>/SKILL.md`
3. **Content Injection**: Skill content appended to conversation via Tool Result

### Workflow

```
User: "帮我 debug nginx-pod"
         │
         ▼
┌─────────────────────────────────────────┐
│  System Prompt contains <available_skills> │
│  LLM sees available Skills              │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│  LLM identifies "debug" matches         │
│  k8s-debug-pod                          │
│  → Calls Read Tool                      │
│    Tool: "Read"                         │
│    args: { path: "~/.kubernetes-agent/  │
│            skills/k8s-debug-pod/         │
│            SKILL.md" }                   │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│  Read Tool returns SKILL.md content     │
│  → Content appended as Tool Result     │
│  → LLM sees skill, executes workflow   │
└─────────────────────────────────────────┘
```

### No New Tool Required

The Skill system reuses the existing Read Tool. LLM decides which Skill to load based on `<available_skills>` description.

## Directory Structure

```
~/.kubernetes-agent/skills/
├── k8s-debug-pod/
│   ├── SKILL.md           # Required: name, description, workflow
│   ├── REFERENCE.md       # Optional: detailed reference
│   ├── EXAMPLES.md       # Optional: usage examples
│   └── scripts/          # Optional: helper scripts
├── k8s-deploy-app/
│   └── ...
├── k8s-scale-app/
│   └── ...
├── k8s-check-health/
│   └── ...
└── k8s-cluster-inspect/
    └── ...
```

## SKILL.md Format

```markdown
---
name: k8s-debug-pod
description: Debug Kubernetes pod issues. Use when user wants to debug, troubleshoot, or diagnose a pod problem.
---

# k8s-debug-pod

## Workflow

### Phase 1: Gather Information
...

### Phase 2: Analyze
...

## Output Format
...
```

## System Prompt Injection

```xml
<available_skills>
  <skill>
    <name>k8s-debug-pod</name>
    <description>Debug Kubernetes pod issues. Use when user wants to debug, troubleshoot, or diagnose a pod problem.</description>
    <location>~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md</location>
  </skill>
  ...
</available_skills>
```

## Configuration

```yaml
skills:
  dir: "~/.kubernetes-agent/skills"
  enabled: true
```

## Components

| Component | Responsibility |
|-----------|---------------|
| Skill Loader | Scan directory, load SKILL.md, parse frontmatter |
| Prompt Builder | Build `<available_skills>` XML for system prompt |
| Skill Registry | Store loaded skills, provide lookup |

## Initial Skills

| Skill | Description |
|-------|-------------|
| `k8s-debug-pod` | Debug Pod issues |
| `k8s-deploy-app` | Deploy applications to Kubernetes |
| `k8s-scale-app` | Scale up/down applications |
| `k8s-check-health` | Health check |
| `k8s-cluster-inspect` | Cluster inspection |

## File Locations

| Component | Path |
|-----------|------|
| Skills Directory | `~/.kubernetes-agent/skills/` |
| Skill Engine | `internal/skills/` |
| Types | `internal/skills/types.go` |
| Loader | `internal/skills/loader.go` |
| Prompt Builder | `internal/skills/prompt.go` |
