package docker

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// PullImages pulls the given images to the local Docker daemon.
func PullImages(t *testing.T, ctx context.Context, images ...string) {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}
	defer cli.Close() //nolint:errcheck // best-effort cleanup; no data to flush

	for _, img := range images {
		t.Logf("Pulling %s...", img)
		rc, err := cli.ImagePull(ctx, img, image.PullOptions{})
		if err != nil {
			t.Fatalf("pull %s: %v", img, err)
		}
		_, copyErr := io.Copy(io.Discard, rc)
		rc.Close() //nolint:errcheck // best-effort cleanup; data already consumed
		if copyErr != nil {
			t.Fatalf("pull %s: reading response: %v", img, copyErr)
		}
	}
}
