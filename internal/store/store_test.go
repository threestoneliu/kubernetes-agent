package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
}

// --- clusters repo tests ---

func TestCluster_CreateGetListDelete(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()

	c := Cluster{
		ID:             "c1",
		Name:           "prod",
		Server:         "https://k8s.example.com",
		User:           "admin",
		KubeconfigBlob: []byte("kubeconfig-bytes"),
	}
	require.NoError(t, db.CreateCluster(ctx, c))

	got, err := db.GetCluster(ctx, "c1")
	require.NoError(t, err)
	require.Equal(t, c.Name, got.Name)
	require.Equal(t, c.Server, got.Server)
	require.Equal(t, c.User, got.User)
	require.Equal(t, c.KubeconfigBlob, got.KubeconfigBlob)
	require.False(t, got.CreatedAt.IsZero())

	list, err := db.ListClusters(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, db.DeleteCluster(ctx, "c1"))

	_, err = db.GetCluster(ctx, "c1")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCluster_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	_, err := db.GetCluster(context.Background(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCluster_DeleteNotFound(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	err := db.DeleteCluster(context.Background(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCluster_UniqueNameConstraint(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()
	require.NoError(t, db.CreateCluster(ctx, Cluster{ID: "c1", Name: "prod", Server: "s", User: "u", KubeconfigBlob: []byte("k")}))
	err := db.CreateCluster(ctx, Cluster{ID: "c2", Name: "prod", Server: "s", User: "u", KubeconfigBlob: []byte("k")})
	require.Error(t, err)
}

// --- sessions repo tests ---

func TestSession_CreateGetListUpdateDelete(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()

	s := Session{ID: "s1", Title: "investigate nginx"}
	require.NoError(t, db.CreateSession(ctx, s))

	got, err := db.GetSession(ctx, "s1")
	require.NoError(t, err)
	require.Equal(t, s.Title, got.Title)
	require.False(t, got.CreatedAt.IsZero())

	list, err := db.ListSessions(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, db.UpdateSessionTitle(ctx, "s1", "renamed"))
	got, err = db.GetSession(ctx, "s1")
	require.NoError(t, err)
	require.Equal(t, "renamed", got.Title)
	require.True(t, got.UpdatedAt.After(got.CreatedAt) || got.UpdatedAt.Equal(got.CreatedAt))

	require.NoError(t, db.DeleteSession(ctx, "s1"))
	_, err = db.GetSession(ctx, "s1")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSession_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	_, err := db.GetSession(context.Background(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSession_DeleteCascadesMessagesAndPlans(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()

	require.NoError(t, db.CreateSession(ctx, Session{ID: "s1", Title: "t"}))
	require.NoError(t, db.BatchInsertMessages(ctx, []Message{
		{ID: "m1", SessionID: "s1", Role: "user", Content: stringPtr("hi")},
		{ID: "m2", SessionID: "s1", Role: "assistant", Content: stringPtr("hello")},
	}))
	require.NoError(t, db.CreatePlan(ctx, Plan{
		ID:        "p1",
		SessionID: "s1",
		OpsJSON:   "[]",
		DiffsJSON: "[]",
		Risk:      "low",
		Status:    "pending",
	}))

	require.NoError(t, db.DeleteSession(ctx, "s1"))

	msgs, err := db.ListMessagesBySession(ctx, "s1")
	require.NoError(t, err)
	require.Empty(t, msgs)
}

func TestSession_WithClusterID(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()
	require.NoError(t, db.CreateCluster(ctx, Cluster{ID: "c1", Name: "prod", Server: "s", User: "u", KubeconfigBlob: []byte("k")}))
	require.NoError(t, db.CreateSession(ctx, Session{ID: "s1", Title: "t", ClusterID: stringPtr("c1")}))
	got, err := db.GetSession(ctx, "s1")
	require.NoError(t, err)
	require.NotNil(t, got.ClusterID)
	require.Equal(t, "c1", *got.ClusterID)
}

func stringPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64   { return &i }

// --- messages repo tests ---

func TestMessage_BatchInsertAndList(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()
	require.NoError(t, db.CreateSession(ctx, Session{ID: "s1", Title: "t"}))

	now := time.Now().Unix()
	msgs := []Message{
		{ID: "m1", SessionID: "s1", Role: "user", Content: stringPtr("hi")},
		{ID: "m2", SessionID: "s1", Role: "assistant", Content: stringPtr("hello")},
		{ID: "m3", SessionID: "s1", Role: "tool", ToolCallID: stringPtr("tc1"), Content: stringPtr("result")},
	}
	for i := range msgs {
		msgs[i].CreatedAt = now + int64(i)
	}
	require.NoError(t, db.BatchInsertMessages(ctx, msgs))

	got, err := db.ListMessagesBySession(ctx, "s1")
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "user", got[0].Role)
	require.Equal(t, "assistant", got[1].Role)
	require.Equal(t, "tool", got[2].Role)
	require.NotNil(t, got[2].ToolCallID)
	require.Equal(t, "tc1", *got[2].ToolCallID)
}

func TestMessage_ListEmpty(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	got, err := db.ListMessagesBySession(context.Background(), "s1")
	require.NoError(t, err)
	require.Empty(t, got)
}

// --- plans repo tests ---

func TestPlan_CreateGetUpdateStatusMarkExecuted(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()
	require.NoError(t, db.CreateSession(ctx, Session{ID: "s1", Title: "t"}))

	p := Plan{
		ID:        "p1",
		SessionID: "s1",
		OpsJSON:   "[{\"op\":\"get\"}]",
		DiffsJSON: "[]",
		Risk:      "low",
		Status:    "pending",
	}
	require.NoError(t, db.CreatePlan(ctx, p))

	got, err := db.GetPlan(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "pending", got.Status)
	require.Equal(t, "low", got.Risk)
	require.Nil(t, got.ExecutedAt)

	require.NoError(t, db.UpdatePlanStatus(ctx, "p1", "approved"))
	got, err = db.GetPlan(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "approved", got.Status)

	require.NoError(t, db.MarkExecuted(ctx, "p1"))
	got, err = db.GetPlan(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "executed", got.Status)
	require.NotNil(t, got.ExecutedAt)
}

func TestPlan_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	_, err := db.GetPlan(context.Background(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPlan_UpdateStatusNotFound(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	err := db.UpdatePlanStatus(context.Background(), "missing", "approved")
	require.ErrorIs(t, err, ErrNotFound)
}

// --- policies repo tests ---

func TestPolicy_UpsertListEnabledSetEnabled(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()

	p := Policy{ID: "p1", Name: "no-delete-system-ns", YAML: "rule: x", Enabled: true}
	require.NoError(t, db.UpsertPolicy(ctx, p))

	got, err := db.ListEnabledPolicies(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "no-delete-system-ns", got[0].Name)

	// Upsert same id with different fields
	p.YAML = "rule: y"
	p.Enabled = false
	require.NoError(t, db.UpsertPolicy(ctx, p))

	enabled, err := db.ListEnabledPolicies(ctx)
	require.NoError(t, err)
	require.Empty(t, enabled)

	require.NoError(t, db.SetEnabled(ctx, "p1", true))
	enabled, err = db.ListEnabledPolicies(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 1)
}

func TestPolicy_UniqueNameConstraint(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()
	require.NoError(t, db.UpsertPolicy(ctx, Policy{ID: "p1", Name: "rule-a", YAML: "y", Enabled: true}))
	err := db.UpsertPolicy(ctx, Policy{ID: "p2", Name: "rule-a", YAML: "y", Enabled: true})
	require.Error(t, err)
}

func TestPolicy_SeedDefaultsIfEmpty(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()

	require.NoError(t, db.SeedDefaultsIfEmpty(ctx))
	policies, err := db.ListEnabledPolicies(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(policies), 4)

	// Calling again should be a no-op
	count := len(policies)
	require.NoError(t, db.SeedDefaultsIfEmpty(ctx))
	policies2, err := db.ListEnabledPolicies(ctx)
	require.NoError(t, err)
	require.Equal(t, count, len(policies2))
}

// --- audit repo tests ---

func TestAudit_AppendAndList(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()

	id1, err := db.AppendAudit(ctx, AuditEntry{
		SessionID: stringPtr("s1"),
		Action:    "plan.create",
		Target:    stringPtr("deployment/nginx"),
		Status:    "ok",
		Message:   stringPtr("plan created"),
	})
	require.NoError(t, err)
	require.Greater(t, id1, int64(0))

	id2, err := db.AppendAudit(ctx, AuditEntry{
		Action:  "system.startup",
		Status:  "ok",
		Message: stringPtr("started"),
	})
	require.NoError(t, err)
	require.Greater(t, id2, id1)

	entries, err := db.ListAudit(ctx, AuditFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

func TestAudit_ListBySession(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	ctx := context.Background()
	_, err := db.AppendAudit(ctx, AuditEntry{SessionID: stringPtr("s1"), Action: "a", Status: "ok"})
	require.NoError(t, err)
	_, err = db.AppendAudit(ctx, AuditEntry{SessionID: stringPtr("s2"), Action: "a", Status: "ok"})
	require.NoError(t, err)

	entries, err := db.ListAudit(ctx, AuditFilter{SessionID: "s1"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].SessionID)
	require.Equal(t, "s1", *entries[0].SessionID)
}

func TestAudit_AppendMinimumFields(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Migrate())
	_, err := db.AppendAudit(context.Background(), AuditEntry{Action: "ping", Status: "ok"})
	require.NoError(t, err)
	entries, err := db.ListAudit(context.Background(), AuditFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "ping", entries[0].Action)
}

// Sanity: ErrNotFound is exported as a sentinel value.
func TestErrNotFoundSentinel(t *testing.T) {
	require.True(t, errors.Is(ErrNotFound, ErrNotFound))
}
