package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TestListTable_DockerDesktop is an integration test against the user's
// real kubeconfig (~/.kube/config). It verifies that ListTable returns
// a Namespace column when listing pods across all namespaces.
func TestListTable_DockerDesktop(t *testing.T) {
	home := os.Getenv("HOME")
	kubeconfig := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Skip("~/.kube/config not found, skipping integration test")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)

	// Create a resolver backed by real discovery.
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	require.NoError(t, err)
	resolver := NewResolver(disco)

	// Verify IsNamespaced returns true for pods.
	require.True(t, resolver.IsNamespaced("pods"), "pods should be namespaced")

	// Build REST config wrapper.
	factory := &liveFactory{cfg: cfg, resolver: resolver}

	// List all pods across all namespaces.
	in := ListInput{Resource: "pods", ClusterID: ""}
	out, err := ListTable(context.Background(), factory, in)
	require.NoError(t, err)

	fmt.Printf("Columns: %v\n", out.Columns)
	fmt.Printf("Row count: %d\n", len(out.Rows))
	if len(out.Rows) > 0 {
		fmt.Printf("First row: %v\n", out.Rows[0])
	}

	// Verify Namespace column exists.
	hasNS := false
	for _, col := range out.Columns {
		if col == "Namespace" {
			hasNS = true
			break
		}
	}
	require.True(t, hasNS, "Namespace column missing from Table output. Columns: %v", out.Columns)
}

// liveFactory is a minimal ClientFactory for integration testing against
// a real cluster using the provided REST config.
type liveFactory struct {
	cfg      *rest.Config
	resolver *Resolver
}

func (f *liveFactory) Get(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	// Not needed for ListTable.
	return nil, nil
}

func (f *liveFactory) Resolver(clusterID string) *Resolver {
	return f.resolver
}

func (f *liveFactory) RESTConfig(clusterID string) (*rest.Config, error) {
	return f.cfg, nil
}

func (f *liveFactory) Invalidate(clusterID string) {}
