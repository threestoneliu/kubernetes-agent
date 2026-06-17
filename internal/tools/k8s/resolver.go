package k8s

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// Resolver maps a lowercase plural resource name to its canonical
// GroupVersionResource by querying the cluster's discovery API once
// and caching the result. Resources not present in the discovery
// payload fall back to WellKnownGV; resources unknown to both are
// returned with empty Group/Version so the dynamic client can run
// its own discovery.
//
// Resolver is safe for concurrent use and is created lazily per
// cluster by KubeconfigClientFactory.Resolver.
type Resolver struct {
	discover discovery.DiscoveryInterface

	mu     sync.RWMutex
	cache  map[string]schema.GroupVersionResource
	loaded bool // true after the first discovery fetch (success or fail)
}

// NewResolver wraps a discovery client. Pass the result of
// kubernetes.NewForConfig / clientset.Discovery() in production.
func NewResolver(d discovery.DiscoveryInterface) *Resolver {
	return &Resolver{discover: d}
}

// ResolverFromMap builds a Resolver pre-populated from a static
// resource→GVR map. Tests use this to skip the discovery round-trip
// when the fake cluster does not implement the discovery API.
func ResolverFromMap(m map[string]schema.GroupVersionResource) *Resolver {
	c := make(map[string]schema.GroupVersionResource, len(m))
	for k, v := range m {
		c[k] = v
	}
	return &Resolver{cache: c, loaded: true}
}

// Resolve returns the canonical GVR for the named resource. The
// discovery cache is populated on the first call. Subsequent calls
// hit the cache. A discovery failure marks the cache as "loaded
// (failed)" so the next call falls back to WellKnownGV without
// re-hitting the network on every request.
func (r *Resolver) Resolve(resource string) schema.GroupVersionResource {
	r.mu.RLock()
	if r.loaded {
		gvr, ok := r.cache[resource]
		r.mu.RUnlock()
		if ok {
			return gvr
		}
		return wellKnownFallback(resource)
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.loaded {
		r.refreshLocked()
	}
	if gvr, ok := r.cache[resource]; ok {
		return gvr
	}
	return wellKnownFallback(resource)
}

// refreshLocked calls the discovery API and populates the cache.
// Must be called with r.mu held in write mode.
func (r *Resolver) refreshLocked() {
	r.cache = map[string]schema.GroupVersionResource{}
	if r.discover == nil {
		r.loaded = true
		return
	}
	groups, err := r.discover.ServerPreferredResources()
	if err != nil {
		// Partial discovery (some apiservices unreachable) still
		// returns the resources it could resolve, so we don't
		// bail outright.
		if groups == nil {
			r.loaded = true
			return
		}
	}
	for _, list := range groups {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, apiRes := range list.APIResources {
			// Skip subresources like "pods/status" — we only
			// want top-level resources the agent can address.
			if len(apiRes.Name) == 0 || containsSlash(apiRes.Name) {
				continue
			}
			r.cache[apiRes.Name] = schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: apiRes.Name,
			}
		}
	}
	r.loaded = true
}

// Invalidate drops the cache so the next Resolve re-fetches. Used
// when a cluster's CRD set changes (operator install, helm chart
// with new CRDs, etc.).
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = nil
	r.loaded = false
}

// wellKnownFallback covers the small set of built-in resources
// whose discovery entries sometimes fail to surface (older
// apiservers, partial discovery) so the agent can still resolve
// them without an explicit user-side workaround.
func wellKnownFallback(resource string) schema.GroupVersionResource {
	if gv, ok := WellKnownGV[resource]; ok {
		return schema.GroupVersionResource{
			Group:    gv.Group,
			Version:  gv.Version,
			Resource: resource,
		}
	}
	return schema.GroupVersionResource{Resource: resource}
}

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}