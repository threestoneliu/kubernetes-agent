# k8s-debug-pod Skill

## Purpose

**TBD**

## Requirements

### Requirement: Pod Debug Workflow

The k8s-debug-pod Skill SHALL provide a structured workflow for debugging Kubernetes Pod issues. The workflow SHALL guide the LLM through systematic diagnosis of common Pod problems including CrashLoopBackOff, ImagePullBackOff, OOMKilled, and Pending states.

#### Scenario: User requests pod debug
- **WHEN** User says "帮我 debug nginx-pod" or similar phrasing
- **THEN** The LLM SHALL load the k8s-debug-pod Skill and follow its workflow

---

### Requirement: Debug Workflow Phases

The debug workflow SHALL consist of at least three phases: Gather Information, Analyze, and Take Action.

#### Scenario: Phase 1 - Gather Information
- **WHEN** The debug workflow begins
- **THEN** The LLM SHALL first describe the pod using k8s_describe to get diagnosis_hints and events

#### Scenario: Phase 2 - Analyze
- **WHEN** Pod information is gathered
- **THEN** The LLM SHALL analyze diagnosis_hints and events to identify the root cause

#### Scenario: Phase 3 - Take Action
- **WHEN** Root cause is identified
- **THEN** The LLM SHALL recommend and optionally execute fixes based on the analysis

---

### Requirement: Diagnosis Hints Integration

The workflow SHALL utilize the diagnosis_hints returned by k8s_describe to automatically identify common pod issues.

#### Scenario: CrashLoopBackOff detection
- **WHEN** diagnosis_hints contains "容器反复崩溃"
- **THEN** The LLM SHALL suggest checking logs and verifying startup configuration

#### Scenario: ImagePullBackOff detection
- **WHEN** diagnosis_hints contains "镜像拉取失败"
- **THEN** The LLM SHALL suggest checking image name and imagePullSecrets

---

### Requirement: Diagnosis Report Output

The debug workflow SHALL conclude with a structured diagnosis report containing status, identified issues, and recommended actions.

#### Scenario: Diagnosis report format
- **WHEN** Debug workflow completes
- **THEN** The LLM SHALL present findings in the format:
  ```
  ## 诊断报告: <pod-name>
  ### 状态
  ### 发现的问题
  ### 建议操作
  ```
