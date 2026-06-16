# K8s Natural Language Agent · Brainstorm

> 状态:brainstormed · 待 spec 自审 + 用户复核

## Design Summary

构建一个本地单机的 Kubernetes agent,用户通过 Web UI 用自然语言操作 K8s 集群。后端 Go 单二进制,前端 SPA 内嵌。LLM 走多 provider 抽象(charmbracelet/fantasy),K8s 工具通过 client-go dynamic client 暴露。写操作必须经过 server-side dry-run 的 Plan 预览 + 全局可配置护栏,UI 用户确认后才执行。所有 K8s 凭证 AES-GCM 加密落 SQLite,不出本机。

**MVP 范围(本 change)**:本地单机部署 · 读(3 工具)+ 写(2 工具:plan / execute)· Plan 预览 + 全局护栏 · 多 LLM provider · SSE 流式 · SQLite 持久化 · 6 张表(含 append-only 审计)。

**非目标(后续 change)**:多用户/多租户 · 多集群 · 多步事务 · 诊断 · Helm/CRD 解析 · 多档位详细度 · docker/安装包分发。

---

## Alternatives Considered

### 后端 + agent 循环(三个候选)

#### 方案 A: Go + kubectl exec + 自写 ReAct 循环
- **做法**:动态生成 `kubectl` 子进程命令作为工具,LLM 通过 Anthropic SDK 自写循环
- **优点**:依赖最少,工具集天然包含 kubectl 全能力(get/describe/apply/scale/delete/logs/exec)
- **缺点**:每次起子进程性能差;解析 stdout 需正则,边界 case 脆弱;错误处理难以统一
- **为何未采用**:与项目 .gitignore 暗示的 Go 路径虽然一致,但 kubectl 包装带来不可控性;护栏在 Go 层编码会更稳

#### 方案 B: Go + client-go dynamic client + charmbracelet/fantasy(已选)
- **做法**:Go 后端,client-go dynamic client 直接调 K8s API,工具集是 5 个通用工具,agent 循环由 fantasy 提供
- **优点**:Go 二进制部署轻;护栏在 Go 层编码(类型系统可强校验);与项目 .gitignore 暗示一致;fantasy 覆盖多 LLM provider + 工具调用 + 流式
- **缺点**:dynamic client 无类型安全,需手写 OpenAPI 解析;MVP 仅 5 个工具,后续可能需要更多
- **为何胜出**:与 fantasy + 用户指定的 client-go 决策一致;最契合"护栏在 Go 层"的硬需求;与 Go 单二进制部署路径匹配

#### 方案 C: Python + LangChain + kubernetes python client
- **做法**:Python + FastAPI + kubernetes python client + LangChain agent
- **优点**:LLM 生态最丰富;示例多
- **缺点**:与项目 .gitignore 暗示的 Go 路径背离;运行重;LangChain 抽象层较厚
- **为何未采用**:与既有 Go 路径冲突;部署形态与单二进制目标不符

### 前端栈(三个候选)

#### 方案 1: React + Vite + TypeScript(已选)
- **优点**:生态最大,招人/查资料最易;Tailwind 等配套成熟
- **缺点**:依赖较重;首屏需 SPA hydration
- **为何胜出**:K8s agent 的"聊天面板 + 集群选择 + Plan 弹窗"是典型多视图组合,React 组件库丰富能省 UI 工作

#### 方案 2: SvelteKit + TypeScript
- **优点**:包体小、启动快;代码量通常比 React 少 30-50%
- **缺点**:生态比 React 小;招人/接手难
- **为何未采用**:生态风险高于代码量收益

#### 方案 3: Go html/template + Alpine.js/hTMX
- **优点**:单语言/单仓库,部署最简,无构建步骤
- **缺点**:复杂交互(动态 plan 步骤、实时 diff 高亮)写起来繁琐
- **为何未采用**:Web UI 涉及多个交互状态(Plan 弹窗、ask_user 表单、工具折叠),纯后端渲染不划算

### 写授权机制(三个候选)

#### 方案 X: Plan 预览 + 一键确认
- **做法**:LLM 给干运行/可读 plan,用户点确认后执行
- **为何未采用(单独采用)**:缺少全局安全护栏,任何用户都能写 production 资源

#### 方案 Y: 编辑后的 plan 确认
- **做法**:用户可在确认前编辑 plan,只提交修改后的部分
- **为何未采用**:实现复杂度高(需要 plan diff 编辑器 + 二次校验);MVP 用户价值不明确

#### 方案 Z: 全局安全护栏 + Plan 预览(已选)
- **做法**:Plan 预览作为用户确认机制;全局可配置护栏在 Go 层强制(allow / confirm / deny 三态)
- **优点**:Plan 预览满足"用户明确授权"诉求;护栏在 Go 层不可被 LLM 绕过;两者互补
- **为何胜出**:用户明确希望"写操作通过 UI 让用户授权后执行";护栏是 Plan 预览的安全冗余

### K8s 客户端(两个候选)

#### 方案 p: Helm/CRD 解析器
- **做法**:支持 Helm chart 安装/升级/回滚,CRD 资源用 OpenAPI schema 解析
- **为何未采用**:用户明确"用 dynamic client 即可";MVP 范围控制

#### 方案 q: 仅 client-go dynamic client(已选)
- **做法**:所有 K8s 资源通过 dynamic client + JSON manifest 传递
- **优点**:实现量小,覆盖 K8s API 全资源
- **缺点**:无类型安全,LLM 需要提供完整 JSON manifest
- **为何胜出**:与用户决策一致;MVP 范围控制

---

## Agreed Approach

**本 change 交付一个本地单机的 K8s 自然语言 agent。**

- **形态**:Go 单二进制 + React SPA(embed.FS);用户本机起服务,浏览器访问 `http://127.0.0.1:8080`
- **核心机制**:LLM(多 provider 通过 fantasy)→ 5 个 K8s 工具(读 3 + 写 2)→ client-go dynamic client → K8s API
- **写安全**:Plan 预览(基于 server-side dry-run)+ 全局护栏(Go 层 policy engine,三态 allow/confirm/deny)
- **凭证**:AES-256-GCM 加密落 SQLite,master.key 来自环境变量或 `~/.kubernetes-agent/master.key`(0600)
- **通信**:SSE 事件流(`reasoning` / `token` / `tool_call` / `tool_result` / `plan_ready` / `plan_awaiting_confirm` / `ask_user` / `cluster_switch` / `session_meta` / `cancelled` / `error` / `message_end`)
- **持久化**:SQLite 6 张表(clusters / sessions / messages / plans / policies / audit_log),其中 audit_log append-only
- **MCP 风格 LLM 主动澄清**:`ask_user` 工具支持单选/多选/文本表单

---

## Key Decisions

### 范围
- 本 change 只做本地单机 + 读/写 + Plan/护栏,后续 change 再做多用户、多集群、多步事务、诊断、Helm、多档位
- LLM 模型支持范围:Anthropic Claude(含 extended thinking)、OpenAI GPT 系列、OpenAI 兼容(Ollama 等)
- K8s 客户端仅用 dynamic client,无 Helm/CRD 解析

### 技术栈(后端)
- 语言:Go(与 .gitignore 模板 + fantasy 一致)
- Agent 框架:`github.com/charmbracelet/fantasy`
- K8s 客户端:`k8s.io/client-go` dynamic client
- HTTP 框架:`go-chi/chi`(轻、显式、易测)
- 配置:(`gopkg.in/yaml.v3`)
- 日志:标准库 `log/slog`
- SQLite 驱动:`modernc.org/sqlite`(纯 Go,无 cgo)
- 加密:标准库 AES-256-GCM
- 测试:标准 `testing` + `testify/assert`

### 技术栈(前端)
- React + Vite + TypeScript
- Tailwind CSS(待定,实施阶段确认)
- 状态管理:React Context + 少量 Zustand(待定)
- SSE 客户端:原生 `EventSource`

### K8s 工具集(5 个)
- `k8s_get` · 读单个资源
- `k8s_list` · 列出 + label selector
- `k8s_describe` · 加 events + owner + diagnosis hints
- `k8s_plan_write` · 写操作预览(server-side dry-run,产出 plan_id + diffs + risk)
- `k8s_execute_plan` · 执行 plan(需 plan_id + confirm_token,二次护栏,失败回滚)

### 护栏系统
- 三态:`allow` / `confirm` / `deny`
- 匹配维度:action、namespace、kind、unsafeFields(简化 JSONPath)
- 双重评估:`k8s_plan_write` 一次,`k8s_execute_plan` 二次(防中途改 policy)
- 默认规则:禁删系统 NS、禁写 Node/ClusterRole/CRD、危险字段(privileged/hostNetwork/hostPID)deny、production NS 写操作 confirm
- 规则存 SQLite,UI 提供 YAML 文本编辑

### SSE 事件契约
12 个事件类型:`session_meta` / `reasoning` / `token` / `tool_call` / `tool_result` / `plan_ready` / `plan_awaiting_confirm` / `ask_user` / `cluster_switch` / `cancelled` / `error` / `message_end`

### 数据持久化
- 6 张表 + 2 索引(`messages` 按 session,`audit_log` 按 created_at)
- `plans` 表保留完整 ops_json 与 diffs_json,支持重放
- `audit_log` append-only,永不删
- 版本化迁移

### 配置与启动
- 配置:`~/.kubernetes-agent/config.yaml`,支持 `${ENV}` 展开
- 启动流程:解析 config → master.key 检查/生成 → SQLite 迁移 → 默认规则插入 → LLM ping → HTTP 启动
- 健康检查:`GET /healthz` → `{ ok, providers }`

### Web UI
- 3 个 MVP 视图:主对话、集群管理、策略编辑
- 5 个 UI 状态:普通对话、Plan 预览(阻塞模态)、ask_user 表单、错误 toast、工具执行中
- 视觉规则:风险用 🟢🟡🔴 emoji + 颜色;reasoning 默认折叠;工具行折叠;Plan 弹窗阻塞模态

### 错误处理
- 启动错误:进程退出 + 明确 stderr
- LLM 错误:401/403 不重试;429 重试 1 次(指数退避 1s);5xx 重试 1 次
- K8s API 错误:透传给 agent 工具结果
- SSE 续传:按 session_id + last_event_id

### 测试策略
- 单元:policy、AES-GCM、repo、JSON schema
- K8s 工具:`client-go` fake clientset
- HTTP:`httptest`
- Agent 循环:mock fantasy client
- 端到端:MVP 不做
- 覆盖率:`internal/*` 70%+,关键路径 90%+

---

## Open Questions

- **charmbracelet/fantasy 的具体 API 形态**:网络受限,无法实时核对最新版本;实施时需对照 README/示例校准用法
- **agent loop 与 SSE 事件桥接的并发模型**:agent 内部可能用 channel/callback,SSE 用 `http.Flusher`;桥接处需要仔细设计(避免阻塞)
- **LLM 流式过程中如何在数据库累积消息**:每个 token 写一次库太频繁,可能采用"流式期间只写内存,message_end 时一次入库"
- **OpenAPI schema 解析范围**:dynamic client 删除/更新需要对象体;我们要求 LLM 传完整 JSON manifest,但描述/列表用 dynamic client 反射出的表头/列是否够友好
- **OpenSpec bridge 与单 change 范围**:本 change 范围较大,实施时可能需要再次确认是否需要进一步拆分;目前按"读 + 写 + Plan/护栏"作为单一可交付单位
- **客户端是否需要 kubectl 兼容**:如果未来用户想"在 kubectl 旁用 kubernetes-agent",可能需要 KUBECONFIG 路径注入而非上传整文件;MVP 用上传
- **多语言回复**:system prompt 写明默认中文,但 LLM 偶尔可能英文;是否需要强制中文(或允许按用户输入语言自适应)— MVP 留作配置项
- **生产命名空间发现**:不同公司 NS 命名不同(可能是 `prod-xxx`、`prd`、`live`),默认规则只覆盖 `production`/`prod`;需要文档引导用户自定义

---

## 后续 Change 路线图(参考)

| 序号 | 主题 | 范围 |
|------|------|------|
| 1 | 本 change | 本地单机 + 读/写 + Plan/护栏 |
| 2 | 多步事务 + 诊断 | k8s_logs 工具、跨资源 plan、解释为什么 pod 起不来 |
| 3 | Helm 集成 | helm-go SDK,agent 能 install/upgrade/rollback chart |
| 4 | 多用户长驻部署 | 登录、会话隔离、租户策略、Postgres 适配 |
| 5 | 多集群 | cluster 切换 UI、跨集群 plan、cluster_group 策略 |
| 6 | UI 打磨 | 审计日志页、可视化 diff 渲染、Plan 历史回放 |
