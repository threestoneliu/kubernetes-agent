# Design: plan-execution-ux

## 1. 问题 1：Modal 确认后不需要 chat 中转

### 根因
`prompt.go` 里 LLM 被要求"用自然语言向用户总结 plan"，导致 Modal 确认后 LLM 在 chat 里输出执行意图消息，用户还需再输入。

### 修复
修改 `internal/llm/prompt.go`，将 plan 工作流步骤 3 改为：

```
3. Modal 确认后，直接调 k8s_execute_plan，不需要在 chat 里再次确认
```

## 2. 问题 2：diffs 结构化展示

### 修复
前端 `PlanModal` 组件：
- 将 `plan.diffs` 从 `JSON.stringify` 改为 `DiffCard` 组件渲染
- 每张 Card 显示：资源类型、名字、操作（CREATE/UPDATE/DELETE）、变更摘要
- YAML 以 `<details><summary>YAML</summary><pre>...</pre></details>` 折叠展示

### DiffCard 数据结构
```typescript
interface DiffCard {
  action: 'create' | 'update' | 'delete'
  kind: string       // e.g. "Deployment"
  name: string
  namespace?: string
  summary: string     // e.g. "replicas: 1 → 3"
  yaml: string        // 完整 manifest
}
```

### 前端解析
`plan.diffs` 当前是 `[]json.RawMessage`（manifest 对象数组），前端从中提取 kind/name/namespace 生成 DiffCard。
