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
前端 MUST 使用浏览器原生 `EventSource` 接收 SSE 流,且 MUST 支持自动重连(`session_id` + `last_event_id` 续传)。

#### Scenario: 断线重连
- **WHEN** SSE 连接意外断开
- **THEN** 前端 MUST 自动用 `last_event_id` 重新连接 `/api/chat`,后端 MUST 从该事件 ID 后续事件续传,且 UI 顶部 MUST 显示"重连中..."状态条

#### Scenario: 重连后不可重放已执行 plan
- **WHEN** 重连时 plan 已进入执行阶段
- **THEN** 后端 MUST NOT 重放 `plan_awaiting_confirm` 事件,前端 MUST 等待下一次用户消息

### Requirement: 单二进制嵌入
前端构建产物 MUST 经 Go `embed.FS` 嵌入后端二进制,服务 MUST 仅暴露一个 HTTP 端口(默认 8080),且 MUST NOT 需要额外静态文件服务。

#### Scenario: 单进程启动
- **WHEN** 用户执行 `./kubernetes-agent`
- **THEN** 浏览器访问 `http://127.0.0.1:8080` MUST 加载完整 UI,MUST NOT 出现 404 或跨域错误

#### Scenario: 开发模式
- **WHEN** `KUBERNETES_AGENT_DEV=1` 环境变量存在
- **THEN** 后端 MUST 代理前端 Vite dev server(`http://localhost:5173`),允许前后端独立开发

### Requirement: 集群切换
主对话视图 MUST 显示当前 cluster 名称(下拉选择),用户切换 cluster MUST 影响后续所有 K8s 工具调用。

#### Scenario: 切换 cluster
- **WHEN** 用户在主对话视图切换 cluster
- **THEN** 后续 LLM 工具调用 MUST 使用新 cluster 的凭证,UI MUST 立即高亮新 cluster
