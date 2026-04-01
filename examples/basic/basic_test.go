// Package basic demonstrates the core kindkit API: create a cluster,
// verify it works, load an image, and tear it down.
package basic_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/IrvingMg/kindkit"
	"github.com/IrvingMg/kindkit/test/util/docker"
)

const busyboxImage = "busybox:1.37"

func TestClusterLifecycle(t *testing.T) {
	ctx := context.Background()

	t.Log("Creating Kind cluster...")
	cluster, err := kindkit.Create(ctx, "kk-basic-e2e",
		kindkit.WithWaitForReady(3*time.Minute),
	)
	if err != nil {
		// Partial failure: creation failed but a cluster was returned.
		// Export logs for debugging, then clean up.
		if cluster != nil {
			if logErr := cluster.ExportLogs(ctx, "./test-logs"); logErr != nil {
				t.Logf("export logs: %v", logErr)
			}
			if delErr := cluster.Delete(ctx); delErr != nil {
				t.Logf("cleanup: %v", delErr)
			}
		}
		t.Fatalf("kindkit.Create: %v", err)
	}
	defer func() {
		t.Log("Deleting Kind cluster...")
		if err := cluster.Delete(ctx); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}()

	restCfg, err := cluster.RESTConfig()
	if err != nil {
		t.Fatalf("RESTConfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		t.Fatalf("create clientset: %v", err)
	}

	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list namespaces: %v", err)
	}
	t.Logf("Cluster has %d namespaces", len(namespaces.Items))

	docker.PullImages(t, ctx, busyboxImage)

	t.Log("Loading images into cluster...")
	if err := cluster.LoadImages(ctx, busyboxImage); err != nil {
		t.Fatalf("LoadImages: %v", err)
	}
}
