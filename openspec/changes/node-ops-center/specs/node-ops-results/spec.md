## ADDED Requirements

### Requirement: Execution Result Summary

The system SHALL display an aggregated summary view of task execution results showing success, failure, and total counts.

#### Scenario: Summary shows aggregated counts
- **WHEN** a task execution completes on multiple nodes
- **THEN** the system SHALL display:
  - Total nodes targeted
  - Success count (nodes with status "success")
  - Failure count (nodes with status "failed" or "skipped")
  - A visual progress bar or percentage indicator

#### Scenario: Per-node structured summary
- **WHEN** a task execution completes
- **THEN** each node result SHALL include:
  - Node name and IP address
  - Execution status (success / failed / skipped)
  - A human-readable summary of the operation performed (e.g., "sysctl: net.ipv4.tcp_keepalive_time=60")
  - Error message if failed

---

### Requirement: Raw Output Access

The system SHALL provide access to raw stdout and stderr for each node's execution.

#### Scenario: Raw output is preserved
- **WHEN** a command is executed on a node
- **THEN** the system SHALL preserve the full stdout and stderr output
- **AND** SHALL make it accessible via the result API

#### Scenario: Raw output is displayed on demand
- **WHEN** the user expands a node result detail
- **THEN** the system SHALL show the raw stdout/stderr in a scrollable text area
- **AND** SHALL preserve whitespace and formatting

---

### Requirement: Execution History and Audit

The system SHALL maintain an audit log of all task executions and auto-rotate to keep only the most recent 100 entries.

#### Scenario: Audit log records executions
- **WHEN** a task is executed
- **THEN** the system SHALL create an audit record containing:
  - Task run ID
  - Task name and type
  - Target node list
  - Triggered by (manual / scheduled)
  - Execution status
  - Started at timestamp
  - Completed at timestamp
  - User who triggered (for manual executions)

#### Scenario: Audit log auto-rotation
- **WHEN** the audit log exceeds 100 entries
- **THEN** the system SHALL delete the oldest entries first
- **AND** SHALL maintain a rolling window of the 100 most recent entries

#### Scenario: Execution history is accessible
- **WHEN** the user views the execution history
- **THEN** the system SHALL list recent task runs in reverse chronological order
- **AND** SHALL allow the user to expand a run to see full results per node

---

## MODIFIED Requirements

（无）

---

## REMOVED Requirements

（无）

---

## RENAMED Requirements

（无）