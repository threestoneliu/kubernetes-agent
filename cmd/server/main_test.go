package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/threestoneliu/kubernetes-agent/internal/config"
)

func TestStartup_Integration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Generate a valid 32-byte master key as base64.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	t.Setenv("KUBERNETES_AGENT_MASTER_KEY", base64.StdEncoding.EncodeToString(key))

	cfgPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "data.db")
	cfgYAML := "server:\n  port: 0\n  host: 127.0.0.1\nstorage:\n  db_path: " + dbPath + "\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0600))
	t.Setenv("KUBERNETES_AGENT_CONFIG", cfgPath)

	cfg, err := config.Load()
	require.NoError(t, err)

	db, aead, err := startup(cfg, t.Context())
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NotNil(t, aead)
	t.Cleanup(func() { _ = db.Close() })

	// Round-trip: encrypt + decrypt a string.
	ct, err := aead.Encrypt([]byte("kubeconfig bytes"))
	require.NoError(t, err)
	pt, err := aead.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, []byte("kubeconfig bytes"), pt)

	// DB is usable.
	clusters, err := db.ListClusters(t.Context())
	require.NoError(t, err)
	assert.Empty(t, clusters)
}
