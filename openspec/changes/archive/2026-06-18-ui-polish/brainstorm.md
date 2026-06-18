# Design Summary

Collaboratively designed via Visual Companion brainstorming session.

## Visual Direction

**Dark Pro + Spacious** — 深石墨背景(#1a1a2e) + 亮白文字 + 霓虹蓝(#00D4FF)强调 + 系统原生字体 + 宽松留白。

科技感强、层次清晰、长时间使用不刺眼，同时有"眼前一亮"的视觉冲击力。

## Alternatives Considered

### 方案 A：现代深色主题 (Modern Dark)
- **做法**：深灰背景 + 霓虹蓝/绿高亮，类似 VS Code / GitHub Dark
- **優點**：科技感强，长时间使用不刺眼
- **缺點**：与最终选择的方案接近但无渐变亮点
- **為何未採用**：与 D 方案本质相同，但 D 更精准

### 方案 B：精致轻量白 (Refined Light)
- **做法**：保留白底但大幅提升质感，柔和阴影、细腻边框
- **優點**：明亮干净
- **缺點**：用户明确表示"不够眼前一亮"
- **為何未採用**：用户反馈太"安全"，缺乏冲击力

### 方案 C：温暖科技风 (Warm Tech)
- **做法**：暖白背景 + 琥珀色主强调，类似 Linear / Notion
- **優點**：有温度，专业但有亲和力
- **缺點**：用户同样反馈"不够眼前一亮"
- **為何未採用**：同上

## Agreed Approach

**Dark Pro + Spacious** (方案 D 变体)

具体规格：
- 背景色：#1a1a2e（深石墨）
- 卡片/面板背景：#252545 / #1e1e38
- 侧边栏背景：#16162a
- 主文字：#e8e8f0 / #f0f0ff（标题加亮）
- 次文字：#7070a0 / #6060a0
- 主强调色：#00D4FF（霓虹蓝）
- 边框：#2a2a4e
- Hover 状态：#ffffff0a ~ #ffffff18
- 按钮渐变：linear-gradient(135deg, #00D4FF, #0088CC)
- 圆角：8px（按钮/输入框）、12px（卡片）、14px（面板/弹窗）
- 阴影：box-shadow 配合 rgba(0,0,0,0.3~0.4) 表现深度层次
- 字体：系统原生 (-apple-system, BlinkMacSystemFont, Segoe UI, PingFang SC)
- 信息密度：宽松舒适，大量留白

## Key Decisions

1. **深色主题为主**：不是纯黑(#000)而是深石墨(#1a1a2e)，减少刺眼感
2. **霓虹蓝渐变按钮**：#00D4FF → #0088CC 渐变 + glow box-shadow，关键交互元素用渐变
3. **Reasoning 区块特殊处理**：左侧霓虹蓝边框 + 斜体 + 次文字色，与普通 assistant 消息区分
4. **工具调用区块**：霓虹蓝文字 + 边框，高亮但不喧宾夺主
5. **SessionsPanel 集群标签**：小号 pill，霓虹蓝背景 + 透明边框
6. **Focus 状态**：霓虹蓝 box-shadow glow，3px 扩展区域
7. **所有组件统一圆角**：8px 起，避免直角带来的生硬感

## Open Questions

无。所有关键设计决策已在 brainstorming 中确认。
