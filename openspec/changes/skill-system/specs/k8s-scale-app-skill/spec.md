## ADDED Requirements

### Requirement: Application Scaling Workflow

The k8s-scale-app Skill SHALL provide a structured workflow for scaling Kubernetes applications up or down.

#### Scenario: User requests scaling
- **WHEN** User says "扩容到 5 个副本" or similar phrasing
- **THEN** The LLM SHALL load the k8s-scale-app Skill and follow its workflow

---

### Requirement: Scaling Workflow Phases

The scaling workflow SHALL consist of: Get Current State, Confirm Target, Execute, and Verify.

#### Scenario: Phase 1 - Get Current State
- **WHEN** The scaling workflow begins
- **THEN** The LLM SHALL get the current replica count using k8s_get

#### Scenario: Phase 2 - Confirm Target
- **WHEN** Current state is obtained
- **THEN** The LLM SHALL confirm the target replica count with the user

#### Scenario: Phase 3 - Execute
- **WHEN** Target is confirmed
- **THEN** The LLM SHALL use k8s_plan_write and k8s_execute_plan to scale

#### Scenario: Phase 4 - Verify
- **WHEN** Scaling is executed
- **THEN** The LLM SHALL verify the new replica count matches the target
