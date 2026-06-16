package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedDefaultPolicies_Empty(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	require.NoError(t, db.SeedDefaultPolicies(context.Background()))

	ps, err := db.ListEnabledPolicies(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(ps), 4)

	// Check the 4 new default names are present and their YAML is real
	// (non-empty, contains an effect line), not a placeholder.
	want := map[string]bool{
		"deny-delete-system-ns": false,
		"deny-dangerous-kinds":  false,
		"deny-privileged":       false,
		"confirm-production":    false,
	}
	for _, p := range ps {
		if _, ok := want[p.Name]; ok {
			want[p.Name] = true
		}
		assert.NotEmpty(t, p.YAML, "policy %q has empty YAML", p.Name)
		assert.Contains(t, p.YAML, "effect:", "policy %q YAML missing effect", p.Name)
	}
	for name, found := range want {
		assert.True(t, found, "expected default rule %q", name)
	}
}

func TestSeedDefaultPolicies_Idempotent(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())

	require.NoError(t, db.SeedDefaultPolicies(context.Background()))
	first, err := db.ListEnabledPolicies(context.Background())
	require.NoError(t, err)

	require.NoError(t, db.SeedDefaultPolicies(context.Background()))
	second, err := db.ListEnabledPolicies(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(first), len(second), "seeding twice should not duplicate")
}
