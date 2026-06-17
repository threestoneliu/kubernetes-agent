# Tasks · Session Management

## 1. Store layer — Sessions schema query

- [ ] 1.1 写 `ListSessionsFiltered(ctx, q, sort, order, limit, offset) ([]Session, error)`,SQL 用 `WHERE title LIKE ? COLLATE NOCASE` + `ORDER BY <col> <dir>` + `LIMIT ? OFFSET ?`;白名单 sort ∈ {created_at, updated_at, title},order ∈ {asc, desc};空 q 时跳过 WHERE
- [ ] 1.2 写 `RenameSession(ctx, id, title string) error`,SQL `UPDATE sessions SET title=?, updated_at=? WHERE id=?`;空 title 返回 error
- [ ] 1.3 写 `DeleteSession(ctx, id string) (int64, error)`,SQL `DELETE FROM sessions WHERE id=?`,返回 `RowsAffected`
- [ ] 1.4 写 `DeleteAllSessions(ctx) (int64, error)`,SQL `DELETE FROM sessions`,返回 `RowsAffected`
- [ ] 1.5 写 `GetMessagesForExport(ctx, sessionID string) ([]Message, error)`,无 limit,ORDER BY id ASC
- [ ] 1.6 写 `ListPlansForExport(ctx, sessionID string) ([]Plan, error)` 和 `ListAuditForSession(ctx, sessionID string) ([]AuditLog, error)` 支持 JSON 导出

## 2. Store unit tests

- [ ] 2.1 用 temp SQLite 测 ListSessionsFiltered 的搜索(大小写不敏感)、3 个 sort 字段、limit/offset 边界
- [ ] 2.2 测 RenameSession 后 updated_at 推进
- [ ] 2.3 测 DeleteSession 级联清 messages/plans/audit
- [ ] 2.4 测 DeleteAllSessions 后 ListSessions 返回空切片

## 3. Handler layer — extended listSessionsHandler

- [ ] 3.1 改 `listSessionsHandler`,解析 `q`/`sort`/`order`/`limit`/`offset` query 参数,默认值 q=""/sort="updated_at"/order="desc"/limit=100/offset=0
- [ ] 3.2 校验 sort 和 order 白名单,非法值返回 400 + `code: invalid_sort`
- [ ] 3.3 校验 limit ≤ 100,超过返回 400
- [ ] 3.4 调 `d.DB.ListSessionsFiltered` 返回 `{sessions: [...]}`

## 4. Handler layer — new endpoints

- [ ] 4.1 写 `putSessionHandler(d Deps)`,PUT /api/sessions/{id},body `{title}`,空 title 返回 422,not_found 返回 404,成功返回更新后的 session
- [ ] 4.2 写 `deleteSessionHandler(d Deps)`,DELETE /api/sessions/{id},检查 `d.Sessions` map,有活跃 → 409 + `code: session_active`;not_found 404;成功 200 + `{deleted: 1}`
- [ ] 4.3 写 `bulkDeleteSessionsHandler(d Deps)`,DELETE /api/sessions,调 DeleteAllSessions,返回 `{deleted: N}`
- [ ] 4.4 写 `exportSessionHandler(d Deps)`,GET /api/sessions/{id}/export?format=md|json,非法 format 返回 400,not_found 404,成功写 Content-Type + Content-Disposition
- [ ] 4.5 `exportSessionHandler` 的 markdown 渲染逻辑:写 `renderSessionMarkdown(s, msgs)` helper,reasoning 用 `<details>`、tool_call 用 fenced JSON
- [ ] 4.6 `exportSessionHandler` 的 json 渲染逻辑:组合 `{session, messages, plans, audit}` map,`json.MarshalIndent` 输出

## 5. Router wiring

- [ ] 5.1 在 `internal/server/router.go` 加 `r.Put("/api/sessions/{id}", putSessionHandler(d))` + `r.Delete("/api/sessions/{id}", deleteSessionHandler(d))` + `r.Delete("/api/sessions", bulkDeleteSessionsHandler(d))` + `r.Get("/api/sessions/{id}/export", exportSessionHandler(d))`
- [ ] 5.2 路由顺序确认:`/api/sessions/{id}/export` 在 `/api/sessions/{id}` 之前注册,避免 chi 把 export 当成 id

## 6. Handler unit + e2e tests

- [ ] 6.1 httptest:listSessionsHandler 各种 query 参数组合 + 非法 sort 400
- [ ] 6.2 httptest:putSessionHandler 200/404/422
- [ ] 6.3 httptest:deleteSessionHandler 200/404/409(用 fake Sessions map 模拟活跃)
- [ ] 6.4 httptest:bulkDeleteSessionsHandler 200 + 数量
- [ ] 6.5 httptest:exportSessionHandler 200 md + 200 json + 400 format + 404 session
- [ ] 6.6 e2e(create 3 个 → list 验证顺序 → search → rename → delete 验证 cascade)

## 7. Frontend API client

- [ ] 7.1 在 `web/src/api.ts` 加 `listSessions({q, sort, order, limit, offset}): Promise<{sessions: Session[]}>` 拼 query string
- [ ] 7.2 加 `renameSession(id, title): Promise<Session>` PUT
- [ ] 7.3 加 `deleteSession(id): Promise<{deleted: number}>` DELETE
- [ ] 7.4 加 `bulkDeleteSessions(): Promise<{deleted: number}>` DELETE
- [ ] 7.5 加 `exportSessionUrl(id, format): string` 返回后端 URL(浏览器直接走 `<a download>`)

## 8. Frontend — ConfirmModal 通用组件

- [ ] 8.1 写 `web/src/components/ConfirmModal.tsx`:title / message / confirmLabel / cancelLabel / onConfirm / onCancel / busy props
- [ ] 8.2 overlay click 关闭(若非 busy),ESC 关闭;busy 期间禁用按钮
- [ ] 8.3 接入 styles,与现有 PlanModal / AskUserForm 视觉一致

## 9. Frontend — SessionsPanel 组件

- [ ] 9.1 写 `web/src/views/SessionsPanel.tsx` 骨架:300px 宽,顶部 toolbar(新建按钮 + 搜索框 + 排序 select),列表 + 底部"清空全部"按钮
- [ ] 9.2 SessionsPanel props:`sessions`, `activeId`, `onSelect`, `onCreate`, `onRename`, `onDelete`, `onExport`, `onBulkClear`, `searchQ`, `onSearch`, `sort`, `order`, `onSort`, `busy`, `streaming`
- [ ] 9.3 每行渲染:title + cluster 标签 + 相对时间;active 高亮;hover 弹 `⋯` 菜单(重命名 / 导出 MD / 导出 JSON / 删除)
- [ ] 9.4 行内重命名:双击 title 进入 edit,Enter 提交 / ESC 取消 / blur 提交
- [ ] 9.5 行 hover 菜单:删除走 ConfirmModal,导出走 `<a href download>`,重命名走 inline edit
- [ ] 9.6 "清空全部"按钮:`streaming` 时 disabled,点击弹 ConfirmModal 显示数量
- [ ] 9.7 搜索框:onChange 节流 300ms 触发 onSearch
- [ ] 9.8 排序 select:选项 updated_at↓(默认) / updated_at↑ / created_at↓ / created_at↑ / title↑ / title↓

## 10. Frontend — ChatView 改 2-pane

- [ ] 10.1 ChatView 根 div 从单列改 flex row:左 SessionsPanel,右 conversation
- [ ] 10.2 ChatView 顶部 toolbar 加折叠按钮(展开/收起左栏),local state `panelCollapsed`
- [ ] 10.3 mount 时 useEffect → listSessions() 拉初始列表
- [ ] 10.4 点 SessionsPanel 行 → setActiveSessionId(newId) + GET /api/sessions/{id}/messages 替换 msgs
- [ ] 10.5 点新建按钮 → POST /api/sessions → listSessions refetch + 自动 setActiveSessionId(newId)
- [ ] 10.6 删/重命名/导出后 refetch sessions 列表

## 11. Frontend — draft preservation

- [ ] 11.1 ChatView state 加 `drafts: Record<sessionId, string>` map
- [ ] 11.2 切换 active session 前:`drafts[oldId] = input`
- [ ] 11.3 切换到 newId 后:`setInput(drafts[newId] ?? '')`
- [ ] 11.4 发送消息后清 `drafts[currentId]`
- [ ] 11.5 active session 被删除后清 `drafts[deletedId]`

## 12. Frontend — sessions panel unit test

- [ ] 12.1 React Testing Library + jsdom 测 SessionsPanel:渲染列表、点选切换回调、搜索输入回调、删除走 ConfirmModal
- [ ] 12.2 测 ConfirmModal:overlay click 关闭、ESC 关闭、busy 期间禁用按钮

## 13. Playwright e2e

- [ ] 13.1 场景:列出已存在的 session(从上次测试遗留的),面板显示 title + cluster + 时间
- [ ] 13.2 场景:在搜索框输入部分标题,验证列表过滤
- [ ] 13.3 场景:点 session A → 切换 → 输入框清空 → 输入文本(草稿) → 切到 session B → 切回 A → 输入文本还在
- [ ] 13.4 场景:双击 title 进入 edit,输入新 title,Enter,验证 PUT + 列表更新
- [ ] 13.5 场景:hover 行 → 弹菜单 → 点删除 → 弹 ConfirmModal → 确认 → 验证列表少一行
- [ ] 13.6 场景:点导出 MD,验证 `session-<id>.md` 下载,内容含元数据 + 消息
- [ ] 13.7 场景:点"清空全部" → ConfirmModal 显示 N → 确认 → 列表空

## 14. Docs + final commit

- [ ] 14.1 README 不变(本 change 是 UI 增强,不发用户面文档)
- [ ] 14.2 全部 substep 跑通 `go test ./...` + `pnpm typecheck` + Playwright 7 个场景
- [ ] 14.3 commit + push(用户手动 push)
- [ ] 14.4 `git log --oneline openspec/changes/session-management/` 列出所有 commits