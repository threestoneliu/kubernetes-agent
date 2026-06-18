# session-bulk-clear Specification

## Purpose
TBD - created by archiving change session-management. Update Purpose after archive.
## Requirements
### Requirement: 一键清空全部
Web UI MUST 在会话面板底部提供"清空全部"按钮,点击 MUST 弹出确认模态,模态 MUST 显示即将删除的会话数量("确认删除全部 N 个会话?这不可恢复");用户确认后 MUST 调 `DELETE /api/sessions` 清空数据库;取消则关闭模态。

#### Scenario: 确认后清空成功
- **WHEN** 用户在确认模态点"确认清空"
- **THEN** UI MUST 发送 DELETE /api/sessions;200 OK + `{deleted: N}` 后 MUST 清空本地列表,若有 active session MUST 退出 active 状态

#### Scenario: 清空前显示数量
- **WHEN** 确认模态打开时数据库有 17 条 session
- **THEN** 模态文案 MUST 显示 "确认删除全部 17 个会话?这不可恢复"

### Requirement: bulk 接口契约
后端 `DELETE /api/sessions` MUST:删除全部 session 行(ON DELETE CASCADE 同步清 messages/plans/audit);返回 `{deleted: <int>}` 表示实际删除数量;无 active session 检查(任何状态都允许清空,但前端应在 idle 时才展示按钮)。

#### Scenario: 删除全部
- **WHEN** DELETE /api/sessions 在数据库有 17 条 session 时被调用
- **THEN** MUST 删除全部 17 行及关联 messages/plans/audit;返回 `{"deleted": 17}`

#### Scenario: 空库调用
- **WHEN** DELETE /api/sessions 在数据库为空时调用
- **THEN** MUST 返回 200 + `{"deleted": 0}`,不报错

### Requirement: 清空按钮的 active 状态保护
前端 MUST 在 `ui.kind === 'streaming'` 时把"清空全部"按钮 disabled(任何 session 都可能在 active 流);用户必须先停止当前流才能清空。

#### Scenario: 活跃时禁用
- **WHEN** 当前会话的 ui.kind === 'streaming'
- **THEN** "清空全部"按钮 MUST 显示为 disabled 且 MUST 不响应点击;其他会话操作不受影响

#### Scenario: idle 时可用
- **WHEN** ui.kind === 'idle' 且 sessions 列表非空
- **THEN** "清空全部"按钮 MUST 可点击

