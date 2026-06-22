# k8s-check-health Skill

## Purpose

**TBD**

## Requirements

### Requirement: Health Check Workflow

The k8s-check-health Skill SHALL provide a structured workflow for checking the health status of Kubernetes resources.

#### Scenario: User requests health check
- **WHEN** User says "检查 pod 健康状态" or similar phrasing
- **THEN** The LLM SHALL load the k8s-check-health Skill and follow its workflow

---

### Requirement: Health Check Coverage

The health check workflow SHALL verify multiple aspects of resource health including running status, readiness, and conditions.

#### Scenario: Check pod health
- **WHEN** User requests health check for a pod
- **THEN** The LLM SHALL use k8s_describe to check status conditions, container states, and diagnosis hints

#### Scenario: Check deployment health
- **WHEN** User requests health check for a deployment
- **THEN** The LLM SHALL check replica status and pod health for the deployment's pods

---

### Requirement: Health Report Output

The health check workflow SHALL present findings in a structured format with clear status indicators.

#### Scenario: Health report format
- **WHEN** Health check completes
- **THEN** The LLM SHALL present findings with clear status indicators:
  ```
  ## 健康报告: <resource-name>
  ### 状态: Healthy/Unhealthy
  ### 详情
  ### 建议
  ```
