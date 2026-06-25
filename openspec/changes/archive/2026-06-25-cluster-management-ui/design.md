# Cluster Management UI Design

## Overview

改 `ClusterView` 布局：把内嵌的「添加集群」表单移到 Modal 弹窗，页面默认只展示集群列表。

## Layout

```
+------------------------------------------+
|  集群管理                                |
|  工具栏: [已配置的集群 (N个)] [刷新] [新建集群]  |
+------------------------------------------+
|  [集群列表 - name / server / user / 删除] |
|  [集群列表 - name / server / user / 删除] |
+------------------------------------------+
```

点击「新建集群」→ Modal 弹窗打开 → 表单提交成功后自动关闭。

## Component Changes

### ClusterView.tsx

- **移除**：页面顶部内嵌的 `<form>` 区块（原有 name + kubeconfig 输入）
- **新增**：
  - `showModal` state：`boolean`，默认 `false`
  - 工具栏按钮：「新建集群」（`onClick={() => setShowModal(true)}`）
  - `<Modal>` 弹窗：内含与原表单相同的 name + kubeconfig + 提交按钮
- **表单提交**：`createCluster` 成功后 `setShowModal(false)` 关闭弹窗
- **取消**：`onCancel={() => setShowModal(false)}`

### Modal

复用 `web/src/components/Modal.tsx` 现有组件。

## API

无变更。复用现有 `createCluster`。

## States

| 状态 | 表现 |
|------|------|
| 初始 | 工具栏 + 集群列表，无弹窗 |
| 弹窗打开 | 遮罩 + 表单弹窗，背景列表可见 |
| 提交中 | 按钮 disabled，显示"提交中…" |
| 提交成功 | 弹窗关闭，列表刷新 |
| 提交失败 | 弹窗保持，错误 toast |
