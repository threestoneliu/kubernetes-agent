# Session Management Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add session management (list/select/rename/delete/search/sort/export/bulk-clear) to the kubernetes-agent web UI and backend.

**Architecture:** Backend extends `GET /api/sessions` with search/sort/pagination, adds 4 new endpoints (PUT/DELETE single, DELETE bulk, GET export). Frontend embeds a 300px SessionsPanel inside ChatView (2-pane layout) with ConfirmModal + draft preservation. Store layer adds 5 new methods. ON DELETE CASCADE on the existing schema handles cascade cleanup.

**Tech Stack:** Go (chi router + sqlite store) · TypeScript + React 19 + Vite · existing infrastructure (no new deps)

---

## Task 1: Store layer — list / rename / delete / export queries

**Files:**
- Modify: `internal/store/sessions.go` (add 5 methods)
- Test: `internal/store/sessions_test.go` (new file)

- [ ] **Step 1:** Write `ListSessionsFiltered` failing test
  ```go
  // in store/sessions_test.go
  func TestListSessionsFiltered_Basic(t *testing.T) {
      db := newTempDB(t)
      // seed 3 sessions
      mustInsertSession(t, db, store.Session{ID: "a", Title: "Alpha", ClusterID: ptr("c1")})
      mustInsertSession(t, db, store.Session{ID: "b", Title: "Beta", ClusterID: ptr("c1")})
      mustInsertSession(t, db, store.Session{ID: "c", Title: "alpha2", ClusterID: ptr("c1")})

      rows, err := db.ListSessionsFiltered(ctx, "", "title", "asc", 10, 0)
      require.NoError(t, err)
      require.Len(t, rows, 3)
      assert.Equal(t, "a", rows[0].ID) // case-sensitive title asc
  }
  ```

- [ ] **Step 2:** Run test, expect compile error (function missing)

- [ ] **Step 3:** Implement `ListSessionsFiltered` in `internal/store/sessions.go`
  ```go
  func (d *DB) ListSessionsFiltered(ctx context.Context, q, sort, order string, limit, offset int) ([]Session, error) {
      allowedSort := map[string]string{"created_at":"created_at","updated_at":"updated_at","title":"title"}
      allowedOrder := map[string]bool{"asc":true,"desc":true}
      col, ok := allowedSort[sort]
      if !ok { return nil, fmt.Errorf("invalid sort %q", sort) }
      if !allowedOrder[order] { return nil, fmt.Errorf("invalid order %q", order) }
      var (
          rows []Session
          args []any
      )
      sql := `SELECT id, title, cluster_id, created_at, updated_at FROM sessions`
      if q != "" {
          sql += ` WHERE title LIKE ? COLLATE NOCASE`
          args = append(args, "%"+q+"%")
      }
      sql += ` ORDER BY ` + col + ` ` + strings.ToUpper(order) + ` LIMIT ? OFFSET ?`
      args = append(args, limit, offset)
      err := d.conn.SelectContext(ctx, &rows, sql, args...)
      return rows, err
  }
  ```

- [ ] **Step 4:** Run test, expect PASS

- [ ] **Step 5:** Add 4 more tests: case-insensitive search (`q="alpha"` returns both "Alpha" and "alpha2"), `sort=updated_at desc` ordering, `limit=2 offset=1` pagination, invalid sort returns error

- [ ] **Step 6:** Write `RenameSession` + test
  ```go
  func (d *DB) RenameSession(ctx context.Context, id, title string) error {
      if title == "" { return fmt.Errorf("title required") }
      res, err := d.conn.ExecContext(ctx,
          `UPDATE sessions SET title=?, updated_at=? WHERE id=?`,
          title, time.Now().UTC(), id)
      if err != nil { return err }
      n, _ := res.RowsAffected()
      if n == 0 { return ErrNotFound }
      return nil
  }
  ```

- [ ] **Step 7:** Write `DeleteSession` + test (verify cascade by also inserting + deleting a message)
  ```go
  func (d *DB) DeleteSession(ctx context.Context, id string) (int64, error) {
      res, err := d.conn.ExecContext(ctx, `DELETE FROM sessions WHERE id=?`, id)
      if err != nil { return 0, err }
      return res.RowsAffected()
  }
  ```

- [ ] **Step 8:** Write `DeleteAllSessions` + test

- [ ] **Step 9:** Write `GetMessagesForExport` + `ListPlansForExport` + `ListAuditForSession` (3 simple SELECTs, ORDER BY id ASC, no limit)

- [ ] **Step 10:** Run `go test ./internal/store/... -v -run "Session"` — expect all pass

- [ ] **Step 11:** Commit `feat(store): session list/rename/delete/export queries`

---

## Task 2: Handler — extended listSessionsHandler with search/sort/pagination

**Files:**
- Modify: `internal/server/handler_sessions.go`
- Test: `internal/server/handler_sessions_test.go` (extend existing)

- [ ] **Step 1:** Write failing test for `?q=foo&sort=title&order=asc&limit=20&offset=40`
  ```go
  func TestListSessionsHandler_QueryParams(t *testing.T) {
      d := newTestDeps(t)
      mustSeedSessions(t, d.DB, 5)
      req := httptest.NewRequest("GET", "/api/sessions?q=session&sort=title&order=asc&limit=2&offset=1", nil)
      w := httptest.NewRecorder()
      d.ServeHTTP(w, req)
      assert.Equal(t, 200, w.Code)
      // parse JSON body and verify shape
  }
  ```

- [ ] **Step 2:** Run test, expect fail (handler doesn't parse query yet)

- [ ] **Step 3:** Modify `listSessionsHandler`:
  ```go
  func listSessionsHandler(d Deps) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          q := r.URL.Query().Get("q")
          sort := r.URL.Query().Get("sort")
          if sort == "" { sort = "updated_at" }
          order := r.URL.Query().Get("order")
          if order == "" { order = "desc" }
          limit := 100
          if s := r.URL.Query().Get("limit"); s != "" {
              n, err := strconv.Atoi(s)
              if err != nil || n <= 0 || n > 100 {
                  writeError(w, 400, "invalid_limit", "limit must be 1..100", false)
                  return
              }
              limit = n
          }
          offset := 0
          if s := r.URL.Query().Get("offset"); s != "" {
              n, _ := strconv.Atoi(s)
              if n < 0 { writeError(w, 400, "invalid_offset", "", false); return }
              offset = n
          }
          rows, err := d.DB.ListSessionsFiltered(r.Context(), q, sort, order, limit, offset)
          if err != nil {
              writeError(w, 400, "invalid_sort", err.Error(), false)
              return
          }
          out := make([]sessionView, 0, len(rows))
          for _, s := range rows { out = append(out, toSessionView(s)) }
          writeJSON(w, 200, map[string]any{"sessions": out})
      }
  }
  ```

- [ ] **Step 4:** Add test for invalid sort returning 400

- [ ] **Step 5:** Run tests, expect PASS

- [ ] **Step 6:** Commit `feat(server): search/sort/pagination on list sessions`

---

## Task 3: Handler — PUT rename + DELETE single (with active-session protection)

**Files:**
- Modify: `internal/server/handler_sessions.go`
- Test: `internal/server/handler_sessions_test.go`

- [ ] **Step 1:** Write failing test for `putSessionHandler` happy path + 404 + 422
  ```go
  func TestPutSessionHandler_Rename(t *testing.T) {
      d := newTestDeps(t)
      s := mustSeedOneSession(t, d.DB)
      body := `{"title":"new name"}`
      req := httptest.NewRequest("PUT", "/api/sessions/"+s.ID, strings.NewReader(body))
      w := httptest.NewRecorder()
      d.ServeHTTP(w, req)
      assert.Equal(t, 200, w.Code)
      // GET session, verify title changed
  }
  func TestPutSessionHandler_EmptyTitle(t *testing.T) { /* expect 422 */ }
  func TestPutSessionHandler_NotFound(t *testing.T) { /* expect 404 */ }
  ```

- [ ] **Step 2:** Implement `putSessionHandler`:
  ```go
  func putSessionHandler(d Deps) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          id := chi.URLParam(r, "id")
          var req struct{ Title string `json:"title"` }
          if !decodeJSON(w, r, &req) { return }
          if strings.TrimSpace(req.Title) == "" {
              writeError(w, 422, "invalid_title", "title must not be empty", false)
              return
          }
          if err := d.DB.RenameSession(r.Context(), id, req.Title); err != nil {
              if errors.Is(err, store.ErrNotFound) {
                  writeError(w, 404, "not_found", "session not found", false)
                  return
              }
              writeError(w, 500, "internal", err.Error(), true)
              return
          }
          row, _ := d.DB.GetSession(r.Context(), id)
          writeJSON(w, 200, toSessionView(*row))
      }
  }
  ```

- [ ] **Step 3:** Write failing test for `deleteSessionHandler` (200 + 404 + 409 active)
  ```go
  func TestDeleteSessionHandler_Active(t *testing.T) {
      d := newTestDeps(t)
      s := mustSeedOneSession(t, d.DB)
      sess := agent.NewSession(s.ID)
      d.Sessions.Set(s.ID, sess)
      req := httptest.NewRequest("DELETE", "/api/sessions/"+s.ID, nil)
      w := httptest.NewRecorder()
      d.ServeHTTP(w, req)
      assert.Equal(t, 409, w.Code)
      assert.Contains(t, w.Body.String(), "session_active")
  }
  ```

- [ ] **Step 4:** Implement `deleteSessionHandler`:
  ```go
  func deleteSessionHandler(d Deps) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          id := chi.URLParam(r, "id")
          if d.Sessions != nil {
              if _, ok := d.Sessions.Get(id); ok {
                  writeError(w, 409, "session_active", "session is in progress", false)
                  return
              }
          }
          n, err := d.DB.DeleteSession(r.Context(), id)
          if err != nil {
              writeError(w, 500, "internal", err.Error(), true)
              return
          }
          if n == 0 {
              writeError(w, 404, "not_found", "session not found", false)
              return
          }
          writeJSON(w, 200, map[string]any{"deleted": n})
      }
  }
  ```

- [ ] **Step 5:** Run all tests, expect PASS

- [ ] **Step 6:** Commit `feat(server): session rename + delete with active protection`

---

## Task 4: Handler — bulk delete + export (md/json)

**Files:**
- Modify: `internal/server/handler_sessions.go`
- Modify: `internal/server/router.go` (route order)

- [ ] **Step 1:** Write `bulkDeleteSessionsHandler`:
  ```go
  func bulkDeleteSessionsHandler(d Deps) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          n, err := d.DB.DeleteAllSessions(r.Context())
          if err != nil { writeError(w, 500, "internal", err.Error(), true); return }
          writeJSON(w, 200, map[string]any{"deleted": n})
      }
  }
  ```

- [ ] **Step 2:** Test (200 + 数量 + 数据库空)

- [ ] **Step 3:** Write `exportSessionHandler` skeleton:
  ```go
  func exportSessionHandler(d Deps) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          id := chi.URLParam(r, "id")
          format := r.URL.Query().Get("format")
          if format != "md" && format != "json" {
              writeError(w, 400, "invalid_format", "format must be md or json", false); return
          }
          session, err := d.DB.GetSession(r.Context(), id)
          if errors.Is(err, store.ErrNotFound) {
              writeError(w, 404, "not_found", "session not found", false); return
          }
          if err != nil { writeError(w, 500, "internal", err.Error(), true); return }
          msgs, _ := d.DB.GetMessagesForExport(r.Context(), id)
          plans, _ := d.DB.ListPlansForExport(r.Context(), id)
          audit, _ := d.DB.ListAuditForSession(r.Context(), id)
          filename := fmt.Sprintf("session-%s.%s", id[:8], format)
          w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
          if format == "md" {
              w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
              w.Write([]byte(renderSessionMarkdown(session, msgs)))
          } else {
              w.Header().Set("Content-Type", "application/json; charset=utf-8")
              payload := map[string]any{"session": session, "messages": msgs, "plans": plans, "audit": audit}
              enc := json.NewEncoder(w)
              enc.SetIndent("", "  ")
              enc.Encode(payload)
          }
      }
  }
  ```

- [ ] **Step 4:** Write `renderSessionMarkdown` helper (in new `internal/server/export_md.go`):
  ```go
  func renderSessionMarkdown(s store.Session, msgs []store.Message) string {
      var b strings.Builder
      fmt.Fprintf(&b, "# 会话: %s\n\n", s.Title)
      fmt.Fprintf(&b, "- session_id: %s\n", s.ID)
      if s.ClusterID != nil { fmt.Fprintf(&b, "- cluster_id: %s\n", *s.ClusterID) }
      fmt.Fprintf(&b, "- created_at: %s\n", s.CreatedAt.Format(time.RFC3339))
      fmt.Fprintf(&b, "- updated_at: %s\n\n", s.UpdatedAt.Format(time.RFC3339))
      b.WriteString("---\n\n")
      for _, m := range msgs {
          role := m.Role
          if role == "" { role = "system" }
          fmt.Fprintf(&b, "## %s\n\n", role)
          for _, part := range m.Parts {
              switch part.Type {
              case "text":
                  b.WriteString(part.Text)
                  b.WriteString("\n\n")
              case "reasoning":
                  fmt.Fprintf(&b, "<details><summary>思考过程</summary>\n\n%s\n\n</details>\n\n", part.Text)
              case "tool_call":
                  name, _ := part.Meta["name"].(string)
                  args, _ := part.Meta["input"].(string)
                  fmt.Fprintf(&b, "🔧 %s\n\n```json\n%s\n```\n\n", name, args)
              case "tool_result":
                  output := part.Text
                  if part.Meta != nil {
                      if out, ok := part.Meta["output"]; ok {
                          output = fmt.Sprintf("%v", out)
                      }
                  }
                  fmt.Fprintf(&b, "输出:\n\n```json\n%s\n```\n\n", output)
              }
          }
      }
      return b.String()
  }
  ```

- [ ] **Step 5:** Tests for export (200 md / 200 json / 400 format / 404 session)

- [ ] **Step 6:** Wire 4 new routes in `internal/server/router.go`:
  ```go
  r.Route("/api/sessions", func(r chi.Router) {
      r.Get("/", listSessionsHandler(d))
      r.Post("/", createSessionHandler(d))
      r.Delete("/", bulkDeleteSessionsHandler(d))  // NEW: bulk before /{id}
      r.Get("/{id}", getSessionHandler(d))
      r.Put("/{id}", putSessionHandler(d))         // NEW
      r.Delete("/{id}", deleteSessionHandler(d))   // NEW
      r.Get("/{id}/messages", listMessagesHandler(d))  // BEFORE /{id} but chi handles static-vs-param
      r.Post("/{id}/resume", resumeHandler(d))
      r.Get("/{id}/export", exportSessionHandler(d))    // NEW: must be before /{id} catch-all
  })
  ```

- [ ] **Step 7:** Run all tests, expect PASS

- [ ] **Step 8:** Commit `feat(server): bulk delete + export md/json endpoints`

---

## Task 5: Frontend API client — listSessions / rename / delete / export URLs

**Files:**
- Modify: `web/src/api.ts`

- [ ] **Step 1:** Read current `web/src/api.ts` to find Cluster / Session type definitions

- [ ] **Step 2:** Add 5 functions:
  ```ts
  export interface Session {
    id: string
    title: string
    cluster_id?: string
    created_at: number
    updated_at: number
  }
  export async function listSessions(opts: {
    q?: string; sort?: 'updated_at'|'created_at'|'title'; order?: 'asc'|'desc';
    limit?: number; offset?: number;
  } = {}): Promise<{sessions: Session[]}> {
    const params = new URLSearchParams()
    if (opts.q) params.set('q', opts.q)
    if (opts.sort) params.set('sort', opts.sort)
    if (opts.order) params.set('order', opts.order)
    if (opts.limit != null) params.set('limit', String(opts.limit))
    if (opts.offset) params.set('offset', String(opts.offset))
    const q = params.toString()
    return fetch(`/api/sessions${q ? '?' + q : ''}`).then(asJson<{sessions:Session[]}>)
  }
  export async function renameSession(id: string, title: string): Promise<Session> {
    return fetch(`/api/sessions/${id}`, {
      method: 'PUT',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({title}),
    }).then(asJson<Session>)
  }
  export async function deleteSession(id: string): Promise<{deleted:number}> {
    return fetch(`/api/sessions/${id}`, {method: 'DELETE'}).then(asJson<{deleted:number}>)
  }
  export async function bulkDeleteSessions(): Promise<{deleted:number}> {
    return fetch('/api/sessions', {method: 'DELETE'}).then(asJson<{deleted:number}>)
  }
  export function exportSessionUrl(id: string, format: 'md'|'json'): string {
    return `/api/sessions/${id}/export?format=${format}`
  }
  ```

- [ ] **Step 3:** Run `pnpm typecheck` — expect PASS

- [ ] **Step 4:** Commit `feat(web): session API client methods`

---

## Task 6: Frontend — ConfirmModal generic component

**Files:**
- Create: `web/src/components/ConfirmModal.tsx`

- [ ] **Step 1:** Write component:
  ```tsx
  import React from 'react'
  export function ConfirmModal({
    title, message, confirmLabel = '确认', cancelLabel = '取消',
    onConfirm, onCancel, busy = false, danger = false,
  }: {
    title: string; message: React.ReactNode
    confirmLabel?: string; cancelLabel?: string
    onConfirm: () => void; onCancel: () => void
    busy?: boolean; danger?: boolean
  }) {
    React.useEffect(() => {
      const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !busy) onCancel() }
      window.addEventListener('keydown', onKey)
      return () => window.removeEventListener('keydown', onKey)
    }, [onCancel, busy])
    return (
      <div className="modal-overlay" role="dialog" aria-modal="true"
           onClick={(e) => { if (e.target === e.currentTarget && !busy) onCancel() }}>
        <div className="modal">
          <h2>{title}</h2>
          <div style={{margin:'12px 0'}}>{message}</div>
          <div className="modal-actions">
            <button onClick={onCancel} disabled={busy}>{cancelLabel}</button>
            <button onClick={onConfirm} disabled={busy} className={danger ? 'danger' : 'primary'}>
              {busy ? '处理中…' : confirmLabel}
            </button>
          </div>
        </div>
      </div>
    )
  }
  ```

- [ ] **Step 2:** `pnpm typecheck`

- [ ] **Step 3:** Commit `feat(web): ConfirmModal component`

---

## Task 7: Frontend — SessionsPanel component

**Files:**
- Create: `web/src/views/SessionsPanel.tsx`

- [ ] **Step 1:** Define props interface and component skeleton:
  ```tsx
  import React from 'react'
  import type { Session } from '../api'
  import { exportSessionUrl } from '../api'
  import { ConfirmModal } from '../components/ConfirmModal'

  interface Props {
    sessions: Session[]
    activeId: string | null
    streaming: boolean  // true when ui.kind==='streaming'
    searchQ: string
    sort: 'updated_at'|'created_at'|'title'
    order: 'asc'|'desc'
    onSearch: (q: string) => void
    onSort: (sort: Props['sort'], order: Props['order']) => void
    onSelect: (id: string) => void
    onCreate: () => void
    onRename: (id: string, title: string) => void
    onDelete: (id: string) => void
    onBulkClear: () => void
    clusterNameById: (id: string) => string  // helper for display
    relativeTime: (epochSecs: number) => string
  }

  export function SessionsPanel(props: Props) {
    const [deleteId, setDeleteId] = React.useState<string|null>(null)
    const [editing, setEditing] = React.useState<{id:string; title:string}|null>(null)
    const [bulkOpen, setBulkOpen] = React.useState(false)
    // ... render
  }
  ```

- [ ] **Step 2:** Add toolbar: 新建 button + 搜索 input (debounced via setTimeout 300ms) + 排序 select with 6 options

- [ ] **Step 3:** Add list rendering — each row: title (double-click → inline edit input) + cluster name + relative time; active row gets `className="active"` highlight

- [ ] **Step 4:** Add hover menu (`⋯` button → small dropdown) with 4 actions: 重命名 / 导出 MD / 导出 JSON / 删除. Deletion triggers `setDeleteId(id)` → renders ConfirmModal

- [ ] **Step 5:** Implement inline rename: `editing` state, Enter commits + calls `onRename`, ESC cancels, blur commits

- [ ] **Step 6:** Add 底部 "清空全部" button (disabled when streaming OR no sessions), click → setBulkOpen(true) → ConfirmModal showing count

- [ ] **Step 7:** `pnpm typecheck`

- [ ] **Step 8:** Commit `feat(web): SessionsPanel with list/search/sort/rename/delete/export/bulk-clear`

---

## Task 8: Frontend — ChatView 2-pane layout + draft preservation

**Files:**
- Modify: `web/src/views/ChatView.tsx`

- [ ] **Step 1:** Add new state:
  ```ts
  const [sessions, setSessions] = React.useState<Session[]>([])
  const [searchQ, setSearchQ] = React.useState('')
  const [sort, setSort] = React.useState<'updated_at'|'created_at'|'title'>('updated_at')
  const [order, setOrder] = React.useState<'asc'|'desc'>('desc')
  const [panelCollapsed, setPanelCollapsed] = React.useState(false)
  const [drafts, setDrafts] = React.useState<Record<string,string>>({})
  ```

- [ ] **Step 2:** Add `useEffect` on mount: `listSessions({sort, order}).then(setSessions)` and debounced search:
  ```ts
  React.useEffect(() => {
      const t = setTimeout(() => {
          listSessions({q: searchQ, sort, order}).then(setSessions)
      }, 300)
      return () => clearTimeout(t)
  }, [searchQ, sort, order])
  ```

- [ ] **Step 3:** Wrap root render: change single column → flex row
  ```tsx
  return (
    <div style={{display:'flex', flexDirection:'row', height:'100%'}}>
      {!panelCollapsed && (
        <SessionsPanel sessions={sessions} activeId={sessionId} /* ... all props ... */ />
      )}
      <button onClick={() => setPanelCollapsed(p => !p)} className="panel-toggle">
        {panelCollapsed ? '»' : '«'}
      </button>
      <div className="conversation">{/* existing chat */}</div>
    </div>
  )
  ```

- [ ] **Step 4:** Implement `switchSession(newId)`:
  ```ts
  function switchSession(newId: string) {
      if (sessionId === newId) return
      const draftMap = {...drafts}; draftMap[sessionId ?? ''] = input
      setDrafts(draftMap)
      setSessionId(newId)
      setInput(draftMap[newId] ?? '')
      // fetch messages
      fetch(`/api/sessions/${newId}/messages`).then(r=>r.json()).then(({messages}) => {
          const reconstructed = messages.map(toMsg)
          setMsgs(reconstructed)
      })
  }
  ```

- [ ] **Step 5:** Implement `handleCreate` / `handleRename` / `handleDelete` / `handleBulkClear`:
  ```ts
  async function handleCreate() {
      const created = await createSession({title: '新会话', cluster_id: clusterId || undefined})
      await refreshSessions()
      switchSession(created.id)
  }
  async function handleDelete(id: string) {
      await deleteSession(id)
      const {draftMap} = {draftMap: {...drafts}}; delete draftMap[id]
      setDrafts(draftMap)
      if (sessionId === id) { setSessionId(null); setInput(''); setMsgs([]) }
      await refreshSessions()
  }
  ```

- [ ] **Step 6:** `send()` modified — clear `drafts[sessionId]` after successful send

- [ ] **Step 7:** `pnpm typecheck`

- [ ] **Step 8:** Commit `feat(web): ChatView 2-pane with SessionsPanel + drafts`

---

## Task 9: Playwright e2e — 7 scenarios

**Files:**
- Create: `/tmp/ui-test/test-sessions.js`

- [ ] **Step 1:** Open browser, navigate to `http://127.0.0.1:8080/`, screenshot — SessionsPanel visible

- [ ] **Step 2:** Scenario 13.1: panel shows existing sessions with title + cluster + time

- [ ] **Step 3:** Scenario 13.2: type partial title in search → list filters

- [ ] **Step 4:** Scenario 13.3: click session A → input empty → type "test draft" → click session B → click session A → input is "test draft"

- [ ] **Step 5:** Scenario 13.4: double-click title → input appears → type new title + Enter → PUT request + list updates

- [ ] **Step 6:** Scenario 13.5: hover row → click delete → ConfirmModal → confirm → list shrinks

- [ ] **Step 7:** Scenario 13.6: click export MD → verify download `session-<id>.md` (use Playwright `page.waitForEvent('download')`) + read file content (assert contains `# 会话:`)

- [ ] **Step 8:** Scenario 13.7: click "清空全部" → ConfirmModal shows N → confirm → list empty

- [ ] **Step 9:** Run script — expect all 7 scenarios pass

---

## Task 10: Final verification + commit

**Files:**
- (no new files)

- [ ] **Step 1:** Run `GOSUMDB=sum.golang.org go test ./...` — expect all green
- [ ] **Step 2:** Run `pnpm typecheck` — expect clean
- [ ] **Step 3:** Run `/tmp/ui-test/test-sessions.js` — expect 7 scenarios pass
- [ ] **Step 4:** Run `git log --oneline openspec/changes/session-management/` — confirm 7 commits (brainstorm/design/proposal/specs/tasks/plan + impl commits)
- [ ] **Step 5:** Final summary report to user

---

**Notes for implementers:**

- Frequent commits: 1 commit per task minimum (some tasks split into 2-3 sub-commits)
- All Go test commands prefixed with `GOSUMDB=sum.golang.org` per repo convention
- Spec deviations must be documented in `openspec/changes/session-management/design.md` under "Drift Summary" before archive
- Use mock `d.Sessions` map (existing `SessionsManager` interface) to test 409 active-session protection without real agent