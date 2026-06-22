## 1. fs_read Tool

- [x] 1.1 Implement `fs_read` tool in `internal/agent/fs_tool.go` with path validation
- [x] 1.2 Add `fs_read` to tool registration alongside existing K8s tools
- [x] 1.3 Add unit tests for `fs_read` tool with path restriction tests

## 2. Skill System Core

- [x] 2.1 Create `internal/skills/types.go` with Skill, SkillEntry, SkillMetadata types
- [x] 2.2 Create `internal/skills/loader.go` with directory scanning and SKILL.md parsing
- [x] 2.3 Create `internal/skills/prompt.go` with `<available_skills>` XML generation
- [x] 2.4 Integrate skill loading into server startup (`cmd/server`)
- [x] 2.5 Add configuration support for `skills.dir` and `skills.enabled` in config

## 3. Skill System - Agent Integration

- [x] 3.1 Register Skill-related tools and matcher in Agent Runner (fs_read tool registered)
- [x] 3.2 Build `<available_skills>` section into system prompt (SkillsPrompt integrated)
- [ ] 3.3 Add integration tests for skill loading and prompt injection

## 4. Initial Skills Creation

- [x] 4.1 Create `~/.kubernetes-agent/skills/k8s-debug-pod/` with SKILL.md, REFERENCE.md, EXAMPLES.md
- [x] 4.2 Create `~/.kubernetes-agent/skills/k8s-deploy-app/` with SKILL.md
- [x] 4.3 Create `~/.kubernetes-agent/skills/k8s-scale-app/` with SKILL.md
- [x] 4.4 Create `~/.kubernetes-agent/skills/k8s-check-health/` with SKILL.md
- [x] 4.5 Create `~/.kubernetes-agent/skills/k8s-cluster-inspect/` with SKILL.md

## 5. Testing

- [x] 5.1 Write unit tests for Skill loader
- [x] 5.2 Write unit tests for path restriction in `fs_read` (completed with task 1.3)
- [ ] 5.3 Write integration test for skill matching workflow
- [x] 5.4 Verify `make build` succeeds
