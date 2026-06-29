## ADDED Requirements

### Requirement: Structured Operation Tasks

The system SHALL define structured operation tasks with typed parameters for sysctl, file_write, service_restart, shell, and reboot operations.

#### Scenario: Sysctl operation
- **WHEN** a task of type `sysctl` is created
- **THEN** it SHALL accept a list of key-value pairs `{key: string, value: string}`
- **AND** each pair SHALL be validated as non-empty key and non-empty value

#### Scenario: File write operation
- **WHEN** a task of type `file_write` is created
- **THEN** it SHALL require `file_path` (non-empty string) and `file_content` (string)
- **AND** `file_path` SHALL be validated as an absolute path

#### Scenario: Service restart operation
- **WHEN** a task of type `service_restart` is created
- **THEN** it SHALL require `service_name` (non-empty string) and `action` (one of: restart, start, stop, status)

#### Scenario: Shell operation
- **WHEN** a task of type `shell` is created
- **THEN** it SHALL require `command` (non-empty string)

#### Scenario: Reboot operation
- **WHEN** a task of type `reboot` is created
- **THEN** it SHALL accept an optional `delay` in seconds (default 10)
- **AND** `delay` SHALL be a non-negative integer

---

### Requirement: Node Targeting

The system SHALL allow targeting nodes via label selectors with AND logic, supplemented by manual node ID overrides.

#### Scenario: Label selector targeting
- **WHEN** a task specifies label filters
- **THEN** the system SHALL select all nodes matching ALL specified label filters (AND logic)
- **AND** each label filter SHALL support multiple values with OR semantics (env ∈ {production, staging})

#### Scenario: Manual node override
- **WHEN** a task specifies manual node IDs
- **THEN** the system SHALL include those nodes in addition to label-matched nodes
- **AND** SHALL deduplicate if the same node appears via both methods

#### Scenario: Empty target resolution
- **WHEN** a task has no label filters AND no manual node IDs
- **THEN** the system SHALL return a validation error before execution

---

### Requirement: Execution Mode

The system SHALL support parallel and sequential execution modes.

#### Scenario: Parallel execution
- **WHEN** a task has `exec_mode: "parallel"`
- **THEN** the system SHALL execute the operation on all target nodes concurrently
- **AND** SHALL complete when all nodes have responded or timed out

#### Scenario: Sequential execution
- **WHEN** a task has `exec_mode: "sequential"`
- **THEN** the system SHALL execute the operation on each node in order
- **AND** SHALL stop on the first node that fails if `stop_on_error` is true

#### Scenario: Execution result aggregation
- **WHEN** execution completes (all nodes for parallel, or stopped for sequential)
- **THEN** the system SHALL aggregate results per node
- **AND** SHALL produce a summary with success/failure counts

---

## MODIFIED Requirements

（无）

---

## REMOVED Requirements

（无）

---

## RENAMED Requirements

（无）