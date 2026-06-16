package helm

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/util"
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
	mergedIndex, _ := UpdateIndex(ctx, helmIndexFile, newHelmIndexFile)

	// Apply blocklist annotations
	if err := applyBlocklist(ctx, mergedIndex); err != nil {
		return err
	}

	// Always write index.yaml to ensure blocklist annotations are persisted
	// Trade-off: ~500ms overhead in concurrent validate runs for consistency

	// Write new index to disk
	err = mergedIndex.WriteFile(absRepositoryHelmIndexFile, os.ModePerm)
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
	allowedPrereleases := []string{"-alpha.", "-beta.", "-rc", "-rancher", "-security"}
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

// applyBlocklist injects hidden annotation for blocklisted chart versions
func applyBlocklist(ctx context.Context, index *helmRepo.IndexFile) error {
	blocklist, err := config.LoadBlockList(ctx)
	if err != nil {
		return errors.New("failed to load blocklist: " + err.Error())
	}

	for chartName, chartVersions := range index.Entries {
		for i, chartVersion := range chartVersions {
			if blocklist.IsBlocked(chartName, chartVersion.Version) {
				// Inject hidden annotation
				if chartVersion.Annotations == nil {
					chartVersion.Annotations = make(map[string]string)
				}
				chartVersion.Annotations["catalog.cattle.io/hidden"] = "true"

				// Update entry in place
				index.Entries[chartName][i] = chartVersion

				logger.Log(ctx, slog.LevelInfo, "marked chart as hidden",
					slog.String("chart", chartName),
					slog.String("version", chartVersion.Version))
			}
		}
	}

	return nil
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
			return util.SortUpstreamAppVersions(versions[i].Version, versions[j].Version)
		})
	}
}
