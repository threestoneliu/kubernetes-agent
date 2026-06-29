## ADDED Requirements

### Requirement: SSH Connection Pool

The system SHALL manage an SSH connection pool that reuses connections per node to minimize connection overhead.

#### Scenario: Connection reuse
- **WHEN** multiple tasks are executed targeting the same node within the pool lifetime
- **THEN** the system SHALL reuse the existing SSH connection
- **AND** the connection SHALL be reused within a configurable timeout (default 30 seconds)

#### Scenario: Connection timeout handling
- **WHEN** an SSH connection attempt exceeds the timeout (default 30 seconds)
- **THEN** the system SHALL mark the node as "offline"
- **AND** SHALL return an appropriate error to the caller

#### Scenario: Connection concurrency control
- **WHEN** the number of concurrent SSH connections exceeds the configured limit (default 10)
- **THEN** the system SHALL queue additional connection attempts
- **AND** SHALL execute them in order as slots become available

---

### Requirement: SSH Authentication

The system SHALL support SSH key-based authentication with password fallback.

#### Scenario: SSH key authentication
- **WHEN** a node is configured with SSH private key
- **THEN** the system SHALL authenticate using the private key
- **AND** SHALL support RSA, ECDSA, and Ed25519 key types

#### Scenario: SSH password fallback
- **WHEN** SSH key authentication fails or no key is configured
- **THEN** the system SHALL attempt password authentication if password is configured

#### Scenario: Encrypted credential storage
- **WHEN** SSH credentials (key or password) are stored
- **THEN** the system SHALL encrypt them using AES-256-GCM before storage
- **AND** SHALL decrypt them only at the moment of SSH connection

---

### Requirement: Command Execution

The system SHALL execute commands over SSH and capture stdout and stderr separately.

#### Scenario: Command execution returns structured output
- **WHEN** a command is executed on a node via SSH
- **THEN** the system SHALL return a structured result containing:
  - Exit code
  - Stdout content
  - Stderr content
  - Execution duration

#### Scenario: Command timeout
- **WHEN** a command execution exceeds the configured timeout
- **THEN** the system SHALL terminate the SSH session
- **AND** SHALL return an error with `reason: "timeout"`

#### Scenario: Command mapping by operation type
- **WHEN** executing operation type `sysctl`
- **THEN** the system SHALL run `sysctl -w key=value` for each entry
- **WHEN** executing operation type `service_restart`
- **THEN** the system SHALL run `systemctl <action> <service_name>`
- **WHEN** executing operation type `file_write`
- **THEN** the system SHALL write content to the target path using `cat > path << EOF`
- **WHEN** executing operation type `shell`
- **THEN** the system SHALL execute the command exactly as provided
- **WHEN** executing operation type `reboot`
- **THEN** the system SHALL run `shutdown -r +<delay>` or `reboot`

---

## MODIFIED Requirements

（无）

---

## REMOVED Requirements

（无）

---

## RENAMED Requirements

（无）