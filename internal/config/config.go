package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Storage struct {
	DBPath string `yaml:"db_path"`
}

type LLMProvider struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	APIKey  string `yaml:"apiKey"`
	BaseURL string `yaml:"baseURL"`
	Model   string `yaml:"model"`
}

type LLM struct {
	Default   string        `yaml:"default"`
	Providers []LLMProvider `yaml:"providers"`
}

type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type Skills struct {
	Dir    string `yaml:"dir"`
	Enabled bool   `yaml:"enabled"`
}

type Config struct {
	Server  Server  `yaml:"server"`
	Storage Storage `yaml:"storage"`
	LLM     LLM     `yaml:"llm"`
	Logging Logging `yaml:"logging"`
	Skills  Skills  `yaml:"skills"`
}

var envPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

func Load() (*Config, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	if v := os.Getenv("KUBERNETES_AGENT_CONFIG"); v != "" {
		path = v
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	expanded := envPattern.ReplaceAllStringFunc(string(data), func(m string) string {
		name := envPattern.FindStringSubmatch(m)[1]
		return os.Getenv(name)
	})
	var c Config
	if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Storage.DBPath == "" {
		c.Storage.DBPath = "~/.kubernetes-agent/data.db"
	}
	c.Storage.DBPath = expandHome(c.Storage.DBPath)
	if c.Skills.Dir == "" {
		c.Skills.Dir = "~/.kubernetes-agent/skills"
	}
	c.Skills.Dir = expandHome(c.Skills.Dir)
	if !c.Skills.Enabled {
		c.Skills.Enabled = true
	}
	return &c, nil
}

func defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".kubernetes-agent", "config.yaml"), nil
}

// expandHome replaces a leading "~" with the user's home directory.
// A leading "~/" expands to "$HOME/", a bare "~" expands to "$HOME".
// "~user" (other user) is NOT supported — we only handle the current user.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p // leave to caller to surface; we don't error in helper
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p // unhandled ~user form; pass through unchanged
}
