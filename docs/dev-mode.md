# 开发模式

## dev vs 生产

| 模式 | 跑法 | 适用 |
|------|------|------|
| 生产 | `make build` 一次性产出 `./kubernetes-agent`，内置 SPA | 用户/分发 |
| dev | 后端 `go run` + 前端 `pnpm dev`（Vite） | 改前端 / 调 API / 改 Go |

`make build` = `pnpm build`（Vite 生产构建）→ `cp -R web/dist/ internal/server/web_dist/` → `go build` 把 dist 嵌入二进制。改一行 React 也要走完整的 pnpm install + build + go build，迭代很慢。开发期绕过这一步，让 Vite 起 dev server 提供 SFC 解析 + HMR，后端单独重启。

## 步骤 1: 启动后端

监听 `127.0.0.1:8080`，配置指向开发用的 yaml（可以指向测试用 sqlite + 测试用 LLM provider）：

```bash
KUBERNETES_AGENT_CONFIG=./configs/config.dev.yaml \
  GOSUMDB=sum.golang.org \
  go run ./cmd/server
```

后端只在 Go 代码变更时需要 `Ctrl-C` 再 `go run`。改 React / CSS / 普通 TS 不影响后端进程。

如果改了 `internal/server/web_dist/`（SPA 嵌入产物），需要重新 `make build` 或 `make copy-web`。

## 步骤 2: 启动前端

新开一个 shell：

```bash
cd web
pnpm install                    # 第一次或 lockfile 变化时
pnpm dev
```

Vite 默认监听 `http://localhost:5173`，并在 `web/vite.config.ts` 里配好了代理：

```ts
server: {
  port: 5173,
  proxy: {
    '/api':     'http://127.0.0.1:8080',
    '/healthz': 'http://127.0.0.1:8080',
  },
}
```

所以前端直接 fetch `/api/chat` / `/healthz` 会被代理到 8080，**不需要**前端配 CORS 或后端开 CORS。

## 步骤 3: 浏览器

打开 `http://localhost:5173`。

- 改 `web/src/**` → Vite 自动 HMR，浏览器无需手动刷新
- 改 Go 代码 → `Ctrl-C` 当前 `go run`，再 `go run ./cmd/server`
- 改 `go.mod` → `go mod tidy` + 重启
- 改 `web/vite.config.ts` → Vite 自己热重启
- 改 `web/package.json` 依赖 → `Ctrl-C` `pnpm dev` 再 `pnpm install` 再 `pnpm dev`

## 配置文件

`configs/config.dev.yaml` 没有自动生成，从 `configs/config.example.yaml` 复制一份并按需修改：

```bash
cp configs/config.example.yaml configs/config.dev.yaml
```

常见的 dev 期调整：

- `storage.db_path`：指向 `./dev-data.db` 或 `/tmp/agent-dev.db`，避免污染 `~/.kubernetes-agent/data.db`
- `llm.providers[].apiKey`：用 `${ANTHROPIC_API_KEY}` 等环境变量，不要把真 key 写进文件
- `logging.level`：改成 `debug` 看 agent 事件流

把 `configs/config.dev.yaml` 加进 `.gitignore` 或确认里面没有真 key。

## 不使用 SPA 嵌入

dev 期**不要**跑 `make build`。原因：

1. `make build` 会跑 `pnpm install --frozen-lockfile && pnpm build`，Vite production build 比 dev build 慢一个数量级
2. 每次改前端都要走完整的 `pnpm build` → `cp -R` → `go build` 才能看到效果
3. production build 会把 React 压成 minified bundle，stack trace 全是字母，调试痛苦

Vite 的 dev server 提供 SFC 解析 + ESM 原生加载 + HMR，开发体验远比每次走 `pnpm build` 强。

只有以下情况才需要 `make build`：

- 验证最终产物的可执行文件能起来
- 跑 e2e 测试（`make test` 内部会调 `make web` 准备 dist）
- 准备给别人分发的单二进制

## Web 单元测试

web 包只配了 typecheck + build，没有 vitest / jest：

```bash
cd web
pnpm typecheck          # tsc --noEmit，TS 类型检查
pnpm build              # vite build，产 web/dist/ 供 make build 嵌入
```

`pnpm build` 的产物会进 `internal/server/web_dist/`（通过 Makefile 的 `copy-web` 目标），进而被 `//go:embed web_dist` 打进二进制。所以 `make build` 之前**必须**先 `pnpm install`，否则 `pnpm build` 找不到依赖。

## 调试技巧

- **看 SSE 事件流**：浏览器 DevTools → Network → 找到 `POST /api/chat` 的 EventStream 类型响应，能看到逐个 event 的 JSON
- **看 provider 健康**：`curl http://127.0.0.1:8080/healthz | jq` 看 `providers[]` 状态
- **看 agent 循环日志**：把 `configs/config.dev.yaml` 的 `logging.level` 改成 `debug`，重启后端
- **看 React 错误**：浏览器 console + React DevTools 扩展

## 下一步

- 完整 dev 模式设计：见 [design.md → D9 前端 React + Vite + TypeScript](../openspec/changes/k8s-natural-language-agent/design.md#d9-前端-react--vite--typescript嵌入-go-embedfs)
- SPA 嵌入机制：见 [design.md → D12 错误处理分层](../openspec/changes/k8s-natural-language-agent/design.md#d12-错误处理分层)（D12 主要讲错误处理；单二进制分发的 embed.FS 实现细节散落在 `internal/server/static.go` 的注释里）
