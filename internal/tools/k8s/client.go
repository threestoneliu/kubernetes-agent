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

// ClientFactory lazily decrypts a cluster's kubeconfig and caches the
// resulting dynamic client. Concurrent calls are safe.
type ClientFactory struct {
	db    *store.DB
	aead  *crypto.AEAD
	mu    sync.Mutex
	cache map[string]*dynamic.DynamicClient
}

func NewClientFactory(db *store.DB, aead *crypto.AEAD) *ClientFactory {
	return &ClientFactory{db: db, aead: aead, cache: map[string]*dynamic.DynamicClient{}}
}

// Get returns a DynamicClient for the given cluster, decrypting and
// parsing the kubeconfig on the first call per cluster.
func (f *ClientFactory) Get(ctx context.Context, clusterID string) (*dynamic.DynamicClient, error) {
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

// Invalidate drops the cached client for a cluster (e.g. after the user
// edits the cluster config).
func (f *ClientFactory) Invalidate(clusterID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, clusterID)
}