package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testOp struct {
	action    string
	resource  string
	namespace string
	kind      string
	manifest  map[string]any
}

func (t testOp) Action() string         { return t.action }
func (t testOp) Resource() string       { return t.resource }
func (t testOp) Namespace() string      { return t.namespace }
func (t testOp) Kind() string           { return t.kind }
func (t testOp) Manifest() map[string]any { return t.manifest }

func TestEngine_NoMatch_Read(t *testing.T) {
	e := &Engine{}
	op := testOp{action: "get", resource: "pod"}
	assert.Equal(t, Allow, e.Evaluate(op))
}

func TestEngine_NoMatch_Write(t *testing.T) {
	e := &Engine{}
	op := testOp{action: "apply", resource: "pod"}
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestEngine_FirstMatchWins(t *testing.T) {
	e := &Engine{Rules: []Rule{
		{Name: "a", Effect: Deny, Match: Match{Action: []string{"delete"}}},
		{Name: "b", Effect: Allow, Match: Match{Action: []string{"delete"}}},
	}}
	op := testOp{action: "delete", resource: "pod"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_KindBlacklist(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "node"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_UnsafeField_Privileged(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	manifest := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"securityContext": map[string]any{"privileged": true}},
					},
				},
			},
		},
	}
	op := testOp{action: "apply", resource: "deployment", manifest: manifest}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_SystemNamespace_Delete(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "delete", resource: "pod", namespace: "kube-system"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_ProductionNamespace_Confirm(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "deployment", namespace: "production"}
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestJSONPathGet_DeepNested(t *testing.T) {
	obj := map[string]any{
		"a": map[string]any{
			"b": []any{
				map[string]any{"c": "hello"},
				map[string]any{"c": "world"},
			},
		},
	}
	got, ok := JSONPathGet(obj, "a.b[*].c")
	// JSONPathGet only returns first array element, so got="hello"
	assert.True(t, ok)
	assert.Equal(t, "hello", got)
}

func TestJSONPathGet_Missing(t *testing.T) {
	obj := map[string]any{"a": 1}
	_, ok := JSONPathGet(obj, "a.b.c")
	assert.False(t, ok)
}
