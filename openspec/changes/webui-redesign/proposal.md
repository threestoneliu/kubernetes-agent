## Why

当前 UI 布局为 sidebar(200px) + main 两栏，视觉风格为白底蓝按钮，被用户明确评价为"不够简约大气"。同时缺乏主题支持，用户无法根据自己的偏好切换浅色/深色。新的三栏布局在信息密度和沉浸感之间更优，Dark Pro 深色主题提供更强的视觉冲击力，主题切换满足个性化需求。

## What Changes

**布局：两栏 → 三栏**

- From: `.app { flex-row: 200px sidebar + flex:1 main }`
- To: `.app { flex-row: 60px nav-icons + 240px sessions + flex:1 chat }`
- Reason: 三栏布局让会话列表始终可见，切换会话无需抽屉操作，信息密度更高
- Impact: 非破坏性——HTML结构调整，React 组件逻辑不变

**主题：单色 → 双主题切换**

- From: 纯白底 #ffffff + 蓝按钮 #2563eb，无切换能力
- To: Dark Pro 深色默认 + Light 浅色可选，支持 localStorage 持久化
- Reason: 用户明确要求"支持主题切换"；Dark Pro 提供视觉冲击力
- Impact: CSS 变量隔离，仅变量值变化，组件零改动

## Capabilities

### New Capabilities

- `three-column-layout`: 三栏恒显布局，左侧图标导航（60px）+ 中间会话面板（240px）+ 右侧聊天区。App.tsx 结构变更，styles.css 布局样式。
- `theme-switching`: Dark/Light 双主题系统，通过 React Context + CSS 变量 + data-theme 属性实现，localStorage 持久化。

### Modified Capabilities

- `web-chat-ui`: 三栏布局变更不影响其功能需求（路由/SSE/模态交互），CSS 类名保留，样式层变更。

## Impact

- **影响文件**: `App.tsx`（结构）、`styles.css`（重写）、新增 `ThemeContext.tsx`
- **依赖项**: 无新依赖
- **API**: 无变化
- **向后兼容**: CSS 类名不变，ChatView/ClusterView/PolicyView 组件逻辑零改动
