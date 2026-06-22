package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/threestoneliu/kubernetes-agent/internal/policy"
)

type PlanInput struct {
	Operations []Operation `json:"operations"`
}

type Diff struct {
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Before    map[string]any `json:"before,omitempty"`
	After     map[string]any `json:"after,omitempty"`
	Risk      string         `json:"risk"`
}

type DeniedOp struct {
	Operation Operation `json:"operation"`
	Reason    string    `json:"reason"`
}

type PlanOutput struct {
	PlanID  string     `json:"plan_id"`
	Summary string     `json:"summary"`
	Diffs   []Diff     `json:"diffs"`
	Denied  []DeniedOp `json:"denied"`
}

// PlanWrite evaluates every operation against the policy engine and, for
// those that are not denied, runs a server-side dry-run to capture the
// predicted diff. The returned PlanOutput can be persisted and later
// re-evaluated for execution.
func PlanWrite(ctx context.Context, f ClientFactory, eng *policy.Engine, in PlanInput) (*PlanOutput, error) {
	planID := uuid.NewString()
	out := &PlanOutput{PlanID: planID}
	for _, op := range in.Operations {
		eff := eng.Evaluate(op)
		if eff == policy.Deny {
			out.Denied = append(out.Denied, DeniedOp{Operation: op, Reason: "policy deny"})
			continue
		}
		dc, err := f.Get(ctx, op.clusterID)
		if err != nil {
			return nil, fmt.Errorf("get client for %s: %w", op.clusterID, err)
		}
		resolver := f.Resolver(op.clusterID)
		diff, err := dryRun(ctx, dc, resolver, op)
		if err != nil {
			return nil, fmt.Errorf("dry-run %s %s/%s: %w", op.action, op.namespace, op.name, err)
		}
		diff.Risk = riskFrom(eff)
		out.Diffs = append(out.Diffs, *diff)
	}
	out.Summary = summarize(out.Diffs, out.Denied)
	return out, nil
}

func dryRun(ctx context.Context, dc dynamic.Interface, resolver *Resolver, op Operation) (*Diff, error) {
	gvr := resolver.Resolve(op.resource)
	res := dc.Resource(gvr).Namespace(op.namespace)
	switch op.action {
	case "apply":
		if op.manifest == nil {
			return nil, fmt.Errorf("apply requires manifest")
		}
		u := &unstructured.Unstructured{Object: *op.manifest}
		dryOpts := metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}
		// GET first to decide create-vs-update. Using server-side
		// apply requires a real apiserver (dynfake doesn't honour
		// its Conflict semantics), so branch explicitly.
		_, gerr := res.Get(ctx, u.GetName(), metav1.GetOptions{})
		if gerr != nil && !isNotFound(gerr) {
			return nil, gerr
		}
		if isNotFound(gerr) {
			created, err := res.Create(ctx, u, dryOpts)
			if err != nil {
				return nil, err
			}
			return &Diff{Action: op.action, Resource: op.resource, Name: u.GetName(), Namespace: op.namespace, After: created.UnstructuredContent()}, nil
		}
		// Resource exists — dry-run a merge-patch to capture the
		// post-update state without mutating anything.
		patched, err := res.Patch(ctx, u.GetName(), "application/merge-patch+json", mustJSON(*op.manifest), metav1.PatchOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, err
		}
		return &Diff{Action: op.action, Resource: op.resource, Name: u.GetName(), Namespace: op.namespace, After: patched.UnstructuredContent()}, nil
	case "delete":
		cur, err := res.Get(ctx, op.name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &Diff{Action: op.action, Resource: op.resource, Name: op.name, Namespace: op.namespace, Before: cur.UnstructuredContent()}, nil
	case "scale":
		cur, err := res.Get(ctx, op.name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &Diff{Action: op.action, Resource: op.resource, Name: op.name, Namespace: op.namespace, Before: cur.UnstructuredContent()}, nil
	default:
		return nil, fmt.Errorf("unknown action %q", op.action)
	}
}

// isNotFound reports whether err is a Kubernetes API NotFound
// (HTTP 404). Used by dryRun's apply branch to fall back from
// server-side apply to Create when the resource does not exist yet.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var s *apierrors.StatusError
	if errors.As(err, &s) {
		return s.ErrStatus.Reason == metav1.StatusReasonNotFound
	}
	return false
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func riskFrom(eff policy.Effect) string {
	switch eff {
	case policy.Allow:
		return "low"
	case policy.Confirm:
		return "high"
	default:
		return "low"
	}
}

func summarize(diffs []Diff, denied []DeniedOp) string {
	var parts []string
	for _, d := range diffs {
		parts = append(parts, summarizeOne(d))
	}
	if len(denied) > 0 {
		parts = append(parts, fmt.Sprintf("全部 %d 个被 policy 拒绝", len(denied)))
	}
	if len(parts) == 0 {
		return "无操作"
	}
	return strings.Join(parts, "; ")
}

func summarizeOne(d Diff) string {
	ns := d.Namespace
	if ns == "" {
		ns = "default"
	}
	kind := getManifestKind(d.After, d.Before)
	name := d.Name
	if name == "" {
		name = "(unnamed)"
	}

	switch d.Action {
	case "delete":
		return fmt.Sprintf("删除 %s %s/%s", kind, ns, name)
	case "scale":
		beforeRep := extractReplicas(d.Before)
		afterRep := extractReplicas(d.After)
		return fmt.Sprintf("调整 %s %s/%s replicas: %s → %s", kind, ns, name, beforeRep, afterRep)
	default: // apply / create
		if d.Before == nil {
			return fmt.Sprintf("创建 %s %s/%s", kind, ns, name)
		}
		// Show what changed
		changes := diffChanges(d.Before, d.After)
		if changes == "" {
			return fmt.Sprintf("更新 %s %s/%s (无变更)", kind, ns, name)
		}
		return fmt.Sprintf("更新 %s %s/%s: %s", kind, ns, name, changes)
	}
}

func getManifestKind(after, before map[string]any) string {
	for _, m := range []map[string]any{after, before} {
		if m == nil {
			continue
		}
		if kind, _ := m["kind"].(string); kind != "" {
			return kind
		}
	}
	return "Unknown"
}

func extractReplicas(m map[string]any) string {
	if m == nil {
		return "?"
	}
	spec, _ := m["spec"].(map[string]any)
	if spec == nil {
		return "?"
	}
	if r, ok := spec["replicas"].(int64); ok {
		return fmt.Sprintf("%d", r)
	}
	if r, ok := spec["replicas"].(float64); ok {
		return fmt.Sprintf("%.0f", r)
	}
	return "?"
}

func diffChanges(before, after map[string]any) string {
	var changes []string
	for _, key := range []string{"replicas", "image", "imagePullPolicy", "ports", "serviceType"} {
		bv := getNested(before, key)
		av := getNested(after, key)
		if fmt.Sprintf("%v", bv) != fmt.Sprintf("%v", av) {
			changes = append(changes, fmt.Sprintf("%s: %v → %v", key, bv, av))
		}
	}
	// Labels
	bl := getNestedLabels(before)
	al := getNestedLabels(after)
	if !labelMapsEqual(bl, al) {
		changes = append(changes, "labels changed")
	}
	if len(changes) == 0 {
		return ""
	}
	if len(changes) > 2 {
		return strings.Join(changes[:2], ", ") + "…"
	}
	return strings.Join(changes, ", ")
}

func getNested(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	spec, _ := m["spec"].(map[string]any)
	if spec == nil {
		return nil
	}
	if key == "image" || key == "imagePullPolicy" {
		containers, _ := spec["containers"].([]any)
		if len(containers) > 0 {
			if c, ok := containers[0].(map[string]any); ok {
				return c[key]
			}
		}
		return nil
	}
	if key == "ports" {
		containers, _ := spec["containers"].([]any)
		if len(containers) > 0 {
			if c, ok := containers[0].(map[string]any); ok {
				return c[key]
			}
		}
		return nil
	}
	return spec[key]
}

func getNestedLabels(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta == nil {
		return nil
	}
	labels, _ := meta["labels"].(map[string]string)
	return labels
}

func labelMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return true
		}
	}
	return false
}