package kindkit

import (
	"context"
	"fmt"
	"slices"

	"sigs.k8s.io/kind/pkg/cluster"
)

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
	o := applyOptions(opts)
	copts := buildCreateOptions(o)

	provider := cluster.NewProvider()

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
