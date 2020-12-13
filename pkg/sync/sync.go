package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

// SynchronizeRepository synchronizes the current repository with the repository in upstreamConfig
func SynchronizeRepository(rootFs billy.Filesystem, compareGeneratedAssetsOptions options.CompareGeneratedAssetsOptions) error {
	// Create directories
	originalAssets := filepath.Join(path.ChartsRepositoryCurrentBranchDir, path.RepositoryAssetsDir)
	originalCharts := filepath.Join(path.ChartsRepositoryCurrentBranchDir, path.RepositoryChartsDir)
	newAssets := filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir)
	newCharts := filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryChartsDir)
	for _, d := range []string{path.RepositoryAssetsDir, path.RepositoryChartsDir, originalAssets, originalCharts, newAssets, newCharts} {
		if err := rootFs.MkdirAll(d, os.ModePerm); err != nil {
			return fmt.Errorf("Failed to make directory %s: %s", d, err)
		}
		defer utils.PruneEmptyDirsInPath(rootFs, d)
		if d == path.RepositoryAssetsDir || d == path.RepositoryChartsDir {
			continue
		}
		defer utils.RemoveAll(rootFs, d)
	}
	defer utils.RemoveAll(rootFs, path.ChartsRepositoryCurrentBranchDir)
	defer utils.RemoveAll(rootFs, path.ChartsRepositoryUpstreamBranchDir)
	// Copy current assets to original assets
	packages, err := charts.GetPackages(rootFs.Root(), "")
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", rootFs.Root(), err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	if err := utils.CopyDir(rootFs, path.RepositoryAssetsDir, originalAssets); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", path.RepositoryAssetsDir, originalAssets, err)
	}
	if err := utils.CopyDir(rootFs, path.RepositoryChartsDir, originalCharts); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", path.RepositoryChartsDir, originalCharts, err)
	}
	// Copy upstream assets to new assets
	newChartsUpstream, err := puller.GetGithubRepository(compareGeneratedAssetsOptions.UpstreamOptions, &compareGeneratedAssetsOptions.Branch)
	if err != nil {
		return fmt.Errorf("Failed to get Github repository pointing to new upstream: %s", err)
	}
	if err := newChartsUpstream.Pull(rootFs, rootFs, path.ChartsRepositoryUpstreamBranchDir); err != nil {
		return fmt.Errorf("Failed to pull chart from upstream: %s", err)
	}
	packages, err = charts.GetPackages(utils.GetAbsPath(rootFs, path.ChartsRepositoryUpstreamBranchDir), "")
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", path.ChartsRepositoryUpstreamBranchDir, err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	// Compare the generated assets and keep the new assets
	err = CompareGeneratedAssets(rootFs, newCharts, newAssets, originalCharts, originalAssets, compareGeneratedAssetsOptions.DropReleaseCandidates, true)
	if err != nil {
		return err
	}
	logrus.Infof("Sync was successful!")
	return nil
}
