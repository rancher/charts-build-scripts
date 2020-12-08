package charts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

const (
	// RepositoryHelmIndexFilepath is the file on your Staging/Live branch that contains your Helm repository index
	RepositoryHelmIndexFilepath = "index.yaml"
	// RepositoryPackagesDirpath is a directory on your Source branch that contains the files necessary to generate your package
	RepositoryPackagesDirpath = "packages"
	// RepositoryAssetsDirpath is a directory on your Staging/Live branch that contains chart archives for each version of your package
	RepositoryAssetsDirpath = "assets"
	// RepositoryChartsDirpath is a directory on your Staging/Live branch that contains unarchived charts for each version of your package
	RepositoryChartsDirpath = "charts"

	// PackageOptionsFilepath is the name of a file that contains information about how to prepare your package
	// The expected structure of this file is one that can be marshalled into a PackageOptions struct
	PackageOptionsFilepath = "package.yaml"
)

// Package represents the configuration of a particular forked Helm chart
type Package struct {
	Chart `yaml:",inline"`

	// Name is the name of the package
	Name string `yaml:"name"`
	// PackageVersion represents the current version of the package. It needs to be incremented whenever there are changes
	PackageVersion int `yaml:"packageVersion"`
	// BranchOptions represents any options that are configured per branch for a package
	BranchOptions options.BranchOptions `yaml:",inline"`
	// AdditionalCharts are other charts that should be packaged together with this
	AdditionalCharts []AdditionalChart `yaml:"additionalCharts,omitempty"`

	// fs is a filesystem rooted at the package
	fs billy.Filesystem
	// repoFs is a filesystem rooted at the repository containing the package
	repoFs billy.Filesystem
}

// Prepare pulls in a package based on the spec to the local git repository
func (p *Package) Prepare() error {
	if err := p.Chart.Prepare(p.fs); err != nil {
		return fmt.Errorf("Encountered error while preparing main chart: %s", err)
	}
	for _, additionalChart := range p.AdditionalCharts {
		if err := additionalChart.Prepare(p.fs); err != nil {
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
	if err := p.Chart.GeneratePatch(p.fs); err != nil {
		return fmt.Errorf("Encountered error while generating patch on main chart: %s", err)
	}
	for _, additionalChart := range p.AdditionalCharts {
		if err := additionalChart.ApplyMainChanges(p.fs); err != nil {
			return fmt.Errorf("Encountered error while applying main changes from %s to main chart: %s", additionalChart.WorkingDir, err)
		}
		if err := additionalChart.GeneratePatch(p.fs); err != nil {
			return fmt.Errorf("Encountered error while generating patch on additional chart %s: %s", additionalChart.WorkingDir, err)
		}
	}
	return nil
}

// GenerateCharts creates Helm chart archives for each chart after preparing it
func (p *Package) GenerateCharts() error {
	if err := p.Prepare(); err != nil {
		return fmt.Errorf("Encountered error while trying to prepare package: %s", err)
	}
	// Export Helm charts
	packageAssetsDirpath := filepath.Join(RepositoryAssetsDirpath, p.Name)
	packageChartsDirpath := filepath.Join(RepositoryChartsDirpath, p.Name)
	err := p.Chart.GenerateChart(p.fs, p.PackageVersion, p.repoFs, packageAssetsDirpath, packageChartsDirpath, p.BranchOptions.ExportOptions)
	if err != nil {
		return fmt.Errorf("Encountered error while exporting main chart: %s", err)
	}
	for _, additionalChart := range p.AdditionalCharts {
		err = additionalChart.GenerateChart(p.fs, p.PackageVersion, p.repoFs, packageAssetsDirpath, packageChartsDirpath, p.BranchOptions.ExportOptions)
		if err != nil {
			return fmt.Errorf("Encountered error while exporting %s: %s", additionalChart.WorkingDir, err)
		}
	}
	return p.CreateOrUpdateHelmIndex()
}

// CreateOrUpdateHelmIndex either creates or updates the index.yaml for the repository this package is within
func (p *Package) CreateOrUpdateHelmIndex() error {
	absRepositoryAssetsDirpath := utils.GetAbsPath(p.repoFs, RepositoryAssetsDirpath)
	absRepositoryHelmIndexFilepath := utils.GetAbsPath(p.repoFs, RepositoryHelmIndexFilepath)
	helmIndexFile, err := helmRepo.IndexDirectory(absRepositoryAssetsDirpath, RepositoryAssetsDirpath)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to generate new Helm index: %s", err)
	}
	helmIndexFile.SortEntries()
	err = helmIndexFile.WriteFile(absRepositoryHelmIndexFilepath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to write updated Helm index into index.yaml: %s", err)
	}
	return nil
}

// Clean removes all other files except for the package.yaml, patch, and overlay/ files from a package
func (p *Package) Clean() error {
	chartPathsToClean := []string{p.Chart.WorkingDir, p.Chart.OriginalDir()}
	for _, additionalChart := range p.AdditionalCharts {
		chartPathsToClean = append(chartPathsToClean, additionalChart.WorkingDir, additionalChart.OriginalDir())
	}
	for _, chartPath := range chartPathsToClean {
		if err := utils.RemoveAll(p.fs, chartPath); err != nil {
			return fmt.Errorf("Encountered error while trying to remove %s from package %s: %s", chartPath, p.Name, err)
		}
	}
	if p.BranchOptions.CleanOptions.PreventCleanAssets {
		return nil
	}
	repositoryPathsToClean := []string{
		RepositoryAssetsDirpath,
		RepositoryChartsDirpath,
	}
	for _, repoPath := range repositoryPathsToClean {
		if err := utils.RemoveAll(p.repoFs, repoPath); err != nil {
			return fmt.Errorf("Encountered error while trying to remove %s from the repository: %s", repoPath, err)
		}
	}
	return nil
}
