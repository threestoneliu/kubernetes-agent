package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("server:\n  port: 9000\n"), 0600))
	t.Setenv("KUBERNETES_AGENT_CONFIG", cfgPath)

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9000, c.Server.Port)
	assert.Equal(t, "127.0.0.1", c.Server.Host)
}

func TestLoad_EnvExpansion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("llm:\n  providers:\n    - name: a\n      type: openai\n      apiKey: ${TEST_API_KEY}\n      model: gpt-4o\n"), 0600))
	t.Setenv("KUBERNETES_AGENT_CONFIG", cfgPath)
	t.Setenv("TEST_API_KEY", "sk-test-123")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "sk-test-123", c.LLM.Providers[0].APIKey)
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUBERNETES_AGENT_CONFIG", filepath.Join(dir, "nope.yaml"))
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_StorageDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("server:\n  port: 9000\n"), 0600))
	t.Setenv("KUBERNETES_AGENT_CONFIG", cfgPath)

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "~/.kubernetes-agent/data.db", c.Storage.DBPath)
}
