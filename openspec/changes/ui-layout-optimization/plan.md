# Global Header Implementation Plan

**Goal:** Add a 48px global header with logo, navigation tabs, and action buttons, replacing the left 60px icon nav.

**Architecture:** Replace the three-column flex layout (nav + sessions-panel + main) with a column flex layout: header on top, sessions-panel + main body below. Navigation moves from left icons to header tabs.

**Tech Stack:** React (App.tsx), CSS (styles.css)

---

## Task 1: CSS / Layout Foundation

**Files:**
- Modify: `web/src/styles.css:1-80`

- [ ] **Step 1: Add header CSS variables to :root**

Insert at the top of `:root {}` block in styles.css:
```css
  --header-height: 48px;
  --header-bg: #010409;
  --header-fg: #e6edf3;
  --header-muted: #8b949e;
  --header-border: #21262d;
```

- [ ] **Step 2: Add header CSS variables to [data-theme="light"]**

Insert at the top of `[data-theme="light"] {}` block:
```css
  --header-bg: #f6f8fa;
  --header-fg: #1f2328;
  --header-muted: #656d76;
  --header-border: #d0d7de;
```

- [ ] **Step 3: Update .app layout from row to column**

Find `.app { display: flex; flex-direction: row; ... }` in styles.css and replace with:
```css
.app {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}
```

- [ ] **Step 4: Add .app-body wrapper styles**

Add after `.app { }`:
```css
.app-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}
```

- [ ] **Step 5: Add header-bar styles**

Add after `.app-body { }`:
```css
.header-bar {
  height: var(--header-height);
  background: var(--header-bg);
  border-bottom: 1px solid var(--header-border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 8px;
  flex-shrink: 0;
}
```

- [ ] **Step 6: Add .header-logo styles**

Add after `.header-bar { }`:
```css
.header-logo {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--header-fg);
  font-weight: 600;
  font-size: 15px;
  flex-shrink: 0;
}
```

- [ ] **Step 7: Add .header-nav and .nav-tab styles**

Add after `.header-logo { }`:
```css
.header-nav {
  display: flex;
  gap: 4px;
  margin-left: 24px;
}
.nav-tab {
  padding: 6px 14px;
  border-radius: 6px;
  cursor: pointer;
  background: transparent;
  border: none;
  color: var(--header-muted);
  font-size: 14px;
  transition: background 120ms, color 120ms;
}
.nav-tab:hover {
  background: rgba(255,255,255,0.06);
  color: var(--header-fg);
}
.nav-tab.active {
  background: var(--primary-glow);
  color: var(--primary);
}
```

- [ ] **Step 8: Add .header-actions and icon button styles**

Add after `.nav-tab.active { }`:
```css
.header-actions {
  display: flex;
  gap: 4px;
  margin-left: auto;
}
.icon-btn {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--header-muted);
  font-size: 16px;
}
.icon-btn:hover {
  background: rgba(255,255,255,0.08);
  color: var(--header-fg);
}
```

- [ ] **Step 9: Remove .nav styles**

Find and remove the `.nav { width: 60px; ... }` block from styles.css.

---

## Task 2: App.tsx Shell Restructure

**Files:**
- Modify: `web/src/App.tsx:1-71`

- [ ] **Step 1: Read current App.tsx structure**

Open `web/src/App.tsx` and confirm the current Shell function uses `<nav className="nav">` and `main className="main"`.

- [ ] **Step 2: Replace nav with header, wrap body in .app-body**

Replace the Shell function return body:
```tsx
return (
  <>
    <header className="header-bar">
      <div className="header-logo">
        <span>🤖</span>
        <span>Kubernetes Agent</span>
      </div>
      <nav className="header-nav">
        <button
          className={`nav-tab ${view === 'chat' ? 'active' : ''}`}
          onClick={() => setView('chat')}
        >
          对话
        </button>
        <button
          className={`nav-tab ${view === 'clusters' ? 'active' : ''}`}
          onClick={() => setView('clusters')}
        >
          集群
        </button>
        <button
          className={`nav-tab ${view === 'policies' ? 'active' : ''}`}
          onClick={() => setView('policies')}
        >
          策略
        </button>
      </nav>
      <div className="header-actions">
        <button className="icon-btn" onClick={toggle} title="切换主题">
          {theme === 'dark' ? '🌙' : '☀️'}
        </button>
        <button className="icon-btn" title="设置">
          ⚙
        </button>
      </div>
    </header>
    <div className="app-body">
      <main className="main">
        {view === 'chat' && <ChatView />}
        {view === 'clusters' && <ClusterView />}
        {view === 'policies' && <PolicyView />}
      </main>
    </div>
    {toast && <ErrorToast message={toast} onDismiss={dismiss} />}
  </>
)
```

- [ ] **Step 3: Remove old nav and main elements**

Confirm the new structure replaces `<nav className="nav">`, `<main className="main">`, and the `useState` for view remains local in Shell.

- [ ] **Step 4: Run the app to verify header renders**

Run `cd web && pnpm dev` and confirm header appears at top with logo, tabs, and theme toggle.

- [ ] **Step 5: Commit CSS and App.tsx changes**

```bash
git add web/src/styles.css web/src/App.tsx
git commit -m "feat(web): add global header with nav tabs and theme toggle"
```

---

## Verification

- [ ] **Verify:** All three views (Chat/Cluster/Policy) switch correctly via header tabs
- [ ] **Verify:** Sessions panel still shows at 240px inside ChatView
- [ ] **Verify:** Theme toggle works from header
- [ ] **Verify:** No console errors in browser

---

**Plan complete.** Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks
2. **Inline Execution** — implement tasks in this session with checkpoints

Which approach?