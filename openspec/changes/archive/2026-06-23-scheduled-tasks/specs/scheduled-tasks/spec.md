# scheduled-tasks

## ADDED Requirements

### Requirement: 定时任务SHALL支持cron表达式和一次性时间两种触发方式

(MUST) cron 表达式支持秒级精度（6 位格式），一次性任务使用 UNIX 时间戳指定执行时间。

#### Scenario: 用户通过 UI 创建 cron 任务
- **WHEN** 用户在定时任务 tab 输入 cron 表达式 `"0 9 * * *"` 并选择关联 session
- **THEN** 系统 SHALL 创建任务，下次执行时间为次日 09:00，并持久化到 SQLite

#### Scenario: 用户通过 UI 创建一次性任务
- **WHEN** 用户指定一次性执行时间为未来某个时间戳
- **THEN** 系统 SHALL 创建任务，执行后该任务状态变为 disabled

#### Scenario: LLM 通过工具创建 cron 任务
- **WHEN** LLM 调用 `schedule_task` 并传入 `cron_expr: "0 9 * * *"` 和 `session_id`
- **THEN** 系统 SHALL 验证 session 归属，受 policy 约束，并通过 SSE 推送执行结果到该 session

---

### Requirement: 定时任务SHALL持久化到SQLite并在重启后恢复

(MUST) 所有 `enabled=1` 的任务在 Scheduler 重启后自动恢复，根据 cron 表达式重新计算 `next_run`。

#### Scenario: Server 重启后恢复定时任务
- **WHEN** Server 重启，Scheduler 初始化
- **THEN** 系统 SHALL 从 SQLite 加载所有 `enabled=1` 的任务，重新计算 `next_run` 并加入调度队列

#### Scenario: 任务执行后更新状态
- **WHEN** 定时任务执行完成（成功或失败）
- **THEN** 系统 SHALL 更新 `scheduled_tasks.last_run`、`scheduled_tasks.run_count` 以及 `scheduled_runs` 表记录

---

### Requirement: 定时任务执行结果SHALL写入对应session

(MUST) 执行时向关联 session 写入消息（`source="scheduled"`），触发 agent loop，用户在 chat 中看到执行结果。

#### Scenario: 定时任务触发并写入 session
- **WHEN** Scheduler 检测到任务到期（`next_run <= now AND enabled=1`）
- **THEN** 系统 SHALL 向该任务的 `session_id` 写入一条 `source="scheduled"` 的消息，触发 agent loop 处理

#### Scenario: 执行结果写入 session 消息
- **WHEN** agent loop 处理完定时任务产生的消息
- **THEN** agent 的回复 SHALL 作为普通消息写入同一 session，`source="scheduled"` 标记该消息为定时任务产生

---

### Requirement: LLMSHALL能够通过工具创建和管理定时任务

(MUST) LLM 通过 `schedule_task` / `get_scheduled_tasks` / `delete_scheduled_task` 工具操作定时任务，受 policy engine 约束。

#### Scenario: LLM 创建定时任务
- **WHEN** LLM 调用 `schedule_task` 工具，输入 `name`、`cron_expr`、`session_id`
- **THEN** 系统 SHALL 验证 session 属于同一 cluster_id，创建任务并返回 `{ task_id, next_run }`

#### Scenario: LLM 查询定时任务
- **WHEN** LLM 调用 `get_scheduled_tasks` 工具
- **THEN** 系统 SHALL 返回当前所有定时任务列表（`task_id`、`name`、`cron_expr`、`next_run`、`enabled`）

#### Scenario: LLM 删除定时任务
- **WHEN** LLM 调用 `delete_scheduled_task` 工具，输入 `task_id`
- **THEN** 系统 SHALL 从 SQLite 删除该任务，并停止后续调度

#### Scenario: LLM 创建任务受 policy 约束
- **WHEN** LLM 调用 `schedule_task` 但被 policy 拒绝
- **THEN** 系统 SHALL 返回 policy 拒绝错误，任务不创建

---

### Requirement: 并发冲突SHALL被正确处理

(MUST) 如果定时触发时 session 正在被另一个 SSE 连接使用（agent loop 运行中），该次触发 skip，不重试。

#### Scenario: Session 正在使用时跳过触发
- **WHEN** 定时任务触发但关联 session 正在被 agent loop 处理
- **THEN** 系统 SHALL 记录 `skipped` 状态到 `scheduled_runs`，不阻塞 agent loop，下个周期继续检查

#### Scenario: 长时任务避免堆积
- **WHEN** 定时任务触发时上一次执行仍在 running
- **THEN** 系统 SHALL 跳过该次触发，避免任务堆积

---

### Requirement: UISHALL提供定时任务管理界面

(MUST) Chat view 侧边栏提供"定时任务"tab，支持查看、创建、启用/禁用、删除任务。

#### Scenario: 查看定时任务列表
- **WHEN** 用户点击"定时任务"tab
- **THEN** 系统 SHALL 显示所有任务（name、触发时间、下次执行时间、状态）

#### Scenario: 创建定时任务
- **WHEN** 用户填写 cron 或时间、选择 session，点击创建
- **THEN** 系统 SHALL 调用 `POST /api/scheduled-tasks` 创建任务并刷新列表

#### Scenario: 删除定时任务
- **WHEN** 用户点击任务删除按钮
- **THEN** 系统 SHALL 调用 `DELETE /api/scheduled-tasks/:id`，任务从列表移除

#### Scenario: 启用/禁用定时任务
- **WHEN** 用户切换任务状态开关
- **THEN** 系统 SHALL 调用 `PATCH /api/scheduled-tasks/:id` 更新 `enabled` 字段
