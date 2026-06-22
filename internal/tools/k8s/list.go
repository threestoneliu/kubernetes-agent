package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	gvr := f.Resolver(in.ClusterID).Resolve(in.Resource)

	base := cfg.Host
	if gvr.Group == "" {
		base += fmt.Sprintf("/api/%s/namespaces/", gvr.Version)
	} else {
		base += fmt.Sprintf("/apis/%s/%s/namespaces/", gvr.Group, gvr.Version)
	}
	ns := in.Namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}
	urlPath := base + ns + "/" + gvr.Resource

	q := url.Values{}
	if in.LabelSelector != "" {
		q.Set("labelSelector", in.LabelSelector)
	}
	if len(q) > 0 {
		urlPath += "?" + q.Encode()
	}

	req, err := http.NewRequest("GET", urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")

	// Build a transport from the REST config and create an HTTP client.
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
	rows := make([][]string, 0, len(table.Rows))
	for _, r := range table.Rows {
		cells := make([]string, len(table.ColumnDefinitions))
		for i, c := range r.Cells {
			cells[i] = fmt.Sprintf("%v", c)
		}
		rows = append(rows, cells)
	}
	return &TableOutput{Columns: cols, Rows: rows}, nil
}
