package helm

import (
	"errors"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmChartUtil "helm.sh/helm/v3/pkg/chartutil"
)

// LoadChartYaml will load a given chart.yaml file for the target chart and version
func LoadChartYaml(rootFs billy.Filesystem, chart, chartVersion string) (*helmChart.Metadata, error) {
	// Get Chart.yaml path and load it
	chartYamlPath := path.RepositoryChartsDir + "/" + chart + "/" + chartVersion + "/Chart.yaml"
	absChartPath := filesystem.GetAbsPath(rootFs, chartYamlPath)

	// Load Chart.yaml file
	chartMetadata, err := helmChartUtil.LoadChartfile(absChartPath)
	if err != nil {
		return nil, errors.New("could not load: " + chartYamlPath + " err: " + err.Error())
	}

	return chartMetadata, nil
}
