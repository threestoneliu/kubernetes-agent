# WebUI Redesign Implementation Plan

**Goal:** 实现三栏布局 + Dark/Light 双主题切换，无需改动业务逻辑。

**Architecture:** App.tsx 三栏结构 + CSS 变量主题系统 + React Context 主题状态，零依赖新增。

**Tech Stack:** Plain CSS custom properties, React Context, localStorage.

---

## Task 1: ThemeContext

**Files:**
- Create: `web/src/contexts/ThemeContext.tsx`

- [ ] **Step 1: Create ThemeContext**

```tsx
import { createContext, useContext, useState, useEffect } from 'react'

type Theme = 'dark' | 'light'

interface ThemeContextValue {
  theme: Theme
  toggle: () => void
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: 'dark',
  toggle: () => {},
})

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    return (localStorage.getItem('app-theme') as Theme) ?? 'dark'
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('app-theme', theme)
  }, [theme])

  return (
    <ThemeContext.Provider value={{ theme, toggle: () => setTheme(t => t === 'dark' ? 'light' : 'dark') }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => useContext(ThemeContext)
```

- [ ] **Step 2: Integrate in App.tsx**

```tsx
import { ThemeProvider } from './contexts/ThemeContext'
// Wrap <Shell /> with <ThemeProvider>
```

Run: `pnpm typecheck` — should pass

---

## Task 2: styles.css — Full Theme Variable System

**File:** `web/src/styles.css` (complete rewrite)

- [ ] **Step 1: Dark Pro CSS variables**

Replace entire file with dark theme `:root` + `[data-theme="light"]` overrides. Key structure:

```css
/* Dark (default) */
:root {
  --bg: #0d1117;
  --bg-elevated: #161b22;
  --bg-sidebar: #010409;
  --fg: #e6edf3;
  --fg-muted: #8b949e;
  --border: #21262d;
  --primary: #58a6ff;
  --primary-glow: rgba(88,166,255,0.15);
  --ok: #238636;
  --tool-text: #7ee787;
  --shadow-sm: 0 2px 8px rgba(0,0,0,0.3);
  --shadow-md: 0 4px 16px rgba(0,0,0,0.4);
  --shadow-lg: 0 8px 40px rgba(0,0,0,0.5);
  --shadow-glow: 0 0 0 3px var(--primary-glow);
}

/* Light override */
:root[data-theme="light"] {
  --bg: #ffffff;
  --bg-elevated: #ffffff;
  --bg-sidebar: #f6f8fa;
  --fg: #1f2328;
  --fg-muted: #656d76;
  --border: #d0d7de;
  --primary: #0969da;
  --primary-glow: rgba(9,105,218,0.1);
  --ok: #1a7f37;
  --tool-text: #116329;
  --shadow-sm: 0 1px 4px rgba(0,0,0,0.08);
  --shadow-md: 0 2px 12px rgba(0,0,0,0.1);
  --shadow-lg: 0 4px 24px rgba(0,0,0,0.12);
  --shadow-glow: 0 0 0 3px var(--primary-glow);
}
```

Add `transition: 150ms` on `body *` for smooth theme switch.

---

## Task 3: App.tsx Three-Column Structure

**File:** `web/src/App.tsx`

- [ ] **Step 1: Replace Shell function body**

```tsx
function Shell() {
  const [view, setView] = useState<View>('chat')
  const { theme, toggle } = useTheme()
  const { toast, dismiss } = useToast()
  return (
    <div className="app">
      {/* Left nav — 60px */}
      <nav className="nav">
        <button className={`nav-item ${view==='chat'?'active':''}`} onClick={()=>setView('chat')} title="对话">💬</button>
        <button className={`nav-item ${view==='clusters'?'active':''}`} onClick={()=>setView('clusters')} title="集群">☸</button>
        <button className={`nav-item ${view==='policies'?'active':''}`} onClick={()=>setView('policies')} title="策略">🛡</button>
        <div style={{flex:1}} />
        <button className="nav-item" onClick={toggle} title="切换主题">🎨</button>
        <button className="nav-item" title="设置">⚙</button>
      </nav>

      {/* Center — sessions panel, always visible when in chat */}
      {view === 'chat' && (
        <aside className="sessions-panel">
          <SessionsPanel ... />
        </aside>
      )}

      {/* Right — main content */}
      <main className="main">
        {view === 'chat' && <ChatView />}
        {view === 'clusters' && <ClusterView />}
        {view === 'policies' && <PolicyView />}
      </main>

      {toast && <ErrorToast message={toast} onDismiss={dismiss} />}
    </div>
  )
}
```

- [ ] **Step 2: Add nav/sessions-panel CSS to styles.css**

```css
.app { display: flex; flex-direction: row; height: 100vh; overflow: hidden; }
.nav { width: 60px; display: flex; flex-direction: column; align-items: center; padding: 12px 0; gap: 4px; background: var(--bg-sidebar); border-right: 1px solid var(--border); flex-shrink: 0; }
.nav-item { width: 40px; height: 40px; border-radius: 10px; border: none; background: transparent; cursor: pointer; display: flex; align-items: center; justify-content: center; font-size: 18px; color: var(--fg-muted); transition: background 0.15s, color 0.15s; }
.nav-item:hover { background: var(--bg-elevated); color: var(--fg); }
.nav-item.active { background: var(--primary-glow); color: var(--primary); }
.sessions-panel { width: 240px; background: var(--bg-elevated); border-right: 1px solid var(--border); display: flex; flex-direction: column; flex-shrink: 0; overflow: hidden; }
.main { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
```

---

## Task 4: Rebuild and Verify

- [ ] **Step 1: `make build`**
```bash
make build 2>&1 | tail -5
```

- [ ] **Step 2: Launch and screenshot**
```bash
./kubernetes-agent &
sleep 2
# screenshot chat view
```

- [ ] **Step 3: Toggle theme and screenshot light theme**

- [ ] **Step 4: Navigate to clusters + policies, screenshot**

---

## Task 5: Commit

- [ ] `git add web/src/ && git commit -m "feat(web): three-column layout + dark/light theme switching"`
- [ ] `git push`
