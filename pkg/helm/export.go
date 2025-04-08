package helm

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	helmAction "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// TODO remove this if it is unused
const (
	numPatchDigits = 2
)

var (
	patchNumMultiplier = uint64(math.Pow10(2))
	maxPatchNum        = patchNumMultiplier - 1
)

// ExportHelmChart creates a Helm chart archive and an unarchived Helm chart at RepositoryAssetDirpath and RepositoryChartDirPath
// helmChartPath is a relative path (rooted at the package level) that contains the chart.
func ExportHelmChart(ctx context.Context, rootFs, fs billy.Filesystem, helmChartPath string, packageVersion *int, version *semver.Version, autoGenBumpVersion *semver.Version, upstreamChartVersion string, omitBuildMetadata bool) error {

	if err := removeOrigFiles(ctx, filesystem.GetAbsPath(fs, helmChartPath)); err != nil {
		return fmt.Errorf("failed to remove .orig files: %s", err)
	}

	chart, err := loadHelmChart(fs, helmChartPath)
	if err != nil {
		return err
	}

	// Parse the chart version (if autoGenBumpVersion is not nil, it will be used as the version)
	chartVersion, err := parseChartVersion(packageVersion, version, upstreamChartVersion, chart.Metadata.Version, autoGenBumpVersion, omitBuildMetadata)
	if err != nil {
		return err
	}

	// Assets are indexed by chart name, independent of which package that chart is contained within
	chartAssetsDirpath := filepath.Join(path.RepositoryAssetsDir, chart.Metadata.Name)
	// All generated charts are indexed by chart name and version
	chartChartsDirpath := filepath.Join(path.RepositoryChartsDir, chart.Metadata.Name, chartVersion)

	// Create directories structure
	if err := handleDirStructure(rootFs, chartAssetsDirpath, chartChartsDirpath); err != nil {
		return err
	}
	defer filesystem.PruneEmptyDirsInPath(ctx, rootFs, chartAssetsDirpath)
	defer filesystem.PruneEmptyDirsInPath(ctx, rootFs, chartChartsDirpath)

	tgzPath, err := GenerateArchive(ctx, rootFs, fs, helmChartPath, chartAssetsDirpath, &chartVersion)
	if err != nil {
		return err
	}
	// Unarchive the generated package
	if err := filesystem.UnarchiveTgz(ctx, rootFs, tgzPath, "", chartChartsDirpath, true); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "exported chart", slog.String("chartChartsDirPath", chartChartsDirpath), slog.String("tgzPath", tgzPath))
	return nil
}

// removeOrigFiles removes all files ending with .orig in the specified directory
func removeOrigFiles(ctx context.Context, dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".orig") {
			if err := os.Remove(path); err != nil {
				return err
			}

			logger.Log(ctx, slog.LevelDebug, "removed file", slog.String("path", path))
		}
		return nil
	})
	return err
}

func loadHelmChart(fs billy.Filesystem, helmChartPath string) (*chart.Chart, error) {
	// Try to load the chart to see if it can be exported
	absHelmChartPath := filesystem.GetAbsPath(fs, helmChartPath)
	chart, err := helmLoader.Load(absHelmChartPath)
	if err != nil {
		return nil, fmt.Errorf("could not load Helm chart: %s", err)
	}
	if err := chart.Validate(); err != nil {
		return nil, fmt.Errorf("failed while trying to validate Helm chart: %s", err)
	}

	return chart, nil
}

// parseChartVersion will parse the chart version based on the packageVersion, version, upstreamChartVersion, metadataVersion, and omitBuildMetadata unless autoGenBumpVersion is not nil, in this case it will use autoGenBumpVersion as the version
func parseChartVersion(packageVersion *int, version *semver.Version, upstreamChartVersion string, metadataVersion string, autoGenBumpVersion *semver.Version, omitBuildMetadata bool) (string, error) {
	if autoGenBumpVersion != nil {
		return autoGenBumpVersion.String(), nil
	}

	metadataSemver, err := semver.Make(metadataVersion)
	if err != nil {
		return "", fmt.Errorf("cannot parse original chart version %s as valid semver", metadataVersion)
	}

	if version != nil {
		metadataSemver = *version
	}

	// Add packageVersion as string, preventing errors due to leading 0s
	if packageVersion != nil {
		if uint64(*packageVersion) >= maxPatchNum {
			return "", fmt.Errorf("maximum number for packageVersion is %d, found %d", maxPatchNum, packageVersion)
		}
		if uint64(*packageVersion) < 1 {
			return "", fmt.Errorf("minimum number for packageVersion is 1, found %d", packageVersion)
		}
		metadataSemver.Patch = patchNumMultiplier*metadataSemver.Patch + uint64(*packageVersion)
	}

	// Add buildMetadataFlag for forked charts
	if !omitBuildMetadata && len(upstreamChartVersion) > 0 && upstreamChartVersion != metadataSemver.String() {
		metadataSemver.Build = append(metadataSemver.Build, fmt.Sprintf("up%s", upstreamChartVersion))
	}

	chartVersion := metadataSemver.String()

	return chartVersion, nil
}

func handleDirStructure(rootFs billy.Filesystem, chartAssetsDirpath, chartChartsDirpath string) error {
	// Create directories
	if err := rootFs.MkdirAll(chartAssetsDirpath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory for assets at %s: %s", chartAssetsDirpath, err)
	}
	// If we remove an overlay file, the file will not be removed from the charts directory if it already exists,
	// the easiest way to solve this problem is to clean the target directory before un-archiving the chart's package
	if err := filesystem.RemoveAll(rootFs, chartChartsDirpath); err != nil {
		return fmt.Errorf("failed to clean directory for charts at %s: %s", chartChartsDirpath, err)
	}
	if err := rootFs.MkdirAll(chartChartsDirpath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory for charts at %s: %s", chartChartsDirpath, err)
	}

	return nil
}

// GenerateArchive produces a Helm chart archive. If an archive exists at that path already, it does a deep check of the internal
// contents of the archive and only updates the archive if something within it has been changed.
func GenerateArchive(ctx context.Context, rootFs, fs billy.Filesystem, helmChartPath, chartAssetsDirpath string, chartVersion *string) (string, error) {
	absHelmChartPath := filesystem.GetAbsPath(fs, helmChartPath)
	// Run helm package
	pkg := helmAction.NewPackage()
	if chartVersion != nil {
		pkg.Version = *chartVersion
	}
	// Create a temporary asset at assets/{chart}.temp/{chart}-{version}.tgz
	pkg.Destination = filesystem.GetAbsPath(rootFs, chartAssetsDirpath) + ".temp"
	pkg.DependencyUpdate = false
	absTgzPath, err := pkg.Run(absHelmChartPath, nil)
	if err != nil {
		return "", err
	}
	tempTgzPath, err := filesystem.GetRelativePath(rootFs, absTgzPath)
	if err != nil {
		return "", err
	}
	defer filesystem.RemoveAll(rootFs, filepath.Dir(tempTgzPath))
	// Path where we expect the tgz file to be deposited
	tgzPath := filepath.Join(chartAssetsDirpath, filepath.Base(tempTgzPath))
	// Check if original tgz existed
	exists, err := filesystem.PathExists(ctx, rootFs, tgzPath)
	if err != nil {
		return "", err
	}
	// If the archive does not exist, it needs to be created
	shouldUpdateArchive := !exists
	if exists {
		// Check if the tgz has been modified
		identical, err := filesystem.CompareTgzs(ctx, rootFs, tgzPath, tempTgzPath)
		if err != nil {
			return "", fmt.Errorf("encountered error while trying to compare contents of %s against %s: %v", tgzPath, tempTgzPath, err)
		}
		// If the archives are not identical, it needs to be updated
		shouldUpdateArchive = !identical
	}
	if shouldUpdateArchive {
		filesystem.CopyFile(ctx, rootFs, tempTgzPath, tgzPath)
		logger.Log(ctx, slog.LevelInfo, "generated archive", slog.String("tgzPath", tgzPath))
	} else {
		logger.Log(ctx, slog.LevelInfo, "archive is up-to-date", slog.String("tgzPath", tgzPath))
	}
	return tgzPath, nil
}
