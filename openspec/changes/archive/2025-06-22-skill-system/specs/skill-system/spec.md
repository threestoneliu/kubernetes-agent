## ADDED Requirements

### Requirement: Skill Directory Structure

The Skill system SHALL support a directory-based Skill storage at `~/.kubernetes-agent/skills/`. Each Skill SHALL be stored in its own subdirectory containing at minimum a `SKILL.md` file. Optional files include `REFERENCE.md`, `EXAMPLES.md`, and a `scripts/` directory.

#### Scenario: Skill directory with minimum required files
- **WHEN** A Skill named `k8s-debug-pod` exists at `~/.kubernetes-agent/skills/k8s-debug-pod/` with only `SKILL.md`
- **THEN** The Skill loader SHALL successfully load the Skill and register it for use

#### Scenario: Skill directory with optional files
- **WHEN** A Skill exists with `SKILL.md`, `REFERENCE.md`, `EXAMPLES.md`, and `scripts/helper.sh`
- **THEN** The Skill loader SHALL load the Skill and ignore the optional files during core loading

---

### Requirement: SKILL.md Format

Each Skill SHALL contain a `SKILL.md` file with YAML frontmatter containing `name` and `description` fields. The description SHALL describe when the Skill should be used, in the format "Use when [trigger condition]."

#### Scenario: Valid SKILL.md with frontmatter
- **WHEN** A `SKILL.md` contains:
  ```yaml
  ---
  name: k8s-debug-pod
  description: Debug Kubernetes pod issues. Use when user wants to debug a pod.
  ---
  ```
- **THEN** The Skill loader SHALL parse the frontmatter and register the Skill with the correct name and description

#### Scenario: SKILL.md without frontmatter
- **WHEN** A `SKILL.md` does not contain YAML frontmatter
- **THEN** The Skill loader SHALL reject the Skill and log an error

---

### Requirement: Available Skills in System Prompt

The Skill system SHALL inject an `<available_skills>` XML section into the system prompt. This section SHALL contain one `<skill>` entry per registered Skill, with `<name>`, `<description>`, and `<location>` child elements.

#### Scenario: System prompt contains all registered Skills
- **WHEN** Three Skills are registered: `k8s-debug-pod`, `k8s-deploy-app`, `k8s-scale-app`
- **THEN** The system prompt SHALL contain an `<available_skills>` section with three `<skill>` entries

#### Scenario: Empty Skill registry
- **WHEN** No Skills are registered
- **THEN** The system prompt SHALL contain an empty `<available_skills>` section

---

### Requirement: Skill Loading at Startup

The Skill system SHALL load all Skills from the configured directory when the kubernetes-agent server starts. The loading SHALL be fail-safe: if a single Skill fails to load, the system SHALL log a warning and continue loading other Skills.

#### Scenario: Successful startup with multiple Skills
- **WHEN** The server starts with 5 Skills in `~/.kubernetes-agent/skills/`
- **THEN** All 5 Skills SHALL be loaded and registered without errors

#### Scenario: One Skill fails to load
- **WHEN** 4 Skills are valid and 1 Skill has malformed `SKILL.md`
- **THEN** The 4 valid Skills SHALL be loaded, and a warning SHALL be logged for the failed Skill

---

### Requirement: Skill Location Resolution

Skill `<location>` in the system prompt SHALL be the absolute path to the Skill's `SKILL.md` file.

#### Scenario: Location path format
- **WHEN** A Skill named `k8s-debug-pod` exists at `~/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md`
- **THEN** The `<location>` element SHALL contain `/home/<user>/.kubernetes-agent/skills/k8s-debug-pod/SKILL.md`
