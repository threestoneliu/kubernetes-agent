# Spec: session-list-and-select

## ADDED Requirements

### Requirement: 会话列表展示
Web UI MUST 在对话视图内左侧(默认 300px 宽,可折叠到 0)展示一个"会话列表"面板,内容为按默认排序(updated_at desc)排列的最多 100 条历史会话。每条 MUST 显示标题、所属 cluster 名称、相对更新时间(例:3 分钟前 / 1 天前)。

#### Scenario: 列出已有会话
- **WHEN** 用户打开对话视图且数据库中存在 ≥ 1 条历史会话
- **THEN** 左侧面板 MUST 按 updated_at 降序显示这些会话;当前 active 会话 MUST 高亮(背景色或左侧标记);每条至少包含 title + cluster name + 相对时间三个字段

#### Scenario: 列表为空
- **WHEN** 数据库中没有历史会话
- **THEN** 左侧面板 MUST 显示空状态文案(例:"暂无历史会话,点击新建开始对话")

### Requirement: 点选切换会话
Web UI MUST 在用户点击某条会话列表项时,把 active 会话切换为该项对应的 session_id,加载并渲染该 session 的历史消息流,且不丢失当前未发送的输入草稿。

#### Scenario: 切换到有历史的会话
- **WHEN** 用户点击一条非 active 的会话项
- **THEN** 对话区 MUST 清空并渲染该 session 的全部历史消息(user / assistant / system);active 标记 MUST 移动到该项;顶栏的 cluster 下拉 MUST 同步为该 session 的 cluster_id

#### Scenario: 切换时保留草稿
- **WHEN** 用户在 active session A 的输入框有未发送文本,点击切换到 session B,之后再切回 A
- **THEN** A 的输入框 MUST 显示切走前未发送的文本;B 切换时 MUST 显示其自身的草稿(若有);空草稿 MUST 显示空字符串

### Requirement: 折叠面板
Web UI MUST 提供一个折叠按钮,点击后把左侧会话面板收起(宽度 0px),主对话区占满整个宽度;再次点击恢复 300px 宽。

#### Scenario: 折叠状态切换
- **WHEN** 用户点击折叠按钮
- **THEN** 左侧面板宽度 MUST 在 0px 与 300px 之间切换;折叠期间 MUST 仍可通过按钮展开;切换过程中 MUST 保留 sessions 列表数据(不重新 fetch)