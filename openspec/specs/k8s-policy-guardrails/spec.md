# k8s-policy-guardrails Specification

## Purpose
TBD - created by archiving change k8s-natural-language-agent. Update Purpose after archive.
## Requirements
### Requirement: 三态评估结果
policy engine 对每个 operation MUST 输出三态结果之一:`allow` / `confirm` / `deny`,且 MUST NOT 引入其他状态。

#### Scenario: allow 通过
- **WHEN** 某条规则匹配且 effect=allow
- **THEN** policy engine MUST 返回 `allow`,后续工具调用继续执行

#### Scenario: confirm 需 UI 确认
- **WHEN** 某条规则匹配且 effect=confirm
- **THEN** policy engine MUST 返回 `confirm`,UI MUST 展示高风险提示并要求用户二次确认

#### Scenario: deny 拒绝
- **WHEN** 某条规则匹配且 effect=deny
- **THEN** policy engine MUST 返回 `deny` 且 MUST 在 plan 中把该 operation 移到 `denied` 字段而非 `diffs`,UI MUST 展示拒绝原因

### Requirement: 匹配维度
policy 规则 MUST 支持以下匹配维度:`action`(apply / delete / scale) · `namespace`(列表) · `kind`(K8s 资源类型) · `unsafeFields`(简化 JSONPath 列表)。

#### Scenario: namespace 黑名单
- **WHEN** 规则 `action: delete` + `namespace: [kube-system, kube-public, kube-node-lease]`,LLM 提交删除 kube-system 中的 pod
- **THEN** policy engine MUST 返回 `deny` 且 reason MUST 包含"system namespace"

#### Scenario: kind 黑名单
- **WHEN** 规则 `kind: [Node, ClusterRole, ClusterRoleBinding, CRD]`,LLM 提交 apply Node
- **THEN** policy engine MUST 返回 `deny`

#### Scenario: unsafeFields 危险字段
- **WHEN** 规则 `unsafeFields: { "spec.template.spec.containers[*].securityContext.privileged": true }`,LLM 提交的 manifest 中任一 container 的 privileged=true
- **THEN** policy engine MUST 返回 `deny`,reason MUST 指明哪个 container

### Requirement: 规则顺序敏感
policy 规则 MUST 按数组顺序求值,第一条匹配的规则胜出,后续规则 MUST NOT 再次评估。

#### Scenario: 规则顺序
- **WHEN** 规则 A 匹配并 effect=deny,规则 B 也匹配但 effect=allow
- **THEN** policy engine MUST 返回 `deny`(A 胜出),B MUST NOT 被评估

### Requirement: 默认无匹配行为
当 operation 不匹配任何规则时,policy engine MUST 对读操作返回 `allow`,对写操作返回 `confirm`(默认写就要确认)。

#### Scenario: 读无匹配
- **WHEN** `k8s_get` 调用且无规则匹配
- **THEN** policy engine MUST 返回 `allow`,无需 UI 确认

#### Scenario: 写无匹配
- **WHEN** `k8s_plan_write` 调用且无规则匹配
- **THEN** policy engine MUST 返回 `confirm`,UI MUST 要求用户确认

### Requirement: 双重评估
policy engine MUST 在 `k8s_plan_write` 阶段评估一次,且 MUST 在 `k8s_execute_plan` 阶段对所有 operations 再评估一次。

#### Scenario: plan 阶段通过
- **WHEN** plan 阶段某 operation 评估为 `allow`
- **THEN** execute 阶段 MUST 重新评估该 operation,且 MUST NOT 跳过评估

#### Scenario: 中途 policy 变更
- **WHEN** plan 阶段某 operation 为 `allow`,用户在 plan 完成后修改 policy 使其变为 `deny`
- **THEN** execute 阶段 MUST 返回 `deny`,整个 plan MUST NOT 执行

### Requirement: 默认规则集
产品首次启动 MUST 插入以下默认规则,且 MUST 全部 enabled:禁删 `kube-system`/`kube-public`/`kube-node-lease` · 禁写 `Node`/`ClusterRole`/`ClusterRoleBinding`/`CustomResourceDefinition` · 危险字段(`privileged` / `hostNetwork` / `hostPID`)deny · `production`/`prod` NS 写操作 confirm。

#### Scenario: 默认规则写入
- **WHEN** 首次启动检测到 `policies` 表为空
- **THEN** 系统 MUST 插入上述默认规则集,后续启动 MUST NOT 重复插入

### Requirement: 规则可由用户编辑
用户 MUST 能通过 Web UI 的策略编辑页查看、启用/禁用、修改、删除任何规则,所有变更 MUST 持久化到 `policies` 表。

#### Scenario: 禁用某条规则
- **WHEN** 用户在策略编辑页将"禁删 kube-system"规则设为 disabled
- **THEN** 系统 MUST 更新 `policies.enabled = 0`,后续 plan 生成 MUST 跳过该规则

#### Scenario: 规则 YAML 校验
- **WHEN** 用户在策略编辑页提交规则 YAML
- **THEN** 系统 MUST 校验 YAML 语法与 schema,失败 MUST 显示具体错误位置,成功 MUST 写入 `policies` 表

### Requirement: 规则变更可审计
policy 变更(增/删/改/启停)MUST 写入 `audit_log`,含变更前后内容、操作时间。

#### Scenario: 规则修改审计
- **WHEN** 用户修改某条规则 YAML
- **THEN** 系统 MUST 在 `audit_log` 追加一条 `action: policy_change` 记录,含 `target: 规则名`、`message: 修改前内容 → 修改后内容`

