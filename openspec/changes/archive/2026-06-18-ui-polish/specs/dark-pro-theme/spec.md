## ADDED Requirements

### Requirement: CSS Variable Color System
`web/src/styles.css` SHALL define all color and design tokens as CSS custom properties in `:root`, covering backgrounds (page, card, panel, sidebar, input), text (primary, title, muted), borders, accent (primary neon blue + dark variant + glow), functional colors (danger, ok, error), and spacing scale.

#### Scenario: Page background renders as deep charcoal
- **WHEN** browser renders any view (Chat / Clusters / Policies)
- **THEN** the page background SHALL be `#1a1a2e` (--bg)

#### Scenario: Primary text renders as bright white
- **WHEN** browser renders body text
- **THEN** primary text SHALL be `#e8e8f0` (--fg), titles SHALL be `#f0f0ff` (--fg-title)

#### Scenario: Accent color is neon blue
- **WHEN** browser renders any element using --primary
- **THEN** that element SHALL render with `#00D4FF` neon blue

---

### Requirement: Button Visual System
Buttons SHALL use the following visual variants:
- Default: `--input-bg` background, `--border` border, `--fg` text
- Primary: `linear-gradient(135deg, #00D4FF, #0088CC)` background, `--primary-fg` text, `box-shadow: 0 4px 16px rgba(0,212,255,0.3)`
- Danger: `--danger` border and text
- Hover: background lightens to `--card-bg` for non-primary
- Focus: `box-shadow: 0 0 0 3px var(--primary-glow)` (3px neon glow ring)
- Disabled: `opacity: 0.5; cursor: not-allowed`
- Border-radius: 8px on all buttons

#### Scenario: Primary button renders with gradient and glow
- **WHEN** browser renders a `button.primary`
- **THEN** it SHALL display `linear-gradient(135deg, #00D4FF, #0088CC)` background with `box-shadow: 0 4px 16px rgba(0,212,255,0.3)`

#### Scenario: Button focus shows neon glow ring
- **WHEN** browser renders a focused `button` (keyboard tab navigation)
- **THEN** it SHALL display `box-shadow: 0 0 0 3px rgba(0,212,255,0.2)`

---

### Requirement: Input Field Visual System
Input fields SHALL use: `--input-bg` background, `--border` border, 8px border-radius, 6px padding, `--fg` text color. Focus state SHALL use `--primary` border and `box-shadow: 0 0 0 3px rgba(0,212,255,0.2)`.

#### Scenario: Input focus renders with neon glow
- **WHEN** browser renders a focused `input` or `textarea` (keyboard or mouse)
- **THEN** border SHALL change to `--primary` and `box-shadow: 0 0 0 3px rgba(0,212,255,0.2)` SHALL appear

---

### Requirement: Shadow Layer System
The CSS SHALL define a shadow scale: `--shadow-sm: 0 2px 8px rgba(0,0,0,0.3)`, `--shadow-md: 0 4px 16px rgba(0,0,0,0.3)`, `--shadow-lg: 0 8px 40px rgba(0,0,0,0.4)`.

#### Scenario: Cards apply medium shadow
- **WHEN** browser renders `.card` or `.sessions-panel`
- **THEN** they SHALL apply `--shadow-md`

#### Scenario: Modals apply large shadow
- **WHEN** browser renders any modal overlay
- **THEN** the modal panel SHALL apply `--shadow-lg`

---

### Requirement: Border Radius System
The CSS SHALL define a unified border-radius scale: 8px for buttons/inputs/selects, 12px for cards/bubbles/session-rows, 14px for panels/modals. No element SHALL use border-radius: 0.

#### Scenario: Elements render with correct border-radius
- **WHEN** browser renders buttons, inputs, bubbles, and modals
- **THEN** buttons/inputs/selects SHALL have `border-radius: 8px`, bubbles/cards/session-rows SHALL have `border-radius: 12px`, and panels/modals SHALL have `border-radius: 14px`

---

### Requirement: Conversation Bubble Styles
All four bubble types SHALL be visually distinct:
- `.bubble.user`: `linear-gradient(135deg, #00D4FF, #0088CC)` background, white text, max-width 75%, `border-radius: 16px` with `border-bottom-right-radius: 4px`, `align-self: flex-end`
- `.bubble.assistant`: `--card-bg` background, `--border-soft` border, `--shadow-sm`, `border-bottom-left-radius: 4px`
- `.bubble.tool`: `--panel-bg` background, `--primary` text color, `--primary` border (semi-transparent), `--shadow-sm`
- `.bubble.reasoning` (or `.reasoning` block): `--panel-bg` background, `border-left: 3px solid --primary`, italic font style, `--muted` text color, max-width 85%

#### Scenario: User message renders as gradient right-aligned bubble
- **WHEN** browser renders a user message bubble
- **THEN** it SHALL display `linear-gradient(135deg, #00D4FF, #0088CC)` background, white text, right-aligned, with bottom-right corner clipped to 4px

#### Scenario: Reasoning block is visually subordinate
- **WHEN** browser renders a reasoning/thinking block
- **THEN** it SHALL display a left border in `--primary` color and italic text, visually distinguishing it from assistant text

---

### Requirement: Sidebar Visual System
The `.sidebar` SHALL use `--sidebar-bg: #16162a` background, 200px width, `--border` right border. Nav buttons SHALL use transparent background, `--muted` text; active state SHALL use `--primary` background at 15% opacity with `--primary` text and border.

#### Scenario: Active nav item highlights with neon accent
- **WHEN** browser renders an active `.sidebar button`
- **THEN** it SHALL display `--primary` at ~15% opacity background, `--primary` text, and a `--primary` border at 30% opacity

---

### Requirement: SessionsPanel Visual System
`.sessions-panel` SHALL use `--panel-bg: #1e1e38` background, `--border` border, 12px border-radius. Session rows SHALL use transparent background, `--muted` secondary text. Active row SHALL use `--primary` at 12% opacity background with `--primary` border at 30% opacity.

Cluster tags SHALL render as inline pills: `font-size: 10px`, `padding: 1px 6px`, `border-radius: 4px`, `--primary` background at 15% opacity, `--primary` border at 30% opacity, `--primary` text.

#### Scenario: Cluster tag renders as neon pill
- **WHEN** browser renders a cluster tag element
- **THEN** it SHALL display a small pill with `--primary` background at 15% opacity and `--primary` text color

---

### Requirement: Modal Visual System
All modals (PlanModal, ConfirmModal) SHALL use: `--card-bg: #252545` background, `--border` border, `--shadow-lg`, 14px border-radius. Overlay SHALL use `rgba(0,0,0,0.6)` backdrop. Primary confirm button SHALL use the primary gradient button style.

#### Scenario: Modal overlay dims background
- **WHEN** browser renders any modal overlay
- **THEN** the backdrop SHALL use `rgba(0,0,0,0.6)` and the modal panel SHALL display `--shadow-lg`

---

### Requirement: Markdown Content Adapts to Dark Theme
Markdown-rendered content inside assistant bubbles SHALL inherit `--fg` and `--muted` CSS variables, rendering correctly on dark backgrounds without white-on-white or black-on-black contrast issues.

#### Scenario: Markdown code blocks render with dark background
- **WHEN** browser renders a `pre code` block inside an assistant bubble
- **THEN** the text SHALL be `--fg` color and background SHALL be `--panel-bg` or darker
