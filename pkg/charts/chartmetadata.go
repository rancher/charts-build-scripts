package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// UpdateChartMetadataWithDependencies updates either the requirements.yaml or Chart.yaml for the dependencies provided
// For each dependency in dependencies, it will replace the entry in the requirements.yaml / Chart.yaml with a URL pointing to the local chart archive
func UpdateChartMetadataWithDependencies(pkgFs billy.Filesystem, mainHelmChartPath string, dependencyMap map[string]*Chart) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(utils.GetAbsPath(pkgFs, mainHelmChartPath))
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
		return chart.Metadata.Dependencies[i].Name > chart.Metadata.Dependencies[i].Name
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
	file, err := pkgFs.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(dataBytes); err != nil {
		return err
	}
	return nil
}
