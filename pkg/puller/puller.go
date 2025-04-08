package puller

import (
	"context"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// Puller represents an interface that is able to pull a directory from a remote source
type Puller interface {
	// Pull grabs the Helm chart and places it on a path in the filesystem
	Pull(ctx context.Context, rootFs, fs billy.Filesystem, path string) error
	// GetOptions returns the options used to construct this Upstream
	GetOptions() options.UpstreamOptions
	// IsWithinPackage returns whether this upstream already exists within the package
	IsWithinPackage() bool
}
