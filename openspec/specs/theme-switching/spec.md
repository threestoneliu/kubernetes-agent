# theme-switching Specification

## Purpose
TBD - created by archiving change webui-redesign. Update Purpose after archive.
## Requirements
### Requirement: Dual Theme System
The application SHALL support two themes: "dark" (Dark Pro — deep charcoal + neon blue) as default, and "light" (clean white + slate). All colors SHALL be expressed as CSS custom properties so switching themes requires only changing variable values, not rewriting styles.

#### Scenario: Default theme is dark
- **WHEN** user first visits the application (no stored preference)
- **THEN** the theme SHALL render as "dark" (Dark Pro)

#### Scenario: Theme stored in localStorage
- **WHEN** user has previously selected a theme
- **THEN** on next visit the application SHALL load the stored theme from `localStorage.getItem('app-theme')`

---

### Requirement: Theme Switching Mechanism
The application SHALL provide a theme toggle control accessible from the UI. Clicking the toggle SHALL switch between dark and light themes without page reload.

#### Scenario: Theme toggle switches theme
- **WHEN** user clicks the theme toggle control
- **THEN** the application SHALL immediately switch to the other theme (dark ↔ light) and persist the choice to localStorage

#### Scenario: Theme toggle accessible from nav
- **WHEN** user is viewing any main view (chat/clusters/policies)
- **THEN** the theme toggle SHALL be accessible from the left navigation area

---

### Requirement: Dark Pro Theme Color Values
When `data-theme="dark"` is set on `<html>`, the CSS custom properties SHALL resolve to the values defined in the table below.

#### Scenario: Dark Pro renders with correct color values
- **WHEN** `<html data-theme="dark">` is rendered
- **THEN** `--bg` SHALL resolve to `#0d1117`, `--primary` to `#58a6ff`, and `--fg` to `#e6edf3`

| Property | Value | Usage |
|---|---|---|
| --bg | #0d1117 | Page background |
| --bg-elevated | #161b22 | Cards, panels |
| --bg-sidebar | #010409 | Sidebar/nav background |
| --fg | #e6edf3 | Primary text |
| --fg-muted | #8b949e | Secondary text |
| --border | #21262d | Borders, dividers |
| --primary | #58a6ff | Accent blue |
| --primary-glow | rgba(88,166,255,0.15) | Focus rings |
| --ok | #238636 | Success/allow |
| --tool-text | #7ee787 | Tool output text |

---

### Requirement: Light Theme Color Values
When `data-theme="light"` is set on `<html>`, the CSS custom properties SHALL resolve to the values defined in the table below.

#### Scenario: Light theme renders with correct color values
- **WHEN** `<html data-theme="light">` is rendered
- **THEN** `--bg` SHALL resolve to `#ffffff`, `--primary` to `#0969da`, and `--fg` to `#1f2328`

| Property | Value | Usage |
|---|---|---|
| --bg | #ffffff | Page background |
| --bg-elevated | #ffffff | Cards, panels |
| --bg-sidebar | #f6f8fa | Sidebar/nav background |
| --fg | #1f2328 | Primary text |
| --fg-muted | #656d76 | Secondary text |
| --border | #d0d7de | Borders, dividers |
| --primary | #0969da | Accent blue |
| --primary-glow | rgba(9,105,218,0.1) | Focus rings |
| --ok | #1a7f37 | Success/allow |
| --tool-text | #116329 | Tool output text |

---

### Requirement: Theme Transition
Theme switches SHALL be visually smooth with a CSS `transition` on background-color, color, and border-color properties (150ms ease).

#### Scenario: Theme switch animates smoothly
- **WHEN** user toggles the theme
- **THEN** all themed elements SHALL transition their colors over 150ms rather than switching instantly

