package helm

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// indexMutex protects concurrent access to index.yaml file operations
// This ensures that only one goroutine can read/modify/write the index at a time
var indexMutex sync.Mutex

// CreateOrUpdateHelmIndex either creates or updates the index.yaml for the repository this package is within
func CreateOrUpdateHelmIndex(ctx context.Context, rootFs billy.Filesystem) error {
	// Acquire the lock to ensure exclusive access to index.yaml
	indexMutex.Lock()
	// Defer the unlock to ensure it happens even if we return early or encounter an error
	defer indexMutex.Unlock()

	absRepositoryAssetsDir := filesystem.GetAbsPath(rootFs, path.RepositoryAssetsDir)
	absRepositoryHelmIndexFile := filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile)

	var helmIndexFile *helmRepo.IndexFile

	// Load index file from disk if it exists
	exists, err := filesystem.PathExists(ctx, rootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return errors.New("encountered error while checking if Helm index file already exists in repository: " + err.Error())
	}

	if exists {
		helmIndexFile, err = helmRepo.LoadIndexFile(absRepositoryHelmIndexFile)
		if err != nil {
			return errors.New("encountered error while trying to load existing index file: " + err.Error())
		}
	} else {
		helmIndexFile = helmRepo.NewIndexFile()
	}

	// Generate the current index file from the assets/ directory
	newHelmIndexFile, err := helmRepo.IndexDirectory(absRepositoryAssetsDir, path.RepositoryAssetsDir)
	if err != nil {
		return errors.New("encountered error while trying to generate new Helm index: " + err.Error())
	}

	if err := CheckVersionStandards(ctx, newHelmIndexFile); err != nil {
		return err
	}

	// Sort entries to ensure consistent ordering
	SortVersions(helmIndexFile)
	SortVersions(newHelmIndexFile)

	// Update index
	helmIndexFile, upToDate := UpdateIndex(ctx, helmIndexFile, newHelmIndexFile)

	if upToDate {
		return nil
	}

	// Write new index to disk
	err = helmIndexFile.WriteFile(absRepositoryHelmIndexFile, os.ModePerm)
	if err != nil {
		return errors.New("encountered error while trying to write updated Helm index into index.yaml: " + err.Error())
	}

	logger.Log(ctx, slog.LevelInfo, "generated index.yaml")
	return nil
}

// CheckVersionStandards validates that all chart versions follow the allowed prerelease standards
// Only -alpha., -beta., and -rc. prerelease identifiers are allowed
// Returns an error if any version contains an invalid prerelease identifier
func CheckVersionStandards(ctx context.Context, new *helmRepo.IndexFile) error {
	allowedPrereleases := []string{"-alpha.", "-beta.", "-rc", "-rancher."}
	logger.Log(ctx, slog.LevelInfo, "checking version standars", slog.Any("allowed", allowedPrereleases))

	for chartName, chartVersions := range new.Entries {
		for _, chartVersion := range chartVersions {
			version := chartVersion.Version

			// Split by '+' to get build metadata
			parts := strings.Split(version, "+")
			if len(parts) != 2 {
				// No build metadata, version is valid
				continue
			}

			buildMetadata := parts[1]

			// Check if there's a prerelease identifier (contains a hyphen)
			if !strings.Contains(buildMetadata, "-") {
				// No prerelease, version is valid
				continue
			}

			// Extract the prerelease part (everything after the first '-' in build metadata)
			dashIndex := strings.Index(buildMetadata, "-")
			prereleaseSection := buildMetadata[dashIndex:]

			// Check if it matches one of the allowed patterns
			isValid := false
			for _, allowed := range allowedPrereleases {
				if strings.HasPrefix(prereleaseSection, allowed) {
					isValid = true
					break
				}
			}

			if !isValid {
				return errors.New("chart '" + chartName + "' version '" + version + "' contains invalid prerelease identifier. Only -alpha., -beta., -rancher., and -rc. are allowed")
			}
		}
	}

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
// Handles alpha, beta, rc, and stable versions
func compareVersions(versionA, versionB string) bool {
	// Parse both versions
	baseA, typeA, numA, isPrereleaseA := parseVersionWithPrerelease(versionA)
	baseB, typeB, numB, isPrereleaseB := parseVersionWithPrerelease(versionB)

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

	// Same base version - handle prerelease logic
	// Compare prerelease types first (stable=4 > rc=3 > beta=2 > alpha=1)
	if typeA != typeB {
		return typeA > typeB // Higher type priority comes first
	}

	// Same prerelease type - compare numbers if both are prereleases
	if isPrereleaseA && isPrereleaseB {
		return numA > numB // Higher prerelease number comes first (descending)
	}

	// Both are stable with same base version - they're equal
	return false
}

// parseVersionWithPrerelease extracts the base version, prerelease type, and prerelease number from a version string
// Example: "108.0.0+up0.14.0-rc.1" returns ("108.0.0+up0.14.0", 3, 1, true)
// Example: "108.0.0+up0.14.0-beta.2" returns ("108.0.0+up0.14.0", 2, 2, true)
// Example: "108.0.0+up0.14.0-alpha.5" returns ("108.0.0+up0.14.0", 1, 5, true)
// Example: "108.0.0+up0.14.0" returns ("108.0.0+up0.14.0", 4, 0, false) - stable version
// Prerelease type priority: stable=4 > rc=3 > beta=2 > alpha=1
func parseVersionWithPrerelease(version string) (baseVersion string, prereleaseType int, prereleaseNumber int, isPrerelease bool) {
	// Split by '+' to separate version from build metadata
	parts := strings.Split(version, "+")
	if len(parts) != 2 {
		// No build metadata, treat as stable
		return version, 4, 0, false
	}

	baseVersionNum := parts[0]
	buildMetadata := parts[1]

	// Check for prerelease types in priority order: alpha, beta, rc
	prereleaseTypes := []struct {
		suffix   string
		priority int
	}{
		{"-alpha.", 1},
		{"-beta.", 2},
		{"-rc.", 3},
	}

	for _, pt := range prereleaseTypes {
		if strings.Contains(buildMetadata, pt.suffix) {
			// Extract prerelease number
			preParts := strings.Split(buildMetadata, pt.suffix)
			if len(preParts) != 2 {
				continue
			}

			preNum, err := strconv.Atoi(preParts[1])
			if err != nil {
				continue
			}

			// Return base version with the non-prerelease part of build metadata
			return baseVersionNum + "+" + preParts[0], pt.priority, preNum, true
		}
	}

	// No prerelease found, treat as stable
	return version, 4, 0, false
}
