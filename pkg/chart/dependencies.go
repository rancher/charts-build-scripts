package chart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/charts-build-scripts/pkg/local"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// PrepareDependencies prepares all of the dependencies of a given chart and regenerates the requirements.yaml or Chart.yaml
// It then stores the patch on the regenerated requirements.yaml or Chart.yaml under PackagePatchDirpath
func (p *Package) PrepareDependencies(helmChartPath string) error {
	if err := p.LoadDependencyPackages(helmChartPath); err != nil {
		return err
	}
	dependencyPackages, err := p.GetDependencyPackages()
	if err != nil {
		return err
	}
	if len(dependencyPackages) == 0 {
		return nil
	}
	// Remove all existing stuff from the charts/ directory
	local.RemoveAll(p.fs, filepath.Join(helmChartPath, "charts"))
	for _, dependencyPackage := range dependencyPackages {
		// Prepare the package and move it into charts/
		if err := local.RemoveAll(dependencyPackage.fs, PackageChartsDirpath); err != nil {
			return err
		}
		if err := dependencyPackage.Upstream.Pull(dependencyPackage.fs, PackageChartsDirpath); err != nil {
			return err
		}
		dependencyChartDirpath := filepath.Join(helmChartPath, "charts", dependencyPackage.Name)
		// Create charts/charts/<dependency.Name>
		if err = p.fs.MkdirAll(filepath.Dir(dependencyChartDirpath), os.ModePerm); err != nil {
			return err
		}
		// Move the generated chart from prepare into charts/charts/<dependency.Name>
		err := os.Rename(local.GetAbsPath(dependencyPackage.fs, PackageChartsDirpath), local.GetAbsPath(p.fs, dependencyChartDirpath))
		if err != nil {
			return err
		}
	}
	// Check if the newly generated Helm chart is valid
	chart, err := helmLoader.Load(local.GetAbsPath(p.fs, helmChartPath))
	if err != nil {
		return err
	}
	// Pick up all existing dependencies
	dependencyMap := make(map[string]*helmChart.Dependency, len(chart.Metadata.Dependencies))
	for _, dependency := range chart.Metadata.Dependencies {
		dependencyMap[dependency.Name] = dependency
	}
	// Update the Repository for each dependency
	newDependencies := make([]*helmChart.Dependency, len(dependencyPackages))
	for i, dependencyPackage := range dependencyPackages {
		d, ok := dependencyMap[dependencyPackage.Name]
		if !ok {
			d = &helmChart.Dependency{
				Name:      dependencyPackage.Name,
				Condition: fmt.Sprintf("%s.enabled", dependencyPackage.Name),
			}
		}
		d.Repository = fmt.Sprintf("file://./charts/%s", dependencyPackage.Name)
		// Version doesn't make a difference with local chart archives
		d.Version = ""
		newDependencies[i] = d
	}
	chart.Metadata.Dependencies = newDependencies
	if chart.Metadata.APIVersion == "v2" {
		newChartsYamlPath := filepath.Join(helmChartPath, "Chart.yaml")
		newChartYamlBytes, err := yaml.Marshal(chart.Metadata)
		if err != nil {
			return err
		}
		newChartYamlFile, err := p.fs.OpenFile(newChartsYamlPath, os.O_RDWR, os.ModePerm)
		if err != nil {
			return err
		}
		defer newChartYamlFile.Close()
		if _, err := newChartYamlFile.Write(newChartYamlBytes); err != nil {
			return err
		}
		return nil
	}
	newRequirementsYamlPath := filepath.Join(helmChartPath, "requirements.yaml")
	newRequirementsYamlBytes, err := yaml.Marshal(map[string][]*helmChart.Dependency{"dependencies": chart.Metadata.Dependencies})
	if err != nil {
		return err
	}
	newRequirementsYamlFile, err := p.fs.OpenFile(newRequirementsYamlPath, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer newRequirementsYamlFile.Close()
	if _, err := newRequirementsYamlFile.Write(newRequirementsYamlBytes); err != nil {
		return err
	}
	return nil
}

// GetDependencyPackages returns all dependencies that are within PackagesDependenciesDirpath as packages
func (p *Package) GetDependencyPackages() ([]*Package, error) {
	exists, err := local.PathExists(p.fs, PackageDependenciesDirpath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []*Package{}, nil
	}
	fileInfos, err := p.fs.ReadDir(PackageDependenciesDirpath)
	if err != nil {
		return nil, err
	}
	var dependencyPackages []*Package
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		dependencyName := fileInfo.Name()
		dependencyFs, err := p.fs.Chroot(p.PackageDependencyDirpath(dependencyName))
		if err != nil {
			return nil, err
		}
		p := &Package{
			Name:          dependencyName,
			ExportOptions: p.ExportOptions,
			CleanOptions:  p.CleanOptions,

			fs:     dependencyFs,
			repoFs: p.repoFs,
		}
		if err := p.ReadPackageOptions(); err != nil {
			return nil, err
		}
		dependencyPackages = append(dependencyPackages, p)
	}
	return dependencyPackages, nil
}

// LoadDependencyPackages takes all the subcharts in the package at helmChartPath and loads them into the PackagesDependenciesDirpath
// If the dependency already exists, it warns if the dependency's URL does not match the URL from upstream
func (p *Package) LoadDependencyPackages(helmChartPath string) error {
	chart, err := helmLoader.Load(local.GetAbsPath(p.fs, helmChartPath))
	if err != nil {
		return err
	}
	if chart.Lock == nil || chart.Lock.Dependencies == nil {
		return nil
	}
	for _, dependency := range chart.Lock.Dependencies {
		dependencyRoot := p.PackageDependencyDirpath(dependency.Name)
		// Parse upstream configuration
		dependencyURL, err := helmRepo.FindChartInRepoURL(
			dependency.Repository,
			dependency.Name,
			dependency.Version,
			"", "", "",
			helmGetter.All(&helmCli.EnvSettings{}),
		)
		if err != nil {
			return fmt.Errorf("Encountered error while trying to find the repository for dependency %s: %s", dependency.Name, err)
		}
		upstreamOptions := UpstreamOptions{URL: &dependencyURL}
		upstream, err := upstreamOptions.GetUpstream()
		if err != nil {
			return err
		}
		// Check if dependency already exists
		dependencyExists, err := local.PathExists(p.fs, dependencyRoot)
		if err != nil {
			return err
		}
		if !dependencyExists {
			// Create a directory for it
			if err := p.fs.MkdirAll(dependencyRoot, os.ModePerm); err != nil {
				return err
			}
			defer local.PruneEmptyDirsInPath(p.fs, dependencyRoot)
		}
		dependencyFs, err := p.fs.Chroot(dependencyRoot)
		if err != nil {
			return err
		}
		dependencyPackage := Package{
			Name:           dependency.Name,
			PackageVersion: p.PackageVersion,
			Upstream:       upstream,
			ExportOptions:  p.ExportOptions,
			CleanOptions:   p.CleanOptions,

			fs:     dependencyFs,
			repoFs: p.repoFs,
		}
		if !dependencyExists {
			// Write into the package.yaml
			if err = dependencyPackage.WritePackageOptions(); err != nil {
				return err
			}
		}
		// Check if the dependency needs to be modified
		if err := dependencyPackage.ReadPackageOptions(); err != nil {
			return err
		}
		dependencyOptions := dependencyPackage.GetPackageOptions()
		if *dependencyOptions.UpstreamOptions.URL != dependencyURL {
			logrus.Infof("Upgrade Notice: The URL for %s in upstream has been updated from %s to %s", dependency.Name, *dependencyOptions.UpstreamOptions.URL, dependencyURL)
		}
	}
	return nil
}
