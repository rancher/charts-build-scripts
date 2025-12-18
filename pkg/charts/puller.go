package charts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// LocalPackage represents source that is a package
type LocalPackage struct {
	// Name represents the name of the package
	Name         string
	Subdirectory *string
}

// Pull grabs the package
func (u LocalPackage) Pull(ctx context.Context, rootFs, fs billy.Filesystem, path string) error {
	if strings.HasPrefix(path, u.Name) {
		return fmt.Errorf("cannot add package to itself")
	}
	pkg, err := GetPackage(ctx, rootFs, u.Name)
	if err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("could not find local package %s", u.Name)
	}
	// Check if the chart's working directory has already been prepared
	packageAlreadyPrepared, err := filesystem.PathExists(ctx, pkg.fs, pkg.Chart.WorkingDir)
	if err != nil {
		return fmt.Errorf("encountered error while checking if package %s was already prepared: %s", u.Name, err)
	}
	if packageAlreadyPrepared {
		logger.Log(ctx, slog.LevelInfo, "package already prepared", slog.String("name", u.Name))
	} else {
		if err := pkg.Prepare(ctx); err != nil {
			return err
		}
		defer pkg.Clean(ctx)
	}
	// Copy
	repositoryPackageWorkingDir, err := filesystem.GetRelativePath(rootFs, filesystem.GetAbsPath(pkg.fs, pkg.Chart.WorkingDir))
	if err != nil {
		return err
	}
	repositoryPath, err := filesystem.GetRelativePath(rootFs, filesystem.GetAbsPath(fs, path))
	if err != nil {
		return err
	}
	if err := filesystem.CopyDir(ctx, rootFs, repositoryPackageWorkingDir, repositoryPath, config.IsSoftError(ctx)); err != nil {
		return fmt.Errorf("encountered error while copying prepared package into path: %s", err)
	}
	if u.Subdirectory != nil && len(*u.Subdirectory) > 0 {
		if err := filesystem.MakeSubdirectoryRoot(ctx, fs, path, *u.Subdirectory, config.IsSoftError(ctx)); err != nil {
			return err
		}
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u LocalPackage) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: fmt.Sprintf("packages/%s", u.Name),
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u LocalPackage) IsWithinPackage() bool {
	return false
}

func (u LocalPackage) String() string {
	return fmt.Sprintf("packages/%s", u.Name)
}

// Local represents a local chart that exists within the package itself
type Local struct{}

// Pull grabs the Helm chart by preparing the package itself
func (u Local) Pull(ctx context.Context, rootFs, fs billy.Filesystem, path string) error {
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u Local) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: "local",
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u Local) IsWithinPackage() bool {
	return true
}

func (u Local) String() string {
	return "local"
}
