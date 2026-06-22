## Why

当前 plan 执行流程有两个 UX 问题：(1) Modal 确认后 agent 还在 chat 里输出一句"我将执行..."等待用户再说"yes"，多了 LLM 中转一步；(2) diffs 直接展示原始 JSON 可读性差。修复这两个问题让 plan 执行流程更直接顺畅。

## What Changes

**Plan Modal 确认流程优化**
- From: Modal 点"确认执行" → agent unblock 后在 chat 输出"我将执行..." → 用户需在 chat 输入"yes" → 才真正调用 k8s_execute_plan
- To: Modal 点"确认执行" → 直接调用 k8s_execute_plan，backend unblock 后 agent 直接输出执行结果
- Reason: 用户体验更直接，少了一步等待
- Impact: 非破坏性，需验证 prompt 修改后 LLM 行为正确

**Plan Modal Diff 卡片化展示**
- From: diffs 以 JSON.stringify 原始展示
- To: 结构化 DiffCard（操作标签 + kind/namespace/name + 变更摘要 + 可折叠 YAML）
- Reason: 用户能快速理解变更内容
- Impact: 前端组件改动，无需 backend 配合

## Capabilities

### New Capabilities

- `plan-execution-ux`: 优化 plan 确认流程和 diff 展示的用户体验。包含：(1) Modal 确认后 agent 直接执行，不需要 chat 中转；(2) diff 展示为结构化 DiffCard。

## Impact

- 修改 `internal/llm/prompt.go`：明确 Modal 确认后直接执行的行为描述
- 修改前端 `PlanModal.tsx`：新增 DiffCard 组件渲染 diff
