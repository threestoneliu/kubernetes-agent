# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
make build   # web (pnpm build) → copy-web_dist → go build ./cmd/server
make run     # make build && ./kubernetes-agent  (需要 KUBERNETES_AGENT_CONFIG 环境变量)
make test    # go test ./...
make vet     # go vet ./...
make clean   # rm -rf internal/server/web_dist kubernetes-agent web/dist
```

> `make run` 默认读取 `KUBERNETES_AGENT_CONFIG` 环境变量指向的配置文件，
> 通常设为 `./configs/config.dev.yaml`。不带环境变量直接运行会使用默认配置。

**Dev mode** (前端热重载，改 Go 需要重启后端，改前端 Vite HMR 自动刷新):
```bash
# Terminal 1: 后端（编译用 make build，运行用 make run）
KUBERNETES_AGENT_CONFIG=./configs/config.dev.yaml make run

# Terminal 2: 前端
cd web && pnpm dev   # 监听 localhost:5173，/api 和 /healthz 代理到 8080
```

首次 dev 需要: `cp configs/config.example.yaml configs/config.dev.yaml`，dev 配置建议 dev-db 路径 + 环境变量注入 API key。

## Architecture

### Backend (Go)

- `cmd/server` — 程序入口，组装 Deps，启动 HTTP server
- `internal/server/` — HTTP 层: chi router、handlers、SSE chat 端点、SPA 嵌入
- `internal/agent/` — Agent 循环核心: `Runner.Run()` 执行 LLM chat + tool_calls + plan confirm/ask_user 阻塞
- `internal/llm/` — LLM 客户端抽象 (Anthropic / OpenAI / Ollama)，`llm.Client` 接口
- `internal/store/` — SQLite 存储: clusters、sessions、messages 表；`store.Message` 按 ROWID 排序
- `internal/policy/` — 护栏引擎，执行 YAML 策略规则
- `internal/tools/k8s/` — Kubernetes 工具集: `execute_plan` (GET→Create/Update) 等
- `internal/config/` — 配置加载 (config.yaml)
- `internal/crypto/` — AES-256-GCM 加密 (kubeconfig 存储)

### Frontend (React/TypeScript)

- `web/src/App.tsx` — Shell 组件: 全局 header (48px 顶栏)、view 路由 (chat/clusters/policies)
- `web/src/views/` — ChatView (SSE 流式对话)、SessionsPanel、ClusterView、PolicyView
- `web/src/components/` — PlanModal、AskUserForm、Markdown、Bubble 等 UI 组件
- `web/src/state.ts` — UI 状态机 (idle / streaming / plan_awaiting / ask_user / error)
- `web/src/api.ts` — fetch 封装，所有 HTTP 调用
- `web/src/sse.ts` — SSE EventSource 封装，解析流式事件
- `web/src/contexts/` — ThemeContext (dark/light 主题)
- `web/src/styles.css` — 全局 CSS 变量主题系统 (`:root` / `:root[data-theme="light"]`)

### 数据流 (Chat)

1. `POST /api/chat` 启动 SSE，`Runner.Run()` 在 goroutine 中执行 agent loop
2. Agent 通过 SSE `Event` channel 发送 `reasoning` / `token` / `tool_call` / `tool_result` / `plan_awaiting_confirm` / `ask_user` 事件
3. 前端 `openChatSse` 消费事件流，更新 React 状态 (bubble 渲染)
4. `persistTurn` 在每次 LLM `Chat()` 调用后立即将 message 写入 SQLite（非 loop 结束时才存）
5. `tool_calls` 以 JSON 数组格式存储在 `message.content` 字段

### 关键约束

- `make build` 产物通过 `//go:embed web_dist` 嵌入二进制；改前端必须重新 `make build`
- `internal/server/web_dist/` 是 build artifact，不应直接修改
- ROWID 是 SQLite 主键，用于 message 排序；切换 session 后按 ROWID ASC 加载历史消息
- agent loop 中 `Session.WaitPlan` / `Session.WaitAsk` 阻塞直到 `/resume` 端点被调用
