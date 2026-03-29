//go:build e2e

package e2e_test

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/IrvingMg/kindkit"
)

func TestLoadImages(t *testing.T) {
	tests := []struct {
		name    string
		pull    []string
		load    []string
		wantErr bool
	}{
		{
			name: "existing image",
			pull: []string{"busybox:latest"},
			load: []string{"busybox:latest"},
		},
		{
			name:    "non-existent image",
			load:    []string{"does-not-exist:99.99.99"},
			wantErr: true,
		},
	}

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, img := range tt.pull {
				pullImage(t, ctx, img)
			}

			err := c.LoadImages(ctx, tt.load...)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadImages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func pullImage(t *testing.T, ctx context.Context, ref string) {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}
	defer cli.Close()

	rc, err := cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		t.Fatalf("pull %s: %v", ref, err)
	}
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("pull %s: reading response: %v", ref, err)
	}
}
