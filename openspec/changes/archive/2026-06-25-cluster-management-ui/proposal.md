# Proposal: cluster-management-ui

## Why

当前 `ClusterView` 把「添加集群」表单内嵌在页面顶部，每次访问都占用屏幕空间。用户希望默认看到集群列表，新建入口通过弹窗完成。

## What Changes

- `web/src/views/ClusterView.tsx`：
  - 移除页面顶部内嵌表单
  - 新增工具栏「新建集群」按钮
  - 新增 Modal 弹窗承载表单
  - 表单提交成功后自动关闭弹窗

## Capabilities

1. **默认展示集群列表**：页面加载时直接展示列表，工具栏提供刷新和新建按钮
2. **Modal 新建集群**：点击「新建集群」打开 Modal，填写 name + kubeconfig 后提交
3. **提交成功关闭弹窗**：自动关闭 + 列表刷新

## Impact

- 仅改动前端 UI，无 API 变更
- 向后兼容，无破坏性变更
