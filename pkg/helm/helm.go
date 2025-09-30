package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// CreateOrUpdateHelmIndex either creates or updates the index.yaml for the repository this package is within
func CreateOrUpdateHelmIndex(ctx context.Context, rootFs billy.Filesystem) error {
	absRepositoryAssetsDir := filesystem.GetAbsPath(rootFs, path.RepositoryAssetsDir)
	absRepositoryHelmIndexFile := filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile)

	var helmIndexFile *helmRepo.IndexFile

	// Load index file from disk if it exists
	exists, err := filesystem.PathExists(ctx, rootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return fmt.Errorf("encountered error while checking if Helm index file already exists in repository: %s", err)
	} else if exists {
		helmIndexFile, err = helmRepo.LoadIndexFile(absRepositoryHelmIndexFile)
		if err != nil {
			return fmt.Errorf("encountered error while trying to load existing index file: %s", err)
		}
	} else {
		helmIndexFile = helmRepo.NewIndexFile()
	}

	// Generate the current index file from the assets/ directory
	newHelmIndexFile, err := helmRepo.IndexDirectory(absRepositoryAssetsDir, path.RepositoryAssetsDir)
	if err != nil {
		return fmt.Errorf("encountered error while trying to generate new Helm index: %s", err)
	}

	// Sort entries to ensure consistent ordering
	SortVersions(helmIndexFile)
	SortVersions(newHelmIndexFile)

	// Update index
	helmIndexFile, upToDate := UpdateIndex(ctx, helmIndexFile, newHelmIndexFile)

	if upToDate {
		logger.Log(ctx, slog.LevelInfo, "index.yaml is up-to-date")
		return nil
	}

	// Write new index to disk
	err = helmIndexFile.WriteFile(absRepositoryHelmIndexFile, os.ModePerm)
	if err != nil {
		return fmt.Errorf("encountered error while trying to write updated Helm index into index.yaml: %s", err)
	}

	logger.Log(ctx, slog.LevelInfo, "generated index.yaml")
	return nil
}

// UpdateIndex updates the original index with the new contents
func UpdateIndex(ctx context.Context, original, new *helmRepo.IndexFile) (*helmRepo.IndexFile, bool) {

	upToDate := true
	// Preserve generated timestamp
	new.Generated = original.Generated

	// Ensure newer version of chart is used if it has been updated
	for chartName, chartVersions := range new.Entries {
		for i, chartVersion := range chartVersions {
			version := chartVersion.Version
			if !original.Has(chartName, version) {
				// Keep the newly generated chart version as-is
				upToDate = false
				logger.Log(ctx, slog.LevelDebug, "chart has introduced a new version", slog.String("chartName", chartName), slog.String("version", version))
				continue
			}
			// Get original chart version
			var originalChartVersion *helmRepo.ChartVersion
			for _, originalChartVersion = range original.Entries[chartName] {
				if originalChartVersion.Version == chartVersion.Version {
					// found originalChartVersion, which must exist since we checked that the original has it
					break
				}
			}
			// Try to preserve it only if nothing has changed.
			if originalChartVersion.Digest == chartVersion.Digest {
				// Don't modify created timestamp
				new.Entries[chartName][i].Created = originalChartVersion.Created
			} else {
				upToDate = false
				logger.Log(ctx, slog.LevelDebug, "chart has been modified", slog.String("chartName", chartName), slog.String("version", version))
			}
		}
	}

	for chartName, chartVersions := range original.Entries {
		for _, chartVersion := range chartVersions {
			if !new.Has(chartName, chartVersion.Version) {
				// Chart was removed
				upToDate = false
				logger.Log(ctx, slog.LevelDebug, "chart has been removed", slog.String("chartName", chartName), slog.String("version", chartVersion.Version))
				continue
			}
		}
	}

	// Sort one more time for safety
	new.SortEntries()
	return new, upToDate
}

// OpenIndexYaml will check and open the index.yaml file in the local repository at the default file path
func OpenIndexYaml(ctx context.Context, rootFs billy.Filesystem) (*helmRepo.IndexFile, error) {
	helmIndexFilePath := filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile)

	exists, err := filesystem.PathExists(ctx, rootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("index.yaml file does not exist in the local repository")
	}

	return helmRepo.LoadIndexFile(helmIndexFilePath)
}

// SortVersions sorts chart versions with custom RC handling
func SortVersions(index *helmRepo.IndexFile) {
	for _, versions := range index.Entries {
		sort.Slice(versions, func(i, j int) bool {
			return compareVersions(versions[i].Version, versions[j].Version)
		})
	}
}

// compareVersions compares two version strings for sorting
// Returns true if versionA should come before versionB (descending order)
func compareVersions(versionA, versionB string) bool {
	// Parse both versions
	baseA, rcA, isRCA := parseVersionWithRC(versionA)
	baseB, rcB, isRCB := parseVersionWithRC(versionB)

	// Parse base versions using semver
	semverA, errA := semver.NewVersion(baseA)
	semverB, errB := semver.NewVersion(baseB)

	if errA != nil {
		return false // push invalid to end
	}
	if errB != nil {
		return true // push invalid to end
	}

	// If base versions are different, use semver comparison (descending)
	if !semverA.Equal(semverB) {
		return semverA.GreaterThan(semverB)
	}

	// Same base version - handle RC logic
	// Stable (non-RC) should come first
	if !isRCA && isRCB {
		return true // A is stable, B is RC - A comes first
	}
	if isRCA && !isRCB {
		return false // A is RC, B is stable - B comes first
	}

	// Both are RCs - higher RC number comes first (descending)
	if isRCA && isRCB {
		return rcA > rcB
	}

	// Both are stable with same base version - they're equal
	return false
}

// parseVersionWithRC extracts the base version and RC number from a version string
// Example: "108.0.0+up0.9.0-rc.1" returns ("108.0.0+up0.9.0", 1, true)
func parseVersionWithRC(version string) (baseVersion string, rcNumber int, isRC bool) {
	// Split by '+' to separate version from build metadata
	parts := strings.Split(version, "+")
	if len(parts) != 2 {
		return version, 0, false
	}

	baseVersionNum := parts[0]
	buildMetadata := parts[1]

	// Check if build metadata contains RC
	if !strings.Contains(buildMetadata, "-rc.") {
		return version, 0, false
	}

	// Extract RC number
	rcParts := strings.Split(buildMetadata, "-rc.")
	if len(rcParts) != 2 {
		return version, 0, false
	}

	rcNum, err := strconv.Atoi(rcParts[1])
	if err != nil {
		return version, 0, false
	}

	// Return base version with the non-RC part of build metadata
	return baseVersionNum + "+" + rcParts[0], rcNum, true
}
