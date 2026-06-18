## Context

当前 UI 布局为 sidebar(200px) + main(flex1) 的两栏结构，视觉风格为白底 + 蓝按钮，缺乏视觉冲击力。会话列表以左面板形式存在，切换会话时视觉变化大。三栏布局在信息密度和沉浸感之间取得最佳平衡。

## Goals / Non-Goals

**Goals:**
- 实现三栏恒显布局（左侧图标导航 + 中间会话列表 + 右侧聊天区）
- 实现 Dark Pro 深色主题（深石墨背景 + 霓虹蓝强调）
- 实现浅色主题作为第二主题，支持实时切换
- 主题偏好持久化到 localStorage

**Non-Goals:**
- 不改变 React 组件的业务逻辑（API 调用、SSE 流处理、状态管理不变）
- 不引入 UI 组件库（保持纯 CSS 变量方案）
- 不修改任何 API 接口和数据结构
- 不改变 ChatView 内部的对话渲染逻辑（bubbles、tool blocks、reasoning 的 JSX 结构不变）

## Decisions

### 1. HTML 结构变更（App.tsx）

将 `div.app` 从 `flex-row` 两栏改为三栏：

```
div.app (flex-row, height:100vh)
  ├── nav (60px, flex-col, icons)
  ├── sessions-panel (240px, flex-col)
  └── main (flex:1, ChatView)
```

**变更文件**: `web/src/App.tsx`

### 2. CSS 变量主题体系

两套主题通过 `[data-theme]` 属性切换：

```css
:root { /* Dark Pro — 默认 */ }
:root[data-theme="light"] { /* 浅色反转 */ }
:root[data-theme="dark"] { /* 显式深色（默认）*/ }
```

**色值对比**:

| Token | Dark | Light |
|---|---|---|
| --bg | #0d1117 | #ffffff |
| --bg-elevated | #161b22 | #ffffff |
| --bg-sidebar | #010409 | #f6f8fa |
| --fg | #e6edf3 | #1f2328 |
| --muted | #8b949e | #656d76 |
| --border | #21262d | #d0d7de |
| --primary | #58a6ff | #0969da |
| --primary-glow | rgba(88,166,255,0.15) | rgba(9,105,218,0.1) |

### 3. 主题切换实现

**Context**: `web/src/contexts/ThemeContext.tsx`
```tsx
const ThemeContext = createContext<{theme: 'dark'|'light', toggle: () => void}>(...)
// App.tsx: <html data-theme={theme}> 顶层设置
// localStorage key: 'app-theme'
```

**切换入口**: 左侧导航栏底部，"🎨"图标，点击弹出两个选项（暗色/浅色）

### 4. 布局 CSS 类名保留

现有 `.sessions-panel`、`.sidebar`、`.main` 等类名保留，仅修改 CSS 属性值。ChatView 内部布局不变。

**变更文件**: `web/src/styles.css`（重写整个文件，加入 CSS 变量主题体系）

### 5. 圆角与阴影

- 按钮/输入框: border-radius: 8px
- 会话行/面板: border-radius: 10~12px
- 消息气泡: border-radius: 14px，4px clip corner
- 导航图标: border-radius: 10px
- 阴影: box-shadow 配合 rgba(0,0,0,0.3~0.5) 深色系，浅色主题用 rgba(0,0,0,0.1~0.15)

## Risks / Trade-offs

- **[Risk]** 左侧图标导航空间小，无文字导航学习成本高 → [Mitigation] hover tooltip 显示完整文字；当前已有 emoji 文字混用可作为过渡
- **[Risk]** 深色 + 浅色两套主题维护成本 → [Mitigation] CSS 变量隔离，仅变量值不同，无重复样式

## Migration Plan

1. 修改 `App.tsx` HTML 结构（三栏）
2. 重写 `styles.css`（完整 CSS 变量主题体系）
3. 创建 `ThemeContext.tsx`（主题状态 + localStorage）
4. 在 `App.tsx` 集成 ThemeContext
5. 浏览器验证 Dark/Light 主题 + 三栏布局
6. 验证 Chat/Clusters/Policies 三视图均正常
7. Playwright 截图回归

无回滚顾虑：`git revert` 即可恢复。
