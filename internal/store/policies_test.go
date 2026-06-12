package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedDefaultsIfEmpty_Empty(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	require.NoError(t, db.SeedDefaultsIfEmpty(context.Background()))

	ps, err := db.ListEnabledPolicies(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(ps), 4)
}

func TestSeedDefaultsIfEmpty_Idempotent(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())

	require.NoError(t, db.SeedDefaultsIfEmpty(context.Background()))
	first, err := db.ListEnabledPolicies(context.Background())
	require.NoError(t, err)

	require.NoError(t, db.SeedDefaultsIfEmpty(context.Background()))
	second, err := db.ListEnabledPolicies(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(first), len(second), "seeding twice should not duplicate")
}
