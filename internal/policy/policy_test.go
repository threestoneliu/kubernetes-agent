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

func TestJSONPathGet_NotAMap(t *testing.T) {
	_, ok := JSONPathGet(map[string]any{"a": "string"}, "a.b")
	assert.False(t, ok)
}

func TestJSONPathGet_ArrayEmpty(t *testing.T) {
	obj := map[string]any{"a": []any{}}
	_, ok := JSONPathGet(obj, "a[*].b")
	assert.False(t, ok)
}

func TestJSONPathGet_ArrayNotSlice(t *testing.T) {
	obj := map[string]any{"a": "not-a-slice"}
	_, ok := JSONPathGet(obj, "a[*].b")
	assert.False(t, ok)
}

func TestJSONPathGet_KeyMissing(t *testing.T) {
	obj := map[string]any{"a": map[string]any{}}
	_, ok := JSONPathGet(obj, "a.b")
	assert.False(t, ok)
}

func TestEngine_NoRulesAllowRead(t *testing.T) {
	e := &Engine{}
	op := testOp{action: "get", resource: "pod", namespace: "default"}
	assert.Equal(t, Allow, e.Evaluate(op))
}

func TestEngine_NoRulesConfirmWrite(t *testing.T) {
	e := &Engine{}
	op := testOp{action: "scale", resource: "deployment"}
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestEngine_KindBlacklist_Delete(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "delete", resource: "node"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_CaseInsensitiveNamespace(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "delete", resource: "pod", namespace: "Kube-System"}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_ProductionScaleConfirm(t *testing.T) {
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "scale", resource: "deployment", namespace: "prod"}
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestEngine_ManifestKindFallback(t *testing.T) {
	manifest := map[string]any{"kind": "Node"}
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "node", manifest: manifest}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_ManifestWithoutKind(t *testing.T) {
	manifest := map[string]any{"spec": "no-kind"}
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "deployment", manifest: manifest}
	// The default rules don't have a Kind match, so this should
	// hit the production namespace Confirm rule (namespace empty).
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestEngine_HostNetworkUnsafe(t *testing.T) {
	manifest := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"hostNetwork": true,
				},
			},
		},
	}
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "deployment", manifest: manifest}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_NoUnsafeFieldMatch(t *testing.T) {
	manifest := map[string]any{
		"spec": map[string]any{
			"replicas": 3,
		},
	}
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "deployment", manifest: manifest}
	// Falls through to production namespace (empty namespace),
	// so Confirm.
	assert.Equal(t, Confirm, e.Evaluate(op))
}

func TestEngine_HostPIDUnsafe(t *testing.T) {
	manifest := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"hostPID": true,
				},
			},
		},
	}
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "deployment", manifest: manifest}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestEngine_KindExplicitOverridesManifest(t *testing.T) {
	// The engine matches Kind against the resource name, not
	// the op.Kind() field — when the resource is a Node-like
	// name the deny rule fires regardless of what the LLM
	// put in op.Kind() or the manifest.
	manifest := map[string]any{"kind": "Pod"}
	e := &Engine{Rules: DefaultRules()}
	op := testOp{action: "apply", resource: "node", kind: "Deployment", manifest: manifest}
	assert.Equal(t, Deny, e.Evaluate(op))
}

func TestCanonicalKind(t *testing.T) {
	assert.Equal(t, "Pod", canonicalKind("pod"))
	assert.Equal(t, "Deployment", canonicalKind("Deployment"))
	assert.Equal(t, "", canonicalKind(""))
}
