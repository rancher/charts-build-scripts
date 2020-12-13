package helm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
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
