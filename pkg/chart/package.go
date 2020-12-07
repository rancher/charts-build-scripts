package chart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/local"
	"github.com/sirupsen/logrus"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

const (
	// Repository Paths

	// RepositoryHelmIndexFilepath is the file on your Staging/Live branch that contains your Helm repository index
	RepositoryHelmIndexFilepath = "index.yaml"
	// RepositoryPackagesDirpath is a directory on your Source branch that contains the files necessary to generate your forked Helm chart
	RepositoryPackagesDirpath = "packages"
	// RepositoryAssetsDirpath is a directory on your Staging/Live branch that contains chart archives for each version of your forked Helm chart
	RepositoryAssetsDirpath = "assets"
	// RepositoryChartsDirpath is a directory on your Staging/Live branch that contains unarchived charts for each version of your forked Helm chart
	RepositoryChartsDirpath = "charts"

	// Package Directory Paths

	// PackageOptionsFilepath is the location of your package.yaml, which contains information about how to prepare your chart
	PackageOptionsFilepath = "package.yaml"
	// PackageChartsDirpath is the location of your current working copy of the chart within the package
	PackageChartsDirpath = "charts"
	// PackageChartsCRDDirpath is the location of your current working copy of the CRD chart within the package
	PackageChartsCRDDirpath = "charts-crd"
	// PackageChartsOriginalDirpath is the location of an upstream copy of your chart with no changes applied
	PackageChartsOriginalDirpath = "charts-original"
	// PackagePatchDirpath is where patches on files are kept in your Source branch
	PackagePatchDirpath = "generated-changes/patch"
	// PackageOverlayDirpath is where any files or directories that are unique to your fork are kept in your Source branch
	PackageOverlayDirpath = "generated-changes/overlay"
	// PackageExcludeDirpath is where any files or directories that were omitted from upstream are kept in your Source branch
	PackageExcludeDirpath = "generated-changes/exclude"
	// PackageDependenciesDirpath is where any dependencies of your package are stored as subcharts
	PackageDependenciesDirpath = "generated-changes/dependencies"

	patchFmt = "%s.patch"
)

// Package represents the configuration of a particular forked Helm chart
type Package struct {
	// Name is the name of the package
	Name string `yaml:"name"`
	// PackageVersion represents the current version of the package. It needs to be incremented whenever there are changes
	PackageVersion int `yaml:"packageVersion"`
	// Upstream represents any options that are configurable for upstream charts
	Upstream Upstream `yaml:"upstream"`
	// CRDChartOptions represents any options that are configurable for CRD charts
	CRDChartOptions CRDChartOptions `yaml:"crdChart"`
	// ExportOptions represents any options that are configurable when exporting a chart
	ExportOptions ExportOptions `yaml:"export"`
	// CleanOptions represents any options that are configurable when cleaning a chart
	CleanOptions CleanOptions `yaml:"clean"`

	// fs is a filesystem rooted at the package
	fs billy.Filesystem
	// repoFs is a filesystem rooted at the repository containing the package
	repoFs billy.Filesystem
}

// Prepare pulls in a package based on the spec to the local git repository
func (p *Package) Prepare() error {
	if err := local.RemoveAll(p.fs, PackageChartsDirpath); err != nil {
		return err
	}
	if err := p.Upstream.Pull(p.fs, PackageChartsDirpath); err != nil {
		return err
	}
	if err := p.PrepareDependencies(PackageChartsDirpath); err != nil {
		return err
	}
	if err := applyChanges(p.fs, PackageChartsDirpath); err != nil {
		return err
	}
	// Split into CRD chart
	return nil
}

// GeneratePatch generates a patch on a forked Helm chart based on local changes
func (p *Package) GeneratePatch() error {
	if exists, err := local.PathExists(p.fs, PackageChartsDirpath); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("Package %s has not been prepared yet", p.Name)
	}
	// Undo split into CRD chart
	// Pull chart from upstream to charts-original/
	if err := p.Upstream.Pull(p.fs, PackageChartsOriginalDirpath); err != nil {
		return err
	}
	if err := p.PrepareDependencies(PackageChartsOriginalDirpath); err != nil {
		return err
	}
	defer local.RemoveAll(p.fs, PackageChartsOriginalDirpath)
	// Remove old stuff in dir
	if err := local.RemoveAll(p.fs, PackagePatchDirpath); err != nil {
		return err
	}
	if err := local.RemoveAll(p.fs, PackageOverlayDirpath); err != nil {
		return err
	}
	if err := local.RemoveAll(p.fs, PackageExcludeDirpath); err != nil {
		return err
	}
	return generateChanges(p.fs, PackageChartsOriginalDirpath, PackageChartsDirpath)
}

// GenerateChart creates a Helm chart archive based on the package.yaml, patch, and overlay/
func (p *Package) GenerateChart() error {
	if err := p.Prepare(); err != nil {
		return err
	}
	defer local.RemoveAll(p.fs, PackageChartsDirpath)
	if err := p.ExportHelmChart(BaseChartType(), PackageChartsDirpath); err != nil {
		return err
	}
	exists, err := local.PathExists(p.fs, PackageChartsCRDDirpath)
	if err != nil {
		return err
	}
	if exists {
		if err := p.ExportHelmChart(CRDChartType(), PackageChartsCRDDirpath); err != nil {
			return err
		}
	}
	// Generate the index.yaml
	helmIndexFile, err := helmRepo.IndexDirectory(local.GetAbsPath(p.repoFs, RepositoryAssetsDirpath), RepositoryAssetsDirpath)
	if err != nil {
		return err
	}
	helmIndexFile.SortEntries()
	if err := helmIndexFile.WriteFile(local.GetAbsPath(p.repoFs, RepositoryHelmIndexFilepath), os.ModePerm); err != nil {
		return err
	}
	return nil
}

// Clean removes all other files except for the package.yaml, patch, and overlay/ files from a package
func (p *Package) Clean() error {
	packagePathsToClean := []string{
		PackageChartsDirpath,
		PackageChartsCRDDirpath,
		PackageChartsOriginalDirpath,
	}
	for _, packagePath := range packagePathsToClean {
		if err := local.RemoveAll(p.fs, packagePath); err != nil {
			return err
		}
	}
	if p.CleanOptions.PreventCleanAssets {
		return nil
	}
	repositoryPathsToClean := []string{
		RepositoryAssetsDirpath,
		RepositoryChartsDirpath,
	}
	for _, repoPath := range repositoryPathsToClean {
		if err := local.RemoveAll(p.repoFs, repoPath); err != nil {
			return err
		}
	}
	return nil
}

// ExportHelmChart creates a Helm chart archive and an unarchived Helm chart at RepositoryAssetDirpath and RepositoryChartDirPath
// chartType indicates what types of chart this is (e.g. "" if there is only one chart or "chart" / "crd" if there are multiple)
// helmChartPath is a relative path (rooted at the package level) that contains the chart. e.g. PackageChartsDirpath or PackageChartsCRDDirpath
func (p *Package) ExportHelmChart(chartType Type, helmChartPath string) error {
	// Get chart version
	chart, err := helmLoader.Load(local.GetAbsPath(p.fs, helmChartPath))
	if err != nil {
		return err
	}
	chartVersion := chart.Metadata.Version + fmt.Sprintf("%02d", p.PackageVersion)
	assetsPath := p.RepositoryAssetDirpath()
	chartsPath := p.RepositoryChartDirpath(chartVersion, chartType)
	// Create directories
	if err := p.repoFs.MkdirAll(assetsPath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory for assets at %s: %s", assetsPath, err)
	}
	defer local.PruneEmptyDirsInPath(p.repoFs, assetsPath)
	if err := p.repoFs.MkdirAll(chartsPath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory for charts at %s: %s", chartsPath, err)
	}
	defer local.PruneEmptyDirsInPath(p.repoFs, chartsPath)
	// Check export options
	if p.ExportOptions.PreventOverwrite {
		empty, err := local.IsEmptyDir(p.repoFs, chartsPath)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("Cannot export chart because %s is not empty", chartsPath)
		}
	}
	// Run helm package
	pkg := helmAction.NewPackage()
	pkg.Destination = local.GetAbsPath(p.repoFs, assetsPath)
	pkg.DependencyUpdate = false
	absTgzPath, err := pkg.Run(local.GetAbsPath(p.fs, helmChartPath), nil)
	if err != nil {
		return err
	}
	tgzPath, err := local.GetRelativePath(p.repoFs, absTgzPath)
	if err != nil {
		return err
	}
	logrus.Infof("Generated archive: %s", tgzPath)
	// Unarchive the generated package
	if err := local.UnarchiveTgz(p.repoFs, tgzPath, "", chartsPath, true); err != nil {
		return err
	}
	logrus.Infof("Generated chart: %s", chartsPath)
	return nil
}

// PackageDependencyDirpath returns the path to the directory that will contain a package for a specific dependency
func (p *Package) PackageDependencyDirpath(dependencyName string) string {
	return filepath.Join(PackageDependenciesDirpath, dependencyName)
}

// RepositoryChartDirpath returns the path to the directory that will contain the generated chart from this package
// chartType indicates what types of chart this is (e.g. "" if there is only one chart or "chart" / "crd" if there are multiple)
// version is the version string that this generated asset should be prefixed by
func (p *Package) RepositoryChartDirpath(version string, chartType Type) string {
	return filepath.Join(RepositoryChartsDirpath, p.Name, version, string(chartType))
}

// RepositoryAssetDirpath returns the path to the directory that will contain the generated charts from this package
func (p *Package) RepositoryAssetDirpath() string {
	return filepath.Join(RepositoryAssetsDirpath, p.Name)
}

// GetRepositoryPath takes a path that is rooted at the package level and returns one rooted at the repository level
func (p *Package) GetRepositoryPath(packagePath string) (string, error) {
	return local.GetRelativePath(p.repoFs, local.GetAbsPath(p.fs, packagePath))
}

// GetPackagePath takes a path that is rooted at the repository level and returns one rooted at the package level
func (p *Package) GetPackagePath(repositoryPath string) (string, error) {
	return local.GetRelativePath(p.fs, local.GetAbsPath(p.repoFs, repositoryPath))
}

// String returns the string representation of a Package
func (p Package) String() string {
	return fmt.Sprintf("%s[packageVersion=%d, upstream=%s, crdChartOptions=%v, exportOptions=%v]", p.Name, p.PackageVersion, p.Upstream, p.CRDChartOptions, p.ExportOptions)
}
