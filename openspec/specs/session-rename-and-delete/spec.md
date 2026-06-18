# session-rename-and-delete Specification

## Purpose
TBD - created by archiving change session-management. Update Purpose after archive.
## Requirements
### Requirement: 重命名会话
Web UI MUST 在每条会话项上提供重命名入口(hover 菜单的"重命名"或双击标题),触发 inline 编辑;用户提交新标题后 MUST 立即调 `PUT /api/sessions/{id}` 更新后端,刷新本地列表;用户取消或 ESC MUST 回退到原标题。

#### Scenario: 重命名成功
- **WHEN** 用户在编辑框内输入非空新标题并按 Enter(或失焦)
- **THEN** UI MUST 发送 PUT 请求,200 OK 后 MUST 立即用新标题替换列表中的旧标题,updated_at 重置为当前时间;网络错误 MUST 弹错误 toast 并保留旧标题

#### Scenario: 重命名为空
- **WHEN** 用户清空标题后提交
- **THEN** UI MUST 不发送请求,显示校验错误"标题不能为空"并保持编辑状态

#### Scenario: 取消编辑
- **WHEN** 用户按 ESC 或点击编辑框外
- **THEN** UI MUST 取消编辑,恢复原标题显示,不发送任何请求

### Requirement: 删除单条会话(带确认)
Web UI MUST 在每条会话项的 hover 菜单提供"删除"入口,点击后 MUST 弹出确认模态("确认删除 <title>? 这不可恢复");用户确认后 MUST 调 `DELETE /api/sessions/{id}`,成功后从列表移除;取消则关闭模态不做任何操作。

#### Scenario: 删除确认后成功
- **WHEN** 用户在确认模态点"确认删除"
- **THEN** UI MUST 发送 DELETE 请求;200 OK 后 MUST 从列表中移除该项;若该项是 active session,MUST 退出 active 状态并显示空对话区

#### Scenario: 删除时网络失败
- **WHEN** 用户确认后 DELETE 请求失败(非 200/非 409)
- **THEN** UI MUST 弹错误 toast,关闭模态,列表保持不变

### Requirement: 活跃 streaming session 不允许删除
后端 MUST 在 DELETE handler 检查目标 session 是否在 agent 活跃 Session map 中;若在,MUST 返回 409 + `code: session_active`;前端 MUST 在活跃 session 的 hover 菜单里把"删除"按钮 disabled 并附 tooltip"请先停止当前会话"。

#### Scenario: 后端拒绝删除活跃 session
- **WHEN** 前端对 ui.kind === 'streaming' 的 active session 发送 DELETE
- **THEN** 后端 MUST 返回 409 + JSON `{"code":"session_active","message":"..."}`;前端 MUST 弹错误 toast "请先停止当前会话",列表保持不变

#### Scenario: 前端禁用按钮
- **WHEN** active session 的 ui.kind === 'streaming'
- **THEN** 该 session 项的 hover 菜单里"删除"按钮 MUST 显示为 disabled 且 MUST 不响应点击;其他 session 的删除按钮不受影响

### Requirement: 活跃 session 删除接口契约
后端 `DELETE /api/sessions/{id}` MUST 在 4 种情况下分别返回:200 + `{deleted: 1}`(成功)/ 404 + `code: not_found`(session 不存在)/ 409 + `code: session_active`(活跃中)/ 500 + `code: internal`(数据库错误)。

#### Scenario: 不存在的 session
- **WHEN** DELETE 收到不存在的 id
- **THEN** MUST 返回 404 + `code: not_found`

