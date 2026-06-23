## Why

用户和 LLM 都需要在特定时间自动触发操作（如定时巡检、健康检查、批量变更）。目前系统没有定时执行能力，用户必须手动触发或依赖外部 cron/Webhook，缺乏与现有 chat/session 体系的集成。

## What Changes

**New Capability: 定时任务调度系统**

支持 cron 表达式和一次性时间两种触发方式，任务持久化到 SQLite，结果写入对应 session，用户和 LLM 都可以创建管理任务。

## Capabilities

### New Capabilities

- `scheduled-tasks`: 定时任务系统。支持 cron 表达式（秒级精度）和一次性时间触发，持久化存储，执行结果写入 session 消息。LLM 可通过 `schedule_task` 工具创建和管理任务，受 policy 约束。用户通过 UI 定时任务 tab 管理。

## Impact

- **新增表**：`scheduled_tasks`（任务定义）、`scheduled_runs`（执行记录）
- **新增依赖**：`github.com/robfig/cron/v3`
- **新增工具**：后端 `schedule_task` / `get_scheduled_tasks` / `delete_scheduled_task` LLM 工具
- **新增 API**：`GET/POST/DELETE /api/scheduled-tasks`
- **新增 UI**：Chat view 侧边栏"定时任务"tab
- **影响组件**：`internal/scheduler/`（新目录）、`internal/server/`（HTTP 路由）、`internal/agent/`（session 消息写入）、`web/src/views/`（前端 UI）
