//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/IrvingMg/kindkit"
)

const (
	readyTimeout     = 3 * time.Minute
	defaultNodeImage = "kindest/node:v1.35.0"
	invalidNodeImage = "kindest/node:v0.0.0-does-not-exist"
)

var nodeImage = envOrDefault("KINDKIT_TEST_NODE_IMAGE", defaultNodeImage)

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
				kindkit.WithNodeImage(nodeImage),
				kindkit.WithWaitForReady(readyTimeout),
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := clusterName(t)
			c, err := kindkit.Create(ctx, name, tt.opts...)
			if err != nil {
				if c != nil {
					if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
						t.Logf("export logs: %v", logErr)
					}
					if delErr := c.Delete(ctx); delErr != nil {
						t.Logf("cleanup: %v", delErr)
					}
				}
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
		kindkit.WithNodeImage(invalidNodeImage),
	)
	if err == nil {
		if delErr := c.Delete(ctx); delErr != nil {
			t.Logf("cleanup: %v", delErr)
		}
		t.Fatal("expected error with invalid node image, got nil")
	}

	if c != nil {
		t.Logf("partial cluster returned with error: %v", err)
		if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
			t.Logf("export logs: %v", logErr)
		}
		if err := c.Delete(ctx); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}
}

func TestDeleteIdempotent(t *testing.T) {
	ctx := context.Background()

	c, err := kindkit.Create(ctx, clusterName(t))
	if err != nil {
		if c != nil {
			if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
				t.Logf("export logs: %v", logErr)
			}
			if delErr := c.Delete(ctx); delErr != nil {
				t.Logf("cleanup: %v", delErr)
			}
		}
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
		if c != nil {
			if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
				t.Logf("export logs: %v", logErr)
			}
			if delErr := c.Delete(ctx); delErr != nil {
				t.Logf("cleanup: %v", delErr)
			}
		}
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
			name: "RESTConfig can list namespaces",
			check: func(t *testing.T) {
				cfg, err := c.RESTConfig()
				if err != nil {
					t.Fatalf("RESTConfig: %v", err)
				}
				clientset, err := kubernetes.NewForConfig(cfg)
				if err != nil {
					t.Fatalf("create clientset: %v", err)
				}
				ns, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
				if err != nil {
					t.Fatalf("list namespaces: %v", err)
				}
				if len(ns.Items) == 0 {
					t.Error("expected at least one namespace")
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

func TestCreateOrReuse(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		precreate bool
	}{
		{
			name:      "new cluster",
			precreate: false,
		},
		{
			name:      "existing cluster",
			precreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := clusterName(t)

			if tt.precreate {
				c, err := kindkit.Create(ctx, name)
				if err != nil {
					if c != nil {
						if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
							t.Logf("export logs: %v", logErr)
						}
						if delErr := c.Delete(ctx); delErr != nil {
							t.Logf("cleanup: %v", delErr)
						}
					}
					t.Fatalf("Create: %v", err)
				}
				defer func() {
					if err := c.Delete(ctx); err != nil {
						t.Logf("cleanup: %v", err)
					}
				}()
			}

			c, err := kindkit.CreateOrReuse(ctx, name)
			if err != nil {
				if c != nil {
					if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
						t.Logf("export logs: %v", logErr)
					}
					if delErr := c.Delete(ctx); delErr != nil {
						t.Logf("cleanup: %v", delErr)
					}
				}
				t.Fatalf("CreateOrReuse: %v", err)
			}
			defer func() {
				if err := c.Delete(ctx); err != nil {
					t.Logf("cleanup: %v", err)
				}
			}()

			if c.Name() != name {
				t.Errorf("Name() = %q, want %q", c.Name(), name)
			}

			cfg, err := c.RESTConfig()
			if err != nil {
				t.Fatalf("RESTConfig: %v", err)
			}
			if cfg.Host == "" {
				t.Error("RESTConfig returned config with empty Host")
			}
		})
	}
}

func TestExportLogs(t *testing.T) {
	ctx := context.Background()

	c, err := kindkit.Create(ctx, clusterName(t))
	if err != nil {
		if c != nil {
			if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
				t.Logf("export logs: %v", logErr)
			}
			if delErr := c.Delete(ctx); delErr != nil {
				t.Logf("cleanup: %v", delErr)
			}
		}
		t.Fatalf("Create: %v", err)
	}
	defer func() {
		if err := c.Delete(ctx); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}()

	dir := t.TempDir()
	if err := c.ExportLogs(ctx, dir); err != nil {
		t.Fatalf("ExportLogs: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected exported logs in directory, got none")
	}
}

func TestCreateWithRawConfig(t *testing.T) {
	tests := []struct {
		name      string
		raw       []byte
		wantNodes int
		check     func(t *testing.T, nodes []corev1.Node)
	}{
		{
			name: "single-cp",
			raw: []byte(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
`),
			wantNodes: 1,
		},
		{
			name: "workers-labels",
			raw: []byte(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  labels:
    kindkit.test/pool: workers
    kindkit.test/index: "0"
- role: worker
  labels:
    kindkit.test/pool: workers
    kindkit.test/index: "1"
`),
			wantNodes: 3,
			check: func(t *testing.T, nodes []corev1.Node) {
				var workers int
				for _, node := range nodes {
					pool := node.Labels["kindkit.test/pool"]
					idx := node.Labels["kindkit.test/index"]
					if pool == "" {
						continue
					}
					if pool != "workers" {
						t.Errorf("node %s: label kindkit.test/pool = %q, want %q", node.Name, pool, "workers")
					}
					if idx != "0" && idx != "1" {
						t.Errorf("node %s: label kindkit.test/index = %q, want \"0\" or \"1\"", node.Name, idx)
					}
					workers++
				}
				if workers != 2 {
					t.Errorf("expected 2 labeled workers, got %d", workers)
				}
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := clusterName(t)
			c, err := kindkit.Create(ctx, name,
				kindkit.WithRawConfig(tt.raw),
				kindkit.WithWaitForReady(readyTimeout),
			)
			if err != nil {
				if c != nil {
					if logErr := c.ExportLogs(ctx, t.TempDir()); logErr != nil {
						t.Logf("export logs: %v", logErr)
					}
					if delErr := c.Delete(ctx); delErr != nil {
						t.Logf("cleanup: %v", delErr)
					}
				}
				t.Fatalf("Create: %v", err)
			}
			defer func() {
				if err := c.Delete(ctx); err != nil {
					t.Logf("cleanup: %v", err)
				}
			}()

			cfg, err := c.RESTConfig()
			if err != nil {
				t.Fatalf("RESTConfig: %v", err)
			}
			clientset, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				t.Fatalf("create clientset: %v", err)
			}

			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("list nodes: %v", err)
			}
			if len(nodes.Items) != tt.wantNodes {
				t.Fatalf("expected %d nodes, got %d", tt.wantNodes, len(nodes.Items))
			}

			if tt.check != nil {
				tt.check(t, nodes.Items)
			}
		})
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func clusterName(t *testing.T) string {
	name := strings.ToLower(t.Name())
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return fmt.Sprintf("kk-%s-%d", name, os.Getpid())
}
