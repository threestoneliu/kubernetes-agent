# Node Ops Center — Design

## Overview

节点运维中心是独立于对话界面的系统管理面板。用户通过结构化表单定义操作任务，选择目标节点，执行后查看汇总结果。高危操作执行前强制预览确认。

## Architecture

```
┌──────────────────────────────────────────────┐
│           Node Ops Center（Web UI）              │
│  节点清单 │ 新建任务 │ 执行结果 │ 设置           │
└──────────────┬──────────────┬───────────────┘
               │  REST API       │
┌──────────────▼──────────────▼───────────────┐
│         Agent（Go HTTP Server）                │
│  ┌────────────────────────────────────┐    │
│  │ SSH Client Pool                     │    │
│  │ 连接复用 │ 超时控制 │ 并发控制 │ 结果聚合│    │
│  └────────────────────────────────────┘    │
└──────────────────────┬─────────────────────┘
                       │ SSH
                  node-A  node-B  node-C（SSH 直连）
```

## Data Model

### Node

```go
type Node struct {
    ID         string    // UUID
    Name       string    // 显示名称（hostname 或 IP）
    Address    string    // SSH 地址（IP:port）
    Labels     []Label   // 标签（role=gpu, env=production）
    Auth       Auth      // SSH 认证信息（key 或 password）
    Source     string    // "k8s"（自动同步）或 "manual"（手动录入）
    CreatedAt  int64
    UpdatedAt  int64
}

type Auth struct {
    Type     string  // "key" 或 "password"
    Key      string  // 私钥内容（encrypted）
    Password string  // 密码（encrypted）
}

type Label struct {
    Key   string
    Value string
}
```

### Operation Task

```go
type OperationTask struct {
    ID          string
    Name        string           // 任务名称
    OpType      OperationType    // sysctl / file_write / service_restart / shell / reboot
    Target      Target           // 目标选择
    Params      TaskParams       // 操作参数（按类型定义）
    ExecMode    ExecMode        // parallel / sequential
    CreatedAt   int64
    CreatedBy   string
}

type Target struct {
    Labels []LabelFilter  // 标签选择器（AND 关系）
    Nodes  []string       // 手动增删的节点 ID
}

type LabelFilter struct {
    Key    string
    Values []string  // OR 匹配（env ∈ {production, staging})
}

type TaskParams struct {
    // sysctl
    Sysctl []SysctlEntry { Key, Value string }

    // file_write
    FilePath  string
    FileContent string

    // service_restart
    ServiceName string
    Action     string // restart / start / stop / status

    // shell
    Command string

    // reboot
    Delay int // 秒后重启
}
```

### Execution Record

```go
type TaskRun struct {
    ID           string
    TaskID       string
    TriggeredBy  string  // "manual" / "scheduled"
    Status       RunStatus
    StartedAt    int64
    CompletedAt  int64
    Results      []NodeResult
}

type NodeResult struct {
    NodeID     string
    NodeName   string
    Status     NodeRunStatus // success / failed / skipped
    Summary    ResultSummary  // 结构化摘要
    RawOutput  string        // 原始 stdout/stderr
    Error      string         // 错误信息
}
```

## Component Design

### NodeListView

左侧面板，展示节点清单。

**数据源：**
- K8s Nodes：定时同步（通过 kubectl get nodes 获取 IP/labels）
- 手动录入：用户新增的外部节点

**UI 交互：**
- 按标签筛选节点
- 节点卡片：显示 hostname/IP + 在线状态 + 标签
- 新增节点：表单（地址 + 端口 + 认证信息）
- 编辑/删除手动录入的节点（K8s 同步节点不可删除）

### TaskFormView

新建/编辑任务。

**字段：**
1. 任务名称（文本框）
2. 操作类型（下拉选择）
3. 目标节点（标签选择器 + 手动增删）
4. 参数（根据类型动态显示）
5. 执行模式（并行/串行单选）

**动态参数面板：**
- sysctl：键值对列表（+ 添加行）
- file_write：路径 + 文件内容编辑器
- service_restart：服务名 + 操作（restart/start/stop）
- shell：命令文本框
- reboot：延迟秒数

### ConfirmModal（高危操作预览）

reboot / disk 类操作执行前弹出。

**展示内容：**
- 节点列表（hostname + IP）
- 操作摘要（类型 + 参数）
- 影响分析：该节点运行的关键服务 / 关联 K8s Workload
- 用户输入 CONFIRM 才执行

### ExecutionResultView

执行结果页。

**汇总视图：**
```
┌─────────────────────────────┐
│ 成功: 8  失败: 2  总计: 10  │
│ ████████████████████░░░░ │
└─────────────────────────────┘
```

**展开节点详情：**
```
node-A (10.1.1.1)
  ✓ sysctl: net.ipv4.tcp_keepalive=60
    原始输出: ...
node-B (10.1.1.2)
  ✗ sysctl: net.ipv4.tcp_keepalive=60
    错误: sysctl: setting not found
    原始输出: ...
```

## SSH Engine

**连接管理：**
- 连接池复用（每节点一个 SSH 连接，超时 30s）
- 并发数可配置（默认 10 并发）

**执行流程：**
```
1. 解析 Target → 展开为节点列表
2. 对每台节点建立 SSH 连接
3. 根据 OpType 执行对应命令
4. 收集结果，聚合返回
5. 结构化输出 + 原始输出
```

**命令映射：**
| OpType | 执行命令 |
|--------|---------|
| sysctl | `sysctl -w key=value` |
| service_restart | `systemctl action service_name` |
| file_write | `cat > path << EOF content EOF` |
| shell | 直接执行 |
| reboot | `shutdown -r +delay` 或 `reboot` |

## Files to Modify

- `internal/ssh/` — 新增 SSH 客户端引擎
- `internal/store/nodes.go` — 节点存储
- `internal/store/tasks.go` — 任务存储
- `internal/server/handler_nodes.go` — 节点 CRUD HTTP handler
- `internal/server/handler_tasks.go` — 任务 HTTP handler
- `internal/server/handler_runs.go` — 执行结果 handler
- `web/src/views/NodeOpsView.tsx` — 节点运维面板（节点列表 + 任务 + 结果）
- `web/src/api.ts` — 新增节点/任务相关 API

## Testing

- 新建节点 → SSH 连通性检查
- 执行 sysctl → 验证节点参数变更
- 执行 reboot → 预览确认弹窗出现
- 并行执行 10 节点 → 汇总视图正确
