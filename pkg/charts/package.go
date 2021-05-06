package charts

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

// Package represents the configuration of a particular forked Helm chart
type Package struct {
	Chart `yaml:",inline"`

	// Name is the name of the package
	Name string `yaml:"name"`
	// PackageVersion represents the current version of the package. It needs to be incremented whenever there are changes
	PackageVersion int `yaml:"packageVersion"`
	// AdditionalCharts are other charts that should be packaged together with this
	AdditionalCharts []AdditionalChart `yaml:"additionalCharts,omitempty"`
	// DoNotRelease represents a boolean flag that indicates a package should not be tracked in make charts
	DoNotRelease bool `yaml:"doNotRelease,omitempty"`

	// fs is a filesystem rooted at the package
	fs billy.Filesystem
	// rootFs is a filesystem rooted at the repository containing the package
	rootFs billy.Filesystem
}

// Prepare pulls in a package based on the spec to the local git repository
func (p *Package) Prepare() error {
	if err := p.Chart.Prepare(p.rootFs, p.fs); err != nil {
		return fmt.Errorf("Encountered error while preparing main chart: %s", err)
	}
	if p.Chart.Upstream.IsWithinPackage() {
		for _, additionalChart := range p.AdditionalCharts {
			exists, err := filesystem.PathExists(p.fs, additionalChart.WorkingDir)
			if err != nil {
				return fmt.Errorf("Encountered error while trying to check if %s exists: %s", additionalChart.WorkingDir, err)
			}
			if !exists {
				continue
			}
			// Local charts need to revert changes before trying to prepare additional charts
			if err := additionalChart.RevertMainChanges(p.fs); err != nil {
				return fmt.Errorf("Encountered error while reverting changes from %s to main chart: %s", additionalChart.WorkingDir, err)
			}
		}
	}
	for _, additionalChart := range p.AdditionalCharts {
		if err := additionalChart.Prepare(p.rootFs, p.fs); err != nil {
			return fmt.Errorf("Encountered error while preparing additional chart %s: %s", additionalChart.WorkingDir, err)
		}
		if err := additionalChart.ApplyMainChanges(p.fs); err != nil {
			return fmt.Errorf("Encountered error while applying main changes from %s to main chart: %s", additionalChart.WorkingDir, err)
		}
	}
	return nil
}

// GeneratePatch generates a patch on a forked Helm chart based on local changes
func (p *Package) GeneratePatch() error {
	for _, additionalChart := range p.AdditionalCharts {
		if err := additionalChart.RevertMainChanges(p.fs); err != nil {
			return fmt.Errorf("Encountered error while reverting changes from %s to main chart: %s", additionalChart.WorkingDir, err)
		}
	}
	if err := p.Chart.GeneratePatch(p.rootFs, p.fs); err != nil {
		return fmt.Errorf("Encountered error while generating patch on main chart: %s", err)
	}
	for _, additionalChart := range p.AdditionalCharts {
		if err := additionalChart.ApplyMainChanges(p.fs); err != nil {
			return fmt.Errorf("Encountered error while applying main changes from %s to main chart: %s", additionalChart.WorkingDir, err)
		}
		if err := additionalChart.GeneratePatch(p.rootFs, p.fs); err != nil {
			return fmt.Errorf("Encountered error while generating patch on additional chart %s: %s", additionalChart.WorkingDir, err)
		}
	}
	return nil
}

// GenerateCharts creates Helm chart archives for each chart after preparing it
func (p *Package) GenerateCharts() error {
	if p.DoNotRelease {
		logrus.Infof("Skipping package marked doNotRelease")
		return nil
	}
	if err := p.Prepare(); err != nil {
		return fmt.Errorf("Encountered error while trying to prepare package: %s", err)
	}
	// Export Helm charts
	packageAssetsDirpath := filepath.Join(path.RepositoryAssetsDir, p.Name)
	packageChartsDirpath := filepath.Join(path.RepositoryChartsDir, p.Name)
	// Add PackageVersion to format
	err := p.Chart.GenerateChart(p.rootFs, p.fs, p.PackageVersion, packageAssetsDirpath, packageChartsDirpath)
	if err != nil {
		return fmt.Errorf("Encountered error while exporting main chart: %s", err)
	}
	for _, additionalChart := range p.AdditionalCharts {
		err = additionalChart.GenerateChart(p.rootFs, p.fs, p.PackageVersion, packageAssetsDirpath, packageChartsDirpath)
		if err != nil {
			return fmt.Errorf("Encountered error while exporting %s: %s", additionalChart.WorkingDir, err)
		}
	}
	if err := helm.CreateOrUpdateHelmIndex(p.rootFs); err != nil {
		return err
	}
	return p.Clean()
}

// Clean removes all other files except for the package.yaml, patch, and overlay/ files from a package
func (p *Package) Clean() error {
	chartPathsToClean := []string{p.Chart.OriginalDir()}
	if !p.Chart.Upstream.IsWithinPackage() {
		chartPathsToClean = append(chartPathsToClean, p.Chart.WorkingDir)
	} else {
		// Local charts should clean up added dependencies
		chartPathsToClean = append(chartPathsToClean, filepath.Join(p.Chart.WorkingDir, "charts"))
	}
	for _, additionalChart := range p.AdditionalCharts {
		if additionalChart.Upstream != nil && (*additionalChart.Upstream).IsWithinPackage() {
			// Working directory never needs to be clean for an additional chart
			continue
		}
		exists, err := filesystem.PathExists(p.fs, additionalChart.WorkingDir)
		if err != nil {
			return fmt.Errorf("Encountered error while trying to check if %s exists: %s", additionalChart.WorkingDir, err)
		}
		if exists {
			if err := additionalChart.RevertMainChanges(p.fs); err != nil {
				return fmt.Errorf("Encountered error while reverting changes from %s to main chart: %s", additionalChart.WorkingDir, err)
			}
		}
		chartPathsToClean = append(chartPathsToClean, additionalChart.OriginalDir(), additionalChart.WorkingDir)
	}
	for _, chartPath := range chartPathsToClean {
		if err := filesystem.RemoveAll(p.fs, chartPath); err != nil {
			return fmt.Errorf("Encountered error while trying to remove %s from package %s: %s", chartPath, p.Name, err)
		}
	}
	return nil
}
