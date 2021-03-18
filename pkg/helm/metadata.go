package helm

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"gopkg.in/yaml.v2"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

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

// TrimRCVersionFromHelmChart updates the chart's metadata to remove RC versions
func TrimRCVersionFromHelmChart(fs billy.Filesystem, mainHelmChartPath string) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return err
	}
	chart.Metadata.Version = strings.SplitN(chart.Metadata.Version, "-rc", 2)[0]
	// Remove RC version from specific annotations
	for annotation, val := range chart.Metadata.Annotations {
		exportAnnotation, ok := exportAnnotations[annotation]
		if !ok {
			// No need to update annotations
			continue
		}
		newVal, updated := exportAnnotation.dropRCVersion(val)
		if !updated {
			continue
		}
		chart.Metadata.Annotations[annotation] = newVal
	}
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
