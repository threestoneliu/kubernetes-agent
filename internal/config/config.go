package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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

type Config struct {
	Server  Server  `yaml:"server"`
	Storage Storage `yaml:"storage"`
	LLM     LLM     `yaml:"llm"`
	Logging Logging `yaml:"logging"`
}

var envPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

func Load() (*Config, error) {
	path := defaultPath()
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
	return &c, nil
}

func defaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kubernetes-agent", "config.yaml")
}
