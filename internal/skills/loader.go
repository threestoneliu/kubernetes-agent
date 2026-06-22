package skills

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader loads skills from directories.
type Loader struct {
	dirs []string
}

// NewLoader creates a new skill loader.
func NewLoader(dirs ...string) *Loader {
	return &Loader{dirs: dirs}
}

// LoadAll loads all skills from configured directories.
func (l *Loader) LoadAll() ([]*SkillEntry, error) {
	var entries []*SkillEntry
	for _, dir := range l.dirs {
		dir = expandHome(dir)
		loaded, err := l.loadFromDir(dir)
		if err != nil {
			continue // fail-safe: log and continue
		}
		entries = append(entries, loaded...)
	}
	return entries, nil
}

func (l *Loader) loadFromDir(dir string) ([]*SkillEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var result []*SkillEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			continue
		}

		skillEntry, err := l.loadSkillFile(skillPath, dir)
		if err != nil {
			continue // fail-safe
		}
		result = append(result, skillEntry)
	}
	return result, nil
}

func (l *Loader) loadSkillFile(skillPath, baseDir string) (*SkillEntry, error) {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}

	frontmatter, body, err := parseFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	// Determine source based on path
	source := "user"
	if strings.Contains(baseDir, "/.claude/") {
		source = "project"
	}

	entry := &SkillEntry{
		Frontmatter: frontmatter,
		Skill: Skill{
			Name:        frontmatter["name"],
			Description: frontmatter["description"],
			FilePath:    skillPath,
			BaseDir:     baseDir,
			Source:      source,
			content:     body,
		},
	}

	return entry, nil
}

// parseFrontmatter extracts YAML frontmatter from markdown content.
func parseFrontmatter(content string) (map[string]string, string, error) {
	if !strings.HasPrefix(content, "---") {
		return nil, content, nil
	}

	lines := strings.Split(content, "\n")
	var frontmatterLines []string
	var bodyLines []string
	inFrontmatter := false

	for i, line := range lines {
		if i == 0 && strings.HasPrefix(line, "---") {
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.HasPrefix(line, "---") {
			bodyLines = lines[i+1:]
			break
		}
		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	var frontmatter map[string]string
	if len(frontmatterLines) > 0 {
		if err := yaml.Unmarshal([]byte(strings.Join(frontmatterLines, "\n")), &frontmatter); err != nil {
			return nil, "", err
		}
	}

	return frontmatter, strings.Join(bodyLines, "\n"), nil
}

// expandHome expands ~ to user's home directory.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
