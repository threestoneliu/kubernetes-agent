package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type DescribeInput struct {
	Resource  string `json:"resource"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterID string `json:"cluster_id"`
}

type DescribeOutput struct {
	Object         map[string]any   `json:"object"`
	Events         []map[string]any `json:"events"`
	OwnerRefs      []map[string]any `json:"owner_refs"`
	DiagnosisHints []string         `json:"diagnosis_hints"`
}

// Describe fetches a resource plus its related events, owner references,
// and a small set of diagnosis hints derived from status conditions and
// container states.
func Describe(ctx context.Context, f ClientFactory, in DescribeInput) (*DescribeOutput, error) {
	if in.Namespace == "" {
		in.Namespace = "default"
	}
	gvr := f.Resolver(in.ClusterID).Resolve(in.Resource)
	dc, err := f.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	res := dc.Resource(gvr).Namespace(in.Namespace)
	obj, err := res.Get(ctx, in.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("describe %s/%s: %w", in.Namespace, in.Name, err)
	}
	content := obj.UnstructuredContent()

	// Get related events (best-effort: tolerate failure).
	events, _ := listEvents(ctx, dc, in.Namespace, string(obj.GetUID()))

	// Owner refs
	ownerRefs := make([]map[string]any, 0)
	if refs, ok := content["metadata"].(map[string]any)["ownerReferences"].([]any); ok {
		for _, ref := range refs {
			if m, ok := ref.(map[string]any); ok {
				ownerRefs = append(ownerRefs, m)
			}
		}
	}

	// Diagnosis hints based on status conditions
	hints := diagnoseStatus(content)

	return &DescribeOutput{
		Object:         content,
		Events:         events,
		OwnerRefs:      ownerRefs,
		DiagnosisHints: hints,
	}, nil
}

func listEvents(ctx context.Context, dc dynamic.Interface, namespace, uid string) ([]map[string]any, error) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}
	res := dc.Resource(gvr).Namespace(namespace)
	list, err := res.List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.uid=%s", uid),
	})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, list.Items[i].UnstructuredContent())
	}
	return out, nil
}

func diagnoseStatus(obj map[string]any) []string {
	hints := []string{}
	status, _ := obj["status"].(map[string]any)
	conditions, _ := status["conditions"].([]any)
	for _, c := range conditions {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		ctype, _ := cm["type"].(string)
		cstatus, _ := cm["status"].(string)
		creason, _ := cm["reason"].(string)
		if cstatus == "False" {
			switch ctype {
			case "Ready":
				hints = append(hints, fmt.Sprintf("容器/Pod 未就绪 (%s)", creason))
			case "PodScheduled":
				hints = append(hints, "Pod 长时间未调度,检查节点资源/亲和性/污点")
			}
		}
	}
	// Check container statuses for image pull / crash loop
	css, _ := status["containerStatuses"].([]any)
	for _, cs := range css {
		csm, ok := cs.(map[string]any)
		if !ok {
			continue
		}
		state, _ := csm["state"].(map[string]any)
		if w, ok := state["waiting"].(map[string]any); ok {
			reason, _ := w["reason"].(string)
			switch reason {
			case "ImagePullBackOff", "ErrImagePull":
				hints = append(hints, "镜像拉取失败,检查 image name / imagePullSecrets / 网络")
			case "CrashLoopBackOff":
				hints = append(hints, "容器反复崩溃,查看最近一次重启的 logs")
			case "Pending":
				hints = append(hints, "容器等待调度/镜像就绪")
			}
		}
	}
	return hints
}