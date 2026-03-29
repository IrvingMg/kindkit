package kindkit

import (
	"testing"
	"time"
)

const defaultWaitForReady = 3 * time.Minute

func TestApplyOptions(t *testing.T) {
	tests := []struct {
		name         string
		opts         []Option
		wantImage    string
		wantWaitFor  time.Duration
	}{
		{
			name:        "defaults",
			opts:        nil,
			wantImage:   "",
			wantWaitFor: 0,
		},
		{
			name:        "node image",
			opts:        []Option{WithNodeImage("kindest/node:v1.31.0")},
			wantImage:   "kindest/node:v1.31.0",
			wantWaitFor: 0,
		},
		{
			name:        "wait for ready",
			opts:        []Option{WithWaitForReady(defaultWaitForReady)},
			wantImage:   "",
			wantWaitFor: defaultWaitForReady,
		},
		{
			name: "all options",
			opts: []Option{
				WithNodeImage("kindest/node:v1.31.0"),
				WithWaitForReady(defaultWaitForReady),
			},
			wantImage:   "kindest/node:v1.31.0",
			wantWaitFor: defaultWaitForReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := applyOptions(tt.opts)

			if o.nodeImage != tt.wantImage {
				t.Errorf("nodeImage = %q, want %q", o.nodeImage, tt.wantImage)
			}
			if o.waitForReady != tt.wantWaitFor {
				t.Errorf("waitForReady = %v, want %v", o.waitForReady, tt.wantWaitFor)
			}
		})
	}
}
