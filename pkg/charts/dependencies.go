package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// PrepareDependencies prepares all of the dependencies of a given chart and regenerates the requirements.yaml or Chart.yaml
func PrepareDependencies(pkgFs billy.Filesystem, mainHelmChartPath string, gcRootDir string) error {
	logrus.Infof("Loading dependencies for chart")
	if err := LoadDependencies(pkgFs, mainHelmChartPath, gcRootDir); err != nil {
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
	if err := utils.RemoveAll(pkgFs, dependenciesDestPath); err != nil {
		return err
	}
	if err := pkgFs.MkdirAll(dependenciesDestPath, os.ModePerm); err != nil {
		return err
	}
	for dependencyName, dependency := range dependencyMap {
		// Pull in the dependency
		dependencyRootPath := filepath.Join(gcRootDir, GeneratedChangesDependenciesDirpath, dependencyName)
		dependencyFs, err := pkgFs.Chroot(dependencyRootPath)
		if err != nil {
			return err
		}
		if utils.RemoveAll(dependencyFs, dependency.WorkingDir); err != nil {
			return err
		}
		if err := dependency.Upstream.Pull(dependencyFs, dependency.WorkingDir); err != nil {
			return err
		}
		// Move the generated chart into the dependencyDestPath
		absDependencyChartSrcPath := utils.GetAbsPath(dependencyFs, dependency.WorkingDir)
		absDependencyChartDestPath := utils.GetAbsPath(pkgFs, filepath.Join(dependenciesDestPath, dependencyName))
		if err = os.Rename(absDependencyChartSrcPath, absDependencyChartDestPath); err != nil {
			return err
		}
	}
	logrus.Infof("Updating chart metadata with dependencies")
	return UpdateChartMetadataWithDependencies(pkgFs, mainHelmChartPath, dependencyMap)
}

func getMainChartUpstreamOptions(pkgFs billy.Filesystem, gcRootDir string) (*options.UpstreamOptions, error) {
	packageOpts, err := options.LoadPackageOptionsFromFile(pkgFs, PackageOptionsFilepath)
	if err != nil {
		return nil, fmt.Errorf("Unable to read %s for PackageOptions: %s", PackageOptionsFilepath, err)
	}
	if gcRootDir == GeneratedChangesDirpath {
		return &packageOpts.MainChartOptions.UpstreamOptions, nil
	}
	additionalChartPrefix := filepath.Join(GeneratedChangesDirpath, GeneratedChangesAdditionalChartDirpath)
	if !strings.HasPrefix(gcRootDir, additionalChartPrefix) {
		return nil, fmt.Errorf("Unable to figure out main chart options given generated changes root directory at %s", gcRootDir)
	}
	// Get additional chart working dir by parsing chart name out of generated-changes/additional-charts/{chart-name}/generated-changes
	additionalChartWorkingDir, err := utils.MovePath(filepath.Dir(gcRootDir), additionalChartPrefix, "")
	if err != nil {
		return nil, err
	}
	for _, additionalChartOption := range packageOpts.AdditionalChartOptions {
		if additionalChartOption.WorkingDir == additionalChartWorkingDir {
			return additionalChartOption.UpstreamOptions, nil
		}
	}
	return nil, fmt.Errorf("Generated changes root directory %s does not point to a valid additional chart", gcRootDir)
}

// LoadDependencies takes all existing subcharts in the package and loads them into the gcRootDir as dependencies
func LoadDependencies(pkgFs billy.Filesystem, mainHelmChartPath string, gcRootDir string) error {
	// Get main chart options
	mainChartUpstreamOpts, err := getMainChartUpstreamOptions(pkgFs, gcRootDir)
	if err != nil {
		return err
	}
	// Load the main chart
	mainChart, err := helmLoader.Load(utils.GetAbsPath(pkgFs, mainHelmChartPath))
	if err != nil {
		return err
	}
	// Handle local chart archives first since version numbers don't make a difference
	for _, dependency := range mainChart.Metadata.Dependencies {
		if !strings.HasPrefix(dependency.Repository, "file://") {
			continue
		}
		dependencyName := dependency.Name
		dependencyOptionsPath := filepath.Join(gcRootDir, GeneratedChangesDependenciesDirpath, dependencyName, DependencyOptionsFilepath)
		dependencyExists, err := utils.PathExists(pkgFs, dependencyOptionsPath)
		if err != nil {
			return err
		}
		if dependencyExists {
			logrus.Infof("Found chart options for %s in %s", dependencyName, dependencyOptionsPath)
			continue
		}
		subdirectory := filepath.Join(strings.TrimPrefix(dependency.Repository, "file://"), dependencyName)
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
		dependencyOptionsPath := filepath.Join(gcRootDir, GeneratedChangesDependenciesDirpath, dependencyName, DependencyOptionsFilepath)
		// Check if dependency already exists
		dependencyExists, err := utils.PathExists(pkgFs, dependencyOptionsPath)
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
			return fmt.Errorf("Encountered error while trying to find the repository for dependency %s: %s", dependency.Name, err)
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
	// Check whether any dependenices exist
	dependenciesRootPath := filepath.Join(gcRootDir, GeneratedChangesDependenciesDirpath)
	exists, err := utils.PathExists(pkgFs, dependenciesRootPath)
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
		dependencyOptionsPath := filepath.Join(dependenciesRootPath, name, DependencyOptionsFilepath)
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
