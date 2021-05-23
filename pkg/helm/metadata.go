package helm

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	"sigs.k8s.io/yaml"
)

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

// UpdateHelmMetadataWithVersion updates the version of the chart in the metadata
func UpdateHelmMetadataWithVersion(fs billy.Filesystem, mainHelmChartPath string, version string) error {
	// Check if Helm chart is valid
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return err
	}
	chart.Metadata.Version = version
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
