## ADDED Requirements

### Requirement: Application Deployment Workflow

The k8s-deploy-app Skill SHALL provide a structured workflow for deploying applications to Kubernetes. The workflow SHALL ensure proper validation, preview, and verification of deployment operations.

#### Scenario: User requests deployment
- **WHEN** User says "帮我部署 nginx" or similar phrasing
- **THEN** The LLM SHALL load the k8s-deploy-app Skill and follow its workflow

---

### Requirement: Deployment Workflow Phases

The deployment workflow SHALL consist of at least four phases: Validate Manifest, Plan Preview, Execute, and Verify.

#### Scenario: Phase 1 - Validate Manifest
- **WHEN** The deployment workflow begins
- **THEN** The LLM SHALL validate the provided YAML or resource definition

#### Scenario: Phase 2 - Plan Preview
- **WHEN** Manifest is validated
- **THEN** The LLM SHALL use k8s_plan_write to generate a preview showing what will be created/updated

#### Scenario: Phase 3 - Execute
- **WHEN** User confirms the plan
- **THEN** The LLM SHALL execute the plan using k8s_execute_plan

#### Scenario: Phase 4 - Verify
- **WHEN** Plan is executed
- **THEN** The LLM SHALL verify rollout status and confirm pods are running

---

### Requirement: Rollback on Failure

If deployment fails, the workflow SHALL attempt to rollback executed operations.

#### Scenario: Deployment fails mid-execution
- **WHEN** k8s_execute_plan returns an error
- **THEN** The system SHALL attempt to rollback previously executed operations
