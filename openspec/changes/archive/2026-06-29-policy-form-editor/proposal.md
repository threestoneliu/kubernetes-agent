## Why

当前策略编辑页面要求用户直接编辑原始 YAML，对不熟悉 Kubernetes 术语和 YAML 语法的普通用户门槛高。改为结构化表单 + YAML 混排模式，让普通用户通过表单字段（复选框、下拉、标签输入器）直观地配置策略，同时保留高级用户的 YAML 直接编辑能力。

## What Changes

**策略编辑页面**
- From: 行内 textarea 编辑纯 YAML，无辅助输入
- To: 结构化表单（60% 左侧）+ YAML 编辑器（40% 右侧）并排，实时双向同步
- Reason: 降低普通用户门槛，保留高级用户灵活性
- Impact: 非破坏 — 后端 API 不变，仅前端展示层改动

**新建策略 Modal**
- From: 纯 YAML textarea
- To: 表单 + YAML 混排 Modal，左右各半
- Reason: 新建和编辑体验一致，减少用户认知负担
- Impact: 非破坏

**标签输入器**
- 新增：namespace 和 kind 字段的标签输入组件（回车追加，×删除）

## Capabilities

### New Capabilities

- `policy-form-editor`: 表单驱动的策略编辑界面，左右混排实时同步
  - 表单字段：name / effect（下拉）/ action（复选框）/ namespace（标签）/ kind（标签）/ unsafeFields（原始文本）
  - YAML → 表单：300ms debounce，解析失败时右侧红色高亮，左侧表单保持不变
  - 表单 → YAML：实时序列化
  - 新建和编辑共用同一 Modal 组件

### Modified Capabilities

（无）

## Impact

- **修改文件**: `web/src/views/PolicyView.tsx`
- **API 变更**: 无（后端 API 保持不变）
- **依赖**: 无新依赖，React 已有状态管理能力
- **测试影响**: 前端手动测试为主（新建/编辑/同步/错误边界）
