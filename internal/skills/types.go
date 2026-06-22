package skills

// Skill represents a loaded skill.
type Skill struct {
	Name        string
	Description string
	FilePath    string // absolute path to SKILL.md
	BaseDir     string
	Source      string // "project" or "user"
	AlwaysInject bool
	Priority    int
	content     string // SKILL.md raw content
}

// SkillEntry is a skill with its parsed metadata.
type SkillEntry struct {
	Skill       Skill
	Frontmatter map[string]string
	Metadata    *SkillMetadata
}

// SkillMetadata contains optional skill metadata.
type SkillMetadata struct {
	Emoji    string
	Homepage string
	OS       []string
	Requires *Requires
}

// Requires lists skill dependencies.
type Requires struct {
	Bins   []string
	Env    []string
	Config []string
}
