# LLM provider 配置

`llm.providers[]` 数组里每一项是一个独立 provider，可以同时配多个（一个作为 `llm.default`，其余作为 UI 选单里的备选）。

## 3 种 type

| `type` | 必需字段 | 用途 |
|--------|----------|------|
| `anthropic` | `name`、`apiKey`、`model` | 直连 Anthropic Messages API |
| `openai` | `name`、`apiKey`、`model` | 直连 OpenAI Chat Completions API |
| `openai-compatible` | `name`、`baseURL`、`apiKey`、`model` | 任何 OpenAI 兼容 HTTP 服务（本地 Ollama / vLLM / LM Studio / 一线大厂网关） |

`apiKey` 仅在 `anthropic` / `openai` 类型下是必填的；`baseURL` 仅在 `openai-compatible` 类型下必填。`openai_compat` 是 `openai-compatible` 的别名，等价。

`apiKey` 支持 `${ENV_VAR}` 占位（见 `internal/config/config.go`），**不要把真实 key 直接写进 yaml**。

## 配置示例

三个独立 provider + 默认指向 anthropic：

```yaml
llm:
  default: anthropic-prod
  providers:
    - name: anthropic-prod
      type: anthropic
      apiKey: ${ANTHROPIC_API_KEY}
      model: claude-sonnet-4-6
    - name: openai-fallback
      type: openai
      apiKey: ${OPENAI_API_KEY}
      model: gpt-4o
    - name: ollama-local
      type: openai-compatible
      baseURL: http://localhost:11434/v1
      model: llama3.1
```

完整可拷贝的示例见 [`configs/config.example.yaml`](../configs/config.example.yaml)。

## 本机 Ollama

Ollama 默认不开 `/v1` 兼容端。先把模型拉下来再启动 server：

```bash
ollama pull llama3.1
ollama serve          # 默认监听 :11434
```

然后在 yaml 里写：

```yaml
- name: ollama-local
  type: openai-compatible
  baseURL: http://localhost:11434/v1
  apiKey: ollama        # 任意非空字符串即可，Ollama 不校验
  model: llama3.1
```

`apiKey` 写 `ollama` 或随便一段非空字符串都行 —— Ollama 不验签；习惯上仍填个占位字符串以防上游 `Authorization` 头解析报错。

## 自建 OpenAI 兼容服务

适用于 vLLM / LocalAI / LM Studio / 一线大厂自建网关等只要暴露 `/v1/chat/completions` 的服务：

```yaml
- name: internal-llm
  type: openai-compatible
  baseURL: http://your-host:8000/v1
  apiKey: ${INTERNAL_LLM_TOKEN}
  model: your-model-id
```

如果服务挂在子路径（如 `http://host/api/v1`），把整个前缀写到 `baseURL`：

```yaml
  baseURL: http://gateway.internal/api/v1
```

## provider 健康检查

`internal/llm/ping.go` 提供 `PingProvider` 与 `PingAll`：

```go
PingProvider(ctx, provider, timeoutSec) (PingStatus, error)
PingAll(ctx, providers, timeoutSec)    map[string]PingStatus
```

- 每个 provider 用 `GET <baseURL>` + `Authorization: Bearer <apiKey>` 打一次
- 单个 provider 超时由 `timeoutSec` 决定
- `PingAll` 并发跑所有 provider，统一收口到一张 map

5xx 视为失败（服务端不可用），4xx 视为成功（服务端可达，只是请求格式问题）。

`GET /healthz` 返回里 `providers[]` 每项的 `status` 字段取值：

| 实际状态 | `status` |
|----------|----------|
| ping OK | `enabled` |
| ping 失败 / `baseURL` 为空 | `disabled` |
| 还没 ping 过 | `unknown` |

UI 选单只显示 `enabled` 的 provider；`disabled` / `unknown` 灰掉并在 chat 框上方显示警告。

> 当前 `cmd/server/main.go` 还没在启动时调用 `PingAll`，所有 provider 首次会显示为 `unknown`，等首次 `GET /healthz` 命中且 UI 走一次 ping 后才更新。如果你需要在启动期就阻塞等 ping，参考 `internal/llm/ping.go` 在 `startup()` 里调用 `PingAll` 并把结果写进 `reg.Health`。

## 切换默认

改 `llm.default: <provider-name>` 即可。新建会话走新的默认 provider；已存在的会话仍用创建时的快照（见 `internal/agent/runner` 的 per-session state）。

## 多 provider 灰度

MVP 不做按 session / 按请求的路由 —— 同时只用一个 provider。但所有 `enabled` 的 provider 都会出现在 UI 顶部选单里，用户可以随手切。同一会话中途切 provider 会让 agent 的 tool-use 上下文丢一部分（不同模型的 tool schema 命名习惯不同），建议开新会话再切。

## 字段参考（`internal/config/config.go`）

```go
type LLMProvider struct {
    Name    string `yaml:"name"`
    Type    string `yaml:"type"`
    APIKey  string `yaml:"apiKey"`
    BaseURL string `yaml:"baseURL"`
    Model   string `yaml:"model"`
}
```

`model` 是 provider 自己识别的 model id（Anthropic 走 `claude-sonnet-4-6` 之类；Ollama 走 `llama3.1`；OpenAI 走 `gpt-4o`），没有统一目录。

## 下一步

- 多 provider 路由 / per-request cost cap：见 [brainstorm.md → Change 4 多用户长驻部署](../openspec/changes/k8s-natural-language-agent/brainstorm.md#后续-change-路线图参考)
- 完整 LLM 抽象设计：见 [design.md → D11 LLM 多 provider + 启动期 ping](../openspec/changes/k8s-natural-language-agent/design.md#d11-llm-多-provider--启动期-ping)
