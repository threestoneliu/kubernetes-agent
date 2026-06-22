# k8s-cluster-inspect Skill

## Purpose

**TBD**

## Requirements

### Requirement: Cluster Inspection Workflow

The k8s-cluster-inspect Skill SHALL provide a structured workflow for performing comprehensive cluster inspections and generating overview reports.

#### Scenario: User requests cluster inspection
- **WHEN** User says "巡检集群" or "cluster overview" or similar phrasing
- **THEN** The LLM SHALL load the k8s-cluster-inspect Skill and follow its workflow

---

### Requirement: Inspection Coverage

The cluster inspection workflow SHALL cover key cluster components including nodes, namespaces, running workloads, and potential issues.

#### Scenario: Node inspection
- **WHEN** Cluster inspection begins
- **THEN** The LLM SHALL list and describe cluster nodes to check resource availability

#### Scenario: Workload overview
- **WHEN** Node inspection completes
- **THEN** The LLM SHALL list deployments and pods across namespaces to assess workload health

#### Scenario: Issue detection
- **WHEN** Workload overview is gathered
- **THEN** The LLM SHALL identify potential issues such as failed pods, pending resources, or configuration problems

---

### Requirement: Inspection Report Output

The cluster inspection workflow SHALL generate a comprehensive report with sections for each inspected component.

#### Scenario: Inspection report format
- **WHEN** Cluster inspection completes
- **THEN** The LLM SHALL present findings in the format:
  ```
  ## 集群巡检报告: <cluster-id>
  ### 节点状态
  ### 工作负载概览
  ### 发现的问题
  ### 建议
  ```
