package validate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

// CompareGeneratedAssets ensures that the current generated assets are a subset of those that exist in the remote pointed to by remoteOpts
func CompareGeneratedAssets(wt *git.Worktree, releasedChartsRepoBranchOpts options.CompareGeneratedAssetsOptions) (bool, error) {
	// Create directories
	if err := pullReleasedAssetsIntoChartsAndAssets(wt.Filesystem, releasedChartsRepoBranchOpts); err != nil {
		return false, err
	}
	// Check whether Git is still clean, which means that it was a subset
	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("Failed to get git status to check if released charts are subset: %s", err)
	}
	return !status.IsClean(), nil
}

func pullReleasedAssetsIntoChartsAndAssets(repoFs billy.Filesystem, releasedChartsRepoBranchOpts options.CompareGeneratedAssetsOptions) error {
	releasedAssets := filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir)
	releasedCharts := filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryChartsDir)
	for _, d := range []string{releasedAssets, releasedCharts} {
		if err := repoFs.MkdirAll(d, os.ModePerm); err != nil {
			return fmt.Errorf("Failed to make directory %s: %s", d, err)
		}
		defer filesystem.RemoveAll(repoFs, filepath.Dir(d))
	}
	// Copy upstream assets to original assets
	releasedChartsRepoBranch, err := puller.GetGithubRepository(releasedChartsRepoBranchOpts.UpstreamOptions, &releasedChartsRepoBranchOpts.Branch)
	if err != nil {
		return fmt.Errorf("Failed to get Github repository pointing to new upstream: %s", err)
	}
	if err := releasedChartsRepoBranch.Pull(repoFs, repoFs, path.ChartsRepositoryUpstreamBranchDir); err != nil {
		return fmt.Errorf("Failed to pull assets from upstream: %s", err)
	}
	releasedAssetReadme := filepath.Join(releasedAssets, "README.md")
	if err := filesystem.RemoveAll(repoFs, releasedAssetReadme); err != nil {
		return fmt.Errorf("Failed to remove %s: %s", releasedAssetReadme, err)
	}
	releasedChartsReadme := filepath.Join(releasedCharts, "README.md")
	if err := filesystem.RemoveAll(repoFs, releasedChartsReadme); err != nil {
		return fmt.Errorf("Failed to remove %s: %s", releasedChartsReadme, err)
	}
	if err := filesystem.CopyDir(repoFs, releasedAssets, path.RepositoryAssetsDir); err != nil {
		return fmt.Errorf("Encountered error while copying over released assets: %s", err)
	}
	if err := filesystem.CopyDir(repoFs, releasedCharts, path.RepositoryChartsDir); err != nil {
		return fmt.Errorf("Encountered error while copying over released charts: %s", err)
	}
	if err := helm.CreateOrUpdateHelmIndex(repoFs); err != nil {
		return err
	}
	return nil
}
