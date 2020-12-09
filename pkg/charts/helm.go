package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// ExportHelmChart creates a Helm chart archive and an unarchived Helm chart at RepositoryAssetDirpath and RepositoryChartDirPath
// helmChartPath is a relative path (rooted at the package level) that contains the chart.
// packageAssetsPath is a relative path (rooted at the repository level) where the generated chart archive will be placed
// packageChartsPath is a relative path (rooted at the repository level) where the generated chart will be placed
func ExportHelmChart(rootFs, fs billy.Filesystem, helmChartPath string, chartVersion string, packageAssetsDirpath, packageChartsDirpath string) error {
	// Try to load the chart to see if it can be exported
	absHelmChartPath := utils.GetAbsPath(fs, helmChartPath)
	chart, err := helmLoader.Load(absHelmChartPath)
	if err != nil {
		return fmt.Errorf("Could not load Helm chart: %s", err)
	}
	if err := chart.Validate(); err != nil {
		return fmt.Errorf("Failed while trying to validate Helm chart: %s", err)
	}
	chartVersion = chart.Metadata.Version + chartVersion

	// All assets of each chart in a package are placed in a flat directory containing all versions
	chartAssetsDirpath := packageAssetsDirpath
	// All generated charts are indexed by version and the working directory
	chartChartsDirpath := filepath.Join(packageChartsDirpath, chart.Metadata.Name, chartVersion)
	// Create directories
	if err := rootFs.MkdirAll(chartAssetsDirpath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory for assets at %s: %s", chartAssetsDirpath, err)
	}
	defer utils.PruneEmptyDirsInPath(rootFs, chartAssetsDirpath)
	if err := rootFs.MkdirAll(chartChartsDirpath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory for charts at %s: %s", chartChartsDirpath, err)
	}
	defer utils.PruneEmptyDirsInPath(rootFs, chartChartsDirpath)
	// Run helm package
	pkg := helmAction.NewPackage()
	pkg.Version = chartVersion
	pkg.Destination = utils.GetAbsPath(rootFs, chartAssetsDirpath)
	pkg.DependencyUpdate = false
	absTgzPath, err := pkg.Run(absHelmChartPath, nil)
	if err != nil {
		return err
	}
	tgzPath, err := utils.GetRelativePath(rootFs, absTgzPath)
	if err != nil {
		return err
	}
	logrus.Infof("Generated archive: %s", tgzPath)
	// Unarchive the generated package
	if err := utils.UnarchiveTgz(rootFs, tgzPath, "", chartChartsDirpath, true); err != nil {
		return err
	}
	logrus.Infof("Generated chart: %s", chartChartsDirpath)
	return nil
}

// CreateOrUpdateHelmIndex either creates or updates the index.yaml for the repository this package is within
func CreateOrUpdateHelmIndex(rootFs billy.Filesystem) error {
	absRepositoryAssetsDirpath := utils.GetAbsPath(rootFs, RepositoryAssetsDirpath)
	absRepositoryHelmIndexFilepath := utils.GetAbsPath(rootFs, RepositoryHelmIndexFilepath)
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

// UpdateHelmMetadataWithDependencies updates either the requirements.yaml or Chart.yaml for the dependencies provided
// For each dependency in dependencies, it will replace the entry in the requirements.yaml / Chart.yaml with a URL pointing to the local chart archive
func UpdateHelmMetadataWithDependencies(fs billy.Filesystem, mainHelmChartPath string, dependencyMap map[string]*Chart) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(utils.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return err
	}
	// Pick up all existing dependencies tracked by Helm by name
	helmDependencyMap := make(map[string]*helmChart.Dependency, len(chart.Metadata.Dependencies))
	for _, dependency := range chart.Metadata.Dependencies {
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
	file, err := fs.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(dataBytes); err != nil {
		return err
	}
	return nil
}

// UpdateHelmMetadataWithName updates the name of the chart in the metadata
func UpdateHelmMetadataWithName(fs billy.Filesystem, mainHelmChartPath string, name string) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(utils.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return err
	}
	chart.Metadata.Name = name
	// Write to either the Chart.yaml or the requirements.yaml, depending on the version
	path := filepath.Join(mainHelmChartPath, "Chart.yaml")
	data := chart.Metadata
	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	file, err := fs.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(dataBytes); err != nil {
		return err
	}
	return nil
}

// CopyCRDsFromChart copies the CRDs from a chart to another chart
func CopyCRDsFromChart(fs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := utils.RemoveAll(fs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	srcCRDsDirpath := filepath.Join(srcHelmChartPath, srcCRDsDir)
	dstCRDsDirpath := filepath.Join(dstHelmChartPath, destCRDsDir)
	logrus.Infof("Copying CRDs from %s to %s", srcCRDsDirpath, dstCRDsDirpath)
	return utils.CopyDir(fs, srcCRDsDirpath, dstCRDsDirpath)
}

// DeleteCRDsFromChart deletes all the CRDs loaded by a chart
func DeleteCRDsFromChart(fs billy.Filesystem, helmChartPath string) error {
	chart, err := helmLoader.Load(utils.GetAbsPath(fs, helmChartPath))
	if err != nil {
		return fmt.Errorf("Could not load Helm chart: %s", err)
	}
	for _, crd := range chart.CRDObjects() {
		crdFilepath := filepath.Join(helmChartPath, crd.File.Name)
		exists, err := utils.PathExists(fs, crdFilepath)
		if err != nil {
			return err
		}
		if exists {
			logrus.Infof("Deleting %s", crdFilepath)
			if err := fs.Remove(crdFilepath); err != nil {
				return err
			}
		}
		if err := utils.PruneEmptyDirsInPath(fs, crdFilepath); err != nil {
			return err
		}
	}
	return nil
}

//TrimRCVersionFromHelmMetadataVersion updates the name of the chart in the metadata
func TrimRCVersionFromHelmMetadataVersion(fs billy.Filesystem, mainHelmChartPath string) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(utils.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return err
	}
	chart.Metadata.Version = strings.SplitN(chart.Metadata.Version, "-rc", 2)[0]
	// Write to either the Chart.yaml or the requirements.yaml, depending on the version
	path := filepath.Join(mainHelmChartPath, "Chart.yaml")
	data := chart.Metadata
	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	file, err := fs.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(dataBytes); err != nil {
		return err
	}
	return nil
}
