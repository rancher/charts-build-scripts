package charts

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/change"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/sirupsen/logrus"
)

// Chart represents the main chart in a given package
type Chart struct {
	// Upstream represents where the chart is sourced from
	Upstream puller.Puller `yaml:"upstream"`
	// WorkingDir represents the working directory of this chart
	WorkingDir string `yaml:"workingDir" default:"charts"`
	// IgnoreDependencies drops certain dependencies from the list that is parsed from upstream
	IgnoreDependencies []string `yaml:"ignoreDependencies"`
	// ReplacePaths marks paths as those that should be replaced instead of patches. Consequently, these paths will exist in both generated-changes/excludes and generated-changes/overlay
	ReplacePaths []string `yaml:"replacePaths"`

	// The version of this chart in Upstream. This value is set to a non-nil value on Prepare.
	// GenerateChart will fail if this value is not set (e.g. chart must be prepared first)
	// If there is no upstream, this will be set to ""
	upstreamChartVersion *string
}

// Prepare pulls in a package based on the spec to the local git repository
func (c *Chart) Prepare(rootFs, pkgFs billy.Filesystem) error {
	upstreamChartVersion := ""
	defer func() { c.upstreamChartVersion = &upstreamChartVersion }()
	if c.Upstream.IsWithinPackage() {
		logrus.Infof("Local chart does not need to be prepared")
		// Ensure local charts standardize the Chart.yaml on prepare
		if err := helm.StandardizeChartYaml(pkgFs, c.WorkingDir); err != nil {
			return err
		}
		if err := PrepareDependencies(rootFs, pkgFs, c.WorkingDir, c.GeneratedChangesRootDir(), c.IgnoreDependencies); err != nil {
			return fmt.Errorf("encountered error while trying to prepare dependencies in %s: %s", c.WorkingDir, err)
		}
		return nil
	}
	if err := filesystem.RemoveAll(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to clean up %s before preparing: %s", c.WorkingDir, err)
	}
	if err := c.Upstream.Pull(rootFs, pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to pull upstream into %s: %s", c.WorkingDir, err)
	}
	// If the upstream is not already a Helm chart, convert it into a dummy Helm chart by moving YAML files to templates and creating a dummy Chart.yaml
	// If the upstream is already a Helm chart, this will standardize the Chart.yaml
	if err := helm.ConvertToHelmChart(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to convert upstream at %s into a Helm chart: %s", c.WorkingDir, err)
	}
	var err error
	upstreamChartVersion, err = helm.GetHelmMetadataVersion(pkgFs, c.WorkingDir)
	if err != nil {
		return fmt.Errorf("encountered error while parsing original chart's version in %s: %s", c.WorkingDir, err)
	}
	if err := PrepareDependencies(rootFs, pkgFs, c.WorkingDir, c.GeneratedChangesRootDir(), c.IgnoreDependencies); err != nil {
		return fmt.Errorf("encountered error while trying to prepare dependencies in %s: %s", c.WorkingDir, err)
	}
	if err := change.ApplyChanges(pkgFs, c.WorkingDir, c.GeneratedChangesRootDir()); err != nil {
		return fmt.Errorf("encountered error while trying to apply changes to %s: %s", c.WorkingDir, err)
	}
	return nil
}

// GeneratePatch generates a patch on a forked Helm chart based on local changes
func (c *Chart) GeneratePatch(rootFs, pkgFs billy.Filesystem) error {
	if c.Upstream.IsWithinPackage() {
		logrus.Infof("Local chart does not need to be patched")
		return nil
	}
	if exists, err := filesystem.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while checking if %s exist: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("working directory %s has not been prepared yet", c.WorkingDir)
	}
	// Standardize the local copy of the Chart.yaml before trying to compare the patch
	if err := helm.StandardizeChartYaml(pkgFs, c.WorkingDir); err != nil {
		return err
	}
	if err := c.Upstream.Pull(rootFs, pkgFs, c.OriginalDir()); err != nil {
		return fmt.Errorf("encountered error while trying to pull upstream into %s: %s", c.OriginalDir(), err)
	}
	// If the upstream is not already a Helm chart, convert it into a dummy Helm chart by moving YAML files to templates and creating a dummy Chart.yaml
	if err := helm.ConvertToHelmChart(pkgFs, c.OriginalDir()); err != nil {
		return fmt.Errorf("encountered error while trying to convert upstream at %s into a Helm chart: %s", c.OriginalDir(), err)
	}
	if err := PrepareDependencies(rootFs, pkgFs, c.OriginalDir(), c.GeneratedChangesRootDir(), c.IgnoreDependencies); err != nil {
		return fmt.Errorf("encountered error while trying to prepare dependencies in %s: %s", c.OriginalDir(), err)
	}
	defer filesystem.RemoveAll(pkgFs, c.OriginalDir())
	if err := change.GenerateChanges(pkgFs, c.OriginalDir(), c.WorkingDir, c.GeneratedChangesRootDir(), c.ReplacePaths); err != nil {
		return fmt.Errorf("encountered error while generating changes from %s to %s and placing it in %s: %s", c.OriginalDir(), c.WorkingDir, c.GeneratedChangesRootDir(), err)
	}
	return nil
}

// GenerateChart generates the chart and stores it in the assets and charts directory
func (c *Chart) GenerateChart(rootFs, pkgFs billy.Filesystem, packageVersion *int, version *semver.Version, omitBuildMetadataOnExport bool) error {
	if c.upstreamChartVersion == nil {
		return fmt.Errorf("cannot generate chart since it has never been prepared: upstreamChartVersion is not set")
	}
	if err := helm.ExportHelmChart(rootFs, pkgFs, c.WorkingDir, packageVersion, version, *c.upstreamChartVersion, omitBuildMetadataOnExport); err != nil {
		return fmt.Errorf("encountered error while trying to export Helm chart for %s: %s", c.WorkingDir, err)
	}
	return nil
}

// OriginalDir returns a working directory where we can place the original chart from upstream
func (c *Chart) OriginalDir() string {
	return fmt.Sprintf("%s-original", c.WorkingDir)
}

// GeneratedChangesRootDir stored the directory rooted at the package level where generated changes for this chart can be found
func (c *Chart) GeneratedChangesRootDir() string {
	return path.GeneratedChangesDir
}
