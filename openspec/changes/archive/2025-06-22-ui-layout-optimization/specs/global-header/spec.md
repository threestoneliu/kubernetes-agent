## ADDED Requirements

### Requirement: Global Header SHALL be 48px tall and fixed at the top of the viewport

The header SHALL occupy a fixed 48px strip at the very top of the browser window, spanning the full width. It SHALL NOT scroll with page content.

#### Scenario: Header remains fixed during scroll
- **WHEN** user scrolls the main content area
- **THEN** the header stays visually anchored at the top with no movement

---

### Requirement: Header SHALL contain Logo, Navigation Tabs, and Action Buttons

The header SHALL contain, from left to right: a logo+app-name block, navigation tabs (对话/集群/策略), and action buttons (theme toggle, settings) aligned to the right edge.

#### Scenario: Logo and app name visible
- **WHEN** page loads
- **THEN** the leftmost element in the header is a logo icon and "Kubernetes Agent" text

#### Scenario: Navigation tabs switch view
- **WHEN** user clicks a navigation tab
- **THEN** the corresponding view (Chat/Cluster/Policy) is displayed and the tab shows active state

#### Scenario: Theme toggle changes color scheme
- **WHEN** user clicks the theme toggle button
- **THEN** the page color scheme switches between dark and light themes

---

### Requirement: Navigation tabs SHALL display active state for current view

The tab corresponding to the currently displayed view SHALL have visual distinction (e.g., primary color background or underline).

#### Scenario: Active tab highlighted
- **WHEN** user is viewing the Chat view
- **THEN** the "对话" tab is visually marked as active

---

### Requirement: Header SHALL adapt to both dark and light themes

All header elements SHALL use CSS variables (--header-bg, --header-fg, --header-border) so they respond to theme changes without separate stylesheets.

#### Scenario: Header in light theme
- **WHEN** user switches to light theme
- **THEN** header background becomes #f6f8fa and text becomes #1f2328

---

### Requirement: Sessions Panel and ChatView content area SHALL fill remaining viewport height

The area below the header SHALL fill 100% of the remaining vertical space, with Sessions Panel (240px) on the left and the active view on the right, both scrolling independently.

#### Scenario: Sessions list scrolls independently from chat
- **WHEN** user scrolls the sessions list
- **THEN** the chat area does not scroll

---

## MODIFIED Requirements

(None — this is a new capability with no existing spec to modify.)
