package kindkit

import (
	"bytes"
	"testing"
	"time"
)

const defaultWaitForReady = 3 * time.Minute

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
			name: "all simple options",
			opts: []Option{
				WithNodeImage("kindest/node:v1.31.0"),
				WithWaitForReady(defaultWaitForReady),
			},
			wantImage:   "kindest/node:v1.31.0",
			wantWaitFor: defaultWaitForReady,
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
				WithNodeImage("kindest/node:v1.31.0"),
				WithWaitForReady(defaultWaitForReady),
			},
			wantRawConfig: rawCfg,
			wantImage:     "kindest/node:v1.31.0",
			wantWaitFor:   defaultWaitForReady,
		},
		{
			name:           "config file",
			opts:           []Option{WithConfigFile("kind.yaml")},
			wantConfigFile: "kind.yaml",
		},
		{
			name: "config file with other options",
			opts: []Option{
				WithConfigFile("kind.yaml"),
				WithNodeImage("kindest/node:v1.31.0"),
				WithWaitForReady(defaultWaitForReady),
			},
			wantConfigFile: "kind.yaml",
			wantImage:      "kindest/node:v1.31.0",
			wantWaitFor:    defaultWaitForReady,
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

