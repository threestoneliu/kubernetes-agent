package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientFactory resolves a cluster id into a dynamic client and a
// discovery-backed Resolver. The production implementation
// decrypts a kubeconfig from the store and builds both clients
// from the same REST config; tests use a fake that hands out
// pre-seeded clients without touching disk or the network.
type ClientFactory interface {
	Get(ctx context.Context, clusterID string) (dynamic.Interface, error)
	// Resolver returns a per-cluster Resolver backed by the
	// cluster's discovery API. The resolver is cached and
	// shared across tool calls.
	Resolver(clusterID string) *Resolver
	// RESTConfig returns the raw REST config for a cluster.
	// Used for Table-format requests that need direct HTTP access.
	RESTConfig(clusterID string) (*rest.Config, error)
	// Invalidate drops the cached client + resolver for a cluster
	// (e.g. after the user edits the cluster config). No-op for fakes.
	Invalidate(clusterID string)
}

// clusterClients bundles the dynamic client and the Resolver that
// share the same REST config / discovery session for one cluster.
type clusterClients struct {
	dynamic   dynamic.Interface
	resolver  *Resolver
	restConfig *rest.Config
}

// KubeconfigClientFactory is the production ClientFactory: it
// lazily decrypts a cluster's kubeconfig and caches the resulting
// dynamic client + Resolver. Concurrent calls are safe.
type KubeconfigClientFactory struct {
	db    *store.DB
	aead  *crypto.AEAD
	mu    sync.Mutex
	cache map[string]*clusterClients
}

// NewKubeconfigClientFactory returns a ClientFactory backed by an
// encrypted kubeconfig store.
func NewKubeconfigClientFactory(db *store.DB, aead *crypto.AEAD) *KubeconfigClientFactory {
	return &KubeconfigClientFactory{db: db, aead: aead, cache: map[string]*clusterClients{}}
}

// NewClientFactory is a thin alias kept for backwards compatibility
// with existing callers (cmd/server, server tests) that constructed
// a *ClientFactory directly. New code should use
// NewKubeconfigClientFactory for clarity.
func NewClientFactory(db *store.DB, aead *crypto.AEAD) *KubeconfigClientFactory {
	return NewKubeconfigClientFactory(db, aead)
}

// RESTConfig returns the REST config for the given cluster.
func (f *KubeconfigClientFactory) RESTConfig(clusterID string) (*rest.Config, error) {
	cc, err := f.clients(context.Background(), clusterID)
	if err != nil {
		return nil, err
	}
	return cc.restConfig, nil
}

// Get returns a dynamic client for the given cluster, decrypting
// and parsing the kubeconfig on the first call per cluster.
func (f *KubeconfigClientFactory) Get(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	cc, err := f.clients(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	return cc.dynamic, nil
}

// Resolver returns the per-cluster discovery-backed Resolver,
// building it (and the dynamic client) on the first call.
func (f *KubeconfigClientFactory) Resolver(clusterID string) *Resolver {
	// Resolver uses no I/O of its own; if clients() fails we
	// still hand back a Resolver with a nil discovery interface
	// (which will simply fall back to WellKnownGV).
	cc, _ := f.clients(context.Background(), clusterID)
	if cc == nil {
		return NewResolver(nil)
	}
	return cc.resolver
}

// clients returns the cached dynamic + resolver pair for the given
// cluster, building it on first call. Returns an error only when
// the kubeconfig cannot be decrypted / parsed; missing discovery
// data is fine (resolver falls back).
func (f *KubeconfigClientFactory) clients(ctx context.Context, clusterID string) (*clusterClients, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cc, ok := f.cache[clusterID]; ok {
		return cc, nil
	}
	cluster, err := f.db.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	plain, err := f.aead.Decrypt(cluster.KubeconfigBlob)
	if err != nil {
		return nil, fmt.Errorf("decrypt kubeconfig: %w", err)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(plain)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		// Discovery is optional; resolver will fall back.
		disco = nil
	}
	cc := &clusterClients{dynamic: dc, resolver: NewResolver(disco), restConfig: cfg}
	f.cache[clusterID] = cc
	return cc, nil
}

// Invalidate drops the cached client + resolver for a cluster.
func (f *KubeconfigClientFactory) Invalidate(clusterID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cc, ok := f.cache[clusterID]; ok {
		cc.resolver.Invalidate()
	}
	delete(f.cache, clusterID)
}
