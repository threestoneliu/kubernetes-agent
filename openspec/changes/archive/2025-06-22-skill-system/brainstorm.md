## Design Summary

为 kubernetes-agent 设计一套 Skill 系统，让 LLM 能够根据用户意图匹配并执行特定工作流。

## Core Design

### 核心机制

1. **触发方式**：LLM 意图匹配（根据 `<available_skills>` 中的 description）
2. **Skill 读取**：LLM 调用 Read Tool 读取 `~/.kubernetes-agent/skills/<name>/SKILL.md`
3. **内容注入**：Read Tool Result 追加到对话，LLM 按 skill 工作流执行

### 工作流程

```
用户: "帮我 debug nginx-pod"
         │
         ▼
┌─────────────────────────────────────────┐
│  System Prompt 包含 <available_skills>   │
│  LLM 看到可用 Skills                    │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│  LLM 识别到 "debug" 匹配 k8s-debug-pod │
│  → 调用 Read Tool                       │
│    Tool: "Read"                         │
│    args: { path: "~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md" } │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│  Read Tool 返回 SKILL.md 内容           │
│  → 内容作为 Tool Result 追加到对话      │
│  → LLM 看到 skill 内容，按工作流执行   │
└─────────────────────────────────────────┘
```

## Alternatives Considered

### 方案 A：LLM Matcher + Skill Tool（采用）

- **做法**：LLM 根据 `<available_skills>` 自行判断需要哪个 Skill，调用 Read Tool 读取内容
- **优点**：灵活，无需预定义匹配规则；复用现有 Read Tool
- **缺点**：依赖 LLM 的判断准确性
- **为何采用**：简洁、与 Claude Code 设计一致

### 方案 B：Go 代码 Matcher + 直接注入

- **做法**：Go 代码做关键词/向量匹配，Skill 内容直接注入 System Prompt
- **优点**：确定性强，不依赖 LLM
- **缺点**：需要维护匹配规则，侵入性更强
- **为何未采用**：增加复杂度，且不够灵活

### 方案 C：显式命令触发

- **做法**：用户输入 `/k8s-debug-pod nginx-pod`
- **优点**：精确控制
- **缺点**：用户体验不如意图触发自然
- **为何未采用**：不够自然，需要用户记忆命令

## Agreed Approach

采用 **方案 A：LLM Matcher + Skill Tool**

- 使用现有 Read Tool 读取 Skill 内容，无需新增 Tool
- LLM 根据 description 自行意图匹配
- Skill 内容通过 Tool Result 追加到对话
- 符合 Claude Code 的 Skill 设计模式

## Key Decisions

1. **Skill 存储位置**：`~/.kubernetes-agent/skills/`（用户级配置目录）
2. **Skill 结构**：完整目录结构（SKILL.md + REFERENCE.md + EXAMPLES.md + scripts/）
3. **System Prompt 注入**：`<available_skills>` 默认始终注入
4. **匹配方式**：LLM 意图匹配，不做精确关键词匹配

## Initial Skills

| Skill | 描述 |
|-------|------|
| `k8s-debug-pod` | 调试 Pod 问题 |
| `k8s-deploy-app` | 部署应用到 Kubernetes |
| `k8s-scale-app` | 扩缩容 |
| `k8s-check-health` | 健康检查 |
| `k8s-cluster-inspect` | 集群巡检 |

## Directory Structure

```
~/.kubernetes-agent/skills/
├── k8s-debug-pod/
│   ├── SKILL.md
│   ├── REFERENCE.md
│   ├── EXAMPLES.md
│   └── scripts/
├── k8s-deploy-app/
│   └── ...
├── k8s-scale-app/
│   └── ...
├── k8s-check-health/
│   └── ...
└── k8s-cluster-inspect/
    └── ...
```

## Configuration

```yaml
skills:
  dir: "~/.kubernetes-agent/skills"
  enabled: true
```

## Open Questions

无

## System Prompt Format

```xml
<available_skills>
  <skill>
    <name>k8s-debug-pod</name>
    <description>Debug Kubernetes pod issues. Use when user wants to debug, troubleshoot, or diagnose a pod problem.</description>
    <location>~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md</location>
  </skill>
  ...
</available_skills>
```
