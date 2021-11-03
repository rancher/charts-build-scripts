package helm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmChartutil "helm.sh/helm/v3/pkg/chartutil"
)

// GetHelmMetadataVersion gets the version of a Helm chart as defined in its Chart.yaml
func GetHelmMetadataVersion(fs billy.Filesystem, mainHelmChartPath string) (string, error) {
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return "", err
	}
	return chart.Metadata.Version, nil
}

// UpdateHelmMetadataWithName updates the name of the chart in the metadata
func UpdateHelmMetadataWithName(fs billy.Filesystem, mainHelmChartPath string, name string) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, mainHelmChartPath))
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
	file, err := fs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(dataBytes); err != nil {
		return err
	}
	return nil
}

// ConvertToHelmChart converts a given path to a Helm chart.
// It does so by moving all YAML files to templates and creating a dummy Chart.yaml and values.yaml
func ConvertToHelmChart(fs billy.Filesystem, dirpath string) error {
	// Check if the Chart.yaml already exists, indicating this is a Helm chart already
	chartYamlPath := filepath.Join(dirpath, "Chart.yaml")
	exists, err := filesystem.PathExists(fs, chartYamlPath)
	if err != nil {
		return fmt.Errorf("encountered error while trying to verify if %s already exists", chartYamlPath)
	}
	if exists {
		// Standardize Chart.yaml
		return StandardizeChartYaml(fs, dirpath)
	}
	// Ensure dirpath exists and is a directory
	fileInfo, err := fs.Lstat(dirpath)
	if err != nil {
		return fmt.Errorf("cannot convert %s to Helm chart: %s", dirpath, err)
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("provided dirpath %s is not a directory: cannot convert to Helm chart", dirpath)
	}
	logrus.Infof("Converting %s into a Helm chart", dirpath)
	// Move all .yaml files to templates directory
	moveYamlToTemplates := func(fs billy.Filesystem, path string, isDir bool) error {
		if isDir {
			// skip creating directories since we will create them when we copy the file anyways
			return nil
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}
		// destPath should be the path to chart + templates + whatever path the original path of the file was within the chart
		dirpathWithinChart, err := filesystem.MovePath(filepath.Dir(path), dirpath, "")
		if err != nil {
			return err
		}
		destPath, err := filesystem.MovePath(path, dirpath, filepath.Join(dirpath, "templates", dirpathWithinChart))
		if err != nil {
			return err
		}
		logrus.Debugf("moving %s to %s", path, destPath)
		return fs.Rename(path, destPath)
	}
	if err := filesystem.WalkDir(fs, dirpath, moveYamlToTemplates); err != nil {
		return fmt.Errorf("unable to move YAML files in %s to templates: %s", dirpath, err)
	}
	// Initialize dummy Chart.yaml
	chartMetadata := &helmChart.Metadata{
		Name:        "OVERRIDE_HELM_CHART_NAME_HERE",
		Description: "A Helm chart for Kubernetes",
		Type:        "application",
		Version:     "0.1.0",
		AppVersion:  "0.1.0",
		APIVersion:  helmChart.APIVersionV2,
	}
	logrus.Infof("Initializing %s", chartYamlPath)
	return helmChartutil.SaveChartfile(filesystem.GetAbsPath(fs, chartYamlPath), chartMetadata)
}

// StandardizeChartYaml marshalls and unmarshalls the Chart.yaml to ensure that it is ordered as expected
func StandardizeChartYaml(fs billy.Filesystem, dirpath string) error {
	chartYamlPath := filepath.Join(dirpath, "Chart.yaml")
	logrus.Debugf("Standardizing order of %s", chartYamlPath)
	chartMetadata, err := helmChartutil.LoadChartfile(filesystem.GetAbsPath(fs, chartYamlPath))
	if err != nil {
		return fmt.Errorf("could not load %s: %s", chartYamlPath, err)
	}
	if err := helmChartutil.SaveChartfile(filesystem.GetAbsPath(fs, chartYamlPath), chartMetadata); err != nil {
		return fmt.Errorf("could not reformat Chart.yaml in %s: %s", chartYamlPath, err)
	}
	return nil
}
