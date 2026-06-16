# Spec: k8s-write-with-plan-preview

## ADDED Requirements

### Requirement: 写操作必须先经 Plan 预览
所有 K8s 写操作(apply / scale / delete)MUST 由 `k8s_plan_write` 工具生成 plan,plan 来自 K8s server-side dry-run,MUST NOT 直接由 LLM 想象产出。

#### Scenario: agent 尝试直接执行写操作
- **WHEN** LLM 试图调 `k8s_execute_plan` 而未先调 `k8s_plan_write` 产生对应 plan_id
- **THEN** 系统 MUST 拒绝该调用,工具结果 MUST 包含 `code: missing_plan_id` 错误,MUST NOT 执行任何 K8s API 写操作

#### Scenario: 同一 plan 一次执行
- **WHEN** 用户在 UI 对同一 plan 重复点击"确认执行"
- **THEN** 系统 MUST 仅首次执行成功,后续 MUST 因 plan status 已变更返回 `code: plan_already_executed`

### Requirement: Plan 数据结构
`k8s_plan_write` MUST 返回的 plan MUST 包含:`plan_id` (UUID) · `summary` (人话描述) · `diffs` (每个 operation 的 before/after) · `risk` (low/medium/high) · `denied` (policy 拒绝的 operation 列表)。

#### Scenario: 单 operation 写 plan
- **WHEN** LLM 提交一个 apply 操作
- **THEN** plan MUST 含单个 diff,`before` 为空(创建),`after` 为 dry-run 后的对象,`risk` 由 policy engine 决定

#### Scenario: 多个 operations
- **WHEN** LLM 提交一个含 N 个 operations 的 plan_write
- **THEN** plan MUST 含 N 个 diffs 数组项,且 MUST 按 LLM 提交顺序排列

#### Scenario: 包含被 policy 拒绝的 operation
- **WHEN** LLM 提交的操作包含一个 policy 拒绝(如 delete kube-system)
- **THEN** 该 operation MUST 出现在 `denied` 字段(含 reason)而非 `diffs`,且 `k8s_execute_plan` MUST NOT 把它加入执行队列

### Requirement: Server-side dry-run
plan 生成 MUST 使用 K8s server-side dry-run(`dryRun=All`),对 apply 操作使用 `dryRun=All` 的 PUT/PATCH,对 delete/scale 使用 GET 拿当前对象作为 `before`。

#### Scenario: dry-run 失败
- **WHEN** K8s API server-side dry-run 返回错误(如 schema 校验失败)
- **THEN** `k8s_plan_write` MUST 返回 `code: dry_run_failed` 含原始错误,UI MUST 展示该错误并 MUST NOT 进入预览态

### Requirement: 风险等级
每个 diff MUST 含 `risk` 字段,取值为 `low` / `medium` / `high`,由 policy engine 在 plan 阶段根据规则决定。

#### Scenario: 风险等级与 policy 一致
- **WHEN** policy 规则标记 operation 为 `confirm` 效应
- **THEN** 该 diff 的 `risk` MUST 为 `high`,且 UI MUST 强制二次确认(即便已 plan)

### Requirement: 二次 Policy 评估
`k8s_execute_plan` MUST 在执行前再次调用 policy engine 评估所有 operations,任何一条现为 deny MUST 阻断整个 plan。

#### Scenario: 中途 policy 变更
- **WHEN** plan 生成后用户在策略编辑页改动了某条规则,使得原本 allow 的 operation 现为 deny
- **THEN** `k8s_execute_plan` MUST 拒绝执行并 MUST 推 `error` 事件含 `code: policy_changed`,UI MUST 提示用户重新生成 plan

### Requirement: 失败回滚
`k8s_execute_plan` MUST 在执行过程中任一 operation 失败时,回滚已成功的前序 operations,基于 dry-run 时获取的 `before` 状态做反向操作。

#### Scenario: 多步 plan 中段失败
- **WHEN** plan 含 3 个 apply operation,执行到第 2 个时 K8s API 返回 5xx
- **THEN** 系统 MUST 尝试对第 1 个 operation 做反向 apply(还原到 before),且 MUST 在 `error` 事件中说明"operation 2 失败,已回滚 operation 1",UI MUST 展示完整回滚日志

#### Scenario: 回滚本身失败
- **WHEN** 反向操作也失败(如 K8s API 仍 5xx)
- **THEN** 系统 MUST 继续尝试后续回滚步骤,MUST 把所有失败信息写入 audit_log,UI MUST 明确标注"部分回滚失败,请人工介入"

### Requirement: Audit 必写
所有 plan 执行结果(成功、失败、denied)MUST 写入 `audit_log` 表,字段含 session_id、cluster_id、action、target、status、message。

#### Scenario: 写操作完成
- **WHEN** `k8s_execute_plan` 完成(无论结果)
- **THEN** 系统 MUST 在 `audit_log` 追加一条记录,含执行时间、用户(本地 MVP 单一用户)、plan_id、所有 operations 的状态汇总
