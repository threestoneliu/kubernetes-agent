# Design · Session Management

## Context

`kubernetes-agent` 在 k8s-natural-language-agent change 后已具备最小闭环:用户能开 session、发自然语言、看 LLM 流式响应、Plan 预览、policy 拒绝、会话级 ask_user。但会话管理能力缺失:

- 后端已经有 `GET /api/sessions` / `POST /api/sessions` / `GET /api/sessions/{id}` / `GET /api/sessions/{id}/messages` / `POST /api/sessions/{id}/resume`,**前端只显示了"新建"按钮,没有列表**
- 用户无法切换回历史会话继续聊天
- 没有重命名、删除、搜索、排序、导出、批量清理
- `ON DELETE CASCADE` 已经在 SQLite schema 上设好,messages/plans/audit 会随 session 删除自动清理

需求来源:用户 2026-06-17 直接要求 + `docs/roadmap.md` 第 6 项 UI 打磨(审计日志页/可视化 diff/Plan 历史回放)+ k8s-natural-language-agent retrospective §6 promote candidate "code pattern atomic commit when marking tasks done" 衍生出的 "atomicity of UI state"(UI 也需要把会话状态显式持久化)。

## Goals / Non-Goals

**Goals**
- 用户能在 ChatView 看到所有历史会话列表(标题 + cluster + 更新时间)
- 点会话切换进去,看到该 session 的历史消息
- 重命名 inline / 删除 / 导出 Markdown 或 JSON
- 标题搜索 + 排序(updated_at / created_at / title,asc / desc)
- 一键清空全部(二次确认)
- 删除前 confirm 模态
- 活跃 streaming session 不能删除
- 切换会话保留未发送的 input 草稿
- 后端 store 层能力可独立单测;前端组件可独立渲染测试

**Non-Goals**
- 全文搜索(messages 内容)→ 后续需要时叠 FTS5
- 会话标签 / 收藏 / 归档 → 后续
- 会话分享 / 导出到 S3 → 后续
- 多用户隔离(本项目始终单用户)→ 不需要

## Architecture

### 2-pane ChatView

```
┌─ ChatView root (flex row) ───────────────────────────────┐
│ ┌─ SessionsPanel (300px, 可折叠到 0) ─┐ ┌─ Chat (flex 1) ┐ │
│ │ toolbar: [+ 新建] [🔍 q] [排序 ▾]   │ │ cluster dropdown│ │
│ │                                    │ │ conversation    │ │
│ │ ▼ active session                   │ │ flow + PlanModal│ │
│ │   lzl  · 3 min ago                 │ │ + AskUserForm   │ │
│ │                                    │ │                 │ │
│ │ □ 列出 default pod   lzl · 1d ago  │ │                 │ │
│ │ □ 删除 demo-cm       lzl · 2d ago  │ │                 │ │
│ │ □ Why is nginx down  lzl · 3d ago  │ │                 │ │
│ │                                    │ │                 │ │
│ │ [清空全部]                          │ │                 │ │
│ └────────────────────────────────────┘ └─────────────────┘ │
└───────────────────────────────────────────────────────────┘
```

折叠按钮放在 SessionsPanel 右上角,点击收起到 0px(留 16px toggle bar),Chat 区占满。状态保存在 ChatView 的 `panelCollapsed` local state,不做 URL 持久化(MVP)。

### State machine

```
ChatView state:
  clusters        : Cluster[]            // fetch on mount
  sessions        : Session[]            // refetch on session create/delete/rename
  activeSessionId : string | null
  input           : string               // draft for active session
  drafts          : Record<sessionId, string>  // preserve input across switches
  searchQ         : string
  sort            : 'updated_at' | 'created_at' | 'title'
  order           : 'asc' | 'desc'
  panelCollapsed  : boolean
  msgs            : Msg[]                // per active session
```

切换会话时:`drafts[oldId] = input; setInput(drafts[newId] ?? ''); setActiveSessionId(newId); fetch /api/sessions/{newId}/messages`

### Backend API surface

| Method | Path | Body | Returns | Notes |
|--------|------|------|---------|-------|
| GET | `/api/sessions` | — | `{sessions: Session[]}` | 加 `?q=&sort=&order=&limit=&offset=` |
| POST | `/api/sessions` | `{title, cluster_id?}` | `Session` (201) | 已有 |
| GET | `/api/sessions/{id}` | — | `Session` | 已有 |
| GET | `/api/sessions/{id}/messages` | — | `{messages: [...]}` | 已有 |
| POST | `/api/sessions/{id}/resume` | — | — | 已有 |
| **PUT** | `/api/sessions/{id}` | `{title}` | `Session` | 新增 |
| **DELETE** | `/api/sessions/{id}` | — | `{deleted: 1}` | 新增,ON DELETE CASCADE 清 messages/plans/audit |
| **DELETE** | `/api/sessions` | — | `{deleted: N}` | 新增,bulk;前端二次确认 |
| **GET** | `/api/sessions/{id}/export` | `?format=md\|json` | 文本流 | 新增,`Content-Disposition: attachment` |

活跃 session 删除保护:DELETE handler 检查 session 是否在 `d.Sessions` map 里(agent 当前持有),如果有 → 409 Conflict + `code: session_active`。

### Store layer

新增方法 `ListSessionsFiltered(ctx, q, sort, order, limit, offset)`:

```sql
SELECT id, title, cluster_id, created_at, updated_at
FROM sessions
WHERE title LIKE ? COLLATE NOCASE
ORDER BY <sort_col> <order>
LIMIT ? OFFSET ?
```

默认值:`q=''`, `sort='updated_at'`, `order='desc'`, `limit=100`, `offset=0`。SQLite `COLLATE NOCASE` 让标题大小写不敏感。

新增 `RenameSession(ctx, id, title)`:`UPDATE sessions SET title=?, updated_at=? WHERE id=?`。
新增 `DeleteSession(ctx, id)`:`DELETE FROM sessions WHERE id=?`。
新增 `DeleteAllSessions(ctx)`:`DELETE FROM sessions`。
新增 `GetMessagesForExport(ctx, sessionID)` — 全量 messages(不应用现有 `?limit=200`)。

### Export format

**Markdown**:
```
# 会话: <title> (cluster_id=<cid> | created_at=... | updated_at=...)

---

## user

列出 default namespace 的 pod

## assistant

<details>
<summary>思考过程</summary>

...

</details>

🔧 k8s_list({...})
  ✓ 输出: {...}

Default namespace 里没有任何 Pod。

---

## user

...
```

**JSON**:原始 store schema dump,包含 session row + 全部 messages + 全部 plans + audit:

```json
{
  "session": {"id": "...", "title": "...", ...},
  "messages": [...],
  "plans": [...],
  "audit": [...]
}
```

JSON 不做 round-trip import — 是 "导出备份" 用途。

## Components / Files

### Backend (新增/修改)

| File | Change |
|------|--------|
| `internal/store/sessions.go` | + `ListSessionsFiltered` / `RenameSession` / `DeleteSession` / `DeleteAllSessions` / `GetMessagesForExport` |
| `internal/store/sessions_test.go` | 新增,SQL 行为单测 |
| `internal/server/handler_sessions.go` | + `putSessionHandler` / `deleteSessionHandler` / `bulkDeleteSessionsHandler` / `exportSessionHandler`;扩展 `listSessionsHandler` 支持 query 参数 |
| `internal/server/router.go` | + 4 个新 route |
| `internal/server/handler_sessions_test.go` | 新增 httptest 覆盖 CRUD + search/sort + export |

### Frontend (新增/修改)

| File | Change |
|------|--------|
| `web/src/api.ts` | + `listSessions({q,sort,order,limit,offset})` / `renameSession(id,title)` / `deleteSession(id)` / `bulkDeleteSessions()` / `exportSessionUrl(id,format)` |
| `web/src/views/SessionsPanel.tsx` | 新增,300px 左栏组件 |
| `web/src/views/SessionsPanel.module.css` | 新增,样式 |
| `web/src/views/ChatView.tsx` | 改 2-pane 布局;加 drafts map;接 sessions API |
| `web/src/components/ConfirmModal.tsx` | 通用确认模态(删除/清空复用) |
| `web/src/views/SessionsPanel.test.tsx` | 新增,组件渲染 + 交互测试 |

## Data Flow

**List load**: mount → `listSessions()` → setSessions
**Switch**: 点 row → `drafts[old] = input; setInput(drafts[new] ?? ''); setActiveSessionId(new); GET /api/sessions/{new}/messages`
**Rename**: row 进入 edit 模式 → input + PUT → 200 OK → setSessions 更新行 + 退出 edit 模式
**Delete**: 点菜单 → 弹 ConfirmModal → DELETE → setSessions 过滤掉该 id
**Export**: 点菜单 → `window.location = exportSessionUrl(id, format)`(直接走浏览器 download)
**Bulk delete**: 底部"清空全部"按钮 → ConfirmModal("确认删除全部 N 个会话?") → DELETE /api/sessions → setSessions([]) + 退出 active session

## Error Handling

| Scenario | Behavior |
|----------|----------|
| DELETE 不存在的 session | 404,toast "会话不存在" |
| DELETE 活跃 streaming session | 409 + `code: session_active`,toast "请先停止当前会话" |
| PUT 标题为空 | 422,toast "标题不能为空" |
| 搜索无结果 | UI 显示"无匹配会话" |
| 导出失败(会话不存在) | 404,toast |
| 网络失败 | toast "网络错误,稍后重试" |

## Testing

**单元**:
- `internal/store/sessions_test.go`:用 temp SQLite DB 跑 ListSessionsFiltered 的 SQL(搜索 LIKE 大小写、排序字段、limit/offset),RenameSession、DeleteSession、DeleteAllSessions 的行为
- `internal/server/handler_sessions_test.go`:每个新 endpoint 的 httptest,覆盖 200/404/409/422

**e2e** (扩展 `internal/server/e2e_test.go`):
- create 3 个 session → list 验证顺序(默认 updated_at desc)
- search "demo" → 验证只返回标题含 demo 的
- rename → list 验证新标题
- delete → list 验证消失 + 验证 messages/plans 也被清(通过 plan 表 SELECT)
- 活跃 session delete → 409

**前端**:
- `web/src/views/SessionsPanel.test.tsx`:React Testing Library 渲染 + fireEvent
- Playwright e2e(扩 `/tmp/ui-test/test-sessions.js`):7 个场景
  - 列出会话
  - 搜索过滤
  - 切换会话(历史消息加载)
  - 重命名
  - 删除(confirm 模态)
  - 导出 Markdown(检查文件内容)
  - 清空全部

## Risks / Trade-offs

- **`updated_at` 不会自动更新**:SQLite 没有 trigger 自动更新,需要在每次 rename + 创建 plan + send message 时显式 `UPDATE sessions SET updated_at=?`。MVP 接受此约束(只在 rename 时更新)。
- **DELETE bulk 没事务**:单条 SQL `DELETE FROM sessions` 是原子的(SQLite 默认),但没有逐行 progress。100 条 session 删除是 sub-ms,OK。
- **Export markdown schema 演进**:消息 ContentPart 字段后续可能加新的 kind(text/reasoning/tool_call),markdown 渲染需要随之更新。MVP 只 cover 现有 3 种。
- **无并发保护**:用户同时开两个 tab,A tab 删除 session,B tab 还在显示 → 后端 404,前端 toast。可以接受。

## Open Questions

None.