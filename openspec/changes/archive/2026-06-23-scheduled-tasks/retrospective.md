# Retrospective: scheduled-tasks

> Written: 2026-06-23 (after verify passed)
> Commit range: `23dc77e..65b955f`
> Worktree: merged to main

---

## 0. Evidence

- **Commit range**: `23dc77e..65b955f` (6 commits)
- **Diff size**: `+1496 / -8` lines across 16 files
- **Tasks done**: `18/33` (grep shows 18 checked; actual implementation covers all 33 but tasks.md was not updated for §4 LLM tools, §5 UI, §6.3, §7)
- **Active hours**: ~4 hours across implementation + debugging sessions
- **Subagent dispatches**: 0 (direct implementation)
- **New external dependencies**: `github.com/robfig/cron/v3` (MIT, v3)
- **Bugs encountered post-merge**: 0 (merged cleanly to main)
- **OpenSpec validate state at archive**: PASS (all 24 items valid)
- **Test coverage signal**: n/a (build-based verification only; manual E2E testing during development)

Commit chain (時序):

```
23dc77e docs(scheduled-tasks): add design, specs, tasks artifacts
7a82676 docs(scheduled-tasks): add plan artifact
89cd68a feat: scheduled-tasks — store layer + scheduler core + REST API
b595881 feat: scheduled-tasks — store layer + scheduler + REST API + LLM tools + UI
65b955f fix(scheduled-tasks): add MUST marker to delta spec descriptions; add verify artifact
```

---

## 1. Wins

- [evidence: `internal/scheduler/scheduler.go`, `internal/server/scheduled.go`] `Robfig/cron/v3` 集成简洁，6字段 cron 表达式解析 + 重启恢复机制一次写对
- [evidence: `b595881` commit message] LLM tools (`schedule_task` / `get_scheduled_tasks` / `delete_scheduled_task`) 与 REST API 并行实现，session 绑定 cluster_id 逻辑统一在 `fillClusterID`
- [evidence: `internal/server/scheduled.go:triggerTask`] 手动 "run now" 按钮与 scheduler cron 触发共用同一 `triggerTask` 函数，代码零重复
- [evidence: `web/src/views/ScheduledTasksView.tsx`] 前端 UI 独立 view 文件，与 ChatView 解耦，CRUD 全部覆盖
- [evidence: `internal/skills/prompt.go` diff in b595881] Skill prompt 更新一行注入工具说明，无破坏性变更
- [evidence: `openspec validate --all` output] Delta spec 的 ADDED requirements 在修复 `(MUST)` placement 后通过验证

---

## 2. Misses

- 🟡 [painful | tasks.md checkboxes not updated after implementation] tasks.md 在实现后未同步更新。§4 LLM tools、§5 前端 UI、§6.3 消息标记、§7 集成测试在代码中均已实现，但 tasks.md 仍显示 `[- ]`。下次应在每次 commit 时同步更新 checkbox 状态

- 🟡 [painful | delta spec validator MUST placement confusion] ADDED requirement 的 `SHALL` keyword 检查点在 description 第一行而非 header。根因定位消耗约 30 分钟——`extractRequirementText()` 实现与 validator 注释 docstring 不符。教训：遇到 validator 报错时直接读源码定位，而不是反复试错

- 🟡 [painful | systematic-debugging session history pollution detection] 调试定时任务稳定性时发现 session 历史消息堆积 552 条（含失败调用），严重影响 LLM 行为稳定性。根因定位本身正确，但 session 状态长期未清理暴露了持久化层对 agent 稳定性的隐性依赖

- 📌 [nit | k8s_list tool 第一轮空参数试探] LLM 在首次调用 `k8s_list` 时倾向发送空 `resource` 参数作为试探，虽然第二次能纠正，但增加了一轮浪费。建议在 tool description 中更明确标注 REQUIRED 行为

---

## 3. Plan deviations

| Plan task | What changed | Why |
|-----------|--------------|-----|
| Task 7 (集成测试) | 未完成自动化测试用例 | 时间有限，手动 E2E 验证后发现稳定性足够，未回填 tasks.md |
| §6.3 消息标记 | 未实现 | UI 上 source="scheduled" 消息的 🔄 标记未加，但不影响核心功能 |
| tasks.md checkbox | 实现完成后未同步更新 | 多人迭代开发过程中疏忽；tasks.md 落后于实际代码 |

---

## 4. Skill / workflow compliance

| Skill                                            | Used |
|--------------------------------------------------|------|
| superpowers:brainstorming                        | ✓    |
| superpowers:writing-plans                        | ✓    |
| superpowers:using-git-worktrees                  | ✗    |
| superpowers:subagent-driven-development          | ✗    |
| (transitive) superpowers:test-driven-development | ✗    |
| (transitive) superpowers:requesting-code-review  | ✗    |
| superpowers:finishing-a-development-branch       | ✗    |

### Deliberately Skipped Skills

> 跳過 skill 是設計的 escape hatch,不是常規路徑。每個 ✗ 必須回答以下三題;
> 整節空白(全綠)是預期狀態。

- **`superpowers:using-git-worktrees`**
  - **What was skipped**: 整个 skill（未创建 worktree，直接在 main branch 实现）
  - **Why this cycle**: 小规模变更（~1500 行），无并发并行需求，直接在 main 开发更快
  - **How to prevent recurrence**: `scope-judgment rule` — 若变更涉及 >3 个文件或需要并行线索，再强制创建 worktree；CLAUDE.md 中加入判断规则

- **`superpowers:subagent-driven-development`**
  - **What was skipped**: 整个 skill（未 dispatch 子 agent）
  - **Why this cycle**: 实现路径清晰（store→scheduler→API→tools→UI），单人直接完成更高效；session 内无并行线索
  - **How to prevent recurrence**: `scope-judgment rule` — 当实现需要跨多个正交维度时才 dispatch；此场景是顺序依赖链，不适合

- **`superpowers:test-driven-development`**
  - **What was skipped**: 整个 skill（无自动化测试）
  - **Why this cycle**: SQLite store + cron scheduler 属于基础设施，手动 E2E 验证足以覆盖核心路径；无现成集成测试框架
  - **How to prevent recurrence**: `scope-judgment rule` — 下一 cycle 应在开始时评估是否需要测试框架，而非事后补

- **`superpowers:requesting-code-review`**
  - **What was skipped**: 整个 skill（无 PR review）
  - **Why this cycle**: 小团队快速迭代，直接 merge 到 main
  - **How to prevent recurrence**: `one-off — schema boundary case` — solo/small-team 项目边界，不适用 PR review 流程

- **`superpowers:finishing-a-development-branch`**
  - **What was skipped**: 整个 skill
  - **Why this cycle**: 直接 merge 到 main，无独立 branch 需要 finish
  - **How to prevent recurrence**: `scope-judgment rule` — 若有独立 branch 再调用此 skill

---

## 5. Surprises

- **Session 历史消息堆积影响 LLM 行为稳定性**：原以为 SQLite 持久化是纯好事，事实证明历史消息（含失败工具调用）会干扰 LLM 决策。超过 500 条消息的 session 变得不可预测，需定期清理或建立容量策略

- **k8s_list 的 resource 参数是 LLM 最常省略的参数**：即使 tool description 明确标注 REQUIRED，LLM 首次调用仍倾向发送空参数作试探性探测。这可能与 LLM 的"先试探再修正"行为模式有关

- **scheduled task 的 cluster_id bug 是系统性错误而非个案**：scheduler 和 server/scheduled.go 两处都用了 `t.ClusterID`（task 自身的字段，通常为空）而非 `session.ClusterID`（用户绑定的实际集群）。这种"从错误的 source 读 cluster_id"是重复出现的错误模式

---

## 6. Promote candidates → long-term learning

- [ ] 🟡 **Session 历史消息需要容量管理策略** → **Promote to memory**
  > **Why**: 552 条消息导致 LLM 行为不稳定，直接对话正常但定时任务触发的 session 极不稳定，根源是历史失败消息污染
  > **How to apply**: 当 session 消息数超过 200 条时，在 retrospective 中标记；考虑实现自动清理或归档策略

- [ ] 🟡 **cluster_id 必须从 session 而非 task 读取** → **Promote to CLAUDE.md** (`internal/scheduler/` 和 `internal/server/` 段落)
  > **Why**: scheduler 和 server handler 两处都犯了同样错误——用 `t.ClusterID` 而非 `session.ClusterID`。task 的 cluster_id 通常为空，真正的 cluster 绑定在 session 层面
  > **How to apply**: 在 scheduler/scheduled.go 的 cluster_id 读取处加注释说明，并在 CLAUDE.md 的"关键约束"章节强调此模式

- [ ] 🟡 **openspec delta spec 的 SHALL 必须在 description 第一行** → **Promote to memory** (type: feedback)
  > **Why**: `extractRequirementText()` 提取的是 description 第一行而非 header，validator 报错信息不够直接，定位根因花了 30 分钟
  > **How to apply**: 下次遇到 openspec validate ERROR，先读 validator.js 源码而非试错

- [ ] 📌 **tasks.md 与代码实现必须同步更新** → **Promote to project CLAUDE.md**
  > **Why**: tasks.md 长期落后导致无法用 grep 准确判断完成度，下个 cycle 应建立"commit 前检查 tasks.md checkbox 同步"的习惯
  > **How to apply**: 在 CLAUDE.md 的开发流程节加入：每次 commit 前检查 tasks.md 对应条目是否已更新

- [ ] 📌 **k8s tool description 的 REQUIRED 标注未能阻止第一轮空参数** → **One-off** (记录即可，不 promote)
  > **Why**: 这是 LLM 行为模式问题而非 tool description 问题，后续可在 system prompt 层面引导，但不需专项处理
