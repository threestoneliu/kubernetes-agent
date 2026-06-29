## ADDED Requirements

### Requirement: Danger Operation Confirmation

The system SHALL require explicit user confirmation before executing danger operations (reboot, disk operations) to prevent accidental execution.

#### Scenario: Danger operation triggers confirmation modal
- **WHEN** the user initiates a reboot or disk operation
- **THEN** the system SHALL display a confirmation modal showing:
  - Node list (hostname + IP)
  - Operation summary (type + parameters)
  - Impact analysis (related services, associated K8s workloads if available)
- **AND** the modal SHALL require the user to type "CONFIRM" before execution proceeds

#### Scenario: User cancels danger operation
- **WHEN** the user clicks cancel or closes the confirmation modal
- **THEN** the system SHALL abort the operation and return to idle state
- **AND** no SSH commands SHALL be sent to any target node

#### Scenario: User confirms danger operation
- **WHEN** the user correctly types "CONFIRM" and submits
- **THEN** the system SHALL proceed with executing the operation on all target nodes
- **AND** the operation SHALL be recorded in the audit log with `danger: true`

#### Scenario: User types incorrect confirmation word
- **WHEN** the user submits a confirmation with text other than "CONFIRM"
- **THEN** the system SHALL reject the submission
- **AND** SHALL display an error message instructing the user to type exactly "CONFIRM"

---

### Requirement: Operation Preview

Before executing any operation (danger or non-danger), the system SHALL display a preview of the execution plan showing affected nodes and the commands that will be run.

#### Scenario: Preview shows target nodes and commands
- **WHEN** the user has configured a task and clicks execute
- **THEN** the system SHALL display a preview showing:
  - Target node list (expanded from label selector)
  - The exact command(s) that will be executed per node
  - Execution mode (parallel or sequential)

#### Scenario: Preview allows cancellation before execution
- **WHEN** the user reviews the preview
- **THEN** the user SHALL be able to cancel and return to task editing
- **AND** no execution SHALL occur until the user explicitly confirms

---

## MODIFIED Requirements

（无）

---

## REMOVED Requirements

（无）

---

## RENAMED Requirements

（无）