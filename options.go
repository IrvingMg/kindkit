package kindkit

import (
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
)

type Option func(*options)

type options struct {
	nodeImage    string
	waitForReady time.Duration
}

func WithNodeImage(image string) Option {
	return func(o *options) {
		o.nodeImage = image
	}
}

func WithWaitForReady(d time.Duration) Option {
	return func(o *options) {
		o.waitForReady = d
	}
}

func applyOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func buildCreateOptions(o options) []cluster.CreateOption {
	var copts []cluster.CreateOption

	copts = append(copts,
		cluster.CreateWithDisplayUsage(false),
		cluster.CreateWithDisplaySalutation(false),
	)

	if o.nodeImage != "" {
		copts = append(copts, cluster.CreateWithNodeImage(o.nodeImage))
	}

	if o.waitForReady > 0 {
		copts = append(copts, cluster.CreateWithWaitForReady(o.waitForReady))
	}

	return copts
}
