package policy

import (
	"context"
	"testing"
)

// mockReloadStore implements the Store interface for testing Reload.
type mockReloadStore struct {
	rows []map[string]any
}

func (m *mockReloadStore) ListEnabledPoliciesForEngine(ctx context.Context) ([]map[string]any, error) {
	return m.rows, nil
}

func TestEngineReload(t *testing.T) {
	ctx := context.Background()
	store := &mockReloadStore{
		rows: []map[string]any{
			{
				"name": "test-rule-1",
				"yaml": "name: test-rule-1\neffect: deny\nmatch:\n    action:\n        - apply\n",
			},
			{
				"name": "test-rule-2",
				"yaml": "name: test-rule-2\neffect: confirm\nmatch:\n    action:\n        - delete\n",
			},
		},
	}

	engine := &Engine{Rules: DefaultRules()}
	initialCount := len(engine.Rules)
	t.Logf("Initial rules count: %d", initialCount)

	if err := engine.ReloadPolicies(ctx, store); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if len(engine.Rules) != 2 {
		t.Errorf("Expected 2 rules after reload, got %d", len(engine.Rules))
	}

	names := make(map[string]bool)
	for _, r := range engine.Rules {
		names[r.Name] = true
		t.Logf("  Rule: %s, effect: %s", r.Name, r.Effect)
	}

	if !names["test-rule-1"] {
		t.Error("test-rule-1 not found after reload")
	}
	if !names["test-rule-2"] {
		t.Error("test-rule-2 not found after reload")
	}

	for _, r := range engine.Rules {
		if r.Name == "test-rule-1" && r.Effect != Deny {
			t.Errorf("test-rule-1 should have effect 'deny', got '%s'", r.Effect)
		}
		if r.Name == "test-rule-2" && r.Effect != Confirm {
			t.Errorf("test-rule-2 should have effect 'confirm', got '%s'", r.Effect)
		}
	}

	t.Logf("Rules count changed from %d to %d", initialCount, len(engine.Rules))
}
