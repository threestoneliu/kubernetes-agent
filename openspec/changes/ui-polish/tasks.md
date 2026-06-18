# Tasks · UI Polish (Dark Pro Theme)

## 1. CSS 变量体系重建

- [x] 1.1 备份现有 `web/src/styles.css`（git commit 前确保可回滚）
- [x] 1.2 替换 `:root` 变量块：背景色阶（--bg #1a1a2e, --card-bg #252545, --panel-bg #1e1e38, --sidebar-bg #16162a, --input-bg #252545）
- [x] 1.3 替换文字色阶（--fg #e8e8f0, --fg-title #f0f0ff, --muted #7070a0）
- [x] 1.4 替换边框色阶（--border #2a2a4e, --border-soft #3a3a6a）
- [x] 1.5 添加主强调色（--primary #00D4FF, --primary-dark #0088CC, --primary-glow rgba(0,212,255,0.2)）
- [x] 1.6 添加功能色（--danger #ff6b6b, --ok-bg rgba(0,212,255,0.12), --err-bg rgba(255,107,107,0.12)）
- [x] 1.7 添加阴影层级（--shadow-sm/md/lg, --shadow-glow）

## 2. 全局基础样式更新

- [x] 2.1 更新 `body / html / #root` 背景为 --bg
- [x] 2.2 更新全局文字颜色为 --fg
- [x] 2.3 更新 button 默认样式：圆角 8px，背景 --input-bg，边框 --border
- [x] 2.4 更新 button.primary：渐变背景 + glow shadow，文字白色
- [x] 2.5 更新 button hover：背景提亮（--card-bg）
- [x] 2.6 更新 button focus：--shadow-glow（3px neon ring）
- [x] 2.7 更新 button.danger：边框和文字 --danger
- [x] 2.8 更新 input/textarea/select：圆角 8px，背景 --input-bg，边框 --border
- [x] 2.9 更新 input focus：边框 --primary + --shadow-glow
- [x] 2.10 更新 textarea：字体 Menlo/Monaco/Consolas，12px

## 3. App 布局样式

- [x] 3.1 更新 .app：flex row，100vh，背景 --bg
- [x] 3.2 更新 .sidebar：背景 --sidebar-bg，宽 200px，右边框 --border
- [x] 3.3 更新 .sidebar h1：文字 --muted，字号 13px，字重 600，letter-spacing
- [x] 3.4 更新 .sidebar button：透明背景，文字 --muted；hover 时背景微亮；active 时 --primary 15% 透明 + 文字 --primary + 边框
- [x] 3.5 更新 .main：背景 --bg，内边距 16px，flex 1，overflow auto

## 4. 会话面板样式（SessionsPanel）

- [x] 4.1 更新 .sessions-panel：背景 --panel-bg，圆角 12px，边框 --border，内边距
- [x] 4.2 更新 .session-list/.session-row：透明背景，圆角 8px，padding
- [x] 4.3 更新 .session-row:hover：背景 #ffffff0a
- [x] 4.4 更新 .session-row.active：背景 --primary 12% 透明，边框 --primary 30% 透明
- [x] 4.5 更新 .title/.muted 文字颜色
- [x] 4.6 更新集群标签 .cluster-tag：pill 样式，--primary 15% 透明背景，--primary 文字，10px，4px 圆角
- [x] 4.7 更新 toolbar 按钮样式（新建按钮用 .primary 渐变）
- [x] 4.8 更新搜索 input 样式（focus glow）

## 5. 对话气泡样式

- [x] 5.1 更新 .bubble.user：渐变背景（135deg #00D4FF→#0088CC），白色文字，最大宽度 75%，圆角 16px，右下角 4px，内边距 12px 16px，flex-end
- [x] 5.2 更新 .bubble.assistant：背景 --card-bg，边框 --border-soft，阴影 --shadow-sm，左下角 4px
- [x] 5.3 新增 .bubble.tool（或 tool_ok/tool_err）：背景 --panel-bg，文字 --primary，边框 --primary 半透明，shadow-sm
- [x] 5.4 新增 .reasoning/reasoning-block：左侧 3px --primary 边框，背景 --panel-bg，斜体，文字 --muted，最大宽度 85%
- [x] 5.5 更新气泡内 markdown 代码块：背景 --panel-bg 或 --sidebar-bg，文字 --fg

## 6. 聊天区域样式（ChatView）

- [x] 6.1 更新 .chat-view 主容器：背景 --bg，flex column，flex 1
- [x] 6.2 更新 .chat-toolbar：下边框 --border，内边距，flex row 居中
- [x] 6.3 更新 .composer：上边框 --border，内边距，flex row
- [x] 6.4 更新 .composer input：圆角 12px，背景 --card-bg，边框 --border，padding 12px 16px，focus glow
- [x] 6.5 更新 .composer .send-btn（或 button.primary）：渐变背景，白色文字，圆角 12px，padding 12px 20px，glow shadow
- [x] 6.6 更新 .messages-area：flex 1，overflow-y auto，内边距

## 7. 弹窗样式（PlanModal / ConfirmModal）

- [x] 7.1 更新 .modal-overlay：背景 rgba(0,0,0,0.6) 遮罩
- [x] 7.2 更新 .modal/.modal-panel：背景 --card-bg，边框 --border，圆角 14px，阴影 --shadow-lg，内边距 24px，最大宽度 520px
- [x] 7.3 更新 .modal h3：文字 --fg-title，字重 600
- [x] 7.4 更新 .modal button.primary：渐变按钮 + glow
- [x] 7.5 更新 .modal button.cancel：背景 --panel-bg，边框 --border，文字 --muted

## 8. 集群和策略视图样式（ClusterView / PolicyView）

- [x] 8.1 更新 .list/.row：背景 --card-bg，边框 --border-soft
- [x] 8.2 更新 .row:hover：背景 --panel-bg
- [x] 8.3 更新输入框和 textarea（YAML editor）：背景 --input-bg，边框 --border，focus glow
- [x] 8.4 更新 form label：文字 --muted，字重 500
- [x] 8.5 更新 badge（allow/confirm/deny）：背景色保持，但文字在深色背景下可读性检查

## 9. 全局圆角体系

- [x] 9.1 button / input / select：border-radius: 8px
- [x] 9.2 .card / .bubble / .session-row：border-radius: 12px
- [x] 9.3 .sessions-panel / .modal / .panel：border-radius: 14px
- [x] 9.4 确保所有组件无 border-radius: 0

## 10. 视觉验证和微调

- [x] 10.1 在 Chrome（localhost:8080）截图 Chat 视图，确认深色背景 + 渐变按钮 + 气泡样式正确
- [x] 10.2 截图 Clusters 视图，检查列表和表单在深色背景下的可读性
- [x] 10.3 截图 Policies 视图，检查 YAML editor 和 badge 可读性
- [x] 10.4 测试 input focus glow 在键盘 Tab 导航下是否可见
- [x] 10.5 测试 session panel 切换 active 行高亮是否清晰
- [x] 10.6 测试 ConfirmModal 遮罩和阴影是否正常
- [x] 10.7 调整任何与设计稿不符的颜色或间距

## 11. 提交

- [ ] 11.1 git commit：feat(web): dark pro theme — styles.css complete redesign
- [ ] 11.2 推送 remote
