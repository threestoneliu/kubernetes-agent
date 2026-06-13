package k8s

// Operation represents a single K8s action requested by the LLM.
// It implements policy.OperationInfo so it can be evaluated by the
// policy engine.
type Operation struct {
	action    string          `json:"action"` // apply | delete | scale
	manifest  *map[string]any `json:"manifest,omitempty"`
	resource  string          `json:"resource,omitempty"`
	name      string          `json:"name,omitempty"`
	namespace string          `json:"namespace,omitempty"`
	replicas  *int            `json:"replicas,omitempty"`
	kind      string          `json:"kind,omitempty"`
	clusterID string          `json:"cluster_id"`
}

// Getter methods for policy.OperationInfo:

func (o Operation) Action() string { return o.action }

func (o Operation) Resource() string { return o.resource }

func (o Operation) Namespace() string { return o.namespace }

func (o Operation) Kind() string {
	if o.kind != "" {
		return o.kind
	}
	if o.manifest != nil {
		if k, ok := (*o.manifest)["kind"].(string); ok {
			return k
		}
	}
	return ""
}

func (o Operation) Manifest() map[string]any {
	if o.manifest == nil {
		return nil
	}
	return *o.manifest
}