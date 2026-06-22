# scheduled-tasks Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development
> to implement this plan task-by-task.

**Goal:** 实现定时任务系统，支持 cron/一次性时间触发，任务持久化到 SQLite，执行结果写入 session

**Architecture:** `Scheduler` goroutine 管理所有定时任务，依赖 SQLite 持久化和 `robfig/cron/v3`；执行时通过 `Session.SendScheduledMessage()` 写入消息触发 agent loop；REST API 和 LLM 工具两条路径操作任务

**Tech Stack:** Go (backend), SQLite, React/TypeScript (frontend), `github.com/robfig/cron/v3`

---

## Task 1: 数据库层（foundation）

**依赖：** 无

### 1.1 添加 SQLite 表

- [ ] **Step 1:** 在 `internal/store/schema.sql` 添加：
```sql
CREATE TABLE IF NOT EXISTS scheduled_tasks (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  cron_expr   TEXT,
  once_at     INTEGER,
  session_id  TEXT NOT NULL,
  enabled     INTEGER DEFAULT 1,
  created_by  TEXT,
  cluster_id  TEXT,
  created_at  INTEGER,
  next_run    INTEGER,
  last_run    INTEGER,
  run_count   INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS scheduled_runs (
  id       TEXT PRIMARY KEY,
  task_id  TEXT REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
  run_at   INTEGER,
  status   TEXT,
  error    TEXT
);
```

### 1.2 添加 Store CRUD

- [ ] **Step 1:** 在 `internal/store/store.go` 添加 `ScheduledTask` 和 `ScheduledRun` struct
- [ ] **Step 2:** 添加 `CreateScheduledTask(task *ScheduledTask) error`
- [ ] **Step 3:** 添加 `GetScheduledTasks(ctx, sessionID string) ([]*ScheduledTask, error)` 支持按 session_id 过滤（空字符串返回全部）
- [ ] **Step 4:** 添加 `GetScheduledTask(id string) (*ScheduledTask, error)`
- [ ] **Step 5:** 添加 `UpdateScheduledTask(id string, updates map[string]any) error`
- [ ] **Step 6:** 添加 `DeleteScheduledTask(id string) error`
- [ ] **Step 7:** 添加 `CreateScheduledRun(run *ScheduledRun) error`
- [ ] **Step 8:** 添加 `UpdateScheduledRun(id string, status, errorMsg string) error`
- [ ] **Step 9:** 添加 `GetEnabledScheduledTasks() ([]*ScheduledTask, error)` 启动恢复用

**验证命令：** `go build ./internal/store/...`

---

## Task 2: Session 消息增强（foundation）

**依赖：** Task 1

### 2.1 Message source 字段

- [ ] **Step 1:** 在 `store.go` 的 `Message` struct 添加 `Source string` 字段（`"user"` / `"llm"` / `"scheduled"`）
- [ ] **Step 2:** 在 `InsertMessage` 时支持 `source` 参数
- [ ] **Step 3:** 添加 `Session.SendScheduledMessage(taskID, msg string) error`，写入 `source="scheduled"` 的消息

**验证命令：** `go test ./internal/store/... -v -run Scheduled`

---

## Task 3: Scheduler 核心

**依赖：** Task 1 + Task 2

### 3.1 Scheduler 结构体

- [ ] **Step 1:** 创建 `internal/scheduler/scheduler.go`
- [ ] **Step 2:** 定义 `Scheduler` struct：`store *store.DB`, `sessions *SessionManager`, `cron *cron.Cron`, `tasks map[string]*ScheduledTask`, `mu sync.Mutex`
- [ ] **Step 3:** 添加 `NewScheduler(store, sessions) *Scheduler`

### 3.2 nextRun 计算

- [ ] **Step 1:** 添加 `nextRunFromCron(expr string, from time.Time) (time.Time, error)` 使用 `robfig/cron/v3` 解析
- [ ] **Step 2:** 添加 `nextRunFromOnce(ts int64) time.Time`

### 3.3 主循环

- [ ] **Step 1:** 实现 `Restore(ctx context.Context)` 从 SQLite 加载 `enabled=1` 的任务并加入 cron
- [ ] **Step 2:** 实现 `Run(ctx context.Context)` 启动 Restore，然后 `select {}` 阻塞或用 ticker 每 10s 检查
- [ ] **Step 3:** 实现 `trigger(task *ScheduledTask)` 方法：创建 `scheduled_runs` 记录（status=running）→ 调用 `session.SendScheduledMessage()` → 更新 `last_run`、`run_count`、`next_run` → 更新 `scheduled_runs` status=success
- [ ] **Step 4:** 实现 `skip(task *ScheduledTask, reason string)` 记录 skipped 状态
- [ ] **Step 5:** 实现 `ScheduleTask(task *ScheduledTask)` 将任务加入 cron调度
- [ ] **Step 6:** 实现 `UnscheduleTask(taskID string)` 从 cron 移除

### 3.4 Server 集成

- [ ] **Step 1:** 在 `cmd/server/main.go` 启动时创建 Scheduler 并调用 `scheduler.Run(ctx)`

**验证命令：** `go build ./internal/scheduler/... && go build ./cmd/server/...`

---

## Task 4: REST API

**依赖：** Task 1

### 4.1 路由注册

- [ ] **Step 1:** 在 `internal/server/server.go` 添加路由：
```go
r.Route("/api/scheduled-tasks", func(r chi.Router) {
    r.Get("/", handleGetScheduledTasks)
    r.Post("/", handleCreateScheduledTask)
    r.Route("/{id}", func(r chi.Router) {
        r.Delete("/", handleDeleteScheduledTask)
        r.Patch("/", handleUpdateScheduledTask)
        r.Post("/run", handleRunScheduledTask)
    })
})
```

### 4.2 Handler 实现

- [ ] **Step 1:** `handleGetScheduledTasks`：调用 `store.GetScheduledTasks(sessionID)` 返回列表
- [ ] **Step 2:** `handleCreateScheduledTask`：解析 body，计算 `next_run`，调用 `store.CreateScheduledTask`，调用 `scheduler.ScheduleTask`
- [ ] **Step 3:** `handleDeleteScheduledTask`：调用 `scheduler.UnscheduleTask` + `store.DeleteScheduledTask`
- [ ] **Step 4:** `handleUpdateScheduledTask`：支持 `enabled`、`name`、`cron_expr` 更新；`enabled=0` 时 unschedule
- [ ] **Step 5:** `handleRunScheduledTask`：立即调用 `scheduler.Trigger()`

**验证命令：** `go build ./internal/server/...`

---

## Task 5: LLM 工具

**依赖：** Task 4

### 5.1 工具注册

- [ ] **Step 1:** 在 `internal/agent/tools.go` 添加 `schedule_task` schema：
```go
"schedule_task": map[string]any{
    "type": "object",
    "properties": map[string]any{
        "name":        map[string]any{"type": "string"},
        "cron_expr":  map[string]any{"type": "string"},
        "once_at":     map[string]any{"type": "number"},
        "session_id": map[string]any{"type": "string"},
        "cluster_id":  map[string]any{"type": "string"},
    },
    "required": []string{"name", "session_id"},
}
```
- [ ] **Step 2:** 实现 `schedule_task` handler：验证 session 归属 → 调用 store → 调用 scheduler
- [ ] **Step 3:** 添加 `get_scheduled_tasks` handler（无参数）
- [ ] **Step 4:** 添加 `delete_scheduled_task` handler

### 5.2 Prompt 更新

- [ ] **Step 1:** 更新 `internal/llm/prompt.go`，在能力列表添加 `定时: schedule_task / get_scheduled_tasks / delete_scheduled_task`

**验证命令：** `go build ./internal/agent/...`

---

## Task 6: 前端 UI

**依赖：** Task 4

### 6.1 API 层

- [ ] **Step 1:** 在 `web/src/api.ts` 添加：
```typescript
export const getScheduledTasks = () => fetch('/api/scheduled-tasks')
export const createScheduledTask = (data) => fetch('/api/scheduled-tasks', { method: 'POST', body: JSON.stringify(data) })
export const updateScheduledTask = (id, data) => fetch(`/api/scheduled-tasks/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deleteScheduledTask = (id) => fetch(`/api/scheduled-tasks/${id}`, { method: 'DELETE' })
export const runScheduledTask = (id) => fetch(`/api/scheduled-tasks/${id}/run`, { method: 'POST' })
```

### 6.2 UI 组件

- [ ] **Step 1:** 在 ChatView 侧边栏添加"定时任务" tab 切换按钮
- [ ] **Step 2:** 实现 `ScheduledTasksPanel`：调用 `getScheduledTasks` 展示列表
- [ ] **Step 3:** 实现"创建任务"弹窗：表单字段（name、cron_expr/once_at、session 选择器）
- [ ] **Step 4:** 实现操作按钮：启用/禁用、立即执行、删除
- [ ] **Step 5:** 渲染时对 `source="scheduled"` 的消息显示"🔄"标记

**验证命令：** `cd web && pnpm build`

---

## Task 7: 集成测试

**依赖：** Task 1-6 全部完成

- [ ] **Step 1:** `go test ./internal/store/... -run Scheduled -v`
- [ ] **Step 2:** 手动测试：创建 cron 任务，等 1 分钟，观察 session 中出现定时消息
- [ ] **Step 3:** 手动测试：创建一次性任务，确认执行后 `enabled=0`
- [ ] **Step 4:** 手动测试：Server 重启后 `GET /api/scheduled-tasks` 返回已恢复的任务

---

**Commit after each task group:**
```bash
git add -A && git commit -m "feat: scheduled-tasks <description> [task N]"
```
