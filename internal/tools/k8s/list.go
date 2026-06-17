package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type ListInput struct {
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace,omitempty"` // empty = all namespaces
	LabelSelector string `json:"label_selector,omitempty"`
	ClusterID     string `json:"cluster_id"`
}

type ListOutput struct {
	Items []map[string]any `json:"items"`
}

// List enumerates resources matching the (optional) namespace and
// label selector. Empty namespace means "all namespaces".
func List(ctx context.Context, f ClientFactory, in ListInput) (*ListOutput, error) {
	gvr := f.Resolver(in.ClusterID).Resolve(in.Resource)
	dc, err := f.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	var res dynamic.ResourceInterface
	if in.Namespace == "" {
		res = dc.Resource(gvr).Namespace(metav1.NamespaceAll)
	} else {
		res = dc.Resource(gvr).Namespace(in.Namespace)
	}
	listOpts := metav1.ListOptions{}
	if in.LabelSelector != "" {
		listOpts.LabelSelector = in.LabelSelector
	}
	list, err := res.List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", in.Resource, err)
	}
	out := &ListOutput{Items: make([]map[string]any, 0, len(list.Items))}
	for i := range list.Items {
		out.Items = append(out.Items, list.Items[i].UnstructuredContent())
	}
	return out, nil
}