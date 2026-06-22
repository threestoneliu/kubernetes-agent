package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

// TableOutput is the structured table response returned by ListTable.
type TableOutput struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
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

// ListTable returns resources in kubectl-style table format by requesting
// the meta.k8s.io Table MIME type from the API server.
func ListTable(ctx context.Context, f ClientFactory, in ListInput) (*TableOutput, error) {
	cfg, err := f.RESTConfig(in.ClusterID)
	if err != nil {
		return nil, err
	}
	resolver := f.Resolver(in.ClusterID)
	gvr := resolver.Resolve(in.Resource)

	urlPath := buildListURL(resolver, in.Resource, cfg.Host, gvr, in.Namespace, in.LabelSelector)

	req, err := http.NewRequest("GET", urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")

	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("build transport: %w", err)
	}
	httpClient := &http.Client{Transport: transport}

	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", in.Resource, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list %s: HTTP %d – %s", in.Resource, resp.StatusCode, string(body))
	}

	var table metav1.Table
	if err := json.NewDecoder(resp.Body).Decode(&table); err != nil {
		return nil, fmt.Errorf("decode table: %w", err)
	}

	cols := make([]string, len(table.ColumnDefinitions))
	for i, c := range table.ColumnDefinitions {
		cols[i] = c.Name
	}

	// Some API servers (e.g. Docker Desktop, kind) don't include a
	// Namespace column in the Table for cluster-level listings of
	// namespaced resources (/api/v1/pods). The namespace is still
	// present in each row's Object, so we inject the column.
	nsIdx := -1
	for i, col := range cols {
		if col == "Namespace" {
			nsIdx = i
			break
		}
	}
	injectNS := nsIdx == -1 && resolver.IsNamespaced(in.Resource)

	if injectNS {
		nsIdx = len(cols)
		cols = append(cols, "Namespace")
	}

	rows := make([][]string, 0, len(table.Rows))
	for _, r := range table.Rows {
		cells := make([]string, len(table.ColumnDefinitions))
		for i, c := range r.Cells {
			cells[i] = fmt.Sprintf("%v", c)
		}
		if injectNS {
			ns := extractNamespace(r.Object)
			cells = append(cells, ns)
		}
		rows = append(rows, cells)
	}
	return &TableOutput{Columns: cols, Rows: rows}, nil
}

// extractNamespace reads the namespace from a TableRow.Object runtime.Object.
// The Object contains the full resource with its metadata.namespace field.
func extractNamespace(obj any) string {
	if obj == nil {
		return ""
	}
	// The Object can be a map[string]any (from JSON unmarshal) or a
	// typed runtime object. Try JSON round-trip which works for both.
	b, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	meta, ok := m["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	ns, _ := meta["namespace"].(string)
	return ns
}

// buildListURL constructs the correct API path for a resource.
// It queries the resolver's discovery cache (APIResource.Namespaced field)
// to determine whether to use a cluster-scoped or namespaced path.
//
// For namespaced resources with an empty namespace (list across all namespaces),
// we use the cluster-level path (e.g. /api/v1/pods) instead of the
// namespaced path (/api/v1/namespaces//pods) — this is the same path kubectl
// uses for `kubectl get <resource> -A` and the returned Table includes a
// "Namespace" column so the user can see which namespace each row belongs to.
func buildListURL(resolver *Resolver, resource, host string, gvr schema.GroupVersionResource, namespace, labelSelector string) string {
	isNS := resolver.IsNamespaced(resource)

	// Cluster-scoped resources, and namespaced resources with empty namespace
	// (list all), use the cluster-level path.
	if !isNS || namespace == "" {
		var base string
		if gvr.Group == "" {
			base = fmt.Sprintf("%s/api/%s/%s", host, gvr.Version, gvr.Resource)
		} else {
			base = fmt.Sprintf("%s/apis/%s/%s/%s", host, gvr.Group, gvr.Version, gvr.Resource)
		}
		if labelSelector != "" {
			return base + "?labelSelector=" + url.QueryEscape(labelSelector)
		}
		return base
	}

	// Namespaced resource with a specific namespace.
	var base string
	if gvr.Group == "" {
		base = fmt.Sprintf("%s/api/%s/namespaces/%s/%s", host, gvr.Version, namespace, gvr.Resource)
	} else {
		base = fmt.Sprintf("%s/apis/%s/%s/namespaces/%s/%s", host, gvr.Group, gvr.Version, namespace, gvr.Resource)
	}
	if labelSelector != "" {
		return base + "?labelSelector=" + url.QueryEscape(labelSelector)
	}
	return base
}
