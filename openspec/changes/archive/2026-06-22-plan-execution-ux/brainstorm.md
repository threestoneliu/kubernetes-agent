## Design Summary

两个 UX 问题：

1. **Modal 确认后 agent 还在 chat 里等 "yes"**：Modal 点"确认执行" → backend `resumeSession(approved: true)` → agent 被 unblock 后在 chat 里输出一句"我将执行..." → 用户还得在 chat 输入"yes" → 才真正调用 `k8s_execute_plan`。多了一步 LLM 中转。
2. **diffs 展示为原始 JSON**：用户看不懂。应结构化展示（资源+操作+折叠 YAML）。

## Alternatives Considered

### 问题 1

#### 方案 A：修改 system prompt
- **做法**：在 system prompt 里明确说"Modal 确认后直接执行，不需要在 chat 里再确认"
- **优点**：无代码改动，只需改 prompt
- **缺点**：LLM 行为不可靠，可能还是会问

#### 方案 B：confirmPlan 时直接调用 execute_plan（推荐）
- **做法**：`confirmPlan()` 点击后，frontend 直接调用 `/api/sessions/:id/resume` 带 `approved: true`，backend unblock agent 后 agent 不再输出确认消息，而是直接调 `k8s_execute_plan` 后输出执行结果
- **优点**：保持现有 backend 结构，LLM 行为由 prompt 控制
- **缺点**：需要验证 prompt 是否足够明确

### 问题 2

#### 方案 A：plan 时生成结构化 diff 卡片
- **做法**：plan 时解析 manifest，生成 `{action, kind, name, namespace, summary, yaml}` 的 diff 列表，前端用 DiffCard 渲染
- **优点**：用户友好，可读性强
- **缺点**：需要修改 plan_write 和 backend diff 生成逻辑

#### 方案 B：前端直接解析 diff（推荐）
- **做法**：前端收到 `plan.diffs`（当前是 JSON manifest 列表），前端解析并渲染为 DiffCard
- **优点**：backend 改动最小，前端独立完成渲染
- **缺点**：前端需要解析 K8s manifest 结构

## Key Decisions

1. 问题1：采用方案 B（修改 prompt 明确 Modal 确认后的行为），同时检查 `confirmPlan` 的 SSE 流是否正常
2. 问题2：采用方案 B（前端解析 diffs 为结构化展示 + YAML 折叠）

## Open Questions

1. 问题1 中"在 chat 里输入 yes"的触发点：是 LLM 自己输出了确认消息并等待用户回复，还是 `k8s_execute_plan` 本身在 tool_result 后有额外输出？
