//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IrvingMg/kindkit"
)

const defaultWaitForReady = 3 * time.Minute

func TestCreate(t *testing.T) {
	tests := []struct {
		name string
		opts []kindkit.Option
	}{
		{
			name: "defaults",
		},
		{
			name: "with node image",
			opts: []kindkit.Option{
				kindkit.WithNodeImage("kindest/node:v1.32.0"),
				kindkit.WithWaitForReady(defaultWaitForReady),
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := clusterName(t)
			c, err := kindkit.Create(ctx, name, tt.opts...)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			defer func() {
				if err := c.Delete(ctx); err != nil {
					t.Logf("cleanup: %v", err)
				}
			}()

			if c.Name() != name {
				t.Errorf("Name() = %q, want %q", c.Name(), name)
			}
		})
	}
}

func TestCreatePartialFailure(t *testing.T) {
	ctx := context.Background()

	c, err := kindkit.Create(ctx, clusterName(t),
		kindkit.WithNodeImage("kindest/node:v0.0.0-does-not-exist"),
	)
	if err == nil {
		if delErr := c.Delete(ctx); delErr != nil {
			t.Logf("cleanup: %v", delErr)
		}
		t.Fatal("expected error with invalid node image, got nil")
	}

	if c != nil {
		t.Logf("partial cluster returned with error: %v", err)
		if err := c.Delete(ctx); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}
}

func TestDeleteIdempotent(t *testing.T) {
	ctx := context.Background()

	c, err := kindkit.Create(ctx, clusterName(t))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := c.Delete(ctx); err != nil {
		t.Fatalf("first Delete: %v", err)
	}

	if err := c.Delete(ctx); err != nil {
		t.Errorf("second Delete: %v", err)
	}
}

func TestClusterConfig(t *testing.T) {
	ctx := context.Background()

	c, err := kindkit.Create(ctx, clusterName(t))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() {
		if err := c.Delete(ctx); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}()

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "RESTConfig returns config with host",
			check: func(t *testing.T) {
				cfg, err := c.RESTConfig()
				if err != nil {
					t.Fatalf("RESTConfig: %v", err)
				}
				if cfg.Host == "" {
					t.Error("RESTConfig returned config with empty Host")
				}
			},
		},
		{
			name: "KubeconfigPath returns non-empty file",
			check: func(t *testing.T) {
				path, err := c.KubeconfigPath()
				if err != nil {
					t.Fatalf("KubeconfigPath: %v", err)
				}
				defer os.Remove(path)

				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("stat kubeconfig file: %v", err)
				}
				if info.Size() == 0 {
					t.Error("kubeconfig file is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

func clusterName(t *testing.T) string {
	name := strings.ToLower(t.Name())
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return fmt.Sprintf("kk-%s-%d", name, os.Getpid())
}
