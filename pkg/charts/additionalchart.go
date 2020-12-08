package charts

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	// GeneratedChangesAdditionalChartDirpath is a directory that contains additionalCharts
	GeneratedChangesAdditionalChartDirpath = "additional-charts"
	// PackageTemplatesDirpath is a directory containing templates used as additional chart options
	PackageTemplatesDirpath = "templates"
	// ChartCRDDirpath represents the directory that we expect to contain CRDs within the chart
	ChartCRDDirpath = "crds"
)

// AdditionalChart represents any additional charts packaged along with the main chart in a package
type AdditionalChart struct {
	// WorkingDir represents the working directory of this chart
	WorkingDir string `yaml:"workingDir" default:"charts"`
	// Upstream represents any options that are configurable for upstream charts
	Upstream *Upstream `yaml:"upstream"`
	// CRDChartOptions represents any options that are configurable for CRD charts
	CRDChartOptions *options.CRDChartOptions `yaml:"crdChart"`
}

// ApplyMainChanges applies any changes on the main chart introduced by the AdditionalChart
func (c *AdditionalChart) ApplyMainChanges(pkgFs billy.Filesystem) error {
	if exists, err := utils.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to check if %s exists: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("Working directory %s has not been prepared yet", c.WorkingDir)
	}
	if c.CRDChartOptions == nil {
		return nil
	}
	mainChartWorkingDir, err := c.getMainChartWorkingDir(pkgFs)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to get the main chart's working directory: %s", err)
	}
	if err := CopyCRDsFromChart(pkgFs, mainChartWorkingDir, ChartCRDDirpath, c.WorkingDir, c.CRDChartOptions.CRDDirectory); err != nil {
		return fmt.Errorf("Encountered error while trying to copy CRDs from %s to %s: %s", mainChartWorkingDir, c.WorkingDir, err)
	}
	if err := DeleteCRDsFromChart(pkgFs, mainChartWorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to delete CRDs from main chart")
	}
	return nil
}

// RevertMainChanges reverts any changes on the main chart introduced by the AdditionalChart
func (c *AdditionalChart) RevertMainChanges(pkgFs billy.Filesystem) error {
	if exists, err := utils.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to check if %s exists: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("Working directory %s has not been prepared yet", c.WorkingDir)
	}
	if c.CRDChartOptions == nil {
		return nil
	}
	mainChartWorkingDir, err := c.getMainChartWorkingDir(pkgFs)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to get the main chart's working directory: %s", err)
	}
	if err := CopyCRDsFromChart(pkgFs, c.WorkingDir, c.CRDChartOptions.CRDDirectory, mainChartWorkingDir, ChartCRDDirpath); err != nil {
		return fmt.Errorf("Encountered error while trying to copy CRDs from %s to %s: %s", c.WorkingDir, mainChartWorkingDir, err)
	}
	// Get the main chart to pull changes from
	return nil
}

// Prepare pulls in a package based on the spec to the local git repository
func (c *AdditionalChart) Prepare(pkgFs billy.Filesystem) error {
	if c.CRDChartOptions == nil && c.Upstream == nil {
		return fmt.Errorf("No options provided to prepare additional chart")
	}

	if err := utils.RemoveAll(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to clean up %s before preparing: %s", c.WorkingDir, err)
	}
	if c.CRDChartOptions != nil {
		mainChartWorkingDir, err := c.getMainChartWorkingDir(pkgFs)
		if err != nil {
			return fmt.Errorf("Encountered error while trying to get the main chart's working directory: %s", err)
		}
		exists, err := utils.PathExists(pkgFs, filepath.Join(mainChartWorkingDir, ChartCRDDirpath))
		if err != nil {
			return fmt.Errorf("Encountered error while trying to check if %s exists: %s", filepath.Join(mainChartWorkingDir, ChartCRDDirpath), err)
		}
		if !exists {
			return fmt.Errorf("Unable to prepare a CRD chart since there are no CRDs at %s", filepath.Join(mainChartWorkingDir, ChartCRDDirpath))
		}
		if err := GenerateCRDChartFromTemplate(pkgFs, c.WorkingDir, filepath.Join(PackageTemplatesDirpath, c.CRDChartOptions.TemplateDirectory), c.CRDChartOptions.CRDDirectory); err != nil {
			return fmt.Errorf("Encountered error while trying to generate CRD chart from template at %s: %s", c.CRDChartOptions.TemplateDirectory, err)
		}
	} else {
		u := *c.Upstream
		if err := u.Pull(pkgFs, c.WorkingDir); err != nil {
			return fmt.Errorf("Encountered error while trying to pull upstream into %s: %s", c.WorkingDir, err)
		}
	}
	if err := PrepareDependencies(pkgFs, c.WorkingDir, c.GeneratedChangesRootDir()); err != nil {
		return fmt.Errorf("Encountered error while trying to prepare dependencies in %s: %s", c.WorkingDir, err)
	}
	if c.Upstream != nil {
		// Only upstream charts support patches
		err := ApplyChanges(pkgFs, c.WorkingDir, c.GeneratedChangesRootDir())
		if err != nil {
			return fmt.Errorf("Encountered error while trying to apply changes to %s: %s", c.WorkingDir, err)
		}
	}
	return nil
}

// getMainChartWorkingDir gets the working directory of the main chart
func (c *AdditionalChart) getMainChartWorkingDir(pkgFs billy.Filesystem) (string, error) {
	packageOpts, err := options.LoadPackageOptionsFromFile(pkgFs, PackageOptionsFilepath)
	if err != nil {
		return "", fmt.Errorf("Unable to read package.yaml: %s", err)
	}
	workingDir := packageOpts.MainChartOptions.WorkingDir
	if len(workingDir) == 0 {
		return "charts", nil
	}
	return workingDir, nil
}

// GeneratePatch generates a patch on a forked Helm chart based on local changes
func (c *AdditionalChart) GeneratePatch(pkgFs billy.Filesystem) error {
	if c.CRDChartOptions == nil && c.Upstream == nil {
		return fmt.Errorf("No options provided to prepare additional chart")
	}
	if exists, err := utils.PathExists(pkgFs, c.WorkingDir); err != nil {
		return fmt.Errorf("Encountered error while trying to check if %s exists: %s", c.WorkingDir, err)
	} else if !exists {
		return fmt.Errorf("Working directory %s has not been prepared yet", c.WorkingDir)
	}

	if c.CRDChartOptions != nil {
		logrus.Warnf("Patches are not supported for additional charts using CRDChartOptions. Any local changes will be overridden; please make the changes directly at %s",
			filepath.Join(PackageTemplatesDirpath, c.CRDChartOptions.TemplateDirectory))
		return nil
	}

	u := *c.Upstream
	if err := u.Pull(pkgFs, c.OriginalDir()); err != nil {
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
func (c *AdditionalChart) GenerateChart(pkgFs billy.Filesystem, packageVersion int, repoFs billy.Filesystem, packageAssetsDirpath, packageChartsDirpath string, opts options.ExportOptions) error {
	if err := ExportHelmChart(pkgFs, c.WorkingDir, packageVersion, repoFs, packageAssetsDirpath, packageChartsDirpath, opts); err != nil {
		return fmt.Errorf("Encountered error while trying to export Helm chart for %s: %s", c.WorkingDir, err)
	}
	return nil
}

// OriginalDir returns a working directory where we can place the original chart from upstream
func (c *AdditionalChart) OriginalDir() string {
	return fmt.Sprintf("%s-original", c.WorkingDir)
}

// GeneratedChangesRootDir stored the directory rooted at the package level where generated changes for this chart can be found
func (c *AdditionalChart) GeneratedChangesRootDir() string {
	return filepath.Join(GeneratedChangesDirpath, GeneratedChangesAdditionalChartDirpath, c.WorkingDir, GeneratedChangesDirpath)
}
