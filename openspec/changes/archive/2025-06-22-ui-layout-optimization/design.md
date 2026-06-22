# UI Layout Optimization Design

## Overview

Add a global header to the WebUI, replace the left icon-only Nav with header-based navigation, and improve overall visual coherence.

## Layout Changes

### Before
```
┌──────────────────────────────────────┐
│ ☰ Nav │        main content         │
│ (60px)│                           │
└──────────────────────────────────────┘
```

### After
```
┌──────────────────────────────────────────────────────┐
│ Logo │ 对话  集群  策略 │ │ Theme │ ⚙ │   │
├──────────────────────────────────────────────────────┤
│ Sessions │                                        │
│ Panel   │              Chat/Messages              │
│ (240px)│                                        │
└──────────────────────────────────────────────────────┘
```

## Component Specifications

### Global Header (48px height)

**CSS Variables:**
```css
--header-height: 48px;
--header-bg: var(--bg-sidebar);  /* #010409 in dark */
--header-border: var(--border);   /* #21262d in dark */
```

**Structure:**
- `header.header-bar` — flex row, 48px height, padding 0 16px
  - `.header-logo` — flex row, gap 8px, align-items center
    - `span.logo-icon` — 20px icon
    - `span.logo-text` — "Kubernetes Agent" text
  - `.header-nav` — flex row, gap 4px
    - `.nav-tab` — padding 8px 16px, border-radius 6px
    - `.nav-tab.active` — background var(--primary-glow), color var(--primary)
  - `.header-actions` — flex row, gap 8px, margin-left auto
    - `.theme-btn` — icon button
    - `.settings-btn` — icon button

**HTML Structure:**
```html
<header class="header-bar">
  <div class="header-logo">
    <span class="logo-icon">🤖</span>
    <span class="logo-text">Kubernetes Agent</span>
  </div>
  <nav class="header-nav">
    <button class="nav-tab active">对话</button>
    <button class="nav-tab">集群</button>
    <button class="nav-tab">策略</button>
  </nav>
  <div class="header-actions">
    <button class="icon-btn theme-btn">🌙</button>
    <button class="icon-btn settings-btn">⚙</button>
  </div>
</header>
```

### App Shell Changes

**Before:**
```css
.app {
  display: flex;
  flex-direction: row;
  height: 100vh;
}
.nav { width: 60px; }
.main { flex: 1; }
```

**After:**
```css
.app {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
.header-bar { height: 48px; flex-shrink: 0; }
.app-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}
.sessions-panel { width: 240px; flex-shrink: 0; }
.main { flex: 1; overflow: hidden; }
```

### Sessions Panel

Sessions panel remains inside ChatView at 240px width, no structural changes.

### ChatView

- Toolbar currently at top of ChatView moves below header (no change in position)
- `.main` now receives the ChatView directly

### Nav Icon Removal

The old `.nav` (60px left sidebar) is removed entirely from App.tsx.

## Theme Support

### Dark Theme (default)
```css
:root {
  --header-bg: #010409;
  --header-border: #21262d;
  --header-fg: #e6edf3;
  --header-muted: #8b949e;
}
```

### Light Theme
```css
:root[data-theme="light"] {
  --header-bg: #f6f8fa;
  --header-border: #d0d7de;
  --header-fg: #1f2328;
  --header-muted: #656d76;
}
```

## Implementation Notes

1. **App.tsx**: Replace `.nav` with `<header class="header-bar">`, wrap main content in `.app-body`
2. **styles.css**: Add `.header-bar`, `.header-logo`, `.header-nav`, `.nav-tab`, `.header-actions` styles
3. **ThemeContext**: Theme toggle button moves from App.tsx to Header
4. **View state**: Managed in App.tsx Shell, passed to Header nav tabs as props
5. **No breaking changes**: SessionsPanel, ChatView, ClusterView, PolicyView remain unchanged internally

## Files to Modify

1. `web/src/App.tsx` — restructure layout, add header
2. `web/src/styles.css` — add header styles
3. `web/src/contexts/ThemeContext.tsx` — expose toggle in context for header access
