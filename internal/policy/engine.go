package policy

import (
	"encoding/json"
	"strings"
)

type Engine struct {
	Rules []Rule
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
