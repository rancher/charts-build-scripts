package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	helmRepo "helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

// PrepareDependencies prepares all of the dependencies of a given chart and regenerates the requirements.yaml or Chart.yaml
func PrepareDependencies(rootFs, pkgFs billy.Filesystem, mainHelmChartPath string, gcRootDir string, ignoreDependencies []string) error {
	logrus.Infof("Loading dependencies for chart")
	ignoreDependencyMap := make(map[string]bool)
	for _, dep := range ignoreDependencies {
		ignoreDependencyMap[dep] = true
	}
	if err := LoadDependencies(pkgFs, mainHelmChartPath, gcRootDir, ignoreDependencyMap); err != nil {
		return err
	}
	dependencyMap, err := GetDependencyMap(pkgFs, gcRootDir)
	if err != nil {
		return err
	}
	if len(dependencyMap) == 0 {
		return nil
	}
	// Remove all existing stuff from the charts/ directory by deleting and recreating it
	dependenciesDestPath := filepath.Join(mainHelmChartPath, "charts")
	if err := filesystem.RemoveAll(pkgFs, dependenciesDestPath); err != nil {
		return err
	}
	if err := pkgFs.MkdirAll(dependenciesDestPath, os.ModePerm); err != nil {
		return err
	}
	for dependencyName, dependency := range dependencyMap {
		// Pull in the dependency
		dependencyRootPath := filepath.Join(gcRootDir, path.GeneratedChangesDependenciesDir, dependencyName)
		dependencyFs, err := pkgFs.Chroot(dependencyRootPath)
		if err != nil {
			return err
		}
		absDependencyChartSrcPath := filesystem.GetAbsPath(dependencyFs, dependency.WorkingDir)
		absDependencyChartDestPath := filesystem.GetAbsPath(pkgFs, filepath.Join(dependenciesDestPath, dependencyName))
		if dependency.Upstream.IsWithinPackage() {
			// Copy the local chart into dependencyDestPath
			repositoryDependencyChartsSrcPath, err := filesystem.GetRelativePath(rootFs, absDependencyChartSrcPath)
			if err != nil {
				return fmt.Errorf("encountered error while getting absolute path of %s in %s: %s", absDependencyChartSrcPath, rootFs.Root(), err)
			}
			repositoryDependencyChartsDestPath, err := filesystem.GetRelativePath(rootFs, absDependencyChartDestPath)
			if err != nil {
				return fmt.Errorf("encountered error while getting absolute path of %s in %s: %s", absDependencyChartDestPath, rootFs.Root(), err)
			}
			if err = filesystem.CopyDir(rootFs, repositoryDependencyChartsSrcPath, repositoryDependencyChartsDestPath); err != nil {
				return fmt.Errorf("encountered while copying local dependency: %s", err)
			}
			if err = helm.UpdateHelmMetadataWithName(rootFs, repositoryDependencyChartsDestPath, dependencyName); err != nil {
				return err
			}
			continue
		}
		if filesystem.RemoveAll(dependencyFs, dependency.WorkingDir); err != nil {
			return err
		}
		if err := dependency.Upstream.Pull(rootFs, dependencyFs, dependency.WorkingDir); err != nil {
			return err
		}
		// Move the generated chart into the dependencyDestPath
		if err = os.Rename(absDependencyChartSrcPath, absDependencyChartDestPath); err != nil {
			return err
		}
		if err = helm.UpdateHelmMetadataWithName(pkgFs, filepath.Join(dependenciesDestPath, dependencyName), dependencyName); err != nil {
			return err
		}
	}
	logrus.Infof("Updating chart metadata with dependencies")
	return UpdateHelmMetadataWithDependencies(pkgFs, mainHelmChartPath, dependencyMap)
}

func getMainChartUpstreamOptions(pkgFs billy.Filesystem, gcRootDir string) (*options.UpstreamOptions, error) {
	packageOpts, err := options.LoadPackageOptionsFromFile(pkgFs, path.PackageOptionsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s for PackageOptions: %s", path.PackageOptionsFile, err)
	}
	if gcRootDir == path.GeneratedChangesDir {
		return &packageOpts.MainChartOptions.UpstreamOptions, nil
	}
	additionalChartPrefix := filepath.Join(path.GeneratedChangesDir, path.GeneratedChangesAdditionalChartDir)
	if !strings.HasPrefix(gcRootDir, additionalChartPrefix) {
		return nil, fmt.Errorf("unable to figure out main chart options given generated changes root directory at %s", gcRootDir)
	}
	// Get additional chart working dir by parsing chart name out of generated-changes/additional-charts/{chart-name}/generated-changes
	additionalChartWorkingDir, err := filesystem.MovePath(filepath.Dir(gcRootDir), additionalChartPrefix, "")
	if err != nil {
		return nil, err
	}
	for _, additionalChartOption := range packageOpts.AdditionalChartOptions {
		if additionalChartOption.WorkingDir == additionalChartWorkingDir {
			return additionalChartOption.UpstreamOptions, nil
		}
	}
	return nil, fmt.Errorf("generated changes root directory %s does not point to a valid additional chart", gcRootDir)
}

// LoadDependencies takes all existing subcharts in the package and loads them into the gcRootDir as dependencies
func LoadDependencies(pkgFs billy.Filesystem, mainHelmChartPath string, gcRootDir string, ignoreDependencyMap map[string]bool) error {
	// Get main chart options
	mainChartUpstreamOpts, err := getMainChartUpstreamOptions(pkgFs, gcRootDir)
	if err != nil {
		return err
	}
	// Load the main chart
	mainChart, err := helmLoader.Load(filesystem.GetAbsPath(pkgFs, mainHelmChartPath))
	if err != nil {
		return err
	}
	var numChartsRemoved int
	for i, dependency := range mainChart.Metadata.Dependencies {
		if ignoreDependencyMap[dependency.Name] {
			// delete this dependency
			mainChart.Metadata.Dependencies = append(mainChart.Metadata.Dependencies[:i-numChartsRemoved], mainChart.Metadata.Dependencies[i+1-numChartsRemoved:]...)
			numChartsRemoved++
		}
	}
	// Handle local chart archives first since version numbers don't make a difference
	for _, dependency := range mainChart.Metadata.Dependencies {
		if !strings.HasPrefix(dependency.Repository, "file://") {
			continue
		}
		dependencyName := dependency.Name
		dependencyOptionsPath := filepath.Join(gcRootDir, path.GeneratedChangesDependenciesDir, dependencyName, path.DependencyOptionsFile)
		dependencyExists, err := filesystem.PathExists(pkgFs, dependencyOptionsPath)
		if err != nil {
			return err
		}
		if dependencyExists {
			logrus.Infof("found chart options for %s in %s", dependencyName, dependencyOptionsPath)
			continue
		}
		subdirectory := filepath.Join(filepath.Dir(strings.TrimPrefix(dependency.Repository, "file://")), dependencyName)
		if mainChartUpstreamOpts.Subdirectory != nil {
			subdirectory = filepath.Join(*mainChartUpstreamOpts.Subdirectory, subdirectory)
		}
		dependencyPackageOptions := options.ChartOptions{
			UpstreamOptions: options.UpstreamOptions{
				URL:          mainChartUpstreamOpts.URL,
				Subdirectory: &subdirectory,
				Commit:       mainChartUpstreamOpts.Commit,
			},
		}
		if err := dependencyPackageOptions.WriteToFile(pkgFs, dependencyOptionsPath); err != nil {
			return err
		}
	}
	// Handle remote chart archives that don't have fixed version numbers
	if mainChart.Lock == nil || mainChart.Lock.Dependencies == nil {
		// No dependencies to parse
		return nil
	}
	for _, dependency := range mainChart.Lock.Dependencies {
		dependencyName := dependency.Name
		dependencyOptionsPath := filepath.Join(gcRootDir, path.GeneratedChangesDependenciesDir, dependencyName, path.DependencyOptionsFile)
		// Check if dependency already exists
		dependencyExists, err := filesystem.PathExists(pkgFs, dependencyOptionsPath)
		if err != nil {
			return err
		}
		if dependencyExists {
			logrus.Infof("Found chart options for %s in %s", dependencyName, dependencyOptionsPath)
			continue
		}
		logrus.Infof("Looking for %s within repository %s", dependencyName, dependency.Repository)
		dependencyURL, err := helmRepo.FindChartInRepoURL(
			dependency.Repository,
			dependencyName,
			dependency.Version,
			"", "", "",
			helmGetter.All(&helmCli.EnvSettings{}),
		)
		if err != nil {
			return fmt.Errorf("encountered error while trying to find the repository for dependency %s: %s", dependency.Name, err)
		}
		dependencyPackageOptions := options.ChartOptions{
			UpstreamOptions: options.UpstreamOptions{
				URL: dependencyURL,
			},
		}
		if err := dependencyPackageOptions.WriteToFile(pkgFs, dependencyOptionsPath); err != nil {
			return err
		}
	}
	return nil
}

// GetDependencyMap gets a map between a dependency's name and a Chart representing that dependency for all rooted at gcRootDir
func GetDependencyMap(pkgFs billy.Filesystem, gcRootDir string) (map[string]*Chart, error) {
	dependencyMap := make(map[string]*Chart)
	// Check whether any dependencies exist
	dependenciesRootPath := filepath.Join(gcRootDir, path.GeneratedChangesDependenciesDir)
	exists, err := filesystem.PathExists(pkgFs, dependenciesRootPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return dependencyMap, nil
	}
	// Read through the dependencies
	fileInfos, err := pkgFs.ReadDir(dependenciesRootPath)
	if err != nil {
		return nil, err
	}
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		name := fileInfo.Name()
		dependencyOptionsPath := filepath.Join(dependenciesRootPath, name, path.DependencyOptionsFile)
		dependencyOptions, err := options.LoadChartOptionsFromFile(pkgFs, dependencyOptionsPath)
		if err != nil {
			return nil, err
		}
		dependencyChart, err := GetChartFromOptions(dependencyOptions)
		if err != nil {
			return nil, err
		}
		dependencyMap[name] = &dependencyChart
	}
	return dependencyMap, nil
}

// UpdateHelmMetadataWithDependencies updates either the requirements.yaml or Chart.yaml for the dependencies provided
// For each dependency in dependencies, it will replace the entry in the requirements.yaml / Chart.yaml with a URL pointing to the local chart archive
func UpdateHelmMetadataWithDependencies(fs billy.Filesystem, mainHelmChartPath string, dependencyMap map[string]*Chart) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return err
	}
	// Pick up all existing dependencies tracked by Helm by name
	helmDependencyMap := make(map[string]*helmChart.Dependency, len(dependencyMap))
	for _, dependency := range chart.Metadata.Dependencies {
		if _, ok := dependencyMap[dependency.Name]; !ok {
			continue
		}
		helmDependencyMap[dependency.Name] = dependency
	}
	// Update the Repository for each dependency
	for dependencyName := range dependencyMap {
		d, ok := helmDependencyMap[dependencyName]
		if !ok {
			// Dependency does not exist, so we add it to the list
			d = &helmChart.Dependency{
				Name:      dependencyName,
				Condition: fmt.Sprintf("%s.enabled", dependencyName),
			}
			helmDependencyMap[dependencyName] = d
		}
		d.Version = "" // Local chart archives don't need a version
		d.Repository = fmt.Sprintf("file://./charts/%s", dependencyName)
	}
	// Convert the map back into a list
	chart.Metadata.Dependencies = make([]*helmChart.Dependency, len(helmDependencyMap))
	i := 0
	for _, dependency := range helmDependencyMap {
		chart.Metadata.Dependencies[i] = dependency
		i++
	}
	// Sort the list
	sort.SliceStable(chart.Metadata.Dependencies, func(i, j int) bool {
		return chart.Metadata.Dependencies[i].Name < chart.Metadata.Dependencies[j].Name
	})
	// Write to either the Chart.yaml or the requirements.yaml, depending on the version
	var path string
	var data interface{}
	if chart.Metadata.APIVersion == "v2" {
		// TODO(aiyengar2): fully test apiVersion V2 charts and remove this warning
		logrus.Warnf("Detected 'apiVersion:v2' within Chart.yaml; these types of charts require additional testing")
		path = filepath.Join(mainHelmChartPath, "Chart.yaml")
		data = chart.Metadata
	} else {
		path = filepath.Join(mainHelmChartPath, "requirements.yaml")
		data = map[string][]*helmChart.Dependency{
			"dependencies": chart.Metadata.Dependencies,
		}
	}
	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	exists, err := filesystem.PathExists(fs, path)
	if err != nil {
		return err
	}
	var file billy.File
	if !exists {
		file, err = filesystem.CreateFileAndDirs(fs, path)
	} else {
		file, err = fs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(dataBytes); err != nil {
		return err
	}
	return nil
}
