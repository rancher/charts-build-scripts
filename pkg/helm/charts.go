package helm

import (
	"errors"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"helm.sh/helm/v3/pkg/chart"
	helmChartutil "helm.sh/helm/v3/pkg/chartutil"
)

// LoadChartYaml will load a given chart.yaml file for the target chart and version
func LoadChartYaml(rootFs billy.Filesystem, chart string, chartVersion string) (*chart.Metadata, error) {
	// Get Chart.yaml path and load it
	chartYamlPath := path.RepositoryChartsDir + "/" + chart + "/" + chartVersion + "/Chart.yaml"
	absChartPath := filesystem.GetAbsPath(rootFs, chartYamlPath)

	// Load Chart.yaml file
	chartMetadata, err := helmChartutil.LoadChartfile(absChartPath)
	if err != nil {
		return nil, errors.New("could not load: " + chartYamlPath + " err: " + err.Error())
	}

	return chartMetadata, nil
}
