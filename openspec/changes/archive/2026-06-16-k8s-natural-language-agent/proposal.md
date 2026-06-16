## Why

K8s 日常操作(查询、排查、改配置)目前依赖 kubectl + YAML + 记忆,门槛高:新手要背命令与字段,SRE 在生产集群下手写 apply 易误操作,业务方排查故障要在 events/logs/状态之间拼凑。本项目从零起步,有机会把"自然语言 + Plan 预览 + 全局护栏"做成产品,让用户用一句话完成查询与计划性写入,写操作在执行前必须看到 diff 与风险等级。本 change 交付首个可用版本:本地单机,Web UI 聊天界面,5 个 K8s 工具(读 3 + 写 2),Plan 预览与 Go 层 policy engine 双重把关,K8s 凭证 AES-GCM 加密不出本机。预期收益是降低 K8s 使用门槛、减少误删/越权写、把"事前授权"做到协议级强制。

## What Changes

**K8s 集群交互方式**
- From: 用户通过 kubectl + YAML 操作 K8s 集群
- To: 用户在 Web UI 用自然语言对话完成查询(get/list/describe)与计划性写入(apply/scale/delete)
- Reason: 降低 K8s 门槛,统一入口,自然语言表达意图
- Impact: non-breaking;新增能力,不替代 kubectl

**写操作执行前授权**
- From: 写操作无 UI 层强制授权,依赖 K8s RBAC
- To: 写操作必须经过基于 K8s server-side dry-run 的 Plan 预览,UI 展示 diff + 风险等级,用户点"确认"才真正执行
- Reason: 用户明确希望"写操作通过 UI 授权后再执行";Plan 来源是真实 dry-run,而非 LLM 想象
- Impact: non-breaking;新增 plan_awaiting_confirm 阻塞点

**全局安全护栏**
- From: 无 Go 层 policy engine
- To: 三态 policy engine(allow / confirm / deny),在 plan 阶段与 execute 阶段双重评估;默认规则随产品发出(系统 NS 禁删、Node/ClusterRole 禁写、危险字段 deny、production NS 写操作 confirm)
- Reason: Plan 预览是 UX 护栏,Go 层护栏是协议级强制,LLM 改不了规则
- Impact: non-breaking;默认规则可由用户在策略编辑页调整

**K8s 凭证存储**
- From: 仓库零状态,无凭证存储
- To: 用户上传 kubeconfig 整体 AES-256-GCM 加密落 SQLite,master.key 来自环境变量或 `~/.kubernetes-agent/master.key`(0600)
- Reason: 凭证不出本机
- Impact: non-breaking;master.key 丢失即数据不可恢复,文档显眼位置说明

**LLM 后端接入**
- From: 无 LLM 接入
- To: 多 provider 抽象(Anthropic Claude / OpenAI / 兼容 OpenAI 协议),`config.yaml` 配置,启动期并发 ping 一次
- Reason: 给用户多 LLM 选择;支持本地模型(Ollama 等)
- Impact: non-breaking;用户首次启动选 provider 后即可用

**前端嵌入方式**
- From: 无前端
- To: React + Vite + TypeScript SPA,构建产物经 Go `embed.FS` 嵌入二进制,单文件分发
- Reason: 单二进制部署最简;前端仅在本地访问,无网络开销
- Impact: non-breaking

## Capabilities

### New Capabilities

- `natural-language-k8s-interaction`: 用户通过 Web UI 用自然语言完成 K8s 资源查询与计划性写入;SSE 流式响应;`ask_user` 工具支持 LLM 主动澄清;消息与会话落 SQLite
- `k8s-write-with-plan-preview`: 写操作(apply/scale/delete)必须先经 `k8s_plan_write` 产生 plan(K8s server-side dry-run),`k8s_execute_plan` 必须带 `plan_id` + `confirm_token` 才执行;失败回滚已成功部分
- `k8s-policy-guardrails`: 三态 policy engine(allow / confirm / deny),匹配 action/namespace/kind/unsafeFields;plan 与 execute 双重评估;默认规则随产品发出;UI 提供 YAML 编辑
- `k8s-credential-encryption`: kubeconfig 整体 AES-256-GCM 加密;master.key 来自 `KUBERNETES_AGENT_MASTER_KEY` 或 `~/.kubernetes-agent/master.key`(0600);解密仅在内存,会话结束清除
- `multi-llm-provider-support`: 配置驱动多 LLM provider(Anthropic / OpenAI / 兼容 OpenAI);启动期 ping 校验连通性,失败 provider 不出现在 UI 选单;system prompt 与工具注册由 `charmbracelet/fantasy` 编排
- `web-chat-ui`: React + Vite + TypeScript SPA;3 视图(主对话 / 集群管理 / 策略编辑);5 状态(普通对话 / Plan 预览阻塞模态 / ask_user 表单 / 错误 toast / 工具执行中);前端经 `embed.FS` 嵌入 Go 二进制

### Modified Capabilities

无(本项目从零起步,无既有 spec)。

## Impact

**新增代码模块**(均新写,无既有代码)
- `cmd/server/main.go` — 入口
- `internal/server/` — HTTP 层(chi),SSE handler
- `internal/agent/` — charmbracelet/fantasy 集成、agent 循环、prompt、事件
- `internal/tools/k8s/` — 5 个工具 + dynamic client 工厂
- `internal/policy/` — 三态 policy engine + 简化 JSONPath
- `internal/store/` — SQLite repo(versioned migrations)
- `internal/crypto/` — AES-256-GCM + master.key
- `internal/llm/` — provider 抽象 + 启动期 ping
- `internal/config/` — YAML 加载 + 环境变量展开
- `internal/logging/` — slog
- `web/` — React + Vite 前端

**新增 HTTP API**
- `POST /api/chat`(SSE)
- `GET /api/clusters` / `POST /api/clusters` / `DELETE /api/clusters/{id}`
- `GET /api/policies` / `PUT /api/policies/{id}`
- `GET /api/sessions` / `POST /api/sessions` / `GET /api/sessions/{id}/messages`
- `GET /healthz`

**新增依赖**(go.mod)
- `github.com/charmbracelet/fantasy`
- `k8s.io/client-go`
- `github.com/go-chi/chi/v5`
- `gopkg.in/yaml.v3`
- `modernc.org/sqlite`
- `github.com/stretchr/testify`(测试)

**新增数据**
- SQLite 6 张表:`clusters` / `sessions` / `messages` / `plans` / `policies` / `audit_log`
- 文件:`~/.kubernetes-agent/master.key`(0600) + `~/.kubernetes-agent/data.db` + `~/.kubernetes-agent/config.yaml`

**不影响**:无既有系统(项目零状态)。

**测试影响**:新增 K8s 工具测试(client-go fake clientset)+ HTTP handler 测试(httptest)+ agent 循环测试(mock fantasy)+ 端到端不做;`internal/*` 覆盖率目标 70%+,关键路径(policy/plan/execute)90%+。
