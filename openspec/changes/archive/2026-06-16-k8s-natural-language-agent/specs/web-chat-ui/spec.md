# Spec: web-chat-ui

## ADDED Requirements

### Requirement: 三视图导航
Web UI MUST 提供三个主视图,通过左侧边栏切换:`chat`(主对话视图,默认)· `clusters`(集群管理)· `policies`(策略编辑)。

#### Scenario: 切换视图
- **WHEN** 用户在侧边栏点击任一视图入口
- **THEN** 主区域 MUST 切换为对应视图,且 MUST 保留当前会话上下文(cluster 选中状态、当前 session 切换不在此范围)

#### Scenario: 浏览器直接访问
- **WHEN** 用户首次访问 `http://127.0.0.1:8080`
- **THEN** 系统 MUST 默认渲染 `chat` 视图,侧边栏 MUST 高亮 "chat" 入口

### Requirement: 主对话视图布局
主对话视图 MUST 采用以下布局:左侧会话历史列表(顶部"新建会话"按钮)· 中部对话区(顶部 cluster + session 切换)· 中部消息流 · 底部输入框(带"停止"和"发送"按钮)。

#### Scenario: 渲染消息流
- **WHEN** 当前 session 有 N 条消息
- **THEN** 对话区 MUST 按时间顺序渲染每条消息,user 消息右对齐,assistant 消息左对齐,工具行默认折叠为单行

#### Scenario: 输入框禁用
- **WHEN** LLM 正在响应或 Plan 预览阻塞中
- **THEN** 输入框 MUST 禁用,placeholder 提示"等待当前操作完成"

### Requirement: 五个 UI 状态
前端 MUST 实现以下状态及状态间切换:对话态 · Plan 预览态(阻塞模态)· ask_user 表单态 · 错误 toast 态 · 工具执行态。

#### Scenario: Plan 预览阻塞模态
- **WHEN** 后端推 `plan_awaiting_confirm` 事件
- **THEN** 前端 MUST 渲染一个居中模态,展示 plan summary + diffs(含风险等级 emoji 与颜色) + "取消"与"确认执行"按钮,输入框 MUST 禁用

#### Scenario: 用户确认 plan
- **WHEN** 用户在 Plan 模态点击"确认执行"
- **THEN** 前端 MUST 生成 `confirm_token`(含 session_id + timestamp + 客户端随机数)并调 `k8s_execute_plan` 工具,模态 MUST 关闭

#### Scenario: 用户取消 plan
- **WHEN** 用户在 Plan 模态点击"取消"
- **THEN** 前端 MUST 推 `cancelled` 事件,模态 MUST 关闭,输入框 MUST 恢复可用

#### Scenario: ask_user 表单
- **WHEN** 后端推 `ask_user` 事件
- **THEN** 前端 MUST 渲染为表单:单选渲染为单选按钮组,多选渲染为复选框组,纯文本渲染为文本框,用户提交后 MUST 作为工具结果回传

#### Scenario: 错误 toast
- **WHEN** 后端推 `error` 事件
- **THEN** 前端 MUST 在顶部展示红色 toast 持续 5 秒,含错误 message,点击 MUST 展开详情

### Requirement: 风险等级视觉化
每个 plan diff 的风险等级 MUST 用 emoji + 颜色双重信号展示:`low` = 🟢 绿 · `medium` = 🟡 琥珀 · `high` = 🔴 红。

#### Scenario: 高风险 diff
- **WHEN** plan 中某 diff.risk = "high"
- **THEN** 前端 MUST 在该 diff 标题前显示 🔴,且 MUST 在模态底部用红色文字提示"高风险操作,请确认"

### Requirement: 折叠展示
- reasoning 块 MUST 默认折叠为"▶ 思考过程(X 秒)",用户点击 MUST 展开
- 工具行 MUST 默认折叠为"🔧 工具名(...)",用户点击 MUST 展开 JSON 输入输出

#### Scenario: 折叠切换
- **WHEN** 用户点击折叠块
- **THEN** 该块 MUST 展开显示完整内容,再次点击 MUST 折叠

### Requirement: SSE 客户端
前端 MUST 使用支持 POST 请求体与自定义 header 的 SSE 客户端(`fetch` + `ReadableStream`)调用 `/api/chat` 接收流;MVP 范围内 MUST NOT 实现自动重连(用户可手动重新发送消息)。

#### Scenario: 正常流接收
- **WHEN** 用户发送消息,后端开始推 SSE 事件
- **THEN** 前端 MUST 按事件类型分别渲染(`token` → 增量文本,`tool_call` / `tool_result` → 工具行,`plan_ready` → 渲染 Plan 模态,`error` → 错误 toast 等),且 MUST 正确处理 `data:` 行的多行 JSON 解析

#### Scenario: 断线后无自动重连
- **WHEN** SSE 连接意外断开
- **THEN** 前端 MUST 在错误 toast 中提示"连接已断开,请重新发送消息",且 MUST NOT 自动尝试重连

> **MVP 偏差**:早期 proposal 提到浏览器原生 `EventSource` 与 `last_event_id` 自动重连。但 `/api/chat` 是 POST 端点(需 body + 自定义 header),`EventSource` 仅支持 GET;`Last-Event-ID` 续传会增加后端事件存储与按需重放的复杂度。MVP 选择不做自动重连,符合 `design.md` R3 的权衡。后续 change 可加入事件持久化 + 重连。

### Requirement: 单二进制嵌入
前端构建产物 MUST 经 Go `embed.FS` 嵌入后端二进制,服务 MUST 仅暴露一个 HTTP 端口(默认 8080),且 MUST NOT 需要额外静态文件服务。

#### Scenario: 单进程启动
- **WHEN** 用户执行 `./kubernetes-agent`
- **THEN** 浏览器访问 `http://127.0.0.1:8080` MUST 加载完整 UI,MUST NOT 出现 404 或跨域错误

#### Scenario: 开发模式
- **WHEN** 开发者分别启动 Vite (`pnpm dev`,默认端口 5173) 与 Go 后端 (`make run`,默认 8080)
- **THEN** Vite MUST 通过其 `server.proxy` 配置将 `/api` 请求代理到 `http://127.0.0.1:8080`,前端 MUST 直接访问 `http://localhost:5173` 即可获得完整 UI 与后端 API,无需配置任何环境变量

> **MVP 偏差**:早期 proposal 提到 `KUBERNETES_AGENT_DEV=1` 触发后端代理 Vite。但 Vite 自带 `server.proxy` 是前端反向代理到后端的更轻量做法,且无需后端代码感知 dev 模式。MVP 选择"Vite 代理到后端"的流向,后端代码不读 `KUBERNETES_AGENT_DEV` 环境变量。详见 [`docs/dev-mode.md`](../../../../docs/dev-mode.md)。

### Requirement: 集群切换
主对话视图 MUST 显示当前 cluster 名称(下拉选择),用户切换 cluster MUST 影响后续所有 K8s 工具调用。

#### Scenario: 切换 cluster
- **WHEN** 用户在主对话视图切换 cluster
- **THEN** 后续 LLM 工具调用 MUST 使用新 cluster 的凭证,UI MUST 立即高亮新 cluster
