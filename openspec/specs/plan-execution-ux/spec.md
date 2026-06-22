# plan-execution-ux Specification

## Purpose
TBD - created by archiving change plan-execution-ux. Update Purpose after archive.
## Requirements
### Requirement: Modal 确认执行 SHALL 直接触发计划执行

当用户在 Plan Modal 中点击"确认执行"后，系统 SHALL 直接调用 k8s_execute_plan 工具执行计划，无需在 chat 中等待 LLM 输出"确认"文字后再由用户输入"yes"。

#### Scenario: 用户在 Modal 点击确认执行
- **WHEN** 用户在 Plan Modal 中点击"确认执行"按钮
- **THEN** 系统 SHALL 直接调用 k8s_execute_plan，agent 输出执行结果，不再需要用户额外输入"yes"

#### Scenario: Modal 取消 SHALL 保持阻塞并等待新计划
- **WHEN** 用户在 Plan Modal 中点击"取消"按钮
- **THEN** 系统 SHALL 发送 CancelPlan 重置 plan 状态，Session 保持阻塞直到收到新计划

---

### Requirement: DiffCard SHALL 展示人类可读的变更摘要

每个 DiffCard SHALL 显示操作标签（CREATE/UPDATE/DELETE/SCALE）、资源类型、namespace/name，以及 backend 生成的人类可读摘要（由 diff.summary 字段提供），而非前端自行计算字段级差异。

#### Scenario: DiffCard 显示 CREATE 操作
- **WHEN** 后端返回 action=CREATE 的 diff 且 diff.summary="创建 Deployment default/nginx"
- **THEN** DiffCard SHALL 显示绿色 CREATE 标签、kind/name、以及摘要文本"创建 Deployment default/nginx"

#### Scenario: DiffCard 显示 UPDATE 操作
- **WHEN** 后端返回 action=apply 且 diff.summary="更新 Deployment default/nginx: replicas: 1 → 3"
- **THEN** DiffCard SHALL 显示蓝色 UPDATE 标签、kind/name、以及变更摘要

#### Scenario: DiffCard 显示 DELETE 操作
- **WHEN** 后端返回 action=delete 且 diff.summary="删除 Deployment default/nginx"
- **THEN** DiffCard SHALL 显示红色 DELETE 标签、kind/name、以及摘要文本

---

### Requirement: DiffCard YAML 内容 SHALL 默认折叠

每个 DiffCard 的完整 YAML 内容 SHALL 默认折叠为"查看完整 YAML"展开项，用户点击后才展开显示。

#### Scenario: YAML 默认折叠
- **WHEN** DiffCard 渲染完成
- **THEN** YAML 内容 SHALL 默认隐藏，页面显示"查看完整 YAML"文字

#### Scenario: 点击展开 YAML
- **WHEN** 用户点击"查看完整 YAML"展开项
- **THEN** YAML 内容 SHALL 展开显示，最多渲染 300px 高度，超出部分滚动

#### Scenario: YAML 跳过不相关信息
- **WHEN** 渲染 YAML 内容
- **THEN** 系统 SHALL 跳过 creationTimestamp、generation、managedFields、ownerReferences、resourceVersion、uid、status 等系统元数据字段

