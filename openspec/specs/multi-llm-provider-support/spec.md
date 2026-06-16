# multi-llm-provider-support Specification

## Purpose
TBD - created by archiving change k8s-natural-language-agent. Update Purpose after archive.
## Requirements
### Requirement: 多 provider 类型
系统 MUST 支持以下三类 LLM provider,经由 `charmbracelet/fantasy` 抽象:`anthropic`(Anthropic Claude,含 extended thinking) · `openai`(OpenAI GPT) · `openai-compatible`(任何兼容 OpenAI 协议的后端,如 Ollama、vLLM、本地 llama.cpp server)。

#### Scenario: Anthropic 配置
- **WHEN** `config.yaml` 含 `type: anthropic` 的 provider
- **THEN** 系统 MUST 使用 fantasy 的 Anthropic adapter,API base 为 Anthropic 官方,认证使用 `x-api-key` header

#### Scenario: OpenAI 兼容配置
- **WHEN** `config.yaml` 含 `type: openai-compatible` 的 provider
- **THEN** 系统 MUST 使用 fantasy 的 OpenAI 兼容 adapter,`baseURL` 与 `apiKey` 来自配置,缺一 MUST 启动失败

### Requirement: 配置驱动
所有 provider MUST 来自 `~/.kubernetes-agent/config.yaml` 的 `llm.providers` 列表,MUST NOT 在代码中硬编码 provider。

#### Scenario: 配置缺失
- **WHEN** `config.yaml` 无 `llm` 段或 `providers` 为空
- **THEN** 启动 MUST 失败并 stderr 提示"至少需要一个 LLM provider"

#### Scenario: 环境变量展开
- **WHEN** provider 配置中 `apiKey` 形如 `${ANTHROPIC_API_KEY}`
- **THEN** 系统 MUST 在启动时展开为环境变量值,未设置 MUST 启动失败

### Requirement: 启动期连通性校验
服务启动 MUST 并发 ping 所有配置的 provider,失败 MUST 标记为 `disabled` 且 MUST NOT 出现在 UI 选单。

#### Scenario: 部分 provider 失败
- **WHEN** 启动期 ping 时 Anthropic 成功,OpenAI 超时
- **THEN** Anthropic MUST 出现在 UI 选单,OpenAI MUST 标 `disabled` 并在 tooltip 展示"启动期不可达"

#### Scenario: 全部 provider 失败
- **WHEN** 启动期 ping 时所有 provider 失败
- **THEN** 启动 MUST 失败并 stderr 列出每个 provider 的失败原因,服务 MUST NOT 启动

### Requirement: 默认 provider
`config.yaml` 的 `llm.default` MUST 指定一个已配置且 ping 成功的 provider 作为默认,缺省或指向失败 provider MUST 启动失败。

#### Scenario: 默认可用
- **WHEN** 用户首次进入对话 UI 且未切换 provider
- **THEN** 系统 MUST 使用 `llm.default` 指定的 provider

#### Scenario: 默认失败
- **WHEN** `llm.default` 指向的 provider 在启动期失败
- **THEN** 启动 MUST 失败并 stderr 提示"默认 provider 不可达"

### Requirement: System prompt 模板
系统 MUST 注入统一 system prompt 模板,内容包含:身份声明 · 工具集说明 · 写工作流(plan→confirm→execute) · 风格约束 · 默认中文。

#### Scenario: 启动首轮调用
- **WHEN** agent 循环首次向 LLM 发起请求
- **THEN** 请求 MUST 包含 system prompt 作为首条 message,且 MUST 早于任何 user 消息

### Requirement: 工具注册抽象
5 个 K8s 工具 + `ask_user` 工具 MUST 全部注册到 fantasy agent 循环,且 MUST 按 LLM 工具调用协议暴露 JSON schema。

#### Scenario: LLM 看到工具清单
- **WHEN** 首次 LLM 请求
- **THEN** 请求 MUST 包含所有 6 个工具的 schema 描述,LLM MUST 能根据 `name` 字段调用

### Requirement: 流式 token 输出
所有 LLM 调用 MUST 使用流式响应,且后端 MUST 把每个 token 推为 SSE `token` 事件。

#### Scenario: LLM 流式响应
- **WHEN** LLM 返回流式响应
- **THEN** 系统 MUST 逐 token 推 `token` 事件,事件顺序 MUST 与 LLM 产生顺序一致

#### Scenario: Extended thinking
- **WHEN** provider 模型支持且启用 extended thinking
- **THEN** thinking 内容 MUST 推 `reasoning` 事件(非 `token` 事件),且 MUST 与文本 token 顺序交错

### Requirement: 错误重试
LLM 调用 MUST 对以下错误重试 1 次(指数退避 1 秒):`429` 限流 · `5xx` 服务端错误,且 MUST NOT 重试:`401`/`403` 认证错误 · `400` 客户端错误。

#### Scenario: 429 重试
- **WHEN** 首次 LLM 调用返回 429
- **THEN** 系统 MUST 等待 1 秒后重试,二次仍 429 MUST 推 `error` 事件含 `code: llm_rate_limit` + `retryable: true`

#### Scenario: 401 不重试
- **WHEN** LLM 调用返回 401
- **THEN** 系统 MUST 立即推 `error` 事件含 `code: llm_auth` + `retryable: false`,UI MUST 提示检查 API key

