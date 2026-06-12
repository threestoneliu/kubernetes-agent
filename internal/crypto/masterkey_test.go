package crypto

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMasterKey_FromEnv(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	envVal := base64.StdEncoding.EncodeToString(raw)
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", envVal)

	key, err := LoadMasterKey()
	require.NoError(t, err)
	assert.Len(t, key, 32)
}

func TestLoadMasterKey_FromEnv_WrongLength(t *testing.T) {
	short := make([]byte, 16)
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", base64.StdEncoding.EncodeToString(short))

	_, err := LoadMasterKey()
	assert.Error(t, err)
}

func TestLoadMasterKey_FromEnv_NotBase64(t *testing.T) {
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", "not base64!!!")

	_, err := LoadMasterKey()
	assert.Error(t, err)
}

func TestLoadMasterKey_Generate(t *testing.T) {
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", "")
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	path, err := DefaultKeyPath()
	require.NoError(t, err)

	key, err := LoadMasterKey()
	require.NoError(t, err)
	assert.Len(t, key, 32)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadMasterKey_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", "")

	// First call generates
	first, err := LoadMasterKey()
	require.NoError(t, err)

	// Second call reads existing
	second, err := LoadMasterKey()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}
