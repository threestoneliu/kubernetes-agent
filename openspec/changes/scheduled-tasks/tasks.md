## 1. 数据库层

- [x] 1.1 在 `internal/store/` 新增 `scheduled_tasks` 和 `scheduled_runs` 表的 schema 定义和 migration
- [x] 1.2 在 `store.go` 新增 `ScheduledTask` struct 和 CRUD 方法（`CreateScheduledTask`、`GetScheduledTasks`、`GetScheduledTask`、`UpdateScheduledTask`、`DeleteScheduledTask`）
- [x] 1.3 新增 `ScheduledRun` struct 和 `CreateScheduledRun`、`UpdateScheduledRun` 方法

## 2. Scheduler 核心

- [ ] 2.1 新增 `internal/scheduler/scheduler.go`，实现 `Scheduler` 结构体和 `Run(ctx)` 方法
- [ ] 2.2 实现 `Scheduler.schedule()` 主循环，每 10s 检查一次到期任务
- [ ] 2.3 实现 `Scheduler.restore()` 从 SQLite 恢复所有 `enabled=1` 任务
- [ ] 2.4 集成 `github.com/robfig/cron/v3`，实现 `nextRunFromCron()` 和 `nextRunFromOnce()`
- [ ] 2.5 实现 `Scheduler.trigger()` 执行任务——调用 `Session.SendScheduledMessage()` 写入消息
- [ ] 2.6 实现 `Scheduler.skip()` 记录 `skipped` 状态到 `scheduled_runs`
- [ ] 2.7 Server 启动时启动 Scheduler goroutine（参考 cmd/server/main.go 启动方式）

## 3. REST API

- [ ] 3.1 在 `internal/server/` 注册 `GET/POST/DELETE/PATCH /api/scheduled-tasks` 路由
- [ ] 3.2 实现 `GET /api/scheduled-tasks`（支持 `?session_id=` 过滤）
- [ ] 3.3 实现 `POST /api/scheduled-tasks`（创建任务，计算 `next_run`）
- [ ] 3.4 实现 `DELETE /api/scheduled-tasks/:id`
- [ ] 3.5 实现 `PATCH /api/scheduled-tasks/:id`（更新 `enabled`/`name`/`cron_expr`）
- [ ] 3.6 实现 `POST /api/scheduled-tasks/:id/run`（立即执行一次）

## 4. LLM 工具

- [ ] 4.1 在 `internal/agent/tools.go` 注册 `schedule_task` 工具（schema + handler）
- [ ] 4.2 在 `internal/agent/tools.go` 注册 `get_scheduled_tasks` 工具
- [ ] 4.3 在 `internal/agent/tools.go` 注册 `delete_scheduled_task` 工具
- [ ] 4.4 实现 session 归属验证（`schedule_task` 中验证 `session_id` 属于同一 `cluster_id`）
- [ ] 4.5 更新 `internal/llm/prompt.go`，在能力列表中添加 `schedule_task` 工具说明

## 5. 前端 UI

- [ ] 5.1 在 `web/src/views/ChatView.tsx` 侧边栏新增"定时任务" tab
- [ ] 5.2 实现 `ScheduledTasksPanel` 组件：任务列表展示（name、触发时间、下次执行、状态）
- [ ] 5.3 实现"创建任务"弹窗：输入 name、cron/时间选择器、选择关联 session
- [ ] 5.4 实现任务操作按钮：启用/禁用（`PATCH`）、立即执行（`POST .../run`）、删除（`DELETE`）
- [ ] 5.5 在 `web/src/api.ts` 添加 `getScheduledTasks`、`createScheduledTask`、`updateScheduledTask`、`deleteScheduledTask` API 调用

## 6. Session 消息增强

- [ ] 6.1 在 `store.go` 的 `Message` struct 新增 `source` 字段（`"user"` / `"llm"` / `"scheduled"`）
- [ ] 6.2 `Session.SendScheduledMessage()` 向 `messages` 表写入 `source="scheduled"` 的消息
- [ ] 6.3 前端渲染 `source="scheduled"` 的消息时显示"🔄 定时任务"标记

## 7. 集成与测试

- [ ] 7.1 端到端测试：创建 cron 任务，等待触发，验证结果写入 session
- [ ] 7.2 端到端测试：创建一次性任务，验证执行后 `enabled=0`
- [ ] 7.3 端到端测试：session 正在使用时触发 skip，验证 `scheduled_runs.status="skipped"`
- [ ] 7.4 端到端测试：Server 重启后任务恢复
