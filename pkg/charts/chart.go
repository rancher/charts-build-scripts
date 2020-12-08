package charts

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
)

const (
	// GeneratedChangesDirpath is a directory that contains GeneratedChanges
	GeneratedChangesDirpath = "generated-changes"
	// GeneratedChangesPatchDirpath is a directory that contains patches within GeneratedChangesDirpath
	GeneratedChangesPatchDirpath = "patch"
	// GeneratedChangesOverlayDirpath is a directory that contains overlays within GeneratedChangesDirpath
	GeneratedChangesOverlayDirpath = "overlay"
	// GeneratedChangesExcludeDirpath is a directory that contains excludes within GeneratedChangesDirpath
	GeneratedChangesExcludeDirpath = "exclude"
	// GeneratedChangesDependenciesDirpath is a directory that contains dependencies within GeneratedChangesDirpath
	GeneratedChangesDependenciesDirpath = "dependencies"

	// DependencyOptionsFilepath is a file that contains information about how to prepare your dependency
	// The expected structure of this file is one that can be marshalled into a ChartOptions struct
	DependencyOptionsFilepath = "dependency.yaml"

	patchFmt = "%s.patch"
)

// Chart represents the main chart in a given package
type Chart struct {
	// Upstream represents any options that are configurable for upstream charts
	Upstream Upstream `yaml:"upstream"`
	// WorkingDir represents the working directory of this chart
	WorkingDir string `yaml:"workingDir" default:"charts"`
}

// Prepare pulls in a package based on the spec to the local git repository
func (c *Chart) Prepare(pkgFs billy.Filesystem) error {
	if err := utils.RemoveAll(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to clean up %s before preparing: %s", c.WorkingDir, err)
	}
	if err := c.Upstream.Pull(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to pull upstream into %s: %s", c.WorkingDir, err)
	}
	if err := PrepareDependencies(pkgFs, c.WorkingDir, c.GeneratedChangesRootDir()); err != nil {
		return fmt.Errorf("Encountered error while trying to prepare dependencies in %s: %s", c.WorkingDir, err)
	}
	if err := ApplyChanges(pkgFs, c.WorkingDir, c.GeneratedChangesRootDir()); err != nil {
		return fmt.Errorf("Encountered error while trying to apply changes to %s: %s", c.WorkingDir, err)
	}
	return nil
}

// GeneratePatch generates a patch on a forked Helm chart based on local changes
func (c *Chart) GeneratePatch(pkgFs billy.Filesystem) error {
	if exists, err := utils.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to clean up %s before preparing: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("Working directory %s has not been prepared yet", c.WorkingDir)
	}
	if err := c.Upstream.Pull(pkgFs, c.OriginalDir()); err != nil {
		return fmt.Errorf("Encountered error while trying to pull upstream into %s: %s", c.OriginalDir(), err)
	}
	if err := PrepareDependencies(pkgFs, c.OriginalDir(), c.GeneratedChangesRootDir()); err != nil {
		return fmt.Errorf("Encountered error while trying to prepare dependencies in %s: %s", c.OriginalDir(), err)
	}
	defer utils.RemoveAll(pkgFs, c.OriginalDir())
	if err := GenerateChanges(pkgFs, c.OriginalDir(), c.WorkingDir, c.GeneratedChangesRootDir()); err != nil {
		return fmt.Errorf("Encountered error while generating changes from %s to %s and placing it in %s: %s", c.OriginalDir(), c.WorkingDir, c.GeneratedChangesRootDir(), err)
	}
	return nil
}

// GenerateChart generates the chart and stores it in the assets and charts directory
func (c *Chart) GenerateChart(pkgFs billy.Filesystem, packageVersion int, repoFs billy.Filesystem, packageAssetsDirpath, packageChartsDirpath string, opts options.ExportOptions) error {
	if err := ExportHelmChart(pkgFs, c.WorkingDir, packageVersion, repoFs, packageAssetsDirpath, packageChartsDirpath, opts); err != nil {
		return fmt.Errorf("Encountered error while trying to export Helm chart for %s: %s", c.WorkingDir, err)
	}
	return nil
}

// OriginalDir returns a working directory where we can place the original chart from upstream
func (c *Chart) OriginalDir() string {
	return fmt.Sprintf("%s-original", c.WorkingDir)
}

// GeneratedChangesRootDir stored the directory rooted at the package level where generated changes for this chart can be found
func (c *Chart) GeneratedChangesRootDir() string {
	return GeneratedChangesDirpath
}
