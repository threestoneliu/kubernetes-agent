## Why

当前系统只能管理 K8s 资源（Pod/Deployment/Service），无法对节点（物理机/虚拟机）执行系统级操作。运维人员需要登录跳板机才能调整内核参数、重启服务、修改配置文件，风险高且无审计。通过独立节点运维面板，把这些操作结构化、可控、可审计。

## What Changes

**节点管理**
- From: 无节点管理能力
- To: 节点清单（K8s Nodes 自动同步 + 手动补充外部机器）+ 标签选择 + SSH 认证（Key 优先，password 兜底）

**操作执行**
- From: 无节点操作能力
- To: 结构化操作类型（sysctl / file_write / service_restart / shell / reboot）；高危操作（reboot / 磁盘）执行前预览 + CONFIRM 确认；并行/串行可选

**结果展示**
- From: 无
- To: 汇总视图（成功 N / 失败 M）+ 结构化摘要 + 原始 stdout/stderr

**审计**
- From: 无节点操作记录
- To: 最近 100 条操作记录自动淘汰

## Capabilities

### New Capabilities

- `node-inventory`: 节点清单管理（K8s 自动同步 + 手动录入 + 标签管理）
- `node-ssh-engine`: SSH 连接引擎（连接池 / 并发控制 / 结果聚合）
- `node-ops-tasks`: 结构化操作任务（类型定义 / 参数校验 / 执行模式）
- `node-ops-confirm`: 高危操作预览确认流程
- `node-ops-results`: 执行结果汇总视图

### Modified Capabilities

（无）

## Impact

- **新增目录**: `internal/ssh/` — SSH 客户端封装
- **新增 API**: `GET/POST/PUT/DELETE /api/nodes`, `/api/tasks`, `/api/runs`
- **新增页面**: `web/src/views/NodeOpsView.tsx`（节点运维独立面板）
- **存储**: 新增 `nodes` / `node_tasks` / `node_runs` 表
- **认证**: SSH Key 和 Password 均加密存储（AES-256-GCM，同 kubeconfig 方案）
