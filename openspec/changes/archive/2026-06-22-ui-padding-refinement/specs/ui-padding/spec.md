## ADDED Requirements

### Requirement: Header bar SHALL have horizontal padding of 20px on each side

The global header bar at the top of the viewport SHALL have `padding: 0 20px` applied to its left and right edges, ensuring logo, navigation tabs, and action buttons do not visually touch the viewport edges.

#### Scenario: Header padding visible
- **WHEN** the page loads in dark or light theme
- **THEN** the header bar content is inset 20px from the left and right viewport edges

---

### Requirement: Sessions panel SHALL have padding of 16px top/bottom and 12px left/right

The sessions panel sidebar SHALL have `padding: 16px 12px` applied, providing vertical breathing room between the panel header and session list items, and horizontal separation from the main content area.

#### Scenario: Sessions panel padding visible
- **WHEN** the page loads
- **THEN** the session list items are inset 16px from the top and bottom of the sessions panel, and 12px from the left and right edges

---

### Requirement: Sessions panel SHALL display a "会话列表" title above the session list

The sessions panel SHALL render a `.panel-title` label with the text "会话列表" (Sessions List) positioned above the session list items, styled in muted color, uppercase, 12px font size.

#### Scenario: Panel title visible
- **WHEN** the sessions panel renders
- **THEN** a "会话列表" label appears at the top of the panel in muted color with uppercase styling

---

### Requirement: Main content area SHALL have padding of 16px on all sides

The main content column (`.main`) SHALL have `padding: 16px` applied on all sides, separating toolbar and chat stream from the viewport edges.

#### Scenario: Main area padding visible
- **WHEN** the page loads
- **THEN** the toolbar and chat stream are inset 16px from the edges of the main column

---

### Requirement: Chat stream SHALL have internal padding of 20px and border-radius of 16px

The chat message area (`.chat-stream`) SHALL have `padding: 20px` applied internally, and its border-radius SHALL be increased from 12px to 16px for a softer appearance.

#### Scenario: Chat stream padding and radius
- **WHEN** the chat stream renders messages
- **THEN** messages are inset 20px from the chat stream borders and the chat stream has rounded corners of 16px radius

---

### Requirement: Panel footer SHALL have top padding of 12px

The bottom action bar within the sessions panel (`.panel-footer`) SHALL have `padding-top: 12px` to visually separate action buttons from the session list above.

#### Scenario: Panel footer padding visible
- **WHEN** the sessions panel renders
- **THEN** the footer buttons are separated from the session list by 12px of vertical space
