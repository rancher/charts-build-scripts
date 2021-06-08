package helm

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/sirupsen/logrus"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

const (
	NumPatchDigits = 2
)

var (
	PatchNumMultiplier = uint64(math.Pow10(2))
	MaxPatchNum        = PatchNumMultiplier - 1
)

// ExportHelmChart creates a Helm chart archive and an unarchived Helm chart at RepositoryAssetDirpath and RepositoryChartDirPath
// helmChartPath is a relative path (rooted at the package level) that contains the chart.
// packageAssetsPath is a relative path (rooted at the repository level) where the generated chart archive will be placed
// packageChartsPath is a relative path (rooted at the repository level) where the generated chart will be placed
func ExportHelmChart(rootFs, fs billy.Filesystem, helmChartPath string, packageVersion *int, version *semver.Version, upstreamChartVersion, packageAssetsDirpath, packageChartsDirpath string) error {
	// Try to load the chart to see if it can be exported
	absHelmChartPath := filesystem.GetAbsPath(fs, helmChartPath)
	chart, err := helmLoader.Load(absHelmChartPath)
	if err != nil {
		return fmt.Errorf("Could not load Helm chart: %s", err)
	}
	if err := chart.Validate(); err != nil {
		return fmt.Errorf("Failed while trying to validate Helm chart: %s", err)
	}
	chartVersionSemver, err := semver.Make(chart.Metadata.Version)
	if err != nil {
		return fmt.Errorf("Cannot parse original chart version %s as valid semver", chart.Metadata.Version)
	}
	if version != nil {
		chartVersionSemver = *version
	} else if packageVersion != nil {
		// Add packageVersion as string, preventing errors due to leading 0s
		if uint64(*packageVersion) >= MaxPatchNum {
			return fmt.Errorf("Maximum number of packageVersions is %d, found %d", MaxPatchNum, packageVersion)
		}
		if uint64(*packageVersion) > 0 {
			chartVersionSemver.Patch = PatchNumMultiplier*chartVersionSemver.Patch + uint64(*packageVersion)
		}
	}

	if len(upstreamChartVersion) > 0 && upstreamChartVersion != chartVersionSemver.String() {
		// Add buildMetadataFlag for forked charts
		chartVersionSemver.Build = append(chartVersionSemver.Build, fmt.Sprintf("up%s", upstreamChartVersion))
	}
	chartVersion := chartVersionSemver.String()

	// All assets of each chart in a package are placed in a flat directory containing all versions
	chartAssetsDirpath := packageAssetsDirpath
	// All generated charts are indexed by version and the working directory
	chartChartsDirpath := filepath.Join(packageChartsDirpath, chart.Metadata.Name, chartVersion)
	// Create directories
	if err := rootFs.MkdirAll(chartAssetsDirpath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory for assets at %s: %s", chartAssetsDirpath, err)
	}
	defer filesystem.PruneEmptyDirsInPath(rootFs, chartAssetsDirpath)
	// If we remove an overlay file, the file will not be removed from the charts directory if it already exists,
	// the easiest way to solve this problem is to clean the target directory before un-archiving the chart's package
	if err := filesystem.RemoveAll(rootFs, chartChartsDirpath); err != nil {
		return fmt.Errorf("Failed to clean directory for charts at %s: %s", chartChartsDirpath, err)
	}
	if err := rootFs.MkdirAll(chartChartsDirpath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory for charts at %s: %s", chartChartsDirpath, err)
	}
	defer filesystem.PruneEmptyDirsInPath(rootFs, chartChartsDirpath)
	// Run helm package
	pkg := helmAction.NewPackage()
	pkg.Version = chartVersion
	pkg.Destination = filesystem.GetAbsPath(rootFs, chartAssetsDirpath)
	pkg.DependencyUpdate = false
	absTgzPath, err := pkg.Run(absHelmChartPath, nil)
	if err != nil {
		return err
	}
	tgzPath, err := filesystem.GetRelativePath(rootFs, absTgzPath)
	if err != nil {
		return err
	}
	logrus.Infof("Generated archive: %s", tgzPath)
	// Unarchive the generated package
	if err := filesystem.UnarchiveTgz(rootFs, tgzPath, "", chartChartsDirpath, true); err != nil {
		return err
	}
	logrus.Infof("Generated chart: %s", chartChartsDirpath)
	return nil
}
