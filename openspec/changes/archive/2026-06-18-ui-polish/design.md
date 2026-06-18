## Context

当前 UI 使用极简纯 CSS：白底 + 蓝按钮 + 系统默认字体，缺乏视觉层次感和精致度。

CSS 变量仅覆盖基础色（白/灰/蓝），无阴影体系、圆角不统一、hover/focus 状态粗糙。

受众：运维/开发人员日常使用 Kubernetes Agent 的工具型界面，需要专业但不冰冷的视觉语言。

## Goals / Non-Goals

**Goals:**
- 整体视觉升级为 **Dark Pro + Spacious** 风格（深石墨背景 + 霓虹蓝强调）
- 覆盖所有 UI 维度：配色、阴影、边框/圆角、间距、字体层级、按钮、输入框、对话气泡、侧边栏、弹窗
- 建立完整的 CSS 变量体系，便于后续维护和主题调整
- 保持 React 组件逻辑不变，仅修改样式层

**Non-Goals:**
- 不引入任何 UI 组件库（如 Material UI、Ant Design）
- 不改变当前布局结构（App.tsx 的 sidebar + main 两栏不变）
- 不添加新的交互动画（仅保留必要的过渡效果如 hover/focus）
- 不修改任何 API 接口和数据结构

## Decisions

### 1. CSS 变量体系重建

将 `web/src/styles.css` 的 `:root` 变量全面替换为 Dark Pro 配色：

```css
:root {
  /* Backgrounds */
  --bg:          #1a1a2e;   /* 深石墨页面背景 */
  --card-bg:     #252545;   /* 卡片/气泡背景 */
  --panel-bg:    #1e1e38;   /* 面板/侧边栏背景 */
  --sidebar-bg:  #16162a;   /* 侧边栏最深层 */
  --input-bg:    #252545;   /* 输入框背景 */

  /* Text */
  --fg:          #e8e8f0;   /* 主文字 */
  --fg-title:    #f0f0ff;   /* 标题/高亮文字 */
  --muted:       #7070a0;   /* 次要文字 */

  /* Borders */
  --border:      #2a2a4e;   /* 普通边框 */
  --border-soft: #3a3a6a;   /* 稍亮的边框 */

  /* Accent */
  --primary:     #00D4FF;   /* 霓虹蓝主强调 */
  --primary-dark: #0088CC;  /* 渐变深端 */
  --primary-glow: rgba(0, 212, 255, 0.2); /* glow 效果 */

  /* Functional */
  --danger:      #ff6b6b;
  --ok-bg:       #00D4FF18;
  --err-bg:     #ff6b6b18;

  /* Shadows */
  --shadow-sm:  0 2px 8px rgba(0,0,0,0.3);
  --shadow-md:  0 4px 16px rgba(0,0,0,0.3);
  --shadow-lg:  0 8px 40px rgba(0,0,0,0.4);
  --shadow-glow: 0 0 0 3px var(--primary-glow);
}
```

**为什么不用 Tailwind**：当前项目无构建工具扩展，引入 Tailwind 需改构建配置。纯 CSS 变量在现有 Vite + PostCSS 基础上零成本，且可完整控制设计体系。

### 2. 全局按钮渐变 + Glow

主按钮使用 `linear-gradient(135deg, #00D4FF, #0088CC)` + `box-shadow: 0 4px 16px rgba(0,212,255,0.3)`。

普通按钮保持纯色边框风格，但背景色在 hover 时微微提亮。

Focus 状态统一用 `--shadow-glow`（3px 霓虹蓝 glow）。

### 3. 对话气泡分级

| 类型 | 背景 | 边框 | 特殊处理 |
|---|---|---|---|
| user | 渐变 primary | 无 | 右下角 4px 尖角 |
| assistant | --card-bg | --border-soft | 左下角 4px 尖角 + shadow-sm |
| tool_call | --panel-bg | primary 半透明 | 文字 primary 色 |
| reasoning | --panel-bg | 左侧 3px primary | 斜体，区分于普通消息 |

### 4. 圆角体系

- `button / input / select`：8px
- `气泡 / 会话行`：12~16px
- `卡片 / 面板 / 弹窗`：12~14px

统一使用 8px 为基础单位，避免混用 4px/6px/8px。

### 5. SessionsPanel 面板

侧边栏内嵌的会话面板从白底改为 `--panel-bg`，集群标签从纯文字改为 pill 小标签（primary 背景 + 透明边框）。

### 6. 弹窗阴影强化

PlanModal / ConfirmModal 加入 `--shadow-lg`（8px 模糊），背景 `--card-bg`，边框 `--border`。边框从 1px solid rgba 白改为深色。

### 7. 保留 Markdown 渲染

Markdown 内容（assistant 消息内）保持原有样式，不强制覆盖——MarkdownRenderer 的 CSS 继承 `var(--fg)` 和 `var(--muted)` 自然适配深色。

## Risks / Trade-offs

- **[Risk]** 深色主题在强光下可读性下降 → [Mitigation] 文字色 #e8e8f0 足够亮，对比度 WCAG AA 以上
- **[Risk]** 用户可能习惯浅色界面 → [Mitigation] 提供 CSS 变量可快速切换主题，架构上已支持，未来可加 theme toggle
- **[Risk]** 修改 styles.css 影响所有视图（Chat/Cluster/Policy） → [Mitigation] 先在本地验证三个视图显示正常再提交

## Migration Plan

1. 备份 `web/src/styles.css`
2. 用新 Dark Pro 变量体系完整替换 `:root` 块
3. 添加 `.app` / `.sidebar` / `.main` / `.sessions-panel` 等已有 class 的新样式
4. 补充 `.bubble` 四种类型、`.btn` 状态、Modal 阴影
5. 浏览器验证 Chat / Clusters / Policies 三个视图
6. Playwright e2e 截图对比

无回滚顾虑——styles.css 是纯 CSS，可通过 git revert 瞬间恢复。

## Open Questions

无。所有关键决策已在 brainstorming 中确定。
