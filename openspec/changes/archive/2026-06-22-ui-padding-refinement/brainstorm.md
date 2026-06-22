## Design Summary

采用方案 A（留白呼吸）优化 UI 布局，针对当前页面元素紧贴边框、缺乏呼吸感的问题，在保持暗色主题不变的前提下，通过增加内边距和微调圆角来改善视觉体验。

**问题诊断：** 当前所有容器 padding 为 0，元素（toolbar、chat-stream、sessions-panel）紧贴边框和边缘，视觉上拥挤不堪。

**方案 A 核心理念：** 统一增加内边距，让每个区域有适度的呼吸空间，但不改变整体布局结构（仍为 header + sessions-panel + main 三栏）。

## Alternatives Considered

### 方案 B：卡片浮层
- **做法**：sessions-panel 和 chat-stream 变成独立悬浮卡片，加 `box-shadow: 0 4px 24px`，背景透出底色
- **优点**：层次分明，视觉质感最强
- **缺点**：改动量中等，需要修改 App.tsx 结构添加额外 div 包裹，阴影样式需要调优
- **为何未采用**：超出本次"仅优化间距"的范围，改动幅度大于实际需要

### 方案 C：极简白
- **做法**：切换默认主题为浅色，20px+ generous spacing，纯白卡片底
- **优点**：风格转变大，轻盈通透感最强
- **缺点**：涉及主题切换（data-theme），超出 CSS 微调范畴，需要更多测试
- **为何未采用**：用户明确要求保持暗色主题不变

## Agreed Approach

**方案 A：留白呼吸** — 在现有暗色主题基础上，通过 CSS 内边距调整让各区域拥有呼吸空间。

具体改动（仅 `web/src/styles.css`）：

| CSS 选择器 | 改动 |
|-----------|------|
| `.header-bar` | `padding: 0` → `padding: 0 20px` |
| `.sessions-panel` | `padding: 0` → `padding: 16px 12px`，底部 `panel-footer` `padding-top: 0` → `12px` |
| `.sessions-panel` 新增标题 | 顶部加 `.panel-title` "会话列表" 标签，12px 字号大写 |
| `.main` | `padding: 0` → `padding: 16px` |
| `.chat-stream` | 新增 `padding: 20px`，`border-radius: 12px` → `16px` |
| `.composer` | `padding: 0 4px` 保持（已存在） |

**选择理由：** 最小改动即能达到目标效果，改动局限于一个文件，无需调整 React 组件结构，不影响现有功能和交互逻辑。

## Key Decisions

- 保持 `--bg: #0d1117` 等暗色变量不变
- 圆角微调：chat-stream `12px` → `16px`，与 sessions-panel 视觉更协调
- sessions-panel 加"会话列表"标题，提升面板可识别性
- 底部 composer 区域间距保持轻微（4px），避免与 chat-stream 分离感过强

## Open Questions

（无 — 方案已明确，无悬而未决问题）
