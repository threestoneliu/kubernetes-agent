## Design Summary

定时任务系统：用户和 LLM 都可以通过 cron 表达式或一次性时间创建任务，持久化到 SQLite，执行结果写入对应 session。

### 核心概念

**ScheduledTask** = 任务元数据（id, name, cron/once, target, cluster_id, created_by, enabled）+ 执行记录（last_run, next_run, run_count）

**触发方式：**
- **Cron 表达式**：`* * * * *` 格式，支持秒级（`cron := cron.New(cron.WithSeconds())`）
- **一次性时间**：指定 `next_run` 时间戳，执行后状态变为 `completed`

**执行内容：**
- **Chat 消息**：像用户发消息一样触发 agent loop，`source="scheduled"`，结果写入该 session
- **执行 Plan/Session**：直接调用已保存的 plan 或 session 脚本

**持久化：** SQLite `scheduled_tasks` 表 + `scheduled_runs` 表

**通知：** 执行结果写入 `messages` 表（source="scheduled"），用户可在 chat 中看到

**权限：**
- **用户**：通过 UI 创建/编辑/删除定时任务（chat view 侧边栏"定时任务"tab）
- **LLM**：通过 `schedule_task` 工具动态创建/查询/删除任务（受 policy 约束）

---

## Alternatives Considered

### 方案 A：独立任务队列 + Agent Polling
- **做法**：定时任务独立于 chat，有自己的任务队列和 worker goroutine，轮询检查到期任务
- **优点**：与 agent loop 解耦，架构清晰
- **缺点**：结果通知需要独立机制（不能复用 session），执行上下文需要序列化/反序列化

### 方案 B：基于 Session 的任务（推荐）
- **做法**：定时任务绑定一个 session_id，执行时像用户发消息一样触发 agent loop，结果写入该 session
- **优点**：天然支持流式输出（SSE）、历史记录、多轮对话上下文
- **缺点**：session 必须是"可恢复状态"，需要考虑 agent loop 正在运行时的冲突

### 方案 C：外部 Cron + Webhook
- **做法**：依赖外部 cron jobs（如 Linux cron），通过 webhooks 触发
- **优点**：稳定可靠，不受应用重启影响
- **缺点**：需要额外部署，架构复杂度高，与当前单体部署不符

---

## Agreed Approach

**方案 B**：基于 Session 的任务。

理由：
1. 结果天然写入 session，用户在 chat 中直接可见，无需单独的通知机制
2. 复用现有的 agent loop，无需为定时任务新建执行上下文
3. SQLite 持久化，应用重启后任务不丢失，重启时恢复 schedule
4. LLM 可通过 `schedule_task` 工具动态创建，受现有 policy engine 约束

---

## Key Decisions

### 1. 表结构

```sql
CREATE TABLE scheduled_tasks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  cron_expr TEXT,           -- nil 表示一次性任务
  once_at INTEGER,          -- UNIX timestamp，一次性任务的执行时间
  session_id TEXT NOT NULL, -- 关联的 session，执行时发消息到这个 session
  enabled INTEGER DEFAULT 1,
  created_by TEXT,          -- 'user' or 'llm'
  cluster_id TEXT,
  created_at INTEGER,
  next_run INTEGER,        -- UNIX timestamp，下次执行时间
  last_run INTEGER,         -- UNIX timestamp，上次执行时间
  run_count INTEGER DEFAULT 0
);

CREATE TABLE scheduled_runs (
  id TEXT PRIMARY KEY,
  task_id TEXT REFERENCES scheduled_tasks(id),
  run_at INTEGER,           -- 实际执行时间
  status TEXT,              -- 'success' / 'failed' / 'running'
  error TEXT
);
```

### 2. Scheduler 实现

- `Scheduler` 在 server 启动时启动（goroutine），周期（每 10s）检查到期任务
- 使用 `robfig/cron/v3` 解析 cron 表达式
- 重启时从 SQLite 恢复所有 `enabled=1` 的任务（根据 cron_expr 计算下次 next_run）
- 执行：触发 agent loop，结果通过 SSE 推送给连接的客户端

### 3. LLM 工具接口

```
schedule_task(input: {
  name: string,
  cron_expr?: string,    -- nil 则用 once_at
  once_at?: number,      -- UNIX timestamp
  session_id: string,
  cluster_id?: string
}) -> { task_id, next_run }
```

### 4. UI

- Chat view 侧边栏新增"定时任务"tab
- 列出当前任务，支持：启用/禁用、立即执行、删除
- 创建任务：指定 cron 或时间 + 选择关联 session

---

## Open Questions

1. **LLM 创建任务的 security 边界**：LLM 通过 `schedule_task` 能否指定任意 session_id？是否需要验证 session 属于同一个 user/cluster？
2. **并发冲突**：定时触发时，如果该 session 正在被另一个 SSE 连接使用，怎么办？（queue？还是 skip？）
3. **cron 秒级精度**：生产环境 cron 精度是秒级，但 agent loop 执行时间可能较长，如何处理漏触发？
