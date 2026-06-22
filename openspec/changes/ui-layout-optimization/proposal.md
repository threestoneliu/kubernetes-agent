## Why

当前 WebUI 采用左侧 60px 图标 Nav + 主内容区布局，顶部缺乏全局导航和品牌标识。用户体验不一致，且图标导航在视觉上较为单薄。增加全局 Header 将导航、主题切换、品牌标识统一到顶部，提升界面协调性和美观度，符合"简约大气"的设计目标。

## What Changes

**Header 替代左侧图标 Nav**
- From: 左侧 60px 图标导航栏（☰/💬/☸/🛡），顶部无全局 Header
- To: 顶部 48px 全局 Header（Logo + 导航标签 + 主题切换/设置），移除左侧 Nav
- Reason: 统一导航入口，提升视觉层次
- Impact: 非破坏性，App.tsx 和 styles.css 结构重组

**主内容区布局调整**
- From: 三栏（Nav + Sessions Panel + Chat），左 Nav 占 60px
- To: 两栏（Sessions Panel + Chat），Sessions Panel 仍在 ChatView 内部，宽度 240px
- Reason: 移除冗余导航，释放屏幕空间
- Impact: App Shell CSS 调整

**Header 导航切换视图**
- From: Nav 图标切换视图，Header 无导航功能
- To: Header 内置对话/集群/策略 三个标签页，点击切换
- Reason: 功能等价迁移，符合设计目标
- Impact: App.tsx 视图状态提升到 Shell 层

## Capabilities

### New Capabilities

- `global-header`: 顶部全局 Header，包含品牌标识、视图导航标签（对话/集群/策略）、主题切换和设置按钮，高度 48px，支持深色/浅色主题，与现有 --header-bg / --header-fg CSS 变量保持一致

### Modified Capabilities

- (none — SessionsPanel、ChatView、ClusterView、PolicyView 内部结构不变)

## Impact

**修改文件:**
- `web/src/App.tsx` — 移除 `.nav` 组件，新增 `.header-bar`，添加视图状态提升逻辑
- `web/src/styles.css` — 新增 `.header-bar`、`.header-logo`、`.header-nav`、`.nav-tab`、`.header-actions` 样式，调整 `.app` 和 `.main` 布局
- `web/src/contexts/ThemeContext.tsx` — 主题切换按钮位置迁移（App → Header）

**依赖:**
- 无新增外部依赖
- ThemeContext 已存在，只需调整按钮挂载位置

**影响范围:**
- 用户交互：导航方式不变（图标→标签页），视觉体验提升
- API：无变更
- 数据：无变更
