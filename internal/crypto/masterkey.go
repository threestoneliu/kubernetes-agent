package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func DefaultKeyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kubernetes-agent", "master.key"), nil
}

func LoadMasterKey() ([]byte, error) {
	if v := os.Getenv("KUBERNETES_AGENT_MASTER_KEY"); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("master key env: base64: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("master key env: must be 32 bytes, got %d", len(key))
		}
		return key, nil
	}

	path, err := DefaultKeyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GenerateAndPersist(path)
		}
		return nil, err
	}
	if fi, _ := os.Stat(path); fi != nil && fi.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("master key %s has perm %o, want 0600", path, fi.Mode().Perm())
	}
	if len(data) != 32 {
		return nil, fmt.Errorf("master key %s: must be 32 bytes, got %d", path, len(data))
	}
	return data, nil
}

func GenerateAndPersist(path string) ([]byte, error) {
	// On Windows os.Geteuid returns -1; the root check is no-op there.
	if os.Geteuid() == 0 {
		return nil, errors.New("refusing to generate master key as root")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}
