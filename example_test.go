package kindkit_test

import (
	"context"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/IrvingMg/kindkit"
)

// ExampleCreate shows the full cluster lifecycle, including the
// partial-failure contract: on error, the returned *Cluster may
// still be non-nil and usable for log export and cleanup.
func ExampleCreate() {
	ctx := context.Background()

	cluster, err := kindkit.Create(ctx, "my-cluster",
		kindkit.WithWaitForReady(3*time.Minute),
	)
	if err != nil {
		if cluster != nil {
			_ = cluster.ExportLogs(ctx, "./logs")
			_ = cluster.Delete(ctx)
		}
		log.Fatal(err)
	}
	defer func() { _ = cluster.Delete(ctx) }()

	// Use cluster.RESTConfig() for client-go, cluster.KubeconfigPath()
	// for kubectl, etc.
}

// ExampleCluster_LoadImages loads locally-built images into every
// cluster node so Pods can reference them without a registry.
func ExampleCluster_LoadImages() {
	ctx := context.Background()

	cluster, err := kindkit.Create(ctx, "my-cluster",
		kindkit.WithWaitForReady(3*time.Minute),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = cluster.Delete(ctx) }()

	if err := cluster.LoadImages(ctx, "my-app:latest", "my-sidecar:latest"); err != nil {
		log.Fatal(err)
	}
}

// ExampleCluster_KubeconfigPath writes the kubeconfig to a temporary
// file so external tools like kubectl or helm can be invoked against
// the cluster.
func ExampleCluster_KubeconfigPath() {
	ctx := context.Background()

	cluster, err := kindkit.Create(ctx, "my-cluster",
		kindkit.WithWaitForReady(3*time.Minute),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = cluster.Delete(ctx) }()

	path, err := cluster.KubeconfigPath()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.Remove(path) }()

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", path, "get", "nodes")
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

// ExampleCluster_ApplyManifests applies a Kubernetes manifest to the
// cluster using server-side apply.
func ExampleCluster_ApplyManifests() {
	ctx := context.Background()

	cluster, err := kindkit.Create(ctx, "my-cluster",
		kindkit.WithWaitForReady(3*time.Minute),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = cluster.Delete(ctx) }()

	manifest := []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: demo
`)
	if err := cluster.ApplyManifests(ctx, manifest); err != nil {
		log.Fatal(err)
	}
}
