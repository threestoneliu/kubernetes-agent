# Node Ops Center — Brainstorm

## Design Summary

节点运维中心：主控通过 SSH 连接到目标节点，执行系统管理操作。独立操作面板，不走对话，通过结构化配置 + 预览确认执行。

## Alternatives Considered

### 方案 A：对话驱动（未采用）

- 用户聊天描述操作，LLM 解析成 SSH 命令
- **缺点**：SSH 命令高风险，LLM 解析错误后果严重；用户无法预览影响范围；对话不适合批量节点操作

### 方案 B：纯 YAML 声明式（未采用）

- 用户写 YAML 配置文件，Agent 解析执行
- **缺点**：学习成本高；错误难发现；没有 UI 可视化确认

### 方案 C：结构化表单 + 独立面板（采用）

- 页面配置操作类型（sysctl / file_write / service_restart / reboot）
- 标签选择节点集合
- 执行前预览确认（高危操作）
- 并行/串行可选
- **优点**：操作精确可控；用户无歧义；可批量；预览降低风险

## Agreed Approach

方案 C：独立操作面板 + 结构化表单 + SSH 直连执行。

## Key Decisions

- **SSH 连接方向**：主控 → 节点（Agent 主动 SSH 连入节点）
- **认证**：SSH Key 优先，password 兜底；均加密存在 DB
- **节点清单**：K8s Nodes 自动同步（IP/labels）+ 手动补充外部机器（IP + SSH 端口 + 认证信息）
- **节点选择**：标签选择器（env / role / zone 等）+ 手动增删节点
- **操作类型**：结构化 schema（sysctl / file_write / service_restart / shell）+ 高危操作（reboot / disk）强制预览确认
- **执行模式**：用户可选并行/串行
- **结果展示**：结构化摘要 + 原始 stdout/stderr；汇总视图（成功 N / 失败 M）
- **审计**：保留最近 100 条，自动淘汰
- **高危确认**：预览节点列表 + 影响分析（关联服务 / 运行环境）
- **Dry Run**：不采用（连接两次成本高）

## Open Questions

无。
