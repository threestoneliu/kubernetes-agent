package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientFactory resolves a cluster id into a dynamic client. The
// production implementation decrypts a kubeconfig from the store
// and builds a real client; tests use a fake that hands out
// pre-seeded clients without touching disk or the network.
type ClientFactory interface {
	Get(ctx context.Context, clusterID string) (dynamic.Interface, error)
	// Invalidate drops the cached client for a cluster (e.g. after the
	// user edits the cluster config). No-op for fakes.
	Invalidate(clusterID string)
}

// KubeconfigClientFactory is the production ClientFactory: it
// lazily decrypts a cluster's kubeconfig and caches the resulting
// dynamic client. Concurrent calls are safe.
type KubeconfigClientFactory struct {
	db    *store.DB
	aead  *crypto.AEAD
	mu    sync.Mutex
	cache map[string]dynamic.Interface
}

// NewKubeconfigClientFactory returns a ClientFactory backed by an
// encrypted kubeconfig store.
func NewKubeconfigClientFactory(db *store.DB, aead *crypto.AEAD) *KubeconfigClientFactory {
	return &KubeconfigClientFactory{db: db, aead: aead, cache: map[string]dynamic.Interface{}}
}

// NewClientFactory is a thin alias kept for backwards compatibility
// with existing callers (cmd/server, server tests) that constructed
// a *ClientFactory directly. New code should use
// NewKubeconfigClientFactory for clarity.
func NewClientFactory(db *store.DB, aead *crypto.AEAD) *KubeconfigClientFactory {
	return NewKubeconfigClientFactory(db, aead)
}

// Get returns a dynamic client for the given cluster, decrypting
// and parsing the kubeconfig on the first call per cluster.
func (f *KubeconfigClientFactory) Get(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.cache[clusterID]; ok {
		return c, nil
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
	f.cache[clusterID] = dc
	return dc, nil
}

// Invalidate drops the cached client for a cluster.
func (f *KubeconfigClientFactory) Invalidate(clusterID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, clusterID)
}
