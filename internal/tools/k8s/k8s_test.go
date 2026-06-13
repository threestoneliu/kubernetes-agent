package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/threestoneliu/kubernetes-agent/internal/policy"
)

func newFakeDynamicClient() *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:              "PodList",
		{Group: "", Version: "v1", Resource: "events"}:            "EventList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:   "DeploymentList",
	})
}

func seedPod(t *testing.T, dc *dynfake.FakeDynamicClient, name, namespace string) {
	t.Helper()
	pod := &unstructured.Unstructured{}
	pod.SetKind("Pod")
	pod.SetAPIVersion("v1")
	pod.SetName(name)
	pod.SetNamespace(namespace)
	_, err := dc.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
}

// Sanity check: the fake client works as expected. ClientFactory.Get
// requires a real store + AEAD, so we exercise the lower-level building
// blocks directly here.
func TestList_Empty(t *testing.T) {
	dc := newFakeDynamicClient()
	list, err := dc.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestList_WithSeededPod(t *testing.T) {
	dc := newFakeDynamicClient()
	seedPod(t, dc, "nginx", "default")
	list, err := dc.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, list.Items, 1)
	assert.Equal(t, "nginx", list.Items[0].GetName())
}

func TestOperation_PolicyInfo(t *testing.T) {
	manifest := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "web"},
	}
	op := Operation{
		action:   "apply",
		manifest: &manifest,
		resource: "deployments",
		name:     "web",
	}
	assert.Equal(t, "apply", op.Action())
	assert.Equal(t, "deployments", op.Resource())
	assert.Equal(t, "Deployment", op.Kind(), "kind should be derived from manifest")
	assert.Equal(t, manifest, op.Manifest())
}

func TestOperation_KindExplicit(t *testing.T) {
	manifest := map[string]any{"kind": "Pod"}
	op := Operation{action: "delete", resource: "pods", kind: "Pod", manifest: &manifest}
	assert.Equal(t, "Pod", op.Kind())
}

func TestOperation_NamespaceField(t *testing.T) {
	op := Operation{action: "delete", namespace: "production"}
	assert.Equal(t, "production", op.Namespace())
}

func TestOperation_ManifestNil(t *testing.T) {
	op := Operation{action: "delete"}
	assert.Nil(t, op.Manifest())
}

func TestAskUser_DeterministicID(t *testing.T) {
	a := AskUser(AskUserInput{Question: "Which namespace?"})
	b := AskUser(AskUserInput{Question: "Which namespace?"})
	c := AskUser(AskUserInput{Question: "Which cluster?"})
	assert.Equal(t, a.QuestionID, b.QuestionID)
	assert.NotEqual(t, a.QuestionID, c.QuestionID)
}

func TestRiskFrom(t *testing.T) {
	assert.Equal(t, "low", riskFrom(policy.Allow))
	assert.Equal(t, "high", riskFrom(policy.Confirm))
	assert.Equal(t, "low", riskFrom(policy.Deny))
}

func TestSummarize(t *testing.T) {
	assert.Equal(t, "3 个操作待确认", summarize([]Diff{{}, {}, {}}, nil))
	assert.Equal(t, "全部 2 个操作被 policy 拒绝", summarize(nil, []DeniedOp{{}, {}}))
	assert.Equal(t, "1 个操作待确认,1 个被 policy 拒绝", summarize([]Diff{{}}, []DeniedOp{{}}))
}

func TestDiagnoseStatus_ImagePullBackOff(t *testing.T) {
	obj := map[string]any{
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{
					"state": map[string]any{
						"waiting": map[string]any{"reason": "ImagePullBackOff"},
					},
				},
			},
		},
	}
	hints := diagnoseStatus(obj)
	assert.NotEmpty(t, hints)
}

func TestDiagnoseStatus_NoStatus(t *testing.T) {
	hints := diagnoseStatus(map[string]any{})
	assert.Empty(t, hints)
}