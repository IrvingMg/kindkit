// Package operatortesting demonstrates using kindkit to test a real
// Kubernetes operator (cert-manager) end-to-end: create a cluster,
// load images, deploy the operator, and verify it works.
package operatortesting_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IrvingMg/kindkit"
	"github.com/IrvingMg/kindkit/test/util/docker"
)

const (
	certManagerVersion = "v1.17.2"
	certManagerURL     = "https://github.com/cert-manager/cert-manager/releases/download/" + certManagerVersion + "/cert-manager.yaml"

	controllerImage = "quay.io/jetstack/cert-manager-controller:" + certManagerVersion
	webhookImage    = "quay.io/jetstack/cert-manager-webhook:" + certManagerVersion
	cainjectorImage = "quay.io/jetstack/cert-manager-cainjector:" + certManagerVersion

	readyTimeout  = 3 * time.Minute
	cancelTimeout = 2 * time.Minute
	pollInterval  = 5 * time.Second
	httpTimeout   = 30 * time.Second
)

func TestCertManagerE2E(t *testing.T) {
	ctx := context.Background()

	t.Log("Creating Kind cluster...")
	cluster, err := kindkit.Create(ctx, "kk-certmgr-e2e",
		kindkit.WithWaitForReady(readyTimeout),
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

	images := []string{controllerImage, webhookImage, cainjectorImage}
	docker.PullImages(t, ctx, images...)

	t.Log("Loading images into cluster...")
	if err := cluster.LoadImages(ctx, images...); err != nil {
		t.Fatalf("LoadImages: %v", err)
	}

	t.Log("Installing cert-manager...")
	manifest := fetchURL(t, certManagerURL)
	if err := cluster.ApplyManifests(ctx, manifest); err != nil {
		t.Fatalf("ApplyManifests: %v", err)
	}

	restCfg, err := cluster.RESTConfig()
	if err != nil {
		t.Fatalf("RESTConfig: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := certmanagerv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add cert-manager scheme: %v", err)
	}

	k8s, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	waitForDeployments(t, ctx, k8s, "cert-manager",
		"cert-manager", "cert-manager-webhook", "cert-manager-cainjector")

	t.Log("Creating self-signed ClusterIssuer...")
	issuer := &certmanagerv1.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{Name: "selfsigned-issuer"},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}
	if err := k8s.Create(ctx, issuer); err != nil {
		t.Fatalf("create ClusterIssuer: %v", err)
	}

	t.Log("Creating Certificate...")
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cert", Namespace: "default"},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: "test-cert-tls",
			IssuerRef: cmmeta.IssuerReference{
				Name: "selfsigned-issuer",
				Kind: "ClusterIssuer",
			},
			DNSNames: []string{"example.com"},
		},
	}
	if err := k8s.Create(ctx, cert); err != nil {
		t.Fatalf("create Certificate: %v", err)
	}

	waitForSecret(t, ctx, k8s, "default", "test-cert-tls")
	t.Log("cert-manager successfully issued a certificate!")
}

func fetchURL(t *testing.T, url string) []byte {
	t.Helper()
	httpClient := &http.Client{Timeout: httpTimeout}
	resp, err := httpClient.Get(url)
	if err != nil {
		t.Fatalf("fetch %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body
}

// waitForDeployments polls until all named deployments have at least
// one available replica.
func waitForDeployments(t *testing.T, ctx context.Context, k8s client.Client, namespace string, names ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()

	for _, name := range names {
		t.Logf("Waiting for deployment %s/%s...", namespace, name)
		key := types.NamespacedName{Namespace: namespace, Name: name}
		for {
			var dep appsv1.Deployment
			err := k8s.Get(ctx, key, &dep)
			if err != nil && !errors.IsNotFound(err) {
				t.Fatalf("get deployment %s/%s: %v", namespace, name, err)
			}
			if err == nil && dep.Status.AvailableReplicas > 0 {
				t.Logf("Deployment %s/%s is ready", namespace, name)
				break
			}
			select {
			case <-ctx.Done():
				t.Fatalf("timed out waiting for deployment %s/%s", namespace, name)
			case <-time.After(pollInterval):
			}
		}
	}
}

// waitForSecret polls until the named Secret exists.
func waitForSecret(t *testing.T, ctx context.Context, k8s client.Client, namespace, name string) {
	t.Helper()
	t.Logf("Waiting for Secret %s/%s...", namespace, name)

	ctx, cancel := context.WithTimeout(ctx, cancelTimeout)
	defer cancel()

	key := types.NamespacedName{Namespace: namespace, Name: name}
	for {
		var secret corev1.Secret
		err := k8s.Get(ctx, key, &secret)
		if err == nil {
			t.Logf("Secret %s/%s exists", namespace, name)
			return
		}
		if !errors.IsNotFound(err) {
			t.Fatalf("get secret: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for Secret %s/%s", namespace, name)
		case <-time.After(pollInterval):
		}
	}
}
