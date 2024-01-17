package charts

import (
	"fmt"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/change"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/sirupsen/logrus"
)

// AdditionalChart represents any additional charts packaged along with the main chart in a package
type AdditionalChart struct {
	// WorkingDir represents the working directory of this chart
	WorkingDir string `yaml:"workingDir" default:"charts"`
	// Upstream represents any options that are configurable for upstream charts
	Upstream *puller.Puller `yaml:"upstream"`
	// CRDChartOptions represents any options that are configurable for CRD charts
	CRDChartOptions *options.CRDChartOptions `yaml:"crdChart"`
	// IgnoreDependencies drops certain dependencies from the list that is parsed from upstream
	IgnoreDependencies []string `yaml:"ignoreDependencies"`
	// ReplacePaths marks paths as those that should be replaced instead of patches. Consequently, these paths will exist in both generated-changes/excludes and generated-changes/overlay
	ReplacePaths []string `yaml:"replacePaths"`

	// The version of this chart in Upstream. This value is set to a non-nil value on Prepare.
	// GenerateChart will fail if this value is not set (e.g. chart must be prepared first)
	// If there is no upstream, this will be set to ""
	upstreamChartVersion *string
}

// ApplyMainChanges applies any changes on the main chart introduced by the AdditionalChart
func (c *AdditionalChart) ApplyMainChanges(pkgFs billy.Filesystem) error {
	if exists, err := filesystem.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to check if %s exists: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("working directory %s has not been prepared yet", c.WorkingDir)
	}
	if c.CRDChartOptions == nil {
		return nil
	}
	mainChartWorkingDir, err := c.getMainChartWorkingDir(pkgFs)
	if err != nil {
		return fmt.Errorf("encountered error while trying to get the main chart's working directory: %s", err)
	}
	if c.CRDChartOptions.UseTarArchive {
		if err := helm.ArchiveCRDs(pkgFs, mainChartWorkingDir, path.ChartCRDDir, c.WorkingDir, path.ChartExtraFileDir); err != nil {
			return fmt.Errorf("encountered error while trying to bundle and compress CRD files from the main chart: %s", err)
		}
	}
	if err := helm.CopyCRDsFromChart(pkgFs, mainChartWorkingDir, path.ChartCRDDir, c.WorkingDir, c.CRDChartOptions.CRDDirectory); err != nil {
		return fmt.Errorf("encountered error while trying to copy CRDs from %s to %s: %s", mainChartWorkingDir, c.WorkingDir, err)
	}
	if err := helm.DeleteCRDsFromChart(pkgFs, mainChartWorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to delete CRDs from main chart: %s", err)
	}
	if c.CRDChartOptions.AddCRDValidationToMainChart {
		if err := AddCRDValidationToChart(pkgFs, mainChartWorkingDir, c.WorkingDir, c.CRDChartOptions.CRDDirectory); err != nil {
			return fmt.Errorf("encountered error while trying to add CRD validation to %s based on CRDs in %s: %s", mainChartWorkingDir, c.WorkingDir, err)
		}
	}
	return nil
}

// RevertMainChanges reverts any changes on the main chart introduced by the AdditionalChart
func (c *AdditionalChart) RevertMainChanges(pkgFs billy.Filesystem) error {
	if exists, err := filesystem.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to check if %s exists: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("working directory %s has not been prepared yet", c.WorkingDir)
	}
	if c.CRDChartOptions == nil {
		// return if the additional chart is not a CRD chart
		return nil
	}
	mainChartWorkingDir, err := c.getMainChartWorkingDir(pkgFs)
	if err != nil {
		return fmt.Errorf("encountered error while trying to get the main chart's working directory: %s", err)
	}
	// copy CRD files from packages/<package>/charts-crd/crd-manifest/ back to packages/<package>/charts/crds/
	if err := helm.CopyCRDsFromChart(pkgFs, c.WorkingDir, c.CRDChartOptions.CRDDirectory, mainChartWorkingDir, path.ChartCRDDir); err != nil {
		return fmt.Errorf("encountered error while trying to copy CRDs from %s to %s: %s", c.WorkingDir, mainChartWorkingDir, err)
	}
	if c.CRDChartOptions.AddCRDValidationToMainChart {
		if err := RemoveCRDValidationFromChart(pkgFs, mainChartWorkingDir); err != nil {
			return fmt.Errorf("encountered error while trying to remove CRD validation from chart: %s", err)
		}
	}
	return nil
}

// Prepare pulls in a package based on the spec to the local git repository
func (c *AdditionalChart) Prepare(rootFs, pkgFs billy.Filesystem, mainChartUpstreamVersion *string) error {
	if c.CRDChartOptions == nil && c.Upstream == nil {
		return fmt.Errorf("no options provided to prepare additional chart")
	}
	if c.Upstream != nil && (*c.Upstream).IsWithinPackage() {
		logrus.Infof("Local chart does not need to be patched")
		// Ensure local charts standardize the Chart.yaml on prepare
		if err := helm.StandardizeChartYaml(pkgFs, c.WorkingDir); err != nil {
			return err
		}
		return nil
	}

	if err := filesystem.RemoveAll(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to clean up %s before preparing: %s", c.WorkingDir, err)
	}
	if c.CRDChartOptions != nil {
		c.upstreamChartVersion = mainChartUpstreamVersion
		mainChartWorkingDir, err := c.getMainChartWorkingDir(pkgFs)
		if err != nil {
			return fmt.Errorf("encountered error while trying to get the main chart's working directory: %s", err)
		}
		exists, err := filesystem.PathExists(pkgFs, filepath.Join(mainChartWorkingDir, path.ChartCRDDir))
		if err != nil {
			return fmt.Errorf("encountered error while trying to check if %s exists: %s", filepath.Join(mainChartWorkingDir, path.ChartCRDDir), err)
		}
		if !exists {
			return fmt.Errorf("unable to prepare a CRD chart since there are no CRDs at %s", filepath.Join(mainChartWorkingDir, path.ChartCRDDir))
		}
		if err := GenerateCRDChartFromTemplate(pkgFs, c.WorkingDir, filepath.Join(path.PackageTemplatesDir, c.CRDChartOptions.TemplateDirectory), c.CRDChartOptions.CRDDirectory); err != nil {
			return fmt.Errorf("encountered error while trying to generate CRD chart from template at %s: %s", c.CRDChartOptions.TemplateDirectory, err)
		}
	} else {
		u := *c.Upstream
		if err := u.Pull(rootFs, pkgFs, c.WorkingDir); err != nil {
			return fmt.Errorf("encountered error while trying to pull upstream into %s: %s", c.WorkingDir, err)
		}
		// If the upstream is not already a Helm chart, convert it into a dummy Helm chart by moving YAML files to templates and creating a dummy Chart.yaml
		if err := helm.ConvertToHelmChart(pkgFs, c.WorkingDir); err != nil {
			return fmt.Errorf("encountered error while trying to convert upstream at %s into a Helm chart: %s", c.WorkingDir, err)
		}
		var err error
		upstreamChartVersion, err := helm.GetHelmMetadataVersion(pkgFs, c.WorkingDir)
		if err != nil {
			return fmt.Errorf("encountered error while parsing original chart's version in %s: %s", c.WorkingDir, err)
		}
		c.upstreamChartVersion = &upstreamChartVersion
	}
	if err := PrepareDependencies(rootFs, pkgFs, c.WorkingDir, c.GeneratedChangesRootDir(), c.IgnoreDependencies); err != nil {
		return fmt.Errorf("encountered error while trying to prepare dependencies in %s: %s", c.WorkingDir, err)
	}
	if c.Upstream != nil {
		// Only upstream charts support patches
		err := change.ApplyChanges(pkgFs, c.WorkingDir, c.GeneratedChangesRootDir())
		if err != nil {
			return fmt.Errorf("encountered error while trying to apply changes to %s: %s", c.WorkingDir, err)
		}
	}
	return nil
}

// getMainChartWorkingDir gets the working directory of the main chart
func (c *AdditionalChart) getMainChartWorkingDir(pkgFs billy.Filesystem) (string, error) {
	packageOpts, err := options.LoadPackageOptionsFromFile(pkgFs, path.PackageOptionsFile)
	if err != nil {
		return "", fmt.Errorf("unable to read package.yaml: %s", err)
	}
	workingDir := packageOpts.MainChartOptions.WorkingDir
	if len(workingDir) == 0 {
		return "charts", nil
	}
	return workingDir, nil
}

// GeneratePatch generates a patch on a forked Helm chart based on local changes
func (c *AdditionalChart) GeneratePatch(rootFs, pkgFs billy.Filesystem) error {
	if c.CRDChartOptions == nil && c.Upstream == nil {
		return fmt.Errorf("no options provided to prepare additional chart")
	}
	if c.Upstream != nil && (*c.Upstream).IsWithinPackage() {
		logrus.Infof("Local chart does not need to be patched")
		return nil
	}
	if exists, err := filesystem.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("encountered error while trying to check if %s exists: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("working directory %s has not been prepared yet", c.WorkingDir)
	}

	if c.CRDChartOptions != nil {
		logrus.Warnf("Patches are not supported for additional charts using CRDChartOptions. Any local changes will be overridden; please make the changes directly at %s",
			filepath.Join(path.PackageTemplatesDir, c.CRDChartOptions.TemplateDirectory))
		return nil
	}

	// Standardize the local copy of the Chart.yaml before trying to compare the patch
	if err := helm.StandardizeChartYaml(pkgFs, c.WorkingDir); err != nil {
		return err
	}

	u := *c.Upstream
	if err := u.Pull(rootFs, pkgFs, c.OriginalDir()); err != nil {
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
func (c *AdditionalChart) GenerateChart(rootFs, pkgFs billy.Filesystem, packageVersion *int, version *semver.Version, omitBuildMetadataOnExport, validateOnly bool) error {
	if c.upstreamChartVersion == nil {
		return fmt.Errorf("cannot generate chart since it has never been prepared: upstreamChartVersion is not set")
	}
	if err := helm.ExportHelmChart(rootFs, pkgFs, c.WorkingDir, packageVersion, version, *c.upstreamChartVersion, omitBuildMetadataOnExport, validateOnly); err != nil {
		return fmt.Errorf("encountered error while trying to export Helm chart for %s: %s", c.WorkingDir, err)
	}
	return nil
}

// OriginalDir returns a working directory where we can place the original chart from upstream
func (c *AdditionalChart) OriginalDir() string {
	return fmt.Sprintf("%s-original", c.WorkingDir)
}

// GeneratedChangesRootDir stored the directory rooted at the package level where generated changes for this chart can be found
func (c *AdditionalChart) GeneratedChangesRootDir() string {
	return filepath.Join(path.GeneratedChangesDir, path.GeneratedChangesAdditionalChartDir, c.WorkingDir, path.GeneratedChangesDir)
}
