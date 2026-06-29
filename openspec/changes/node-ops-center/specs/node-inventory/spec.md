## ADDED Requirements

### Requirement: Node Inventory Management

The system SHALL provide a node inventory that maintains a list of target nodes accessible via SSH, supporting both automatic synchronization from Kubernetes clusters and manual node entry.

#### Scenario: Kubernetes nodes are automatically synchronized
- **WHEN** a cluster is registered and the user enables auto-sync
- **THEN** the system SHALL fetch node list from `kubectl get nodes` and import nodes with hostname, IP address, and labels
- **AND** the imported nodes SHALL have `source: "k8s"` and SHALL NOT be deletable by the user

#### Scenario: Manual node entry
- **WHEN** the user adds a node manually with address, port, and authentication
- **THEN** the system SHALL create a node record with `source: "manual"`
- **AND** the node SHALL be editable and deletable by the user

#### Scenario: Node labels are managed
- **WHEN** a node is displayed or selected
- **THEN** the system SHALL show node labels as key-value pairs
- **AND** the user SHALL be able to filter nodes by label selectors (AND logic across multiple labels)

#### Scenario: SSH authentication is configurable per node
- **WHEN** the user configures SSH authentication for a node
- **THEN** the system SHALL support SSH key (private key content) as the primary method
- **AND** SHALL support password as a fallback
- **AND** both key and password SHALL be stored encrypted using AES-256-GCM

#### Scenario: Node editing and deletion
- **WHEN** the user edits a manually-added node
- **THEN** the system SHALL allow updating address, port, labels, and authentication
- **WHEN** the user deletes a manually-added node
- **THEN** the system SHALL remove the node from inventory
- **WHEN** the user attempts to delete a K8s-synced node
- **THEN** the system SHALL reject the request with an appropriate error

---

### Requirement: Node Connectivity Status

The system SHALL display the connectivity status of each node based on the last SSH connection attempt.

#### Scenario: Online node
- **WHEN** SSH connection to a node succeeds within the timeout period
- **THEN** the node SHALL be marked as "online"

#### Scenario: Offline node
- **WHEN** SSH connection to a node fails or times out
- **THEN** the node SHALL be marked as "offline"
- **AND** the system SHALL NOT block task execution for offline nodes (user chooses whether to skip)

---

## MODIFIED Requirements

（无）

---

## REMOVED Requirements

（无）

---

## RENAMED Requirements

（无）