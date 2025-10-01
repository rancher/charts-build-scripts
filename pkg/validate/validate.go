package validate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	bashGit "github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/standardize"
	"github.com/rancher/charts-build-scripts/pkg/zip"

	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// CompareGeneratedAssetsResponse tracks resources that are added, deleted, and modified when comparing two charts repositories
type CompareGeneratedAssetsResponse struct {
	// UntrackedInRelease represents charts that need to be added to the release.yaml
	UntrackedInRelease options.ReleaseOptions `yaml:"untrackedInRelease,omitempty"`
	// RemovedPostRelease represents charts that have been removed from the upstream
	RemovedPostRelease options.ReleaseOptions `yaml:"removedPostRelease,omitempty"`
	// ModifiedPostRelease represents charts that have been modified from the upstream
	ModifiedPostRelease options.ReleaseOptions `yaml:"modifiedPostRelease,omitempty"`
}

// PassedValidation returns whether the response seems to indicate that the chart repositories are in sync
func (r CompareGeneratedAssetsResponse) PassedValidation() bool {
	return len(r.UntrackedInRelease) == 0 && len(r.RemovedPostRelease) == 0 && len(r.ModifiedPostRelease) == 0
}

// LogDiscrepancies produces logs that can be used to pretty-print why a validation might have failed
func (r CompareGeneratedAssetsResponse) LogDiscrepancies(ctx context.Context) {
	logger.Log(ctx, slog.LevelError, "new assets introduced", slog.Any("UntrackedInRelease", r.UntrackedInRelease))
	logger.Log(ctx, slog.LevelError, "assets removed", slog.Any("RemovedPostRelease", r.RemovedPostRelease))
	logger.Log(ctx, slog.LevelError, "assets modified", slog.Any("ModifiedPostRelease", r.ModifiedPostRelease))
	logger.Log(ctx, slog.LevelError, "If this was intentional, to allow validation to pass, these charts must be added to the release.yaml.")
}

// DumpReleaseYaml takes the response collected by this CompareGeneratedAssetsResponse and automatically creates the appropriate release.yaml,
// assuming that the user does indeed intend to add, delete, or modify all assets that were marked in this comparison
func (r CompareGeneratedAssetsResponse) DumpReleaseYaml(ctx context.Context, repoFs billy.Filesystem) error {
	releaseYaml, err := options.LoadReleaseOptionsFromFile(ctx, repoFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	if releaseYaml == nil {
		releaseYaml = make(map[string][]string)
	}

	releaseYaml.Merge(r.UntrackedInRelease)
	releaseYaml.Merge(r.RemovedPostRelease)
	releaseYaml.Merge(r.ModifiedPostRelease)

	return releaseYaml.WriteToFile(ctx, repoFs, path.RepositoryReleaseYaml)
}

// CompareGeneratedAssets checks to see if current assets and charts match upstream, aside from those indicated in the release.yaml
// It returns a boolean indicating if the comparison has passed or an error
func CompareGeneratedAssets(ctx context.Context, repoRoot string, repoFs billy.Filesystem, u options.UpstreamOptions, branch string, releaseOptions options.ReleaseOptions) (CompareGeneratedAssetsResponse, error) {
	response := CompareGeneratedAssetsResponse{
		UntrackedInRelease:  options.ReleaseOptions{},
		ModifiedPostRelease: options.ReleaseOptions{},
		RemovedPostRelease:  options.ReleaseOptions{},
	}

	// Initialize lifecycle package for validating with assets lifecycle rules
	lifeCycleDep, err := lifecycle.InitDependencies(ctx, repoFs, repoRoot, lifecycle.ExtractBranchVersion(branch), "", false)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to initialize lifecycle dependencies", logger.Err(err))
		return response, err
	}

	// Pull repository
	releasedChartsRepoBranch, err := puller.GetGithubRepository(u, &branch)
	if err != nil {
		return response, fmt.Errorf("failed to get Github repository pointing to new upstream: %s", err)
	}

	if err := releasedChartsRepoBranch.Pull(ctx, repoFs, repoFs, path.ChartsRepositoryUpstreamBranchDir); err != nil {
		return response, fmt.Errorf("failed to pull assets from upstream: %s", err)
	}
	defer filesystem.RemoveAll(repoFs, path.ChartsRepositoryUpstreamBranchDir)

	// Standardize the upstream repository
	logger.Log(ctx, slog.LevelInfo, "standardizing upstream repository to compare it against local")

	releaseFs, err := repoFs.Chroot(path.ChartsRepositoryUpstreamBranchDir)
	if err != nil {
		return response, fmt.Errorf("failed to get filesystem for %s: %s", path.ChartsRepositoryUpstreamBranchDir, err)
	}

	if err := standardize.RestructureChartsAndAssets(ctx, releaseFs); err != nil {
		return response, fmt.Errorf("failed to standardize upstream: %s", err)
	}

	// Walk through directories and execute release logic
	localOnly := func(ctx context.Context, fs billy.Filesystem, localPath string, isDir bool) error {
		if isDir {
			// We only care about original files
			return nil
		}
		isAsset := strings.HasPrefix(localPath, path.RepositoryAssetsDir+"/")
		hasTgzExtension := filepath.Ext(localPath) == ".tgz"
		if !isAsset || !hasTgzExtension {
			// We only care about assets
			return nil
		}
		// Check if the chart is tracked in release
		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, localPath))
		if err != nil {
			return err
		}
		if releaseOptions.Contains(chart.Metadata.Name, chart.Metadata.Version) {
			// Chart is tracked in release.yaml
			return nil
		}
		// Chart exists in local and is not tracked by release.yaml
		logger.Log(ctx, slog.LevelWarn, "chart is untracked", slog.String("name", chart.Metadata.Name), slog.String("version", chart.Metadata.Version))
		// If the chart exists in local and not on the upstream it may have been removed by the lifecycle rules
		isVersionInLifecycle := lifeCycleDep.VR.CheckChartVersionForLifecycle(chart.Metadata.Version)
		if isVersionInLifecycle {
			// this chart should not be removed
			response.UntrackedInRelease = response.UntrackedInRelease.Append(chart.Metadata.Name, chart.Metadata.Version)
		}
		return nil
	}

	upstreamOnly := func(ctx context.Context, fs billy.Filesystem, upstreamPath string, isDir bool) error {
		if isDir {
			// We only care about original files
			return nil
		}
		isAsset := strings.HasPrefix(upstreamPath, filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir)+"/")
		hasTgzExtension := filepath.Ext(upstreamPath) == ".tgz"
		if !isAsset || !hasTgzExtension {
			// We only care about assets
			return nil
		}
		// Check if the chart is tracked in release
		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, upstreamPath))
		if err != nil {
			return err
		}
		if releaseOptions.Contains(chart.Metadata.Name, chart.Metadata.Version) {
			// Chart is tracked in release.yaml; this chart was removed intentionally
			return nil
		}
		// Chart was removed from local and is not tracked by release.yaml
		logger.Log(ctx, slog.LevelWarn, "chart was removed", slog.String("name", chart.Metadata.Name), slog.String("version", chart.Metadata.Version))

		response.RemovedPostRelease = response.RemovedPostRelease.Append(chart.Metadata.Name, chart.Metadata.Version)
		// Found asset that only exists in upstream and is not tracked by release.yaml
		localPath, err := filesystem.MovePath(ctx, upstreamPath, path.ChartsRepositoryUpstreamBranchDir, "")
		if err != nil {
			return err
		}
		return copyAndUnzip(ctx, repoFs, upstreamPath, localPath)
	}

	localAndUpstream := func(ctx context.Context, fs billy.Filesystem, localPath, upstreamPath string, isDir bool) error {
		if isDir {
			// We only care about modified files
			return nil
		}
		isAsset := strings.HasPrefix(localPath, path.RepositoryAssetsDir+"/")
		hasTgzExtension := filepath.Ext(localPath) == ".tgz"
		if !isAsset || !hasTgzExtension {
			// We only care about assets
			return nil
		}
		// Check if the chart is tracked in release
		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, upstreamPath))
		if err != nil {
			return err
		}
		if releaseOptions.Contains(chart.Metadata.Name, chart.Metadata.Version) {
			// Chart is tracked in release.yaml
			return nil
		}
		// Deep compare the inner contents of the tgzs
		identical, err := filesystem.CompareTgzs(ctx, fs, upstreamPath, localPath)
		if err != nil {
			return err
		}
		if identical {
			return nil
		}
		// Chart was modified in local and is not tracked by release.yaml
		logger.Log(ctx, slog.LevelWarn, "chart was modified", slog.String("name", chart.Metadata.Name), slog.String("version", chart.Metadata.Version))

		response.ModifiedPostRelease = response.ModifiedPostRelease.Append(chart.Metadata.Name, chart.Metadata.Version)
		return copyAndUnzip(ctx, repoFs, upstreamPath, localPath)
	}
	// Compare the directories
	logger.Log(ctx, slog.LevelInfo, "comparing standardized upstream assets against local assets")

	if err := filesystem.CompareDirs(ctx, repoFs, "", path.ChartsRepositoryUpstreamBranchDir, localOnly, upstreamOnly, localAndUpstream); err != nil {
		return response, fmt.Errorf("encountered error while trying to compare local against upstream: %s", err)
	}
	return response, nil
}

func copyAndUnzip(ctx context.Context, repoFs billy.Filesystem, upstreamPath, localPath string) error {
	specificAsset, err := filesystem.MovePath(ctx, upstreamPath, filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir), "")
	if err != nil {
		return fmt.Errorf("encountered error while trying to find repository path for upstream path %s: %s", upstreamPath, err)
	}
	if err := filesystem.CopyFile(ctx, repoFs, upstreamPath, localPath); err != nil {
		return err
	}
	if err := zip.DumpAssets(ctx, repoFs.Root(), specificAsset); err != nil {
		return fmt.Errorf("encountered error while copying over contents of modified upstream asset to charts: %s", err)
	}
	return nil
}

// StatusExceptions checks if the git repository is clean and if it is not, it checks if the changes are allowed
func StatusExceptions(ctx context.Context, status git.Status) error {
	if !status.IsClean() {
		if err := validateExceptions(status); err != nil {
			logger.Log(ctx, slog.LevelError, "git is not clean", slog.String("status", status.String()))
			logger.Log(ctx, slog.LevelError, "error", logger.Err(err))
			return errors.New("repository must be clean to run validation")
		}

		g, err := bashGit.OpenGitRepo(ctx, ".")
		if err != nil {
			return err
		}
		if err := g.FullReset(); err != nil {
			return err
		}
	}

	return nil
}

// validateExceptions checks if the changes are allowed
func validateExceptions(status git.Status) error {
	/**
	* The following exceptions are allowed to be modified, they were wrongly released with .orig files on the final production version.
	* This does not break anything and it is not allowed to modify already released charts.
	 */
	exceptions := map[string][]string{
		"rancher-istio":        {"105.4.0+up1.23.2"},
		"prometheus-federator": {"103.0.0+up0.4.0", "103.0.1+up0.4.1", "104.0.0+up0.4.0"},
	}

	for changedFile, _ := range status {
		if changedFile == "index.yaml" {
			continue
		}

		for exceptionChart, exceptionVersions := range exceptions {
			if !strings.Contains(changedFile, exceptionChart) {
				continue
			}

			for _, exceptionVersion := range exceptionVersions {
				if !strings.Contains(changedFile, exceptionVersion) {
					return fmt.Errorf("chart: %s - version: %s is not allowed to be modified", exceptionChart, exceptionVersion)
				}
			}
		}
	}

	return nil
}
