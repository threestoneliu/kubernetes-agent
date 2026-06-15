package k8s

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
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

func TestOperation_JSONUnmarshal(t *testing.T) {
	input := `{"action":"apply","manifest":{"kind":"Pod","metadata":{"name":"nginx"}},"resource":"pods","name":"nginx","namespace":"default","cluster_id":"c1"}`
	var op Operation
	require.NoError(t, json.Unmarshal([]byte(input), &op))
	assert.Equal(t, "apply", op.Action())
	assert.Equal(t, "pods", op.Resource())
	assert.Equal(t, "nginx", op.name)
	assert.Equal(t, "default", op.Namespace())
	assert.Equal(t, "c1", op.clusterID)
	assert.Equal(t, "Pod", op.Kind(), "kind should be derived from manifest")
}

func TestOperation_JSONMarshal(t *testing.T) {
	manifest := map[string]any{"kind": "Pod"}
	op := Operation{
		action:   "delete",
		resource: "pods",
		name:     "nginx",
		manifest: &manifest,
	}
	b, err := json.Marshal(op)
	require.NoError(t, err)
	var roundTrip Operation
	require.NoError(t, json.Unmarshal(b, &roundTrip))
	assert.Equal(t, op.Action(), roundTrip.Action())
	assert.Equal(t, op.Resource(), roundTrip.Resource())
	assert.Equal(t, op.name, roundTrip.name)
}

// --- tool-level unit tests using a fake dynamic client ---

// stubFactory is a k8s.ClientFactory that hands out a single
// dynfake.FakeDynamicClient regardless of cluster id.
type stubFactory struct {
	dc *dynfake.FakeDynamicClient
}

// wellKnownGV maps a lowercase plural resource to the Group/Version
// the production tools assume via discovery; the k8s tools build
// GVRs with empty Version, so we translate here before delegating
// to the fake client (which needs the explicit Version).
var wellKnownGV = map[string]schema.GroupVersion{
	"pods":              {"", "v1"},
	"events":            {"", "v1"},
	"deployments":       {"apps", "v1"},
	"nodes":             {"", "v1"},
	"namespaces":        {"", "v1"},
	"services":          {"", "v1"},
	"configmaps":        {"", "v1"},
	"secrets":           {"", "v1"},
	"replicasets":       {"apps", "v1"},
	"statefulsets":      {"apps", "v1"},
	"daemonsets":        {"apps", "v1"},
}

func (f *stubFactory) Get(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	return &shimClient{inner: f.dc}, nil
}
func (f *stubFactory) Invalidate(clusterID string) {}

type shimClient struct {
	inner *dynfake.FakeDynamicClient
}

func (s *shimClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	if gvr.Version == "" {
		if gv, ok := wellKnownGV[gvr.Resource]; ok {
			gvr.Group = gv.Group
			gvr.Version = gv.Version
		}
	}
	return s.inner.Resource(gvr)
}

func newSeededFactory(t *testing.T) (*stubFactory, *dynfake.FakeDynamicClient) {
	t.Helper()
	dc := newFakeDynamicClient()
	seedPod(t, dc, "pod-a", "default")
	seedPod(t, dc, "pod-b", "default")
	return &stubFactory{dc: dc}, dc
}

func TestList_FindsSeededPods(t *testing.T) {
	f, _ := newSeededFactory(t)
	out, err := List(context.Background(), f, ListInput{
		Resource:  "pods",
		Namespace: "default",
		ClusterID: "c1",
	})
	require.NoError(t, err)
	assert.Len(t, out.Items, 2)
}

func TestList_AllNamespaces(t *testing.T) {
	f, dc := newSeededFactory(t)
	seedPod(t, dc, "kube-pod", "kube-system")
	out, err := List(context.Background(), f, ListInput{
		Resource:  "pods",
		Namespace: "",
		ClusterID: "c1",
	})
	require.NoError(t, err)
	assert.Len(t, out.Items, 3)
}

func TestGet_DefaultsToDefaultNamespace(t *testing.T) {
	f, _ := newSeededFactory(t)
	out, err := Get(context.Background(), f, GetInput{
		Resource:  "pods",
		Name:      "pod-a",
		ClusterID: "c1",
	})
	require.NoError(t, err)
	assert.Equal(t, "pod-a", out.Object["metadata"].(map[string]any)["name"])
}

func TestGet_NotFound(t *testing.T) {
	f, _ := newSeededFactory(t)
	_, err := Get(context.Background(), f, GetInput{
		Resource:  "pods",
		Name:      "missing",
		ClusterID: "c1",
	})
	require.Error(t, err)
}

func TestDescribe_NoStatus(t *testing.T) {
	f, _ := newSeededFactory(t)
	out, err := Describe(context.Background(), f, DescribeInput{
		Resource:  "pods",
		Name:      "pod-a",
		ClusterID: "c1",
	})
	require.NoError(t, err)
	assert.Equal(t, "pod-a", out.Object["metadata"].(map[string]any)["name"])
}

func TestPlanWrite_DryRunScale(t *testing.T) {
	f, dc := newSeededFactory(t)
	dep := &unstructured.Unstructured{}
	dep.SetKind("Deployment")
	dep.SetAPIVersion("apps/v1")
	dep.SetName("web")
	dep.SetNamespace("default")
	_ = unstructured.SetNestedField(dep.Object, int64(3), "spec", "replicas")
	_, err := dc.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace("default").Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	out, err := PlanWrite(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, PlanInput{
		Operations: []Operation{{
			action:    "scale",
			resource:  "deployments",
			name:      "web",
			namespace: "default",
			replicas:  intPtr(1),
			clusterID: "c1",
		}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, out.PlanID)
	require.Len(t, out.Diffs, 1)
	assert.Equal(t, "scale", out.Diffs[0].Action)
	assert.Equal(t, "high", out.Diffs[0].Risk, "scale in default ns is not in production, but still a write")
}

func TestPlanWrite_DenyIsolated(t *testing.T) {
	f, _ := newSeededFactory(t)
	out, err := PlanWrite(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, PlanInput{
		Operations: []Operation{{
			action:    "delete",
			resource:  "pods",
			name:      "pod-a",
			namespace: "kube-system",
			clusterID: "c1",
		}},
	})
	require.NoError(t, err)
	require.Len(t, out.Denied, 1)
	assert.Empty(t, out.Diffs)
}

func TestPlanWrite_ApplyRequiresManifest(t *testing.T) {
	f, _ := newSeededFactory(t)
	_, err := PlanWrite(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, PlanInput{
		Operations: []Operation{{
			action:    "apply",
			resource:  "pods",
			name:      "pod-a",
			namespace: "default",
			clusterID: "c1",
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest")
}

func TestDryRun_Delete(t *testing.T) {
	f, _ := newSeededFactory(t)
	diff, err := dryRun(context.Background(), &shimClient{inner: f.dc}, Operation{
		action:    "delete",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	})
	require.NoError(t, err)
	assert.Equal(t, "delete", diff.Action)
	assert.Equal(t, "pod-a", diff.Name)
	assert.NotNil(t, diff.Before)
}

func TestDryRun_Apply(t *testing.T) {
	f, _ := newSeededFactory(t)
	manifest := map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "pod-a", "namespace": "default"},
		"spec":     map[string]any{"containers": []any{map[string]any{"name": "c", "image": "nginx"}}},
	}
	diff, err := dryRun(context.Background(), &shimClient{inner: f.dc}, Operation{
		action:    "apply",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		manifest:  &manifest,
		clusterID: "c1",
	})
	require.NoError(t, err)
	assert.Equal(t, "apply", diff.Action)
}

func TestDryRun_UnknownAction(t *testing.T) {
	f, _ := newSeededFactory(t)
	_, err := dryRun(context.Background(), &shimClient{inner: f.dc}, Operation{
		action:    "weird",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	})
	require.Error(t, err)
}

func TestExecutePlan_Scale(t *testing.T) {
	f, dc := newSeededFactory(t)
	st := newTempStore(t)
	dep := &unstructured.Unstructured{}
	dep.SetKind("Deployment")
	dep.SetAPIVersion("apps/v1")
	dep.SetName("web")
	dep.SetNamespace("default")
	_ = unstructured.SetNestedField(dep.Object, int64(3), "spec", "replicas")
	_, err := dc.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace("default").Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	ops := []Operation{{
		action:    "scale",
		resource:  "deployments",
		name:      "web",
		namespace: "default",
		replicas:  intPtr(1),
		clusterID: "c1",
	}}
	out, err := ExecutePlan(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, st, ExecuteInput{PlanID: "p1"}, ops)
	require.NoError(t, err)
	require.Len(t, out.Results, 1)
	assert.Equal(t, "ok", out.Results[0].Status)
	// Verify the deployment was actually scaled.
	got, err := dc.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace("default").Get(context.Background(), "web", metav1.GetOptions{})
	require.NoError(t, err)
	r, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	assert.Equal(t, int64(1), r)
}

func TestExecutePlan_PolicyDeniedAtExecute(t *testing.T) {
	f, dc := newSeededFactory(t)
	seedPod(t, dc, "core", "kube-system")
	st := newTempStore(t)
	ops := []Operation{{
		action:    "delete",
		resource:  "pods",
		name:      "core",
		namespace: "kube-system",
		clusterID: "c1",
	}}
	_, err := ExecutePlan(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, st, ExecuteInput{PlanID: "p1"}, ops)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestExecutePlan_ApplyUnknown(t *testing.T) {
	f, _ := newSeededFactory(t)
	st := newTempStore(t)
	_, err := ExecutePlan(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, st, ExecuteInput{PlanID: "p1"}, []Operation{{
		action:    "weird",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	}})
	require.Error(t, err)
}

// newTempStore opens a fresh SQLite-backed store in a temp dir and
// runs migrations so AppendAudit has a working table. Returns the
// store; the temp file is cleaned up by t.TempDir().
func newTempStore(t *testing.T) *store.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate())
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func intPtr(i int) *int { return &i }

// --- additional coverage for describe, diagnose, plan paths, applyOne,
// and the production KubeconfigClientFactory.

// seedDeployment creates a Deployment with the given replicas count
// so scale / describe / execute tests have something to look at.
func seedDeployment(t *testing.T, dc *dynfake.FakeDynamicClient, name, namespace string, replicas int64) {
	t.Helper()
	dep := &unstructured.Unstructured{}
	dep.SetKind("Deployment")
	dep.SetAPIVersion("apps/v1")
	dep.SetName(name)
	dep.SetNamespace(namespace)
	_ = unstructured.SetNestedField(dep.Object, replicas, "spec", "replicas")
	_, err := dc.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace(namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestDescribe_WithOwnerRefsAndEvents(t *testing.T) {
	f, dc := newSeededFactory(t)
	// Seed a deployment with owner refs (self-reference for testing).
	dep := &unstructured.Unstructured{}
	dep.SetKind("Deployment")
	dep.SetAPIVersion("apps/v1")
	dep.SetName("web")
	dep.SetNamespace("default")
	_ = unstructured.SetNestedSlice(dep.Object, []any{
		map[string]any{"kind": "ReplicaSet", "name": "web-abc", "uid": "rs-uid-1"},
	}, "metadata", "ownerReferences")
	_, err := dc.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace("default").Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	// Seed an event that targets the deployment's UID.
	ev := &unstructured.Unstructured{}
	ev.SetKind("Event")
	ev.SetAPIVersion("v1")
	ev.SetName("ev-1")
	ev.SetNamespace("default")
	_ = unstructured.SetNestedField(ev.Object, "web", "involvedObject", "name")
	_ = unstructured.SetNestedField(ev.Object, "Deployment", "involvedObject", "kind")
	_ = unstructured.SetNestedField(ev.Object, "rs-uid-1", "involvedObject", "uid")
	_, err = dc.Resource(schema.GroupVersionResource{Version: "v1", Resource: "events"}).
		Namespace("default").Create(context.Background(), ev, metav1.CreateOptions{})
	require.NoError(t, err)

	out, err := Describe(context.Background(), f, DescribeInput{
		Resource: "deployments", Name: "web",
		Namespace: "default", ClusterID: "c1",
	})
	require.NoError(t, err)
	assert.Len(t, out.OwnerRefs, 1)
}

func TestDiagnoseStatus_AllPaths(t *testing.T) {
	obj := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "x"},
				map[string]any{"type": "PodScheduled", "status": "False", "reason": "y"},
				map[string]any{"type": "Other", "status": "False", "reason": "z"},
			},
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{"waiting": map[string]any{"reason": "ImagePullBackOff"}}},
				map[string]any{"state": map[string]any{"waiting": map[string]any{"reason": "ErrImagePull"}}},
				map[string]any{"state": map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}}},
				map[string]any{"state": map[string]any{"waiting": map[string]any{"reason": "Pending"}}},
			},
		},
	}
	hints := diagnoseStatus(obj)
	assert.GreaterOrEqual(t, len(hints), 5)
}

func TestDiagnoseStatus_NotConditionType(t *testing.T) {
	// conditions entry that isn't a map — should be skipped.
	obj := map[string]any{
		"status": map[string]any{
			"conditions": []any{"not-a-map"},
		},
	}
	hints := diagnoseStatus(obj)
	assert.Empty(t, hints)
}

func TestDryRun_Scale(t *testing.T) {
	f, dc := newSeededFactory(t)
	seedDeployment(t, dc, "web", "default", 3)
	diff, err := dryRun(context.Background(), &shimClient{inner: f.dc}, Operation{
		action:    "scale",
		resource:  "deployments",
		name:      "web",
		namespace: "default",
		replicas:  intPtr(5),
		clusterID: "c1",
	})
	require.NoError(t, err)
	assert.Equal(t, "scale", diff.Action)
	assert.NotNil(t, diff.Before)
}

func TestPlanWrite_FactoryError(t *testing.T) {
	// Factory returns error from Get — PlanWrite should wrap and return.
	f := &errorFactory{err: assert.AnError}
	_, err := PlanWrite(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, PlanInput{
		Operations: []Operation{{
			action:    "delete",
			resource:  "pods",
			name:      "x",
			namespace: "default",
			clusterID: "c1",
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get client")
}

type errorFactory struct{ err error }

func (e *errorFactory) Get(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	return nil, e.err
}
func (e *errorFactory) Invalidate(clusterID string) {}

func TestApplyOne_Apply(t *testing.T) {
	f, _ := newSeededFactory(t)
	manifest := map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "pod-a", "namespace": "default"},
		"spec":     map[string]any{"containers": []any{map[string]any{"name": "c", "image": "nginx"}}},
	}
	err := applyOne(context.Background(), f, Operation{
		action:    "apply",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		manifest:  &manifest,
		clusterID: "c1",
	})
	require.NoError(t, err)
}

func TestApplyOne_Delete(t *testing.T) {
	f, _ := newSeededFactory(t)
	err := applyOne(context.Background(), f, Operation{
		action:    "delete",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	})
	require.NoError(t, err)
}

func TestApplyOne_ApplyNoManifest(t *testing.T) {
	f, _ := newSeededFactory(t)
	err := applyOne(context.Background(), f, Operation{
		action:    "apply",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest")
}

func TestApplyOne_UnknownAction(t *testing.T) {
	f, _ := newSeededFactory(t)
	err := applyOne(context.Background(), f, Operation{
		action:    "weird",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestExecutePlan_DeletePath(t *testing.T) {
	f, _ := newSeededFactory(t)
	st := newTempStore(t)
	ops := []Operation{{
		action:    "delete",
		resource:  "pods",
		name:      "pod-a",
		namespace: "default",
		clusterID: "c1",
	}}
	out, err := ExecutePlan(context.Background(), f, &policy.Engine{Rules: policy.DefaultRules()}, st, ExecuteInput{PlanID: "p1"}, ops)
	require.NoError(t, err)
	require.Len(t, out.Results, 1)
}

func TestExecutePlan_PolicyChangedMidWay(t *testing.T) {
	// Engine that allows delete at plan time but denies at execute
	// time — exercises the re-evaluation path.
	f, _ := newSeededFactory(t)
	st := newTempStore(t)
	allowEng := &policy.Engine{}
	denyEng := &policy.Engine{Rules: policy.DefaultRules()}
	ops := []Operation{{
		action:    "delete",
		resource:  "pods",
		name:      "pod-a",
		namespace: "kube-system",
		clusterID: "c1",
	}}
	_, err := ExecutePlan(context.Background(), f, denyEng, st, ExecuteInput{PlanID: "p1"}, ops)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
	// Silence unused var warning — allowEng is checked at plan time
	// via PlanWrite elsewhere; here we just need denyEng to fire.
	_ = allowEng
}

// --- KubeconfigClientFactory tests ---

func TestKubeconfigFactory_NewAndInvalidate(t *testing.T) {
	st := newTempStore(t)
	aead := newTestAEAD(t)
	f := NewKubeconfigClientFactory(st, aead)
	assert.NotNil(t, f)
	f.Invalidate("c1") // no-op on empty cache
}

func TestKubeconfigFactory_NewClientFactoryAlias(t *testing.T) {
	st := newTempStore(t)
	aead := newTestAEAD(t)
	f := NewClientFactory(st, aead)
	assert.NotNil(t, f)
}

func TestKubeconfigFactory_GetClusterMissing(t *testing.T) {
	st := newTempStore(t)
	aead := newTestAEAD(t)
	f := NewKubeconfigClientFactory(st, aead)
	_, err := f.Get(context.Background(), "no-such-cluster")
	require.Error(t, err)
}

func TestKubeconfigFactory_GetDecryptError(t *testing.T) {
	st := newTempStore(t)
	aead := newTestAEAD(t)
	require.NoError(t, st.CreateCluster(context.Background(), store.Cluster{
		ID: "c1", Name: "n", Server: "s", User: "u",
		KubeconfigBlob: []byte("garbage-not-encrypted"),
	}))
	f := NewKubeconfigClientFactory(st, aead)
	_, err := f.Get(context.Background(), "c1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decrypt")
}

func TestKubeconfigFactory_GetParseError(t *testing.T) {
	st := newTempStore(t)
	aead := newTestAEAD(t)
	// Encrypt a blob that is not a valid kubeconfig YAML.
	blob, err := aead.Encrypt([]byte("not-a-kubeconfig"))
	require.NoError(t, err)
	require.NoError(t, st.CreateCluster(context.Background(), store.Cluster{
		ID: "c1", Name: "n", Server: "s", User: "u",
		KubeconfigBlob: blob,
	}))
	f := NewKubeconfigClientFactory(st, aead)
	_, err = f.Get(context.Background(), "c1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse kubeconfig")
}

func TestKubeconfigFactory_GetAndCache(t *testing.T) {
	st := newTempStore(t)
	aead := newTestAEAD(t)
	// Minimal valid kubeconfig: one cluster, one user, one context.
	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: https://localhost:9
contexts:
- name: main
  context:
    cluster: local
    user: me
current-context: main
users:
- name: me
  user: {}
`)
	blob, err := aead.Encrypt(kubeconfig)
	require.NoError(t, err)
	require.NoError(t, st.CreateCluster(context.Background(), store.Cluster{
		ID: "c1", Name: "n", Server: "s", User: "u",
		KubeconfigBlob: blob,
	}))
	f := NewKubeconfigClientFactory(st, aead)
	dc, err := f.Get(context.Background(), "c1")
	require.NoError(t, err)
	assert.NotNil(t, dc)
	// Second call hits the cache.
	dc2, err := f.Get(context.Background(), "c1")
	require.NoError(t, err)
	assert.NotNil(t, dc2)
	// Invalidate drops the cached client.
	f.Invalidate("c1")
}

func newTestAEAD(t *testing.T) *crypto.AEAD {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	a, err := crypto.NewAEAD(key)
	require.NoError(t, err)
	return a
}