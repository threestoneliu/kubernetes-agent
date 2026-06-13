package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GetInput struct {
	Resource  string `json:"resource"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterID string `json:"cluster_id"`
}

type GetOutput struct {
	Object map[string]any `json:"object"`
}

// Get fetches a single resource by name. Defaults namespace to "default"
// when empty.
func Get(ctx context.Context, f *ClientFactory, in GetInput) (*GetOutput, error) {
	if in.Namespace == "" {
		in.Namespace = "default"
	}
	gvr := schema.GroupVersionResource{Resource: in.Resource}
	dc, err := f.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	res := dc.Resource(gvr).Namespace(in.Namespace)
	obj, err := res.Get(ctx, in.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", in.Namespace, in.Name, err)
	}
	return &GetOutput{Object: obj.UnstructuredContent()}, nil
}