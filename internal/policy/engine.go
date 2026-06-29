package policy

import (
	"context"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

type Engine struct {
	Rules []Rule
}

// ReloadPolicies re-reads all enabled policies from the store and replaces
// e.Rules so that policy edits take effect at runtime without a
// server restart.
func (e *Engine) ReloadPolicies(ctx context.Context, store Store) error {
	rows, err := store.ListEnabledPoliciesForEngine(ctx)
	if err != nil {
		return err
	}
	rules := make([]Rule, 0, len(rows))
	for _, row := range rows {
		yamlStr, _ := row["yaml"].(string)
		nameStr, _ := row["name"].(string)
		if yamlStr == "" {
			continue
		}
		var rule Rule
		if err := yaml.Unmarshal([]byte(yamlStr), &rule); err != nil {
			return err
		}
		if nameStr != "" {
			rule.Name = nameStr
		}
		rules = append(rules, rule)
	}
	e.Rules = rules
	return nil
}

// Store is implemented by *store.DB. It is satisfied via the
// ListEnabledPoliciesForEngine method which returns raw map rows
// so the policy package never imports the store package.
type Store interface {
	ListEnabledPoliciesForEngine(context.Context) ([]map[string]any, error)
}

func (e *Engine) Evaluate(op OperationInfo) Effect {
	for _, r := range e.Rules {
		if r.Match.matches(op) {
			return r.Effect
		}
	}
	if isWrite(op.Action()) {
		return Confirm
	}
	return Allow
}

func (m Match) matches(op OperationInfo) bool {
	if len(m.Action) > 0 && !contains(m.Action, op.Action()) {
		return false
	}
	if len(m.Namespace) > 0 && !contains(m.Namespace, op.Namespace()) {
		return false
	}
	if len(m.Kind) > 0 {
		k := canonicalKind(op.Resource())
		if !contains(m.Kind, k) {
			return false
		}
	}
	if len(m.UnsafeFields) > 0 {
		if op.Manifest() == nil {
			return false
		}
		// Any single unsafe field matching is enough to consider the rule
		// applicable. We do NOT require every entry to be present in the
		// manifest.
		matched := false
		for path, want := range m.UnsafeFields {
			got, ok := JSONPathGet(op.Manifest(), path)
			if !ok {
				continue
			}
			if deepEqual(got, want) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func canonicalKind(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func isWrite(a string) bool { return a == "apply" || a == "delete" || a == "scale" }

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

func deepEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
