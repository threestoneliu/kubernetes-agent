# UI Polish (Dark Pro Theme) Implementation Plan

**Goal:** Replace `web/src/styles.css` with a complete Dark Pro visual design system — deep charcoal background + neon blue accents — covering CSS variables, buttons, inputs, conversation bubbles, sidebar, sessions panel, chat area, and modals.

**Architecture:** Single CSS file change (`web/src/styles.css`). React components remain untouched — CSS class names are preserved, only property values change.

**Tech Stack:** Plain CSS with CSS custom properties (no framework, no build changes). Target: Chrome/Firefox/Safari.

---

## Task 1: Replace `:root` CSS Variables

**File:** `web/src/styles.css:1-19`

- [ ] **Step 1: Replace the entire `:root` block**

Delete lines 1–19 of `web/src/styles.css` and replace with:

```css
:root {
  /* Backgrounds */
  --bg:          #1a1a2e;
  --card-bg:     #252545;
  --panel-bg:    #1e1e38;
  --sidebar-bg:  #16162a;
  --input-bg:    #252545;

  /* Text */
  --fg:          #e8e8f0;
  --fg-title:    #f0f0ff;
  --muted:       #7070a0;

  /* Borders */
  --border:      #2a2a4e;
  --border-soft: #3a3a6a;

  /* Primary accent — neon blue */
  --primary:     #00D4FF;
  --primary-dark: #0088CC;
  --primary-fg:  #0a0a1a;
  --primary-glow: rgba(0, 212, 255, 0.2);

  /* Functional */
  --danger:      #ff6b6b;
  --ok-bg:       rgba(0, 212, 255, 0.12);
  --err-bg:      rgba(255, 107, 107, 0.12);
  --warn-bg:     rgba(251, 191, 36, 0.15);

  /* Shadows */
  --shadow-sm:   0 2px 8px rgba(0,0,0,0.3);
  --shadow-md:   0 4px 16px rgba(0,0,0,0.3);
  --shadow-lg:   0 8px 40px rgba(0,0,0,0.4);
  --shadow-glow: 0 0 0 3px var(--primary-glow);

  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  font-size: 14px;
  color: var(--fg);
}
```

Run: `grep -n "^--" web/src/styles.css | head -20` — confirm all new variables present

---

## Task 2: Global Element Styles

**File:** `web/src/styles.css:21-77`

- [ ] **Step 1: Update `html, body, #root` background**

Lines 23–28 — change `background: var(--bg)` already references `--bg` so it inherits automatically once :root is updated. No change needed.

- [ ] **Step 2: Update `button` default (lines 30–38)**

Replace with:
```css
button {
  font: inherit;
  border: 1px solid var(--border);
  background: var(--input-bg);
  padding: 6px 12px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--fg);
  transition: background 0.15s, box-shadow 0.15s;
}
```

- [ ] **Step 3: Update `button:hover:not(:disabled)` (lines 40–42)**

Replace with:
```css
button:hover:not(:disabled) {
  background: var(--card-bg);
}
```

- [ ] **Step 4: Update `button.primary` (lines 49–53)**

Replace with:
```css
button.primary {
  background: linear-gradient(135deg, var(--primary), var(--primary-dark));
  color: var(--primary-fg);
  border-color: transparent;
  box-shadow: 0 4px 16px rgba(0, 212, 255, 0.3);
}
```

- [ ] **Step 5: Update `button.primary:hover` (lines 55–58)**

Replace with:
```css
button.primary:hover:not(:disabled) {
  filter: brightness(1.08);
  box-shadow: 0 6px 24px rgba(0, 212, 255, 0.4);
  background: linear-gradient(135deg, var(--primary), var(--primary-dark));
}
```

- [ ] **Step 6: Update `button.danger` (lines 60–63)** — no change needed, uses `var(--danger)` which is updated

- [ ] **Step 7: Add button focus ring after `button.danger`**
```css
button:focus-visible {
  outline: none;
  box-shadow: var(--shadow-glow);
}
```

- [ ] **Step 8: Update `input, textarea, select` (lines 65–72)**

Replace with:
```css
input, textarea, select {
  font: inherit;
  border: 1px solid var(--border);
  background: var(--input-bg);
  border-radius: 8px;
  padding: 6px 8px;
  color: var(--fg);
  transition: border-color 0.15s, box-shadow 0.15s;
}
```

- [ ] **Step 9: Add `input:focus, textarea:focus, select:focus`**
```css
input:focus, textarea:focus, select:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: var(--shadow-glow);
}
```

- [ ] **Step 10: `textarea` (lines 74–77) — add monospace font remains, update background**
```css
textarea {
  font-family: 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  background: var(--input-bg);
  color: var(--fg);
}
```

---

## Task 3: App Layout (`.app`, `.sidebar`, `.main`)

**File:** `web/src/styles.css:79-117`

- [ ] **Step 1: Update `.app` (lines 79–82)** — already uses `display: flex; height: 100vh`, `background: var(--bg)` inherits from updated :root. No change needed.

- [ ] **Step 2: Update `.sidebar` (lines 84–92)**

Replace with:
```css
.sidebar {
  width: 200px;
  border-right: 1px solid var(--border);
  padding: 20px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: var(--sidebar-bg);
  flex-shrink: 0;
}
```

- [ ] **Step 3: Update `.sidebar h1` (lines 94–98)**

Replace with:
```css
.sidebar h1 {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
  margin: 0 0 20px 4px;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}
```

- [ ] **Step 4: Update `.sidebar button` (lines 100–103)**
```css
.sidebar button {
  text-align: left;
  width: 100%;
  background: transparent;
  border: 1px solid transparent;
  color: var(--muted);
}
```

- [ ] **Step 5: Update `.sidebar button.active` (lines 105–109)**

Replace with:
```css
.sidebar button.active {
  background: rgba(0, 212, 255, 0.12);
  color: var(--primary);
  border-color: rgba(0, 212, 255, 0.3);
  font-weight: 500;
}
```

- [ ] **Step 6: Update `.main` (lines 111–117)**

Replace with:
```css
.main {
  flex: 1;
  padding: 20px 24px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  background: var(--bg);
}
```

---

## Task 4: SessionsPanel Classes

**File:** `web/src/styles.css` — add new block after `.sidebar button.active`

Note: SessionsPanel uses CSS classes: `.sessions-panel`, `.session-list`, `.session-row`, `.title`, `.cluster-tag`, `.time-line`, `.row-main`, `.row-menu-btn`, `.row-menu`, `.panel-footer`, `.toolbar`

- [ ] **Step 1: Add `.sessions-panel`**
```css
.sessions-panel {
  width: 240px;
  background: var(--panel-bg);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex-shrink: 0;
}
```

- [ ] **Step 2: Add `.session-list`**
```css
.session-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
```

- [ ] **Step 3: Add `.session-row`**
```css
.session-row {
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
  border: 1px solid transparent;
  display: flex;
  align-items: center;
  gap: 8px;
}
.session-row:hover {
  background: rgba(255,255,255,0.05);
}
.session-row.active {
  background: rgba(0, 212, 255, 0.1);
  border-color: rgba(0, 212, 255, 0.25);
}
```

- [ ] **Step 4: Add `.row-main` (flex 1 for title + time)**
```css
.row-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
```

- [ ] **Step 5: Add `.title` and `.time-line`**
```css
.title {
  font-size: 13px;
  font-weight: 500;
  color: var(--fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.time-line {
  font-size: 11px;
  color: var(--muted);
}
```

- [ ] **Step 6: Add `.cluster-tag` pill**
```css
.cluster-tag {
  display: inline-block;
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(0, 212, 255, 0.12);
  color: var(--primary);
  border: 1px solid rgba(0, 212, 255, 0.25);
  flex-shrink: 0;
}
```

- [ ] **Step 7: Add `.row-menu-btn`**
```css
.row-menu-btn {
  background: transparent;
  border: none;
  padding: 4px 6px;
  color: var(--muted);
  cursor: pointer;
  border-radius: 6px;
  font-size: 14px;
  flex-shrink: 0;
}
.row-menu-btn:hover {
  background: rgba(255,255,255,0.08);
  color: var(--fg);
}
```

- [ ] **Step 8: Add `.row-menu` dropdown**
```css
.row-menu {
  position: absolute;
  right: 0;
  top: 100%;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 4px;
  list-style: none;
  margin: 0;
  min-width: 140px;
  box-shadow: var(--shadow-md);
  z-index: 50;
}
.row-menu li {
  padding: 7px 10px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: var(--fg);
}
.row-menu li:hover {
  background: rgba(255,255,255,0.06);
}
.row-menu li.danger {
  color: var(--danger);
}
```

- [ ] **Step 9: Add `.panel-footer`**
```css
.panel-footer {
  padding-top: 10px;
  border-top: 1px solid var(--border);
}
.panel-footer button {
  width: 100%;
  justify-content: center;
}
```

---

## Task 5: Chat Bubbles (`.msg`, `.bubble`)

**File:** `web/src/styles.css:163-207`

- [ ] **Step 1: Update `.chat-stream` (lines 163–170)**
```css
.chat-stream {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
```

- [ ] **Step 2: Update `.msg` (lines 172–181)**

Replace with:
```css
.msg {
  display: flex;
  flex-direction: column;
  margin: 0;
  max-width: 75%;
}
.msg.user { align-self: flex-end; align-items: flex-end; }
.msg.assistant { align-self: flex-start; align-items: flex-start; }
.msg.tool { align-self: stretch; align-items: stretch; max-width: 100%; }
```

- [ ] **Step 3: Update `.bubble` base (lines 183–188)**
```css
.bubble {
  padding: 12px 16px;
  border-radius: 16px;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
  font-size: 14px;
}
```

- [ ] **Step 4: Update `.bubble.user` (line 190)**
```css
.bubble.user {
  background: linear-gradient(135deg, var(--primary), var(--primary-dark));
  color: var(--primary-fg);
  border-bottom-right-radius: 4px;
  font-weight: 500;
}
```

- [ ] **Step 5: Update `.bubble.assistant` (line 191)**
```css
.bubble.assistant {
  background: var(--card-bg);
  border: 1px solid var(--border-soft);
  border-bottom-left-radius: 4px;
  box-shadow: var(--shadow-sm);
  color: var(--fg);
}
```

- [ ] **Step 6: Update `.bubble.tool-ok` and `.bubble.tool-err` (lines 192–193)**
```css
.bubble.tool-ok {
  background: var(--panel-bg);
  border: 1px solid rgba(0, 212, 255, 0.3);
  color: var(--primary);
  border-bottom-left-radius: 4px;
  font-size: 12px;
  font-family: 'Menlo', 'Consolas', monospace;
}
.bubble.tool-err {
  background: rgba(255, 107, 107, 0.08);
  border: 1px solid rgba(255, 107, 107, 0.3);
  color: var(--danger);
  border-bottom-left-radius: 4px;
  font-size: 12px;
  font-family: 'Menlo', 'Consolas', monospace;
}
```

- [ ] **Step 7: Add `.reasoning` block (after `.bubble.tool-err`)**
```css
.reasoning {
  background: var(--panel-bg);
  border-left: 3px solid var(--primary);
  padding: 10px 14px;
  border-radius: 0 8px 8px 0;
  font-size: 12px;
  color: var(--muted);
  font-style: italic;
  max-width: 85%;
  align-self: flex-start;
  margin-top: -4px;
}
```

- [ ] **Step 8: Update `details pre` (lines 200–207)** — markdown code blocks inside bubbles
```css
details pre {
  background: var(--panel-bg);
  padding: 10px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  overflow-x: auto;
  white-space: pre-wrap;
  font-family: 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  color: var(--fg);
  margin-top: 8px;
}
```

- [ ] **Step 9: Update `details summary` (lines 195–198)**
```css
details summary {
  cursor: pointer;
  user-select: none;
  color: var(--muted);
  font-size: 12px;
  padding: 2px 0;
}
```

---

## Task 6: ChatView Layout (`.composer`)

**File:** `web/src/styles.css:209-217`

- [ ] **Step 1: Update `.composer` (lines 209–213)**
```css
.composer {
  display: flex;
  gap: 10px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}
```

- [ ] **Step 2: Update `.composer input` (lines 215–216)**
```css
.composer input {
  flex: 1;
  padding: 12px 16px;
  border-radius: 12px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  font-size: 14px;
}
```

---

## Task 7: Modals (`.modal-overlay`, `.modal`)

**File:** `web/src/styles.css:219-264`

- [ ] **Step 1: Update `.modal-overlay` (lines 219–227)** — already has correct rgba, just confirm:
```css
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  backdrop-filter: blur(2px);
}
```

- [ ] **Step 2: Update `.modal` (lines 229–238)**
```css
.modal {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 24px;
  width: 520px;
  max-width: 90vw;
  max-height: 80vh;
  overflow: auto;
  box-shadow: var(--shadow-lg);
}
```

- [ ] **Step 3: Update `.modal h2, .modal h3` (add after line 243)**
```css
.modal h2, .modal h3 {
  color: var(--fg-title);
  font-weight: 600;
}
```

- [ ] **Step 4: Add `.modal-actions` styles (lines 245–250) remain, add cancel style**
```css
.modal-actions button.cancel {
  background: var(--panel-bg);
  border: 1px solid var(--border);
  color: var(--muted);
}
```

- [ ] **Step 5: Update `.toast` (lines 252–264)**
```css
.toast {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--danger);
  color: white;
  padding: 10px 16px;
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  z-index: 200;
  max-width: 80vw;
}
```

---

## Task 8: ClusterView / PolicyView (`.list`, `.row`, `.form-grid`, `.badge`)

**File:** `web/src/styles.css:266-296`

- [ ] **Step 1: Update `.form-grid label` (lines 271–277)**
```css
.form-grid label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
  font-weight: 500;
}
```

- [ ] **Step 2: Update `.list` (lines 287–292)**
```css
.list {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--card-bg);
  padding: 8px 12px;
}
```

- [ ] **Step 3: Update `.row` (lines 126–136)**
```css
.row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid var(--border);
  color: var(--fg);
}
.row:last-child {
  border-bottom: none;
}
.row:hover {
  background: rgba(255,255,255,0.02);
}
```

- [ ] **Step 4: Update badge backgrounds for dark theme**
```css
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--muted);
}
.badge.allow { background: var(--ok-bg); border-color: rgba(0,212,255,0.3); color: var(--primary); }
.badge.confirm { background: var(--warn-bg); border-color: rgba(251,191,36,0.3); color: #fbbf24; }
.badge.deny { background: var(--err-bg); border-color: rgba(255,107,107,0.3); color: var(--danger); }
```

---

## Task 9: Global Radius Consistency Pass

Scan for any remaining `border-radius: 6px` or `border-radius: 0` and update to consistent values:

- [ ] **Step 1: Scan for inconsistent border-radius**
```bash
grep -n "border-radius" web/src/styles.css
```
Fix any remaining 6px → 8px for buttons/inputs, 8px → 12px for bubbles, 8px → 14px for panels.

---

## Task 10: Visual Verification

- [ ] **Step 1: Start server and open Chrome**
```bash
# Server already running on PID 15654, verify it's up:
lsof -i :8080 | grep LISTEN
```

- [ ] **Step 2: Take screenshot of Chat view**
```bash
node -e "
const {chromium} = require('/tmp/ui-test/node_modules/playwright-core');
(async()=>{
  const b = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,args:['--no-sandbox']});
  const p = await b.newPage({viewport:{width:1400,height:900}});
  await p.goto('http://127.0.0.1:8080/',{waitUntil:'networkidle'});
  await p.screenshot({path:'/tmp/ui-test/shots/ui-dark-chat.png',fullPage:true});
  await b.close();
})().catch(e=>{console.error(e);process.exit(1);});
"
```

- [ ] **Step 3: Navigate to Clusters view and screenshot**
```bash
node -e "
const {chromium} = require('/tmp/ui-test/node_modules/playwright-core');
(async()=>{
  const b = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,args:['--no-sandbox']});
  const p = await b.newPage({viewport:{width:1400,height:900}});
  await p.goto('http://127.0.0.1:8080/',{waitUntil:'networkidle'});
  await p.click('button:has-text(\"集群\")');
  await p.waitForTimeout(500);
  await p.screenshot({path:'/tmp/ui-test/shots/ui-dark-clusters.png',fullPage:true});
  await b.close();
})().catch(e=>{console.error(e);process.exit(1);});
"
```

- [ ] **Step 4: Navigate to Policies view and screenshot**
```bash
node -e "
const {chromium} = require('/tmp/ui-test/node_modules/playwright-core');
(async()=>{
  const b = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,args:['--no-sandbox']});
  const p = await b.newPage({viewport:{width:1400,height:900}});
  await p.goto('http://127.0.0.1:8080/',{waitUntil:'networkidle'});
  await p.click('button:has-text(\"策略\")');
  await p.waitForTimeout(500);
  await p.screenshot({path:'/tmp/ui-test/shots/ui-dark-policies.png',fullPage:true});
  await b.close();
})().catch(e=>{console.error(e);process.exit(1);});
"
```

- [ ] **Step 5: Review screenshots and note any adjustments needed**

---

## Task 11: Commit

- [ ] **Step 1: Commit**
```bash
git add web/src/styles.css
git commit -m "feat(web): dark pro theme — complete styles.css redesign"
```

- [ ] **Step 2: Push**
```bash
git push
```
