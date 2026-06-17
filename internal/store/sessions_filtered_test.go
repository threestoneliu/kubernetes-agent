package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSessionsTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate())
	return db
}

func seedSessions(t *testing.T, db *DB, sessions []Session) {
	t.Helper()
	ctx := context.Background()
	for _, s := range sessions {
		require.NoError(t, db.CreateSession(ctx, s))
	}
}

func TestListSessionsFiltered_OrderByTitleAsc(t *testing.T) {
	db := newSessionsTestDB(t)
	seedSessions(t, db, []Session{
		{ID: "a", Title: "Charlie"},
		{ID: "b", Title: "alpha"},
		{ID: "c", Title: "Bravo"},
	})
	got, err := db.ListSessionsFiltered(context.Background(), "", "title", "asc", 10, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "b", got[0].ID)
	assert.Equal(t, "c", got[1].ID)
	assert.Equal(t, "a", got[2].ID)
}

func TestListSessionsFiltered_SearchCaseInsensitive(t *testing.T) {
	db := newSessionsTestDB(t)
	seedSessions(t, db, []Session{
		{ID: "a", Title: "DemoConfig"},
		{ID: "b", Title: "demo-2"},
		{ID: "c", Title: "unrelated"},
	})
	got, err := db.ListSessionsFiltered(context.Background(), "demo", "title", "asc", 10, 0)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListSessionsFiltered_LimitOffset(t *testing.T) {
	db := newSessionsTestDB(t)
	seedSessions(t, db, []Session{
		{ID: "a", Title: "a-1"},
		{ID: "b", Title: "a-2"},
		{ID: "c", Title: "a-3"},
		{ID: "d", Title: "a-4"},
	})
	got, err := db.ListSessionsFiltered(context.Background(), "", "title", "asc", 2, 1)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "b", got[0].ID)
	assert.Equal(t, "c", got[1].ID)
}

func TestListSessionsFiltered_InvalidSort(t *testing.T) {
	db := newSessionsTestDB(t)
	_, err := db.ListSessionsFiltered(context.Background(), "", "password", "desc", 10, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sort")
}

func TestListSessionsFiltered_InvalidOrder(t *testing.T) {
	db := newSessionsTestDB(t)
	_, err := db.ListSessionsFiltered(context.Background(), "", "title", "sideways", 10, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order")
}

func TestDeleteSession_ReturnsCount(t *testing.T) {
	db := newSessionsTestDB(t)
	seedSessions(t, db, []Session{{ID: "a", Title: "a"}})
	n, err := db.DeleteSession(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	_, err = db.DeleteSession(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDeleteAllSessions_Empty(t *testing.T) {
	db := newSessionsTestDB(t)
	n, err := db.DeleteAllSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestDeleteAllSessions_Populated(t *testing.T) {
	db := newSessionsTestDB(t)
	seedSessions(t, db, []Session{
		{ID: "a", Title: "a"},
		{ID: "b", Title: "b"},
		{ID: "c", Title: "c"},
	})
	n, err := db.DeleteAllSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	rows, _ := db.ListSessions(context.Background())
	assert.Empty(t, rows)
}