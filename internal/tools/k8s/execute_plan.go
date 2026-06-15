package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

type ExecuteInput struct {
	PlanID       string `json:"plan_id"`
	ConfirmToken string `json:"confirm_token"`
}

type Result struct {
	Action  string `json:"action"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ExecuteOutput struct {
	Results []Result `json:"results"`
}

// ExecutePlan re-evaluates every operation against the current policy
// (so a rule change between plan and execute cannot silently bypass a
// deny), then applies them in order. If an op fails, it attempts to
// roll back the previously executed ops.
//
// NOTE on rollback: a full rollback requires the per-op "before" state
// captured in PlanOutput. The current stub logs the failure and returns
// an error so a human can intervene. A future improvement would thread
// PlanOutput (or a persisted plan record) into ExecutePlan and re-apply
// the before-state for "apply" ops or re-create the resource for "delete"
// ops.
func ExecutePlan(ctx context.Context, f ClientFactory, eng *policy.Engine, st *store.DB, in ExecuteInput, ops []Operation) (*ExecuteOutput, error) {
	for _, op := range ops {
		if eng.Evaluate(op) == policy.Deny {
			return nil, fmt.Errorf("plan aborted: policy changed and op is now denied")
		}
	}
	out := &ExecuteOutput{}
	executed := 0
	for i, op := range ops {
		if err := applyOne(ctx, f, op); err != nil {
			rolledBack := 0
			for j := executed - 1; j >= 0; j-- {
				if rbErr := rollbackOne(ctx, f, ops[j]); rbErr == nil {
					rolledBack++
				}
			}
			msg := fmt.Sprintf("plan %s op %d failed: %v, rolled back %d", in.PlanID, i, err, rolledBack)
			_, _ = st.AppendAudit(ctx, store.AuditEntry{
				Action:  "execute_plan",
				Status:  "failed",
				Message: &msg,
			})
			return nil, fmt.Errorf("op %d failed: %w (rolled back %d)", i, err, rolledBack)
		}
		out.Results = append(out.Results, Result{Action: op.action, Status: "ok"})
		executed++
	}
	msg := fmt.Sprintf("plan %s executed %d ops", in.PlanID, executed)
	_, _ = st.AppendAudit(ctx, store.AuditEntry{
		Action:  "execute_plan",
		Status:  "ok",
		Message: &msg,
	})
	return out, nil
}

func applyOne(ctx context.Context, f ClientFactory, op Operation) error {
	dc, err := f.Get(ctx, op.clusterID)
	if err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Resource: op.resource}
	res := dc.Resource(gvr).Namespace(op.namespace)
	switch op.action {
	case "apply":
		if op.manifest == nil {
			return fmt.Errorf("apply requires manifest")
		}
		u := &unstructured.Unstructured{Object: *op.manifest}
		_, err := res.Patch(ctx, u.GetName(), "application/merge-patch+json", mustJSON(*op.manifest), metav1.PatchOptions{})
		return err
	case "delete":
		return res.Delete(ctx, op.name, metav1.DeleteOptions{})
	case "scale":
		patch := map[string]any{"spec": map[string]any{"replicas": op.replicas}}
		_, err := res.Patch(ctx, op.name, "application/merge-patch+json", mustJSON(patch), metav1.PatchOptions{})
		return err
	default:
		return fmt.Errorf("unknown action %q", op.action)
	}
}

func rollbackOne(_ context.Context, _ ClientFactory, op Operation) error {
	// Rollback strategy limitation: without the per-op "before" state we
	// cannot symmetrically undo "apply" or re-create "delete". Return an
	// error so the caller can surface the failure to the user. A future
	// improvement threads PlanOutput (with its Before field) through.
	return fmt.Errorf("rollback not implemented for action %s (would need before-state)", op.action)
}