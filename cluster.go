package kindkit

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
)

const reachabilityTimeout = 5 * time.Second

// Cluster represents a Kind cluster managed by kindkit.
//
// A Cluster is obtained by calling Create. Use Delete to tear it down.
type Cluster struct {
	name     string
	provider *cluster.Provider
}

// Create creates a new Kind cluster. On partial failure, both a
// non-nil *Cluster and an error may be returned so the caller can
// still inspect or clean up. ctx is accepted for forward compatibility.
func Create(ctx context.Context, name string, opts ...Option) (*Cluster, error) {
	return create(cluster.NewProvider(), name, opts...)
}

// CreateOrReuse returns an existing cluster if its API server is
// reachable, otherwise creates a new one. Options only apply on create.
// Like Create, both a non-nil *Cluster and an error may be returned.
func CreateOrReuse(ctx context.Context, name string, opts ...Option) (*Cluster, error) {
	provider := cluster.NewProvider()

	clusters, err := provider.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	if slices.Contains(clusters, name) {
		c := &Cluster{name: name, provider: provider}
		if err := c.isReachable(); err != nil {
			return c, fmt.Errorf("cluster %q exists but is not reachable: %w", name, err)
		}
		return c, nil
	}

	return create(provider, name, opts...)
}

func create(provider *cluster.Provider, name string, opts ...Option) (*Cluster, error) {
	o := applyOptions(opts)
	copts, err := buildCreateOptions(o)
	if err != nil {
		return nil, err
	}

	c := &Cluster{name: name, provider: provider}

	if err := provider.Create(name, copts...); err != nil {
		clusters, listErr := provider.List()
		if listErr == nil && slices.Contains(clusters, name) {
			return c, fmt.Errorf("cluster %q was created but is not ready: %w", name, err)
		}
		return nil, fmt.Errorf("failed to create cluster %q: %w", name, err)
	}

	return c, nil
}

func (c *Cluster) Name() string {
	return c.name
}

// RESTConfig returns a *rest.Config for the cluster.
func (c *Cluster) RESTConfig() (*rest.Config, error) {
	kubeconfig, err := c.provider.KubeConfig(c.name, false)
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig for cluster %q: %w", c.name, err)
	}
	return clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
}

// KubeconfigPath writes the kubeconfig to a temporary file and returns
// its path. The caller is responsible for removing the file.
func (c *Cluster) KubeconfigPath() (string, error) {
	kubeconfig, err := c.provider.KubeConfig(c.name, false)
	if err != nil {
		return "", fmt.Errorf("get kubeconfig for cluster %q: %w", c.name, err)
	}
	f, err := os.CreateTemp("", "kindkit-kubeconfig-*")
	if err != nil {
		return "", fmt.Errorf("create temp kubeconfig file: %w", err)
	}
	if _, err := f.WriteString(kubeconfig); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write kubeconfig: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("close kubeconfig file: %w", err)
	}
	return f.Name(), nil
}

func (c *Cluster) isReachable() error {
	cfg, err := c.RESTConfig()
	if err != nil {
		return fmt.Errorf("get REST config: %w", err)
	}
	cfg.Timeout = reachabilityTimeout

	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}

	if _, err := disc.ServerVersion(); err != nil {
		return fmt.Errorf("API server health check: %w", err)
	}
	return nil
}

// Delete deletes the cluster. It is safe to call on an already-deleted
// cluster. ctx is accepted for forward compatibility.
func (c *Cluster) Delete(ctx context.Context) error {
	clusters, err := c.provider.List()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}
	if !slices.Contains(clusters, c.name) {
		return nil
	}
	if err := c.provider.Delete(c.name, ""); err != nil {
		return fmt.Errorf("failed to delete cluster %q: %w", c.name, err)
	}
	return nil
}

// ExportLogs exports cluster logs to the given directory for debugging.
// ctx is accepted for forward compatibility.
func (c *Cluster) ExportLogs(ctx context.Context, dir string) error {
	if err := c.provider.CollectLogs(c.name, dir); err != nil {
		return fmt.Errorf("failed to export logs for cluster %q: %w", c.name, err)
	}
	return nil
}
