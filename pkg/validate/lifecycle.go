package validate

import (
	"context"
	"log/slog"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// Status struct holds the results of the assets versions comparison,
// this data will all be logged and saved into log files for further analysis.
//
// When marshaled to YAML, it produces the following structure:
//
//	release:
//	  chart-name:
//	    - version1
//	    - version2
//	forward-port:
//	  chart-name:
//	    - version1
type Status struct {
	ToRelease     map[string][]string `yaml:"release"`
	ToForwardPort map[string][]string `yaml:"forward-port"`
}

// LifecycleStatus compares chart versions between dev and release branches to identify
// assets that need to be released or forward-ported.
//
// The function performs the following steps:
//  1. Validates the branch version exists in configuration
//  2. Fetches index.yaml from both dev and release branches
//  3. Identifies versions present in dev but missing in release
//  4. Filters out deprecated charts
//  5. Categorizes remaining versions as either "to release" or "to forward-port"
//
// Parameters:
//   - ctx: Context containing the initialized Config
//   - branchVersion: The numeric version (e.g., "2.13") without dev/release prefix
//
// Returns:
//   - Status struct containing charts/versions categorized by action needed
//   - Error if validation, fetching, or processing fails
func LifecycleStatus(ctx context.Context, branchVersion string) (*Status, error) {
	logger.Log(ctx, slog.LevelInfo, "lifecycle-status", slog.String("branch-version", branchVersion))

	status := &Status{}

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := cfg.CheckBranchVersion(branchVersion); err != nil {
		return nil, err
	}

	devBranch := cfg.VersionRules.DevPrefix + branchVersion
	releaseBranch := cfg.VersionRules.ReleasePrefix + branchVersion

	// Get the assets versions map
	devIndex, err := helm.RemoteIndexYaml(ctx, devBranch)
	if err != nil {
		return nil, err
	}
	devIndexMap := helm.ConvertIndexToVersionsMap(devIndex)

	releaseIndex, err := helm.RemoteIndexYaml(ctx, releaseBranch)
	if err != nil {
		return nil, err
	}
	releaseIndexMap := helm.ConvertIndexToVersionsMap(releaseIndex)

	missingMap := ListMissingAssetsVersions(devIndexMap, releaseIndexMap)

	missingAndNotDeprecated, err := FilterDeprecated(ctx, missingMap)
	if err != nil {
		return nil, err
	}

	toRelease, toPort, err := FilterPortsAndToRelease(ctx, missingAndNotDeprecated)
	if err != nil {
		return nil, err
	}

	status.ToRelease = toRelease
	status.ToForwardPort = toPort
	return status, writeStateFile(ctx, status)
}

// ListMissingAssetsVersions is a helper function that shows what is present at `source` but not in `target`
func ListMissingAssetsVersions(source, target map[string][]string) map[string][]string {
	missing := make(map[string][]string)

	for sourceChart, sourceVersions := range source {
		targetVersions := target[sourceChart]
		for _, sourceVersion := range sourceVersions {
			found := false
			for _, targetVersion := range targetVersions {
				if sourceVersion == targetVersion {
					found = true
					break
				}
			}
			if !found {
				missing[sourceChart] = append(missing[sourceChart], sourceVersion)
			}
		}
	}

	return missing
}

// FilterDeprecated will filter out deprecated chart from an assets versions map
func FilterDeprecated(ctx context.Context, indexMap map[string][]string) (map[string][]string, error) {
	cfg, err := config.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make(map[string][]string)

	for chart, versions := range indexMap {
		if !cfg.TrackedCharts.IsActiveOrLegacy(chart) {
			logger.Log(ctx, slog.LevelWarn, "skipping deprecated...", slog.String("chart", chart))
			continue
		}
		filtered[chart] = append(filtered[chart], versions...)
	}

	return filtered, nil
}

// writeStateFile writes the lifecycle status to a YAML state file.
// The Status struct's YAML tags define the output format with "release" and "forward-port" top-level keys.
func writeStateFile(ctx context.Context, status *Status) error {
	file, err := filesystem.CreateAndOpenYamlFile(ctx, config.PathStateYaml, true)
	if err != nil {
		return err
	}
	defer file.Close()

	return filesystem.UpdateYamlFile(file, status)
}

// LoadStateFile reads and parses the lifecycle state file into a Status struct.
// Returns an error if the file doesn't exist or cannot be parsed.
func LoadStateFile(ctx context.Context) (*Status, error) {
	status, err := filesystem.LoadYamlFile[Status](ctx, config.PathStateYaml, false)
	if err != nil {
		return nil, err
	}

	if status == nil {
		return &Status{
			ToRelease:     make(map[string][]string),
			ToForwardPort: make(map[string][]string),
		}, nil
	}

	return status, nil
}
