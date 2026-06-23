## Context

系统目前没有定时执行能力，用户必须手动触发操作。chat/session 体系已完成，支持 SSE 流式输出和 SQLite 持久化。需要在此基础上增加定时任务能力。

## Goals / Non-Goals

**Goals:**
- 支持 cron 表达式（秒级精度）和一次性时间触发
- 任务持久化到 SQLite，应用重启不丢失
- 执行结果写入 session，用户在 chat 中可见
- 用户通过 UI 创建/管理任务
- LLM 通过工具动态创建/查询/删除任务，受 policy 约束

**Non-Goals:**
- 不支持外部 cron/Webhook 触发
- 不支持 distributed scheduling（多实例部署）
- 不支持任务结果的多样化通知（暂时只写 session）
- 不支持任务链/依赖执行

---

## Decisions

### 1. 表结构：`scheduled_tasks` + `scheduled_runs`

```sql
CREATE TABLE scheduled_tasks (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  cron_expr   TEXT,              -- nil = 一次性任务
  once_at     INTEGER,           -- UNIX timestamp，一次性任务执行时间
  session_id  TEXT NOT NULL,     -- 执行时向此 session 发消息
  enabled     INTEGER DEFAULT 1,
  created_by  TEXT,              -- 'user' or 'llm'
  cluster_id  TEXT,
  created_at  INTEGER,
  next_run    INTEGER,           -- UNIX timestamp，下次执行时间
  last_run    INTEGER,           -- UNIX timestamp，上次执行时间
  run_count   INTEGER DEFAULT 0
);

CREATE TABLE scheduled_runs (
  id       TEXT PRIMARY KEY,
  task_id  TEXT REFERENCES scheduled_tasks(id),
  run_at   INTEGER,
  status   TEXT,   -- 'success' / 'failed' / 'running'
  error    TEXT
);
```

### 2. `Scheduler` Goroutine

- Server 启动时在后台 goroutine 运行 `Scheduler.Run(ctx)`
- 主循环每 10 秒唤醒一次，检查到期任务（`next_run <= now AND enabled=1`）
- 使用 `github.com/robfig/cron/v3` 解析 cron 表达式（秒级精度）
- 重启时从 SQLite 恢复所有 `enabled=1` 的任务，按 `cron_expr` 计算下次 `next_run`
- 一次性任务执行后 `enabled=0`

### 3. 执行触发机制

- 到期时，调用 `Session.SendScheduledMessage(taskID, sessionID, msg)` 将消息写入 session
- 本质上相当于"用户向该 session 发送了一条消息"，触发现有的 agent loop
- 消息 `source="scheduled"` 写入 `messages` 表
- 执行结果（agent 回复）正常写入同一 session

### 4. LLM 工具接口

```
schedule_task(input: {
  name: string,
  cron_expr?: string,
  once_at?: number,     -- UNIX timestamp
  session_id: string,
  cluster_id?: string
}) -> { task_id, next_run }

get_scheduled_tasks() -> [{ id, name, cron_expr, once_at, session_id, enabled, next_run, last_run, run_count }]

delete_scheduled_task(input: { task_id: string }) -> { success: bool }
```

### 5. REST API

```
GET    /api/scheduled-tasks          -- 列出所有任务（支持 ?session_id= 过滤）
POST   /api/scheduled-tasks          -- 创建任务
DELETE /api/scheduled-tasks/:id       -- 删除任务
PATCH  /api/scheduled-tasks/:id       -- 更新任务（enabled/name/cron_expr）
POST   /api/scheduled-tasks/:id/run  -- 立即执行一次
```

### 6. UI

- Chat view 侧边栏新增 "定时任务" tab
- 任务列表：name、触发时间（cron 或 once）、下次执行时间、状态（启用/禁用）
- 操作：启用/禁用、立即执行、删除
- 创建弹窗：输入 cron 或选择时间 + 选择关联 session

### 7. Security / Policy

- LLM 创建任务受 `policy.Engine` 约束（与 k8s_execute_plan 同级）
- LLM 不能指定任意 `session_id`——必须属于同一个 cluster_id 或用户可访问的 session
- 用户创建的 task 直接允许

### 8. 并发冲突处理

- 如果定时触发时 session 正在被一个 SSE 连接使用（agent loop 正在运行）：
  - 该次触发 skip，不重试（`skip` 状态写入 `scheduled_runs`）
  - 下次 cron 周期继续（不会累积）
- 这避免了多 agent 实例并发操作同一 session 的竞态

---

## Risks / Trade-offs

- **[Risk] 应用重启期间的任务漏触发**：重启后 cron 不会补发，只能等下次周期。这在频繁重启的开发环境可能看不到效果。
  - **Mitigation**：可接受。重启漏触发由用户承担，重启后 Scheduler 自动恢复。

- **[Risk] 一次性任务（once_at）精度**：用 `next_run <= now` 轮询，最坏延迟等于轮询间隔（10s）。
  - **Mitigation**：对于用户指定时间的一次性任务，10s 延迟可接受。cron 任务不受影响。

- **[Risk] cron 秒级精度下的长时任务堆积**：如果 cron 间隔小于 agent loop 执行时间，任务会堆积。
  - **Mitigation**：Scheduler 检测到 task 正在 running 时 skip 下次触发，避免堆积。

- **[Trade-off] 单机调度**：重启后 cron 任务自动恢复，但如果机器时间不准，`next_run` 会漂移。
  - **Mitigation**：使用 `time.Now()` 计算，每次重启后重新对齐。

---

## Migration Plan

1. 新增 SQLite 表（向后兼容 ALTER TABLE）
2. 新增 `internal/scheduler/` 包，实现 Scheduler
3. 注册 `/api/scheduled-tasks` HTTP 路由
4. 注册 LLM 工具 `schedule_task` / `get_scheduled_tasks` / `delete_scheduled_task`
5. 前端新增定时任务 UI tab
6. 灰度：先只允许用户创建，观察无异常后开放 LLM 创建

---

## Open Questions

1. ~~LLM 创建任务的 security 边界~~ → 决策：session 必须属于同一 cluster_id，由 store 层验证
2. ~~并发冲突~~ → 决策：running 时 skip，标记 `skipped` 状态
3. ~~cron 秒级漏触发~~ → 决策：不补发，接受漏触发
4. **新增**：session 删除时，其上的定时任务如何处理？→ 建议：级联删除或禁止删除有活跃任务的 session
