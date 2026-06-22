# Proposal — UI Padding Refinement

## What

优化 Kubernetes Agent Web UI 的内边距布局，将当前紧贴边框的元素（header、sessions-panel、chat-stream、composer）通过增加统一 padding 来改善视觉呼吸感，同时为 sessions-panel 顶部添加"会话列表"标题。

## Why

当前 UI 所有容器 padding 设为 0，toolbar、chat-stream、sessions-panel 直接堆叠在边框上，视觉拥挤。方案 A（留白呼吸）以最小改动（仅改 CSS）立即改善这一问题。

## What Changes

**纯 CSS 调整，无逻辑变更：**
- `.header-bar`: 左右加 20px padding
- `.sessions-panel`: 加 16px 上下、12px 左右 padding；底部加 12px padding
- `.sessions-panel` 新增 `.panel-title` "会话列表" 标题
- `.main`: 加 16px padding
- `.chat-stream`: 加 20px 内 padding，圆角 12px→16px
- `.composer`: 保持 0 4px padding（已有）

## Capabilities

- 用户在视觉上感到页面"透气"，元素有清晰边界
- sessions-panel 面板有明确标题标识
- 暗色主题保持一致，无突兀色块

## Impact

- 零破坏性：仅改 styles.css
- 零新依赖：纯 CSS
- 零测试变更：无功能逻辑变化
