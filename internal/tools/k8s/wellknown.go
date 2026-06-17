package k8s

import "k8s.io/apimachinery/pkg/runtime/schema"

// WellKnownGV maps a lowercase plural resource to its canonical
// Kubernetes Group/Version. The dynamic client requires a complete
// GroupVersionResource to construct a request URL — a GVR with
// empty Group and Version produces a malformed path that the
// apiserver answers with 404 ("the server could not find the
// requested resource"). We cover the resources a local user is
// most likely to ask about; anything else falls back to the empty
// GVR which lets the dynamic client run discovery.
var WellKnownGV = map[string]schema.GroupVersion{
	"pods":         {Group: "", Version: "v1"},
	"events":       {Group: "", Version: "v1"},
	"deployments":  {Group: "apps", Version: "v1"},
	"nodes":        {Group: "", Version: "v1"},
	"namespaces":   {Group: "", Version: "v1"},
	"services":     {Group: "", Version: "v1"},
	"configmaps":   {Group: "", Version: "v1"},
	"secrets":      {Group: "", Version: "v1"},
	"replicasets":  {Group: "apps", Version: "v1"},
	"statefulsets": {Group: "apps", Version: "v1"},
	"daemonsets":   {Group: "apps", Version: "v1"},
}

// resolveGVR fills in the Group/Version for a resource name when
// known. Returns the original GVR unchanged for unknown resources
// so the dynamic client can fall back to discovery.
func resolveGVR(gvr schema.GroupVersionResource) schema.GroupVersionResource {
	if gvr.Group != "" || gvr.Version != "" {
		return gvr
	}
	if gv, ok := WellKnownGV[gvr.Resource]; ok {
		gvr.Group = gv.Group
		gvr.Version = gv.Version
	}
	return gvr
}