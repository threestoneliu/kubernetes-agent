package k8s

import "encoding/json"

// Operation represents a single K8s action requested by the LLM.
// It implements policy.OperationInfo so it can be evaluated by the
// policy engine.
//
// Fields are unexported so the public API stays clean (callers use the
// Action() / Resource() / etc. methods instead of touching fields). The
// custom MarshalJSON / UnmarshalJSON below bridge the wire format to the
// internal fields. Without them, encoding/json would silently skip the
// unexported fields and the LLM-driven tool calls would never populate
// the struct.
type Operation struct {
	action    string
	manifest  *map[string]any
	resource  string
	name      string
	namespace string
	replicas  *int
	kind      string
	clusterID string
}

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

// operationWire is the JSON wire format for Operation. Exported fields so
// encoding/json can populate them.
type operationWire struct {
	Action    string          `json:"action"`
	Manifest  *map[string]any `json:"manifest,omitempty"`
	Resource  string          `json:"resource,omitempty"`
	Name      string          `json:"name,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	Replicas  *int            `json:"replicas,omitempty"`
	Kind      string          `json:"kind,omitempty"`
	ClusterID string          `json:"cluster_id"`
}

func (o Operation) MarshalJSON() ([]byte, error) {
	return json.Marshal(operationWire{
		Action:    o.action,
		Manifest:  o.manifest,
		Resource:  o.resource,
		Name:      o.name,
		Namespace: o.namespace,
		Replicas:  o.replicas,
		Kind:      o.kind,
		ClusterID: o.clusterID,
	})
}

func (o *Operation) UnmarshalJSON(data []byte) error {
	var w operationWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	o.action = w.Action
	o.manifest = w.Manifest
	o.resource = w.Resource
	o.name = w.Name
	o.namespace = w.Namespace
	o.replicas = w.Replicas
	o.kind = w.Kind
	o.clusterID = w.ClusterID
	return nil
}
