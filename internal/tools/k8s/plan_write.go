package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
func PlanWrite(ctx context.Context, f *ClientFactory, eng *policy.Engine, in PlanInput) (*PlanOutput, error) {
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
		diff, err := dryRun(ctx, dc, op)
		if err != nil {
			return nil, fmt.Errorf("dry-run %s %s/%s: %w", op.action, op.namespace, op.name, err)
		}
		diff.Risk = riskFrom(eff)
		out.Diffs = append(out.Diffs, *diff)
	}
	out.Summary = summarize(out.Diffs, out.Denied)
	return out, nil
}

func dryRun(ctx context.Context, dc dynamic.Interface, op Operation) (*Diff, error) {
	gvr := schema.GroupVersionResource{Resource: op.resource}
	res := dc.Resource(gvr).Namespace(op.namespace)
	switch op.action {
	case "apply":
		if op.manifest == nil {
			return nil, fmt.Errorf("apply requires manifest")
		}
		u := &unstructured.Unstructured{Object: *op.manifest}
		got, err := res.Patch(ctx, u.GetName(), "application/merge-patch+json", mustJSON(*op.manifest), metav1.PatchOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, err
		}
		return &Diff{Action: op.action, Resource: op.resource, Name: u.GetName(), Namespace: op.namespace, After: got.UnstructuredContent()}, nil
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
	if len(diffs) == 0 && len(denied) > 0 {
		return fmt.Sprintf("全部 %d 个操作被 policy 拒绝", len(denied))
	}
	if len(diffs) > 0 && len(denied) > 0 {
		return fmt.Sprintf("%d 个操作待确认,%d 个被 policy 拒绝", len(diffs), len(denied))
	}
	return fmt.Sprintf("%d 个操作待确认", len(diffs))
}