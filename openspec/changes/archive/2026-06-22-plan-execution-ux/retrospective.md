# Retrospective: plan-execution-ux

> Written: 2026-06-22 (after verify passed)
> Commit range: `3ecf2f2~1..1eedfd6`
> Worktree: merged to main

---

## 0. Evidence

- **Commit range**: `3ecf2f2~1..1eedfd6` (4 commits)
  - `3ecf2f2` feat: streamline plan execution UX
  - `de0a54d` fix: improve plan execution UX
  - `a4cf5fe` fix: use backend-generated diff summary in PlanModal DiffCard
  - `1eedfd6` docs: add plan-execution-ux opsx artifacts (proposal/specs/tasks/plan)
- **Diff size**: ~480 insertions, 70 deletions across ~40 files
- **Tasks done**: 8/8 implementation tasks verified complete (18 subtasks)
- **Active hours**: ~2h (implementation in prior session, opsx artifacts in this session)
- **Subagent dispatches**: 0 (manual implementation)
- **New external dependencies**: none
- **Bugs encountered post-merge**: none
- **OpenSpec validate state at archive**: PASS (all items valid: true)
- **Test coverage signal**: `go test ./internal/tools/k8s/...` — `TestSummarize` covers backend summary logic

Commit chain (時序):

```
3ecf2f2  feat: streamline plan execution UX
de0a54d  fix: improve plan execution UX
a4cf5fe  fix: use backend-generated diff summary in PlanModal DiffCard
1eedfd6  docs: add plan-execution-ux opsx artifacts (proposal/specs/tasks/plan)
```

---

## 1. Wins

- [evidence: `plan_write.go:69`] `diff.Summary = summarizeOne(*diff)` 直接在 `PlanWrite()` 中填充摘要，后端生成人类可读描述，无需前端计算
- [evidence: `PlanModal.tsx:103`] DiffCard 使用 `diff.summary ?? null`，完全使用 backend 摘要而非前端 `summarizeChange()`
- [evidence: `PlanModal.tsx:148–169`] YAML 折叠使用原生 `<details><summary>`，实现简洁
- [evidence: `agent/tools.go:plan_write handler`] `Session.ResetPlan()` 在 `WaitPlan()` 前调用，解决 cancel-replay bug
- [evidence: `prompt.go:14`] step 3 已更新为"Modal 确认后，直接调 k8s_execute_plan"，prompt 与期望行为一致
- [evidence: `a4cf5fe`] DiffCard 颜色映射正确：CREATE=绿/UPDATE=蓝/DELETE=红/SCALE=黄

---

## 2. Misses

- 🟡 [painful | evidence: commits 3ecf2f2, de0a54d, a4cf5fe pushed before opsx artifacts created] **Opsx workflow 逆向使用**：实现代码在 artifact 创建前已推送 main。brainstorm/design/proposal 是在实现完成后补充的文档，而非实现前的设计工具。这削弱了 artifact 驱动开发的价值
- 📌 [nit | evidence: this session] `verify.md` 模板中 `Total items` 命令因 shell 语法错误未执行，但通过手动检查确认 0 failures
- 📌 [nit | evidence: `openspec validate` output] 多条 WARNING: Purpose section too brief — 多个 spec 的 overview 不足 50 字符，建议归档后作为 follow-up 统一修正

---

## 3. Plan deviations

| Plan task | What changed | Why |
|-----------|--------------|-----|
| 所有 tasks | 实现先于 artifact 创建 | 用户在上一会话中直接要求修复，opsx artifacts 是后续补录 |
| verify / retrospective | 在 opsx workflow 框架外完成 | Superpowers 插件技能不可用，手动完成 verify |

---

## 4. Skill / workflow compliance

| Skill                                            | Used |
|--------------------------------------------------|------|
| superpowers:brainstorming                        | ✗    |
| superpowers:writing-plans                        | ✗    |
| superpowers:using-git-worktrees                   | ✗    |
| superpowers:subagent-driven-development          | ✗    |
| (transitive) superpowers:test-driven-development | ✗    |
| (transitive) superpowers:requesting-code-review  | ✗    |
| superpowers:finishing-a-development-branch       | ✗    |

### Deliberately Skipped Skills

> Opsx workflow 在本会话中作为手动备选路径使用。Superpowers 插件技能（subagent-driven-development 等）在本会话中不可用，因此 opsx artifacts 手动创建。

- **superpowers:subagent-driven-development + transitive skills**
  - **What was skipped**: 整个 apply 阶段技能链（subagent 调度、TDD、code review）
  - **Why this cycle**: 插件技能在当前会话中不可用；用户要求使用 `/opsx:apply`，但 apply 指令依赖的 Superpowers 技能未激活。实现已完成（commit 在 origin/main），走手动备选路径完成 verify + retrospective
  - **How to prevent recurrence**: opsx:apply 应检测 Superpowers 技能可用性，若不可用则提示用户 skill 缺失而非尝试执行

---

## 5. Surprises

- `Session.ResetPlan()` 的 bug（cancel 后 channel 关闭导致新 plan 立即 unblock）是在测试取消流程时才暴露的，design.md 中未预见到
- `toYAML()` 的实现比预期复杂——需要递归跳过系统字段、保留结构化嵌套，直接 JSON.stringify 会被 managedFields/creationTimestamp 填满

---

## 6. Promote candidates → long-term learning

- [ ] 🟡 **opsx:apply should gate on Superpowers skill availability** → **Promote to skill** (openspec-opsx-apply SKILL.md)
  > **Why**: 当前 `/opsx:apply` 对不存在的 skill 静默失败或执行不完整，导致需要手动备选路径
  > **How to apply**: 在 opsx:apply 指令的 PRECHECK 阶段检测 Superpowers 技能是否激活，若缺失则明确告知用户而非尝试执行

- [ ] 🟡 **prompt.go 与前端行为一致性** → **Promote to CLAUDE.md** (`internal/llm/prompt.go` section)
  > **Why**: plan execution UX 问题根因是 `prompt.go` step 3 描述与实际行为不一致。设计时未对照实际 prompt 文档
  > **How to apply**: 每次涉及工作流 UX 变更时，先读 `internal/llm/prompt.go` 确认当前 prompt 行为，再对比期望行为

- [ ] 📌 **spec overview 字符数警告** → **One-off** (mark stale after fix)
  > **Why**: 多个 spec 的 Purpose section 不足 50 字符是长期 tech debt，影响 openspec validate 的 WARNING 数量
  > **How to apply**: 在 schema 层面要求 Purpose 最小字符数，或在 validate 规则中移除该 WARNING
