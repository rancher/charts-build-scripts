package helm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"sigs.k8s.io/yaml"

	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmChartutil "helm.sh/helm/v3/pkg/chartutil"
)

// GetHelmMetadataVersion gets the version of a Helm chart as defined in its Chart.yaml
func GetHelmMetadataVersion(ctx context.Context, fs billy.Filesystem, mainHelmChartPath string) (string, error) {
	logger.Log(ctx, slog.LevelInfo, "get helm metadata version")
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, mainHelmChartPath))
	if err != nil {
		return "", err
	}

	logger.Log(ctx, slog.LevelDebug, "metadata", slog.String("version", chart.Metadata.Version))
	return chart.Metadata.Version, nil
}

// UpdateHelmMetadataWithName updates the name of the chart in the metadata
func UpdateHelmMetadataWithName(ctx context.Context, fs billy.Filesystem, mainHelmChartPath string, name string) error {
	logger.Log(ctx, slog.LevelInfo, "update helm metadata with name", slog.String("name", name))
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
func ConvertToHelmChart(ctx context.Context, fs billy.Filesystem, dirPath string) error {
	logger.Log(ctx, slog.LevelInfo, "convert helm chart")

	// Check if the Chart.yaml already exists, indicating this is a Helm chart already
	chartYamlPath := filepath.Join(dirPath, "Chart.yaml")
	exists, err := filesystem.PathExists(ctx, fs, chartYamlPath)
	if err != nil {
		return fmt.Errorf("encountered error while trying to verify if %s already exists", chartYamlPath)
	}
	if exists {
		// Standardize Chart.yaml
		return StandardizeChartYaml(ctx, fs, dirPath)
	}

	// Ensure dirPath exists and is a directory
	fileInfo, err := fs.Lstat(dirPath)
	if err != nil {
		return fmt.Errorf("cannot convert %s to Helm chart: %s", dirPath, err)
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("provided dirPath %s is not a directory: cannot convert to Helm chart", dirPath)
	}

	// Move all .yaml files to templates directory
	moveYamlToTemplates := func(ctx context.Context, fs billy.Filesystem, path string, isDir bool) error {
		logger.Log(ctx, slog.LevelDebug, "moving YAML files to templates", slog.String("path", path))

		if isDir {
			// skip creating directories since we will create them when we copy the file anyways
			return nil
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		// destPath should be the path to chart + templates + whatever path the original path of the file was within the chart
		dirPathWithinChart, err := filesystem.MovePath(ctx, filepath.Dir(path), dirPath, "")
		if err != nil {
			return err
		}
		destPath, err := filesystem.MovePath(ctx, path, dirPath, filepath.Join(dirPath, "templates", dirPathWithinChart))
		if err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelDebug, "moving", slog.String("from", path), slog.String("to", destPath))
		return fs.Rename(path, destPath)
	}

	if err := filesystem.WalkDir(ctx, fs, dirPath, moveYamlToTemplates); err != nil {
		return fmt.Errorf("unable to move YAML files in %s to templates: %s", dirPath, err)
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

	logger.Log(ctx, slog.LevelInfo, "initializing helm chart", slog.String("path", chartYamlPath))
	return helmChartutil.SaveChartfile(filesystem.GetAbsPath(fs, chartYamlPath), chartMetadata)
}

// StandardizeChartYaml marshalls and unmarshalls the Chart.yaml to ensure that it is ordered as expected
func StandardizeChartYaml(ctx context.Context, fs billy.Filesystem, dirPath string) error {
	chartYamlPath := filepath.Join(dirPath, "Chart.yaml")
	absChartYamlPath := filesystem.GetAbsPath(fs, chartYamlPath)
	logger.Log(ctx, slog.LevelInfo, "standardize helm chart", slog.String("path", absChartYamlPath))

	chartMetadata, err := helmChartutil.LoadChartfile(absChartYamlPath)
	if err != nil {
		return fmt.Errorf("could not load %s: %s", chartYamlPath, err)
	}

	if err := helmChartutil.SaveChartfile(absChartYamlPath, chartMetadata); err != nil {
		return fmt.Errorf("could not reformat Chart.yaml in %s: %s", chartYamlPath, err)
	}

	return nil
}
