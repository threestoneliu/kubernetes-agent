# Policy Form Editor

## ADDED Requirements

### Requirement: PolicyFormModal SHALL display a structured form alongside a YAML editor in a left-right split layout

The modal SHALL render the form on the left (approximately 60% width) and a YAML text editor on the right (approximately 40% width). Both panels SHALL be visible simultaneously without tab switching.

### Requirement: PolicyFormModal SHALL support both create and edit modes

When `policy` prop is `null`, the modal SHALL operate in create mode with empty default values. When `policy` is a `Policy` object, the modal SHALL operate in edit mode and populate all fields by parsing `policy.yaml`.

### Requirement: Form fields SHALL be serialized to YAML in real time when any field changes

When the user modifies any form field, the YAML panel SHALL update immediately to reflect the current form state without requiring a save action.

#### Scenario: name field update propagates to YAML
- **WHEN** the user changes the `name` field value
- **THEN** the YAML panel SHALL update immediately with the new `name` value

#### Scenario: action checkbox toggle propagates to YAML
- **WHEN** the user toggles the `apply` checkbox
- **THEN** the YAML panel SHALL update the `action` list accordingly

### Requirement: YAML editor SHALL be parsed and populate the form with 300ms debounce

When the user manually edits the YAML panel, the form fields SHALL update after 300ms of inactivity to reflect the parsed YAML content.

#### Scenario: valid YAML update populates form fields
- **WHEN** the user enters valid YAML in the right panel and stops typing for 300ms
- **THEN** the form fields SHALL update to match the parsed YAML content

#### Scenario: invalid YAML highlights error without clearing form
- **WHEN** the user enters YAML that fails to parse
- **THEN** the YAML panel SHALL display a red border to indicate the error and the form fields SHALL remain unchanged from their last valid state

### Requirement: name field SHALL be a plain text input

The `name` field SHALL be rendered as a single-line text input. The field is required.

#### Scenario: name is required
- **WHEN** the user clears the name field
- **THEN** the save action SHALL be disabled

### Requirement: effect field SHALL be a dropdown select

The `effect` field SHALL be rendered as a `<select>` element with three options: `allow`, `confirm`, and `deny`. Default value is `deny`.

#### Scenario: effect dropdown options
- **WHEN** the user opens the effect dropdown
- **THEN** the dropdown SHALL show exactly three options: allow, confirm, deny

### Requirement: action field SHALL be a group of checkboxes

The `action` field SHALL be rendered as three checkboxes labeled "apply", "delete", and "scale". At least one checkbox MUST be selected to enable save.

#### Scenario: action requires at least one selection
- **WHEN** all action checkboxes are unchecked
- **THEN** the save button SHALL be disabled

### Requirement: namespace and kind fields SHALL use a tag input component

The `namespace` and `kind` fields SHALL be rendered as tag input components where the user types a value and presses Enter to add it as a tag. Tags SHALL be removable via a delete button.

#### Scenario: tag added on Enter
- **WHEN** the user types "production" in the namespace input and presses Enter
- **THEN** a tag labeled "production" SHALL appear and the input SHALL be cleared

#### Scenario: tag removed on delete click
- **WHEN** the user clicks the × button on a namespace tag
- **THEN** that tag SHALL be removed from the list

#### Scenario: duplicate tag prevented
- **WHEN** the user attempts to add a tag with a name that already exists
- **THEN** the tag SHALL NOT be added

### Requirement: unsafeFields field SHALL be a plain text input

The `unsafeFields` field SHALL be rendered as a multiline textarea accepting raw YAML/JSON input without parsing or validation. The user provides JSONPath expressions directly.

### Requirement: save SHALL validate all fields before submitting

Before calling the API, the component SHALL verify: `name` is non-empty, at least one `action` is checked, and the YAML content is valid. Save SHALL be disabled until all validations pass.

#### Scenario: save button disabled with empty name
- **WHEN** the name field is empty
- **THEN** the save button SHALL be disabled

#### Scenario: save button disabled when YAML has parse error
- **WHEN** the YAML panel contains invalid YAML
- **THEN** the save button SHALL be disabled

### Requirement: PolicyView SHALL use PolicyFormModal for all create and edit operations

The list view SHALL trigger `PolicyFormModal` for creating new policies and for editing existing policies. The inline YAML textarea editor SHALL be removed from the list view.
