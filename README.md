# kubernetes-agent

本地单机的 Kubernetes 自然语言 agent。通过 Web UI 用自然语言查询与计划性操作 K8s 集群。

## 启动

```bash
make build
./kubernetes-agent
```

默认监听 `http://127.0.0.1:8080`。首次启动会引导你：

1. 上传 kubeconfig（自动 AES-256-GCM 加密落 SQLite）
2. 选 LLM provider（Anthropic / OpenAI / 本地 Ollama / 自建 OpenAI 兼容服务）
3. 在对话视图发自然语言指令，写操作会先出 Plan 预览，需确认后才执行

配置路径优先级：`KUBERNETES_AGENT_CONFIG` 环境变量 → `./configs/config.yaml` → `~/.kubernetes-agent/config.yaml`。
完整示例见 [`configs/config.example.yaml`](configs/config.example.yaml)。

## ⚠️ 备份警告

`~/.kubernetes-agent/master.key` 与 `~/.kubernetes-agent/data.db` **必须**一起备份，丢失任一即数据不可恢复。
master.key 也可通过环境变量 `KUBERNETES_AGENT_MASTER_KEY` 提供（32 字节 base64）。
`master.key` 权限必须是 `0600`，且拒绝以 root 身份自动生成。

## 文档

- [默认护栏规则](docs/default-policies.md) — 启动时自动种入的 4 条规则，以及如何自定义
- [LLM provider 配置](docs/llm-providers.md) — Anthropic / OpenAI / 本地 Ollama / 自建 OpenAI 兼容服务
- [开发模式](docs/dev-mode.md) — Vite + 后端分离热重载

## 路线图

本 change 落地了「读 + 写 + Plan 预览 + 护栏」的最小闭环。后续 change 路线图见
[openspec/changes/k8s-natural-language-agent/brainstorm.md](openspec/changes/k8s-natural-language-agent/brainstorm.md#后续-change-路线图参考)。

## 测试与构建

```bash
make test       # 跑全部 Go 测试
make vet        # go vet
make clean      # 清理 build 产物
```

Web 端校验：

```bash
cd web && pnpm typecheck   # TS 类型检查
cd web && pnpm build       # 生产构建（产物供 make build 嵌入）
```
