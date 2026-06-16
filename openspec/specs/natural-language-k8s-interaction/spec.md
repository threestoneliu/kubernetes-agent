# natural-language-k8s-interaction Specification

## Purpose
TBD - created by archiving change k8s-natural-language-agent. Update Purpose after archive.
## Requirements
### Requirement: 自然语言消息入口
系统 MUST 在 Web UI 暴露一个文本输入框,用户输入自然语言后 MUST 触发与 LLM 的新回合,并 MUST 通过 SSE 流式回传响应。

#### Scenario: 用户发送首条消息
- **WHEN** 用户在主对话视图输入框输入文本并提交
- **THEN** 系统 MUST 立即建立 SSE 连接(`/api/chat`),将消息作为 `role: user` 加入会话,并 MUST 在 SSE 流上开始回传 LLM 响应事件

#### Scenario: 用户在 LLM 响应中再发消息
- **WHEN** LLM 仍在流式输出时用户再次提交消息
- **THEN** 系统 MUST 拒绝该消息并 MUST 在 UI 提示"请等待当前回合完成或点击停止",且 MUST NOT 取消当前回合

### Requirement: SSE 事件流契约
后端 MUST 通过 SSE 推送以下事件类型,且前端 MUST 按事件类型分别渲染:`session_meta`、`reasoning`、`token`、`tool_call`、`tool_result`、`plan_ready`、`plan_awaiting_confirm`、`ask_user`、`cluster_switch`、`cancelled`、`error`、`message_end`。

#### Scenario: LLM 工具调用透明展示
- **WHEN** agent 循环调用任一 K8s 工具
- **THEN** 系统 MUST 先推 `tool_call` 事件(包含工具名、输入),工具完成后 MUST 推 `tool_result` 事件(包含结果或错误),且前端 MUST 折叠默认显示为单行"🔧 工具名(...)"

#### Scenario: reasoning 块折叠
- **WHEN** LLM 返回 extended thinking 内容
- **THEN** 系统 MUST 推 `reasoning` 事件,前端 MUST 默认折叠为"▶ 思考过程"块,用户点击 MUST 展开

#### Scenario: 错误事件
- **WHEN** 后端发生可恢复错误(LLM 5xx、限流等)
- **THEN** 系统 MUST 推 `error` 事件含 `code` + `message` + `retryable`,前端 MUST 在顶部展示红色 toast

### Requirement: 消息与会话持久化
每条消息 MUST 在 `message_end` 事件触发时一次性落 SQLite,且 MUST 可通过 `GET /api/sessions/{id}/messages` 完整回放。

#### Scenario: 流式结束后入库
- **WHEN** LLM 流式响应结束
- **THEN** 系统 MUST 把累积的 token、reasoning、tool_calls 序列化为一条 `messages` 记录写入,期间 MUST NOT 每个 token 写一次库

#### Scenario: 会话重放
- **WHEN** 用户在侧边栏选择历史会话
- **THEN** 系统 MUST 从 SQLite 加载该会话的所有 `messages` 记录,前端 MUST 按时间顺序渲染,每条消息 MUST 保留原 reasoning 与 tool_calls

### Requirement: ask_user 主动澄清
agent 循环 MUST 暴露 `ask_user` 工具,LLM 可在信息不足时调用,前端 MUST 渲染为表单(单选/多选/文本),用户答复后 MUST 回流到 agent 循环继续。

#### Scenario: LLM 主动询问目标 namespace
- **WHEN** LLM 决定询问"目标 namespace?"
- **THEN** 系统 MUST 推 `ask_user` 事件含 `question` + `options` + `multiSelect`,前端 MUST 渲染为单选/多选表单,用户选择后 MUST 作为工具结果回传 LLM,LLM MUST 继续生成

#### Scenario: 用户取消 ask_user
- **WHEN** ask_user 表单渲染中用户点击取消
- **THEN** 系统 MUST 推 `cancelled` 事件,agent 循环 MUST 终止当前回合,前端 MUST 回到普通对话态

### Requirement: 历史截断
当会话历史 token 数超过所选 LLM provider context window 的 80% 时,系统 MUST 丢弃最旧的非 system 消息,保留 system prompt 与最近 5 轮对话。

#### Scenario: 接近 context 上限
- **WHEN** 历史消息累计 token 数 ≥ context window × 0.8
- **THEN** 系统 MUST 从最早的非 system 消息开始丢弃,直到剩余 token < 0.7 × context window,期间 MUST NOT 丢弃 system prompt 或最近 5 轮对话

### Requirement: 用户中断生成
用户在 LLM 响应中 MUST 能点击"停止"按钮,系统 MUST 立即中断当前回合并推 `cancelled` 事件。

#### Scenario: 流式期间停止
- **WHEN** LLM 仍在流式输出时用户点击停止
- **THEN** 系统 MUST 取消 agent 循环,关闭 SSE 连接,推 `cancelled` 事件,前端 MUST 清理加载态并保留已收到的内容

