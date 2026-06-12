# Design · K8s Natural Language Agent

## Context

`kubernetes-agent` 项目从零起步,目前仓库仅含 `openspec/`(本次 change 的根)、`README.md`(只有项目名)、`LICENSE`、`LICENSE 风格的 .gitignore`(暗示 Go 路径)。无任何源码、配置或外部依赖声明。

本 change 交付第一个可用形态:本地单机的 K8s 自然语言 agent。约束包括:
- 仅一个用户,本地运行(浏览器访问 `http://127.0.0.1:8080`)
- K8s 凭证(用户上传的 kubeconfig)必须 AES-GCM 加密落本地 SQLite,不出本机
- 写操作必须经过 Plan 预览(基于 K8s server-side dry-run)与全局可配置护栏
- LLM 通过多 provider 抽象接入(Anthropic / OpenAI / 兼容 OpenAI)
- 单二进制分发(前端 SPA 通过 `embed.FS` 嵌入)

后续 change 路线图(本 change **不**覆盖):多用户/多租户 · 多集群 · 多步事务/复杂编排 · 诊断(为什么 pod 起不来)· Helm · CRD 解析 · 多档位详细度 · docker 镜像/安装包。

## Goals / Non-Goals

**Goals**
- 本机浏览器即可用,无需远端服务
- 用户在 Web UI 中能用自然语言完成:查询资源(get/list/describe)、计划性写入(apply/scale/delete)
- 写操作流程强制:Plan 生成(来自 K8s server-side dry-run,非 LLM 想象)→ 浏览器弹窗展示 diff + 风险等级 → 用户点"确认"才执行
- Go 层 policy engine 在 plan 阶段与 execute 阶段两次评估,任何一次 deny 都阻断
- K8s 凭证不出本机;主密钥 0600 权限;不引入 KMS
- agent 循环、工具集、护栏、SSE 协议、SQLite schema 五个层之间的边界明确、可独立测试

**Non-Goals**
- 多用户、登录、租户隔离 → 后续 change
- 多集群切换 UI、跨集群 plan → 后续 change
- 多步事务(创建一个 Deployment 后再 create Service + Ingress)→ 后续 change
- 诊断(为什么 pod 起不来、为什么 service 无法访问)→ 后续 change
- Helm chart install/upgrade/rollback → 后续 change
- CRD 资源解析、kubectl explain 级别智能 → 后续 change
- 多语言模型档位(对"开发者" vs "SRE" vs "新人"用不同详细度)→ 后续 change
- docker 镜像 / 安装包 / 自动更新 / 远程部署 → 后续 change
- 端到端测试连真 K8s 集群 → 留作后续(本 change 用 client-go fake clientset 测)

## Decisions

### D1. 后端语言锁定 Go

**Why**:项目 `.gitignore` 是 Go 社区模板;`charmbracelet/fantasy` 是 Go 库;单二进制分发与 Go 静态链接天然契合;护栏在 Go 类型系统中可做强校验。

**Alternatives**:Python + LangChain(否决:运行重,LangChain 抽象层厚,与 .gitignore 暗示背离);Node.js(否决:LLM SDK 不如 Go 简洁,且无 native 静态分发)。

### D2. Agent 框架选 charmbracelet/fantasy

**Why**:charmbracelet 系列库(Bubble Tea / Lip Gloss)在 Go 终端生态成熟;fantasy 抽象 OpenAI 兼容协议(覆盖 Anthropic / OpenAI / 兼容 OpenAI);用户明确指定。

**Trade-off**:网络受限,无法实时核对最新 API;`Open Questions` 中已列为待核对点。实施时按 README/示例校准。

**Alternatives**:Anthropic SDK 直调(否决:多 provider 需自己写抽象);LangChainGo(否决:抽象较厚,迭代不如 fantasy 轻量)。

### D3. K8s 客户端用 client-go dynamic client

**Why**:用户明确指定;dynamic client 覆盖 K8s API 全资源,无需为每种 kind 写类型;plan 阶段 server-side dry-run 与 execute 阶段真实调用同一套客户端。

**Trade-off**:无类型安全(LLM 需提供完整 JSON manifest);但 MVP 范围内可接受。

**Alternatives**:Helm/CRD 解析器(否决:用户不要,MVP 控制范围);typed client(否决:每加一种 kind 都要重编,扩展性差)。

### D4. 工具集 5 个(读 3 + 写 2)

**Why**:5 个工具足够覆盖 MVP 读写。`plan_write` 与 `execute_plan` 分离把"Plan 预览"机制落到协议层:
- `plan_write` 返回 `plan_id` + `diffs` + `risk`(来自 K8s server-side dry-run)
- `execute_plan` 必须带 `plan_id` + `confirm_token`(UI 在用户点确认时生成)
- 即便 LLM 试图跳过 plan,execute 工具因缺 plan_id 直接拒绝
- LLM 拿不到 policy 拒绝的操作(denied 字段单独返回,不在 diffs 中)

**Trade-off**:工具数少,LLM 描述/列表的资源类型需要后端 dynamic client 反射出的表头/列;若体验不友好,后续可加资源类型化工具。

### D5. 护栏三态(allow / confirm / deny)+ Go 层强制

**Why**:`deny` 是协议级阻断(不进入 plan);`confirm` 是 UI 阻塞模态(用户必须显式确认);`allow` 默认通过。Go 层编码确保 LLM 改不了规则——这是安全的硬底线。

**匹配维度**:action(apply/delete/scale)、namespace(列表)、kind(列表)、unsafeFields(简化 JSONPath 检测 `privileged`/`hostNetwork`/`hostPID` 等)。

**双重评估**:`plan_write` 一次、`execute_plan` 一次(防中途改 policy)。

**默认规则**:禁删 `kube-system`/`kube-public`/`kube-node-lease`、禁写 `Node`/`ClusterRole`/`ClusterRoleBinding`/`CRD`、危险字段 deny、`production`/`prod` NS 写操作 confirm。

**Trade-off**:简化 JSONPath 不支持完整 JMESPath(常见危险字段够用);`production` 命名空间在不同公司各异,需要文档引导自定义。

### D6. SSE 事件契约(12 事件)

**Why**:SSE 是前后端唯一通信,事件顺序保证状态机清晰。`plan_awaiting_confirm` 是 agent 循环的阻塞点(等用户事件);前端不需要懂 agent 内部。

**事件清单**:`session_meta` / `reasoning` / `token` / `tool_call` / `tool_result` / `plan_ready` / `plan_awaiting_confirm` / `ask_user` / `cluster_switch` / `cancelled` / `error` / `message_end`

**Trade-off**:协议较"宽"(12 事件),需要前端状态机对得上;不过每个事件语义清晰,前端实现可控。

### D7. SQLite 6 张表 + append-only audit_log

**Why**:6 张表覆盖所有 MVP 持久化需求(clusters/sessions/messages/plans/policies/audit_log)。`audit_log` append-only 永不删,合规与可追溯。`plans` 表保留完整 `ops_json` + `diffs_json`,用户回看会话时能重放 plan 内容。

**驱动选择**:`modernc.org/sqlite`(纯 Go,无 cgo,交叉编译友好,部署单二进制)。

**Trade-off**:MVP 不支持 Postgres;多用户模式需要 Postgres 时,store 层抽象化后切换。

### D8. AES-256-GCM + 本地 master.key

**Why**:标准库实现、无外部依赖;master.key 来自 `K8S_AGENT_MASTER_KEY` 环境变量或 `~/.k8s-agent/master.key`(0600);MVP 不引入 KMS。

**存储格式**:`nonce(12B) | ciphertext | tag(16B)` 单 BLOB。

**Trade-off**:本地 master.key 安全性依赖 OS 文件权限;**MVP 接受**。

### D9. 前端 React + Vite + TypeScript(嵌入 Go embed.FS)

**Why**:Web UI 涉及多视图(主对话/集群管理/策略编辑)+ 多个交互状态(普通对话/Plan 弹窗/ask_user 表单/错误 toast),React 组件库丰富能省 UI 工作。`embed.FS` 实现单二进制分发。

**Trade-off**:依赖较重;首屏需 SPA hydration;但本地单机模式无网络加载开销,可接受。

**Alternatives**:SvelteKit(否决:生态比 React 小);Go html/template(否决:复杂交互吃力)。

### D10. Web UI 3 视图 + 5 状态

**3 视图**:主对话视图(默认)· 集群管理(上传/删除 kubeconfig)· 策略编辑(YAML 文本)

**5 状态**:普通对话 · Plan 预览(阻塞模态)· ask_user 表单 · 错误 toast · 工具执行中

**视觉规则**:风险用 🟢🟡🔴 emoji + 颜色;reasoning 折叠;工具行折叠;Plan 弹窗阻塞模态(写操作期间输入框禁用)。

**Trade-off**:MVP 不做"审计日志页"——已通过 audit_log 表记录,UI 在后续 change 打磨。

### D11. LLM 多 provider + 启动期 ping

**Why**:`config.yaml` 列出所有 provider,启动时并发 ping 一次,失败的标 disabled 不出现在 UI 选单。本地模型(Ollama)无 extended thinking 时,`reasoning` 事件不推。

**Trade-off**:启动稍慢(几秒);但确保用户首次进入对话时所有 provider 都可用。

### D12. 错误处理分层

- 启动错误:进程退出 + 明确 stderr(配置错/SQLite 错/master.key 失败)
- LLM 错误:401/403 不重试;429 重试 1 次(指数退避 1s);5xx 重试 1 次
- K8s API 错误:透传给 agent 工具结果(让 LLM 解释)
- SSE 中断:前端 EventSource 自动重连,后端按 `session_id` + `last_event_id` 续传

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| **R1. 单 change 范围较大**(读+写+plan+policy+UI+多 LLM 抽象) | 实施时严格按模块拆分 task(internal/server / internal/agent / internal/tools / internal/policy / internal/store / internal/crypto / internal/llm / internal/config / web);每个模块单独 PR + 测试;若发现 task 列表过长,考虑拆为 2 个 change |
| **R2. charmbracelet/fantasy 的具体 API 形态未核对** | 实施前在 plan 阶段对照 README/示例写一段"对接小样";若 API 与假设差异大,改用 Anthropic SDK 直调 + 自写多 provider 抽象 |
| **R3. SSE 续传语义不明确** | 实施时明确"按 last_event_id 重放"的范围(MVP 仅重放未结束的 tool_call/result,plan_ready 后必须用户重新确认) |
| **R4. master.key 丢失即数据不可恢复** | 文档显眼位置说明:master.key 与 data.db 必须一起备份;MVP 不提供 key rotation |
| **R5. SQLite 在高并发写下的锁竞争** | MVP 单用户,无并发;若未来多用户,切到 Postgres(store 层抽象化) |
| **R6. dynamic client 错误信息对用户不友好** | 工具层捕获 K8s API 错误,翻译为人话后返回给 LLM(让 LLM 二次解释) |
| **R7. `production` 命名空间命名不一致** | 文档 + README 提供"如何自定义"小节;`policies` 表的 UI 允许编辑 |
| **R8. Plan 弹窗阻塞期间用户无法中断** | 提供"取消"按钮,推 `cancelled` 事件,agent 循环退出当前消息 |
| **R9. LLM 流式过程中数据库写入频繁** | 实施策略:流式期间 token 只存内存,`message_end` 一次性入库;工具调用事件独立写一行(可回放) |
| **R10. 前端 SSE 客户端断线重连状态** | 实施策略:重连时按 session_id + last_event_id 续传,UI 顶部显示"重连中"状态条 |
| **R11. unsafeFields 简化 JSONPath 不够** | 实施时覆盖 K8s 常见危险字段即可;若需要更复杂规则,后续接入完整 JMESPath |
| **R12. 测试覆盖率 70% 难达到的模块** | policy engine / execute_plan / plan_write 走 90%+;HTTP handler 70%;web/ 端到端不做 |

## Migration Plan

本 change 是项目从零到 0.1.0 的首个发布,无既有用户。

**首次部署**:
1. `go build -o k8s-agent ./cmd/server` 构建单二进制
2. `cd web && pnpm install && pnpm build` 构建前端(产物 `web/dist/` 被 `embed.FS` 引用)
3. `./k8s-agent` 启动
4. 首次启动:自动创建 `~/.k8s-agent/master.key`(0600)+ `data.db` + 插入默认护栏规则
5. 浏览器访问 `http://127.0.0.1:8080`
6. 引导页:上传 kubeconfig → 选 LLM provider → 开始对话

**配置升级路径**(本 change 内):无;本 change 是 0.1.0,后续 change 可能加迁移脚本。

**回滚策略**:
- 本 change 无"上线"概念,本机即用即弃
- 若 SQLite schema 不兼容,删除 `~/.k8s-agent/` 整个目录(用户主动操作)

## Open Questions

- **Q1. charmbracelet/fantasy 的具体 API 形态**:网络受限,无法实时核对最新版本;实施时需对照 README/示例校准用法
- **Q2. agent loop 与 SSE 事件桥接的并发模型**:agent 内部可能用 channel/callback,SSE 用 `http.Flusher`;桥接处需要仔细设计(避免阻塞)
- **Q3. LLM 流式过程中如何在数据库累积消息**:每个 token 写一次库太频繁,可能采用"流式期间只写内存,message_end 时一次入库"
- **Q4. OpenAPI schema 解析范围**:dynamic client 删除/更新需要对象体;我们要求 LLM 传完整 JSON manifest,但描述/列表用 dynamic client 反射出的表头/列是否够友好
- **Q5. OpenSpec bridge 与单 change 范围**:本 change 范围较大,实施时可能需要再次确认是否需要进一步拆分
- **Q6. 客户端是否需要 kubectl 兼容**:如果未来用户想"在 kubectl 旁用 k8s-agent",可能需要 KUBECONFIG 路径注入而非上传整文件;MVP 用上传
- **Q7. 多语言回复**:system prompt 写明默认中文,但 LLM 偶尔可能英文;是否需要强制中文(或允许按用户输入语言自适应)— MVP 留作配置项
- **Q8. 生产命名空间发现**:不同公司 NS 命名不同,默认规则只覆盖 `production`/`prod`;需要文档引导用户自定义
- **Q9. 续传范围与 last_event_id 语义**:明确"重连后能拿到什么 / 不能拿到什么"(MVP 倾向:plan_ready 后必须用户重新确认,不重放已执行的部分)
- **Q10. 前端状态管理选型**:Tailwind + Zustand 是建议,最终在 web/ 初始化时敲定
