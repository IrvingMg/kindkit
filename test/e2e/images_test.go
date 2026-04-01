//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/IrvingMg/kindkit"
	"github.com/IrvingMg/kindkit/test/util/docker"
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docker.PullImages(t, ctx, tt.pull...)

			err := c.LoadImages(ctx, tt.load...)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadImages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

