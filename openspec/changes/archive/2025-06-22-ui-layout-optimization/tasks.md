## 1. CSS / Layout Foundation

- [x] 1.1 Add `--header-height: 48px` and header color variables to `:root` and `[data-theme="light"]`
- [x] 1.2 Add `.header-bar` styles (flex row, 48px height, border-bottom, background)
- [x] 1.3 Add `.header-logo`, `.header-nav`, `.nav-tab`, `.header-actions` styles
- [x] 1.4 Add `.nav-tab.active` active state style
- [x] 1.5 Update `.app` from `flex-direction: row` to `flex-direction: column`
- [x] 1.6 Update `.app-body` to `display: flex; flex: 1` wrapper
- [x] 1.7 Remove `.nav` styles (no longer used) — set to `display: none` for backward compat

## 2. App.tsx Shell Restructure

- [x] 2.1 Replace `<nav className="nav">` with `<header className="header-bar">` in Shell
- [x] 2.2 Move view state (`useState<View>`) from local to Shell level (already local in Shell)
- [x] 2.3 Pass `view` and `setView` as props to Header nav tabs
- [x] 2.4 Wrap main content in `<div className="app-body">` with SessionsPanel + main side by side
- [x] 2.5 Move theme toggle button from Header's left nav to Header's right actions

## 3. Header Component Implementation

- [x] 3.1 Add logo (🤖) and "Kubernetes Agent" text to header-logo
- [x] 3.2 Add nav tabs ("对话", "集群", "策略") with click handlers to setView
- [x] 3.3 Add theme toggle and settings buttons to header-actions
- [x] 3.4 Ensure header adapts to light theme variables (uses CSS vars)

## 4. Cleanup & Verification

- [x] 4.1 Verify all three views (Chat/Cluster/Policy) load correctly via header nav
- [x] 4.2 Verify sessions panel displays correctly at 240px width inside ChatView
- [x] 4.3 Verify theme toggle works from new header location
- [x] 4.4 Take screenshot to confirm visual layout matches design
