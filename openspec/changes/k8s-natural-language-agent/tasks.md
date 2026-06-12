# Tasks · K8s Natural Language Agent

## 1. 项目脚手架

- [ ] 1.1 初始化 `go.mod`(`module github.com/threestoneliu/kubernetes-agent`),声明 Go 1.22+
- [ ] 1.2 建立目录结构:`cmd/server/`、`internal/{server,agent,tools,policy,store,crypto,llm,config,logging}/`、`web/`、`configs/`
- [ ] 1.3 写 `cmd/server/main.go` 最小骨架:解析 CLI 参数、加载 config、调用 `slog.SetDefault`、启动占位 HTTP server
- [ ] 1.4 创建 `internal/config`:YAML 解析 + `${ENV}` 展开 + 默认值(server.host/port、storage.db_path、llm.{default,providers}、logging)
- [ ] 1.5 创建 `internal/logging`:封装 `slog` JSON/text 输出,按 `logging.level` 过滤
- [ ] 1.6 添加 `configs/config.example.yaml` 完整示例

## 2. 存储层

- [ ] 2.1 创建 `internal/store/db.go`:用 `modernc.org/sqlite` 打开 db、应用 PRAGMA(journal_mode=WAL, foreign_keys=ON)、`SetMaxOpenConns(1)`(SQLite 串行写)
- [ ] 2.2 创建 `internal/store/migrations.go`:维护 `schema_migrations` 表,实现 `versioned migrations` 模式(每条迁移 `up` SQL,按顺序执行)
- [ ] 2.3 写第一条 migration:6 张表(clusters / sessions / messages / plans / policies / audit_log)+ 2 索引
- [ ] 2.4 创建 `internal/store/clusters.go`:CRUD + `ListByName` + `ListAll` + 加密 blob 读写
- [ ] 2.5 创建 `internal/store/sessions.go`:CRUD + `UpdateTitle` + `TouchUpdatedAt`
- [ ] 2.6 创建 `internal/store/messages.go`:按 `session_id` 列表 + 批量插入(用于 `message_end` 一次性写入)
- [ ] 2.7 创建 `internal/store/plans.go`:CRUD + `UpdateStatus` + `MarkExecuted`
- [ ] 2.8 创建 `internal/store/policies.go`:CRUD + `ListEnabled` + `SetEnabled` + 启动期默认规则插入
- [ ] 2.9 创建 `internal/store/audit.go`:append-only 插入(无 update/delete 方法暴露)
- [ ] 2.10 写表驱动测试:每个 repo 覆盖 happy path + 不存在错误 + 唯一约束违反

## 3. 加密层

- [ ] 3.1 创建 `internal/crypto/aead.go`:封装 AES-256-GCM,接口 `Encrypt(plain []byte) (blob []byte, err error)` / `Decrypt(blob []byte) (plain []byte, err error)`,blob 格式 = `nonce(12) | ciphertext | tag(16)`
- [ ] 3.2 创建 `internal/crypto/masterkey.go`:实现"env 优先,文件后备"加载,首次启动生成随机 32 字节,chmod 0600,root 启动拒绝
- [ ] 3.3 写 round-trip 测试:任意明文加密-解密一致、密文不重复、GCM tag 篡改必失败
- [ ] 3.4 写 `KUBERNETES_AGENT_MASTER_KEY` 环境变量支持:base64 解码 32 字节,长度不符报错

## 4. 启动流程

- [ ] 4.1 实现 `cmd/server/main.go` 启动序列:解析 config → 加载/生成 master key → 打开 SQLite + 跑 migrations → 插入默认护栏规则(若空)→ LLM provider ping → 启动 HTTP
- [ ] 4.2 默认护栏规则 seed:4 条规则(系统 NS 禁删 / kind 黑名单 / unsafeFields / production confirm),按 design 文档 D5
- [ ] 4.3 启动期错误统一:任何一步失败 → stderr + `os.Exit(1)`,绝不静默
- [ ] 4.4 写 `internal/config` 解析测试 + `internal/store` 迁移测试

## 5. K8s 工具层

- [ ] 5.1 创建 `internal/tools/k8s/client.go`:`ClientFactory` 接口,根据 cluster_id 从 store 读 kubeconfig、AES-GCM 解密、构造 `*dynamic.DynamicClient`、缓存
- [ ] 5.2 写 `k8s_get` 工具:参数 {resource, name, namespace, cluster_id},返回 K8s 对象 JSON;namespace 缺省 `default`
- [ ] 5.3 写 `k8s_list` 工具:参数 {resource, namespace?, label_selector?, cluster_id},支持 ListAll namespaces(空 namespace)
- [ ] 5.4 写 `k8s_describe` 工具:参数同 get + cluster_id,返回对象 + 相关 events(按 involvedObject UID 过滤)+ owner references + diagnosis hints 字段(静态映射,如 ImagePullBackOff / CrashLoopBackOff / Pending)
- [ ] 5.5 写 `k8s_plan_write` 工具:参数 {operations: [...], cluster_id};policy 预检 → server-side dry-run(apply 用 `dryRun=All`,delete/scale 用 GET)→ 组装 plan,返回 {plan_id, summary, diffs, risk, denied}
- [ ] 5.6 写 `k8s_execute_plan` 工具:参数 {plan_id, confirm_token};二次 policy 评估 → 顺序执行 operations → 失败时反向 apply 已成功部分 → 写 audit_log
- [ ] 5.7 写 `ask_user` 工具:不调 K8s,只产生 SSE 事件让前端渲染表单;用户答复后作为工具结果回传
- [ ] 5.8 写工具测试:用 `client-go` 的 `fake` clientset 覆盖 happy path + dry-run 失败 + policy 拒绝

## 6. 护栏层

- [ ] 6.1 创建 `internal/policy/rule.go`:Rule 结构(Name, Effect[allow/confirm/deny], Match{Action, Namespace, Kind, UnsafeFields}) + YAML 解析
- [ ] 6.2 创建 `internal/policy/jsonpath.go`:简化 JSONPath(支持 `[*]` 数组通配),匹配值
- [ ] 6.3 创建 `internal/policy/engine.go`:顺序求值,返回首个匹配规则的 effect;无匹配 → 读=allow,写=confirm
- [ ] 6.4 创建 `internal/policy/default.go`:4 条默认规则常量
- [ ] 6.5 写 engine 测试:规则覆盖、顺序敏感、默认行为、unsafeFields 嵌套匹配

## 7. LLM 抽象

- [ ] 7.1 创建 `internal/llm/provider.go`:统一接口 `Chat(ctx, messages, tools) (Stream, error)`,Stream 提供 `Next() (Event, error)`
- [ ] 7.2 创建 anthropic / openai / openai-compatible 三个 adapter,经由 `charmbracelet/fantasy` 构造 client
- [ ] 7.3 创建 `internal/llm/ping.go`:启动期并发 ping(发最小 messages 列表 + 1 秒超时),失败标 `disabled`
- [ ] 7.4 创建 `internal/llm/prompt.go`:system prompt 模板(身份 + 工具集 + 写工作流 + 风格约束 + 默认中文)
- [ ] 7.5 写 ping 测试:用 `httptest.Server` 模拟 provider 端点

## 8. Agent 循环

- [ ] 8.1 创建 `internal/agent/events.go`:12 个 SSE 事件类型 + JSON 序列化
- [ ] 8.2 创建 `internal/agent/tools.go`:把 6 个工具注册为 fantasy 的 tool(每个工具签名 = (name, description, json_schema, handler))
- [ ] 8.3 创建 `internal/agent/agent.go`:agent 循环主体 — 调用 fantasy stream → 解析每条 event → 翻译为 SSE event 推给 HTTP 层
- [ ] 8.4 创建 `internal/agent/session.go`:会话状态管理(消息列表 + plan 缓存 + last_event_id)
- [ ] 8.5 实现"流式期间 token 累积在内存,message_end 一次性入库"
- [ ] 8.6 实现 `plan_awaiting_confirm` 阻塞:agent 循环在推完该事件后 MUST 暂停,等用户事件(resume token)
- [ ] 8.7 实现 `ask_user` 阻塞:同上
- [ ] 8.8 实现 LLM 错误重试:401/403 不重试,429 + 5xx 重试 1 次(指数退避 1s)
- [ ] 8.9 实现历史截断:token 数 ≥ 80% context window 时丢最旧非 system 消息
- [ ] 8.10 写 agent 循环测试:mock fantasy client(注入假 LLM 响应序列),验证事件序列与 plan 阻塞语义

## 9. HTTP 层

- [ ] 9.1 创建 `internal/server/router.go`:用 `go-chi/chi/v5` 注册路由
- [ ] 9.2 写 `GET /healthz` handler:返回 `{ ok: bool, providers: [{name, status}] }`
- [ ] 9.3 写 `POST /api/chat` SSE handler:接收 `{session_id, message, cluster_id}`,调 agent 循环,SSE 推流
- [ ] 9.4 写 `GET /api/clusters` / `POST /api/clusters` / `DELETE /api/clusters/{id}`:含上传 kubeconfig 时解析 + 加密落库
- [ ] 9.5 写 `GET /api/policies` / `PUT /api/policies/{id}`:YAML 校验 + 写 audit_log
- [ ] 9.6 写 `GET /api/sessions` / `POST /api/sessions` / `GET /api/sessions/{id}/messages`
- [ ] 9.7 SSE 续传:支持 `Last-Event-ID` header,按 session_id + event_id 续推(plan_ready 后不重放)
- [ ] 9.8 错误响应标准化:`{ code, message, retryable }`
- [ ] 9.9 写 HTTP 测试:用 `httptest`,覆盖各 handler + SSE 流

## 10. Web UI 脚手架

- [ ] 10.1 `web/package.json` 初始化:Vite + React + TypeScript + Tailwind(可选)
- [ ] 10.2 `web/vite.config.ts`:开发期代理 `/api` 到 `:8080`,支持 `KUBERNETES_AGENT_DEV=1` 后端代理前端 dev server
- [ ] 10.3 创建 `web/src/sse.ts`:基于原生 `EventSource` 的客户端,支持 last_event_id 重连
- [ ] 10.4 创建状态管理:5 个 UI 状态机(对话 / Plan 阻塞 / ask_user / 错误 toast / 工具执行)
- [ ] 10.5 风险等级 emoji + 颜色组件 + 折叠组件(reasoning / 工具行)

## 11. Web UI 视图

- [ ] 11.1 实现 `views/ChatView.tsx`:主对话视图布局(侧边栏 + cluster/session 切换 + 消息流 + 输入框)
- [ ] 11.2 实现 `views/ClusterView.tsx`:上传 / 列出 / 删除 kubeconfig,显示 server + user
- [ ] 11.3 实现 `views/PolicyView.tsx`:YAML 文本编辑 + 启用/禁用 checkbox + 校验提示
- [ ] 11.4 实现 Plan 预览模态:展示 diffs + 风险等级 + 确认/取消按钮,生成 confirm_token
- [ ] 11.5 实现 ask_user 表单:单选 / 多选 / 文本三种模式
- [ ] 11.6 实现错误 toast + 顶部重连状态条
- [ ] 11.7 视觉:侧边栏 / 风险 emoji / 折叠块全部按 design 文档 D10 落实

## 12. 嵌入与单二进制

- [ ] 12.1 Go 端 `embed.FS` 引入 `web/dist` 全部产物
- [ ] 12.2 `GET /*` fallback handler:命中非 `/api` 与非 `/healthz` 的请求 → 返回 `index.html`(SPA 路由兜底)
- [ ] 12.3 静态资源 cache-control:hash 命名的 chunk 永久缓存,`index.html` 不缓存
- [ ] 12.4 验证:`go build` 产出单二进制,运行后浏览器访问 `http://127.0.0.1:8080` 完整可用

## 13. 测试与端到端验证

- [ ] 13.1 `go test ./...` 全部通过,`internal/*` 行覆盖 ≥ 70%
- [ ] 13.2 关键路径(policy / plan_write / execute_plan)覆盖 ≥ 90%
- [ ] 13.3 手工 e2e:启动服务 → 上传 kubeconfig(可用 `kind` 起的本地集群或 `client-go` fake)→ 选 provider → 发送"列出 default namespace 的 pod" → 收到响应
- [ ] 13.4 手工 e2e:发"删除 nginx deployment" → 看到 Plan 弹窗 → 确认 → 集群中 pod 被删
- [ ] 13.5 手工 e2e:发"删除 kube-system 中任意 pod" → 看到拒绝原因
- [ ] 13.6 手工 e2e:发问"目标 namespace?"(通过 ask_user 工具模拟)→ 表单渲染 → 答复 → 继续

## 14. 文档

- [ ] 14.1 写 README:本地启动步骤、首次体验、master.key/data.db 一起备份警告
- [ ] 14.2 写默认护栏规则说明 + 如何自定义
- [ ] 14.3 写 LLM provider 配置示例
- [ ] 14.4 写开发模式说明(`KUBERNETES_AGENT_DEV=1` + Vite dev server)
- [ ] 14.5 写后续 change 路线图(链接 brainstorm.md 末尾)
