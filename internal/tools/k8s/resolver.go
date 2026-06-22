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
// Resolver also caches whether each resource is namespaced or
// cluster-scoped (from APIResource.Namespaced), so callers can
// construct correct API paths without hard-coding knowledge of
// which resources are cluster-scoped.
//
// Resolver is safe for concurrent use and is created lazily per
// cluster by KubeconfigClientFactory.Resolver.
type Resolver struct {
	discover discovery.DiscoveryInterface

	mu         sync.RWMutex
	cache      map[string]schema.GroupVersionResource
	namespaced map[string]bool // true = namespaced, false = cluster-scoped
	loaded     bool            // true after the first discovery fetch (success or fail)
}

// NewResolver wraps a discovery client. Pass the result of
// kubernetes.NewForConfig / clientset.Discovery() in production.
func NewResolver(d discovery.DiscoveryInterface) *Resolver {
	return &Resolver{discover: d}
}

// ResolverFromMap builds a Resolver pre-populated from a static
// resource→GVR map. Tests use this to skip the discovery round-trip
// when the fake cluster does not implement the discovery API.
// The namespaced map is left nil, meaning all resources fall through
// to wellKnownFallback which does not track namespaced info.
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

// IsNamespaced returns whether the resource is namespace-scoped (true)
// or cluster-scoped (false). It queries discovery on first use and caches
// the result. For resources not found in discovery, it falls back to
// WellKnownGV; if still unknown, it returns true (assumes namespaced for
// safety — cluster-scoped resources that aren't found will 404, which is
// the same failure mode as before).
func (r *Resolver) IsNamespaced(resource string) bool {
	r.mu.RLock()
	if r.loaded {
		if ns, ok := r.namespaced[resource]; ok {
			r.mu.RUnlock()
			return ns
		}
		r.mu.RUnlock()
		return wellKnownNamespacedFallback(resource)
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.loaded {
		r.refreshLocked()
	}
	if ns, ok := r.namespaced[resource]; ok {
		return ns
	}
	return wellKnownNamespacedFallback(resource)
}

// refreshLocked calls the discovery API and populates the cache.
// Must be called with r.mu held in write mode.
func (r *Resolver) refreshLocked() {
	r.cache = map[string]schema.GroupVersionResource{}
	r.namespaced = map[string]bool{}
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
			r.namespaced[apiRes.Name] = apiRes.Namespaced
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
	r.namespaced = nil
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

// wellKnownNamespacedFallback mirrors wellKnownFallback but returns
// whether the resource is namespace-scoped. If the resource is unknown
// we return true (assume namespaced) since cluster-scoped resources
// that aren't found will 404 anyway.
func wellKnownNamespacedFallback(resource string) bool {
	switch resource {
	case "nodes", "namespaces", "persistentvolumes", "storageclasses",
		"componentstatuses", "events":
		return false
	default:
		return true
	}
}

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}