## Why

当前 kubernetes-agent 的 LLM Agent 缺乏结构化的工作流支持。用户与 Agent 的交互是开放式的，当需要执行复杂任务（如调试 Pod、部署应用）时，LLM 缺乏明确的执行指导，容易遗漏关键步骤或执行顺序不规范。

通过引入 Skill 系统，为常见 K8s 操作定义标准化工作流，让 LLM 能够根据用户意图自动匹配并执行，提升交互效率和执行质量。

## What Changes

**System Prompt 增强**
- From: System Prompt 仅包含基础角色定义和可用 Tools 列表
- To: System Prompt 额外注入 `<available_skills>` XML，包含所有已注册 Skill 的名称、描述和路径
- Reason: 让 LLM 知道有哪些工作流可用，支持意图匹配

**Skill 加载引擎**
- 新增 `internal/skills/` 包，负责扫描 Skill 目录、解析 SKILL.md、构建 `<available_skills>` XML
- 支持 `~/.kubernetes-agent/skills/` 作为 Skill 根目录
- 每个 Skill 包含完整目录结构：SKILL.md + REFERENCE.md + EXAMPLES.md + scripts/

**Skill 匹配与执行**
- LLM 根据 `<available_skills>` 中的 description 自行判断需要哪个 Skill
- LLM 调用 `fs_read` Tool 读取对应 SKILL.md 内容
- Tool 内容通过 Tool Result 追加到对话，LLM 按工作流执行

**新增 fs_read 工具**
- From: Agent 仅有 K8s 操作工具（k8s_get, k8s_list, k8s_describe, k8s_plan_write, k8s_execute_plan, k8s_ask_user）
- To: Agent 新增 `fs_read` 工具，支持读取本地文件系统（用于读取 Skill 文件）
- Reason: Skill 系统依赖读取本地 Skill 文件，需要文件系统访问能力
- Impact: 新增工具，不影响现有工具
- 访问范围：`~/.kubernetes-agent/` 下所有文件（安全限制，防止读取任意文件）

**内置初始 Skills**
- k8s-debug-pod: 调试 Pod 问题
- k8s-deploy-app: 部署应用到 Kubernetes
- k8s-scale-app: 扩缩容
- k8s-check-health: 健康检查
- k8s-cluster-inspect: 集群巡检

## Capabilities

### New Capabilities

- `skill-system`: Skill 系统的核心能力，包括 Skill 加载、System Prompt 注入、Skill 注册表管理
- `fs-read-tool`: 本地文件系统读取工具，用于读取 Skill 文件
- `k8s-debug-pod-skill`: Pod 调试工作流 Skill，定义调试 Pod 的标准流程
- `k8s-deploy-app-skill`: 应用部署工作流 Skill，定义部署应用到 K8s 的标准流程
- `k8s-scale-app-skill`: 扩缩容工作流 Skill
- `k8s-check-health-skill`: 健康检查工作流 Skill
- `k8s-cluster-inspect-skill`: 集群巡检工作流 Skill

### Modified Capabilities

（无）

## Impact

**新增代码**
- `internal/agent/fs_tool.go`: `fs_read` 工具实现，读取本地文件系统
- `internal/skills/types.go`: Skill 数据类型定义
- `internal/skills/loader.go`: Skill 目录扫描和加载
- `internal/skills/prompt.go`: System Prompt 构建，包含 `<available_skills>` XML 生成
- `~/.kubernetes-agent/skills/<skill-name>/`: 5 个初始 Skill 目录

**配置变更**
- 新增 `skills.dir` 和 `skills.enabled` 配置项（可选，有默认值）

**无 Breaking Changes**
- Skill 系统是纯新增功能，不影响现有 Agent Loop 和 Tools 行为
