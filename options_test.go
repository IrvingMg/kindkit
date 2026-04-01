package kindkit

import (
	"bytes"
	"testing"
	"time"
)

const (
	readyTimeout = 3 * time.Minute
	nodeImage    = "kindest/node:v1.31.0"
	configFile   = "kind.yaml"
)

func TestApplyOptions(t *testing.T) {
	rawCfg := []byte("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\n")

	tests := []struct {
		name           string
		opts           []Option
		wantImage      string
		wantWaitFor    time.Duration
		wantRawConfig  []byte
		wantConfigFile string
	}{
		{
			name:        "defaults",
			opts:        nil,
			wantImage:   "",
			wantWaitFor: 0,
		},
		{
			name:        "node image",
			opts:        []Option{WithNodeImage(nodeImage)},
			wantImage:   nodeImage,
			wantWaitFor: 0,
		},
		{
			name:        "wait for ready",
			opts:        []Option{WithWaitForReady(readyTimeout)},
			wantImage:   "",
			wantWaitFor: readyTimeout,
		},
		{
			name: "all simple options",
			opts: []Option{
				WithNodeImage(nodeImage),
				WithWaitForReady(readyTimeout),
			},
			wantImage:   nodeImage,
			wantWaitFor: readyTimeout,
		},
		{
			name:          "raw config",
			opts:          []Option{WithRawConfig(rawCfg)},
			wantRawConfig: rawCfg,
		},
		{
			name: "raw config with other options",
			opts: []Option{
				WithRawConfig(rawCfg),
				WithNodeImage(nodeImage),
				WithWaitForReady(readyTimeout),
			},
			wantRawConfig: rawCfg,
			wantImage:     nodeImage,
			wantWaitFor:   readyTimeout,
		},
		{
			name:           "config file",
			opts:           []Option{WithConfigFile(configFile)},
			wantConfigFile: configFile,
		},
		{
			name: "config file with other options",
			opts: []Option{
				WithConfigFile(configFile),
				WithNodeImage(nodeImage),
				WithWaitForReady(readyTimeout),
			},
			wantConfigFile: configFile,
			wantImage:      nodeImage,
			wantWaitFor:    readyTimeout,
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
			if !bytes.Equal(o.rawConfig, tt.wantRawConfig) {
				t.Errorf("rawConfig = %q, want %q", o.rawConfig, tt.wantRawConfig)
			}
			if o.configFile != tt.wantConfigFile {
				t.Errorf("configFile = %q, want %q", o.configFile, tt.wantConfigFile)
			}
		})
	}
}

func TestBuildCreateOptionsErrors(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "raw config and config file conflict",
			opts: []Option{
				WithRawConfig([]byte("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\n")),
				WithConfigFile("kind.yaml"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := applyOptions(tt.opts)
			if _, err := buildCreateOptions(o); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

