## ADDED Requirements

### Requirement: Three-Column Layout Structure
The application shell `.app` SHALL render as a horizontal three-column flex layout: left nav (60px fixed width) + center sessions panel (240px fixed width) + right main chat area (flex: 1).

#### Scenario: Three columns render simultaneously
- **WHEN** browser renders the application shell
- **THEN** left nav, center sessions panel, and right chat area SHALL all be visible simultaneously at all times

#### Scenario: Left nav contains icon-only navigation
- **WHEN** browser renders the left nav column
- **THEN** it SHALL display three icon buttons (💬/☸/🛡) for chat/clusters/policies navigation, each with hover tooltip

#### Scenario: Center panel is session list
- **WHEN** browser renders the center column
- **THEN** it SHALL display the session list with search input and "新建会话" button at top

#### Scenario: Right area is chat main
- **WHEN** browser renders the right column
- **THEN** it SHALL display the ChatView component (chat header + messages + composer)

---

### Requirement: Three-Column Layout Responsive Behavior
The three-column layout SHALL maintain fixed widths regardless of viewport size. On viewports narrower than 768px, the layout MAY collapse to a single-column view at the implementer's discretion.

#### Scenario: Layout does not reflow on resize
- **WHEN** browser window is resized (above 768px)
- **THEN** the three-column widths SHALL remain: nav 60px, sessions 240px, chat flex:1

---

### Requirement: Session Panel Active State
The session list SHALL highlight the currently active session with a visible active state (background color change), and switching sessions SHALL NOT affect the visibility of the sessions panel.

#### Scenario: Active session highlighted
- **WHEN** a session is selected
- **THEN** that session row SHALL show active background color and the chat area SHALL load that session's messages

#### Scenario: Sessions panel always visible
- **WHEN** user is viewing chat, clusters, or policies view
- **THEN** the sessions panel SHALL remain visible on the left
