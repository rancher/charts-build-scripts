package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

// ValidateRepository validates that the generated assets of the current repository doesn't conflict with the generated assets of the repository in upstreamConfig
func ValidateRepository(rootFs billy.Filesystem, compareGeneratedAssetsOptions options.CompareGeneratedAssetsOptions, currentChart string) error {
	// Create directories
	originalAssets := filepath.Join(path.ChartsRepositoryCurrentBranchDir, path.RepositoryAssetsDir)
	originalCharts := filepath.Join(path.ChartsRepositoryCurrentBranchDir, path.RepositoryChartsDir)
	newAssets := filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir)
	newCharts := filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryChartsDir)
	removeHelmIndex := false
	for _, d := range []string{path.RepositoryAssetsDir, path.RepositoryChartsDir, originalAssets, originalCharts, newAssets, newCharts} {
		existed, err := filesystem.PathExists(rootFs, d)
		if err != nil {
			return fmt.Errorf("Failed to check if path exists %s: %s", d, err)
		}
		if err := rootFs.MkdirAll(d, os.ModePerm); err != nil {
			return fmt.Errorf("Failed to make directory %s: %s", d, err)
		}
		defer filesystem.PruneEmptyDirsInPath(rootFs, d)
		if d == path.RepositoryAssetsDir || d == path.RepositoryChartsDir {
			if existed {
				continue
			}
			removeHelmIndex = true
		}
		defer filesystem.RemoveAll(rootFs, d)
	}
	if removeHelmIndex {
		defer filesystem.RemoveAll(rootFs, path.RepositoryHelmIndexFile)
	}
	defer filesystem.RemoveAll(rootFs, path.ChartsRepositoryCurrentBranchDir)
	defer filesystem.RemoveAll(rootFs, path.ChartsRepositoryUpstreamBranchDir)
	// Copy current assets to new assets
	packages, err := charts.GetPackages(rootFs.Root(), currentChart)
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", path.ChartsRepositoryUpstreamBranchDir, err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	if err := filesystem.CopyDir(rootFs, path.RepositoryAssetsDir, newAssets); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", path.RepositoryAssetsDir, newAssets, err)
	}
	if err := filesystem.CopyDir(rootFs, path.RepositoryChartsDir, newCharts); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", path.RepositoryChartsDir, newCharts, err)
	}
	// Copy upstream assets to original assets
	originalChartsUpstream, err := puller.GetGithubRepository(compareGeneratedAssetsOptions.UpstreamOptions, &compareGeneratedAssetsOptions.Branch)
	if err != nil {
		return fmt.Errorf("Failed to get Github repository pointing to new upstream: %s", err)
	}
	if err := originalChartsUpstream.Pull(rootFs, rootFs, path.ChartsRepositoryCurrentBranchDir); err != nil {
		return fmt.Errorf("Failed to pull chart from upstream: %s", err)
	}
	packages, err = charts.GetPackages(filesystem.GetAbsPath(rootFs, path.ChartsRepositoryCurrentBranchDir), "")
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", path.ChartsRepositoryCurrentBranchDir, err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	// Compare the generated assets, but don't keep the new assets
	err = CompareGeneratedAssets(rootFs, newCharts, newAssets, originalCharts, originalAssets, compareGeneratedAssetsOptions.DropReleaseCandidates, false)
	if err != nil {
		return err
	}
	return nil
}
