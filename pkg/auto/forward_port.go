package auto

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// ForwardPort automates the process of forward-porting chart versions from a source branch to the current branch.
// It compares charts between the source branch and the current branch, identifies versions that exist in the source
// but not in the target, and copies those versions to the current branch.
//
// The function performs the following steps for each missing chart version:
//  1. Pulls the chart asset (.tgz) from the source branch
//  2. Unzips the asset into the charts directory
//  3. Updates the Helm index
//  4. Pulls the chart icon if it exists
//  5. Updates the release.yaml file
//  6. Creates a git commit with the changes
//
// Parameters:
//   - ctx: Context containing the initialized Config (must be set via config.WithConfig)
//   - sourceBranch: The git branch name to forward-port from (e.g., "dev-v2.9")
//
// Returns an error if any step fails during the forward-port process.
func ForwardPort(ctx context.Context, sourceBranch string) error {
	logger.Log(ctx, slog.LevelInfo, "auto forward port", slog.String("from", sourceBranch))
	if sourceBranch == "" {
		return errors.New("please provide a source branch to forward port from")
	}

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	// List charts and versions
	srcIndex, err := helm.RemoteIndexYaml(ctx, sourceBranch)
	if err != nil {
		return err
	}
	sourceMap := helm.ConvertIndexToVersionsMap(srcIndex)

	targetMap, err := helm.GetAssetsVersionsMap(ctx)
	if err != nil {
		return err
	}

	toForwardPortAssets, skippedAssets := listAssetsToForwardPort(ctx, sourceMap, targetMap, cfg)
	if len(skippedAssets) > 0 {
		logger.Log(ctx, slog.LevelWarn, "entire assets skipped")
		for asset, versions := range skippedAssets {
			logger.Log(ctx, slog.LevelWarn, "skipped", slog.String("asset", asset))
			logger.Log(ctx, slog.LevelWarn, "", slog.Any("versions", versions))
		}
	}

	// For loop
	for chart, versions := range toForwardPortAssets {
		for _, version := range versions {
			// Pull the asset version from source branch
			assetPath, assetTgz := mountAssetVersionPath(chart, version)
			if err := PullAsset(sourceBranch, assetPath, cfg.Repo); err != nil {
				return err
			}

			// Unzip asset = chart/chart-version.tgz
			assetTgzPath := chart + "/" + assetTgz
			if err := helm.DumpAssets(ctx, assetTgzPath); err != nil {
				return err
			}
			if err := helm.CreateOrUpdateHelmIndex(ctx); err != nil {
				return err
			}
			// PullIcon
			if err := PullIcon(ctx, cfg.RootFS, cfg.Repo, chart, version, sourceBranch); err != nil {
				return err
			}
			// UpdateReleaseYaml
			if err := UpdateReleaseYaml(ctx, false, chart, version, config.PathReleaseYaml); err != nil {
				return err
			}
			// Git Add && Commit
			if err := cfg.Repo.AddAndCommit("fp: " + chart + " " + version); err != nil {
				return err
			}
		}
	}

	return nil
}

// listAssetsToForwardPort compares chart versions between source and target branches and identifies
// which versions need to be forward-ported. It filters charts based on the TrackedCharts configuration,
// only including charts that are marked as "active" or "legacy".
//
// The function performs a version-by-version comparison: for each chart in the source, it checks if
// each version exists in the target. Versions that exist in source but not in target are added to the
// result map for forward-porting.
//
// Parameters:
//   - ctx: Context for logging
//   - source: Map of chart names to version lists from the source branch
//   - target: Map of chart names to version lists from the target branch
//   - trackedCharts: Configuration defining which charts are tracked and their status
//
// Returns:
//   - A map of chart names to lists of versions that need to be forward-ported
func listAssetsToForwardPort(ctx context.Context, source, target map[string][]string, cfg *config.Config) (map[string][]string, map[string][]string) {
	toForwardPortAssets := make(map[string][]string)
	skippedAssets := make(map[string][]string)

	for sourceChart, sourceVersions := range source {
		logger.Log(ctx, slog.LevelDebug, "checking chart to forward-port", slog.String("chart", sourceChart))

		// Skip charts that are not tracked or not active
		// This checks both main chart names and their targets (e.g., CRD charts)
		if !cfg.TrackedCharts.IsActiveOrLegacy(sourceChart) {
			logger.Log(ctx, slog.LevelDebug, "skipping chart", slog.String("chart", sourceChart), slog.String("reason", "not active or not tracked"))
			skippedAssets[sourceChart] = sourceVersions
			continue
		}

		targetVersions := target[sourceChart]

		for _, sourceVersion := range sourceVersions {
			found := false
			if cfg.VersionRules.IsVersionCandidate(sourceVersion) {
				logger.Log(ctx, slog.LevelDebug, "skipping version", slog.String("chart", sourceVersion), slog.String("reason", "release candidate"))
				continue
			}

			for _, targetVersion := range targetVersions {
				if sourceVersion == targetVersion {
					found = true
					break
				}
			}
			if !found {
				toForwardPortAssets[sourceChart] = append(toForwardPortAssets[sourceChart], sourceVersion)
			}
		}
	}

	return toForwardPortAssets, skippedAssets
}
