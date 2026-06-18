## Why

当前 UI 过于简陋：白底 + 蓝按钮 + 基础 CSS 变量，无阴影体系、无层次感、无品牌调性。用户明确表示"不够眼前一亮"。运维/开发人员日常使用的工具界面需要专业但不冰冷的视觉语言。Dark Pro 主题（深石墨背景 + 霓虹蓝强调）在科技感与易用性之间取得平衡，同时具备视觉冲击力。

## What Changes

**styles.css 全局样式重设计**

- From: 白底 #ffffff + 浅灰边框 + #2563eb 蓝按钮 + 无阴影
- To: 深石墨背景 #1a1a2e + 霓虹蓝 #00D4FF 渐变按钮 + 多层阴影体系 + 统一圆角
- Reason: 建立完整的视觉设计体系，让界面从"能看"提升到"想用"
- Impact: 非破坏性——仅修改 `web/src/styles.css`，React 组件逻辑不变，三个视图自动应用新样式

**会话气泡分级重设计**

- From: 用户青色 #cffafe / 助手白底无边框 / 工具绿/红小字
- To: 用户渐变蓝气泡 / 助手深灰卡片 + 边框 + 阴影 / 工具霓虹蓝文字 / reasoning 左侧蓝色边框斜体
- Reason: 视觉层次更清晰，消息类型一目了然

**侧边栏和面板重设计**

- From: 透明背景 + 基础 hover
- To: #16162a 深色 sidebar + #1e1e38 面板背景 + hover 微亮 + 集群标签 pill 化

**弹窗重设计**

- From: 白色背景 + 基础阴影
- To: #252545 背景 + #2a2a4e 边框 + --shadow-lg 深层阴影

## Capabilities

### New Capabilities

- `dark-pro-theme`: 完整视觉主题系统，涵盖 Dark Pro 配色体系（6 个背景色阶、3 个文字色阶、渐变按钮、glow 效果、阴影层级、圆角体系、气泡分级样式、focus 状态）。作为 `web-chat-ui` 的视觉扩展，不改变任何功能需求。

### Modified Capabilities

（无——本变更仅修改视觉层，`web-chat-ui` spec 中的功能需求（路由、布局、SSE、模态交互）保持不变）

## Impact

- **影响文件**: 仅 `web/src/styles.css`（CSS 变量 + 全局样式，覆盖 App/ChatView/ClusterView/PolicyView 所有视图）
- **依赖项**: 无新依赖（已有 marked + DOMPurify）
- **构建产物**: 无变化
- **API**: 无变化
- **向后兼容**: CSS 类名保持不变，现有组件零改动自动应用新样式
