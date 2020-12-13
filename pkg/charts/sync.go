package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	// ChartsRepositoryRebasePatchesDirpath is the directory where a synchronize call will generate any conflicts
	ChartsRepositoryRebasePatchesDirpath = "rebase/generated-changes"
	// ChartsRepositoryCurrentBranchDirpath is a directory that will be used to store your current assets
	ChartsRepositoryCurrentBranchDirpath = "original-assets"
	// ChartsRepositoryUpstreamBranchDirpath is a directory that will be used to store the latest copy of a branch you want to sync with
	ChartsRepositoryUpstreamBranchDirpath = "new-assets"
)

// SynchronizeRepository synchronizes the current repository with the repository in upstreamConfig
func SynchronizeRepository(rootFs billy.Filesystem, upstreamConfig repository.GithubConfiguration, compareGeneratedAssetsOptions repository.CompareGeneratedAssetsOptions) error {
	repo, err := utils.GetRepo(rootFs.Root())
	if err != nil {
		return fmt.Errorf("Could not retrieve the repository: %s", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("Could not retrieve current worktree: %s", err)
	}
	// Create directories
	originalAssets := filepath.Join(ChartsRepositoryCurrentBranchDirpath, RepositoryAssetsDirpath)
	originalCharts := filepath.Join(ChartsRepositoryCurrentBranchDirpath, RepositoryChartsDirpath)
	newAssets := filepath.Join(ChartsRepositoryUpstreamBranchDirpath, RepositoryAssetsDirpath)
	newCharts := filepath.Join(ChartsRepositoryUpstreamBranchDirpath, RepositoryChartsDirpath)
	for _, d := range []string{RepositoryAssetsDirpath, RepositoryChartsDirpath, originalAssets, originalCharts, newAssets, newCharts} {
		if err := rootFs.MkdirAll(d, os.ModePerm); err != nil {
			return fmt.Errorf("Failed to make directory %s: %s", d, err)
		}
		defer utils.PruneEmptyDirsInPath(rootFs, d)
		if d == RepositoryAssetsDirpath || d == RepositoryChartsDirpath {
			continue
		}
		defer utils.RemoveAll(rootFs, d)
	}
	defer utils.RemoveAll(rootFs, ChartsRepositoryCurrentBranchDirpath)
	defer utils.RemoveAll(rootFs, ChartsRepositoryUpstreamBranchDirpath)
	// Copy current assets to original assets
	packages, err := GetPackages(rootFs.Root(), "")
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", rootFs.Root(), err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	if err := utils.CopyDir(rootFs, RepositoryAssetsDirpath, originalAssets); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", RepositoryAssetsDirpath, originalAssets, err)
	}
	if err := utils.CopyDir(rootFs, RepositoryChartsDirpath, originalCharts); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", RepositoryChartsDirpath, originalCharts, err)
	}
	// Copy upstream assets to new assets
	newChartsUpstream := GetUpstreamForBranch(upstreamConfig, compareGeneratedAssetsOptions.WithBranch)
	if err := newChartsUpstream.Pull(rootFs, rootFs, ChartsRepositoryUpstreamBranchDirpath); err != nil {
		return fmt.Errorf("Failed to pull upstream configuration pointing to new charts: %s", err)
	}
	packages, err = GetPackages(utils.GetAbsPath(rootFs, ChartsRepositoryUpstreamBranchDirpath), "")
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", ChartsRepositoryUpstreamBranchDirpath, err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	// Compare the generated assets and keep the new assets
	compareGAErr := CompareGeneratedAssets(rootFs, newCharts, newAssets, originalCharts, originalAssets, compareGeneratedAssetsOptions.DropReleaseCandidates, true)
	// Delete the Helm index if it was the only thing updated, whether or not changes failed
	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("Could not retrieve current git status: %s", err)
	}
	for path, fileStatus := range status {
		if path == RepositoryHelmIndexFilepath {
			continue
		}
		if fileStatus.Worktree == git.Untracked && fileStatus.Staging == git.Untracked {
			// Some charts were added
			if err := CreateOrUpdateHelmIndex(rootFs); err != nil {
				return fmt.Errorf("Sync was successful but was unable to update the Helm index")
			}
			logrus.Infof("Sync was successful! Your working directory is ready for a commit.")
			return nil
		}
	}
	if compareGAErr != nil {
		return compareGAErr
	}
	logrus.Infof("Nothing to sync. Working directory is up to date.")
	return nil
}

// ValidateRepository validates that the generated assets of the current repository doesn't conflict with the generated assets of the repository in upstreamConfig
func ValidateRepository(rootFs billy.Filesystem, upstreamConfig repository.GithubConfiguration, compareGeneratedAssetsOptions repository.CompareGeneratedAssetsOptions, currentChart string) error {
	// Create directories
	originalAssets := filepath.Join(ChartsRepositoryCurrentBranchDirpath, RepositoryAssetsDirpath)
	originalCharts := filepath.Join(ChartsRepositoryCurrentBranchDirpath, RepositoryChartsDirpath)
	newAssets := filepath.Join(ChartsRepositoryUpstreamBranchDirpath, RepositoryAssetsDirpath)
	newCharts := filepath.Join(ChartsRepositoryUpstreamBranchDirpath, RepositoryChartsDirpath)
	removeHelmIndex := false
	for _, d := range []string{RepositoryAssetsDirpath, RepositoryChartsDirpath, originalAssets, originalCharts, newAssets, newCharts} {
		existed, err := utils.PathExists(rootFs, d)
		if err != nil {
			return fmt.Errorf("Failed to check if path exists %s: %s", d, err)
		}
		if err := rootFs.MkdirAll(d, os.ModePerm); err != nil {
			return fmt.Errorf("Failed to make directory %s: %s", d, err)
		}
		defer utils.PruneEmptyDirsInPath(rootFs, d)
		if d == RepositoryAssetsDirpath || d == RepositoryChartsDirpath {
			if !existed {
				removeHelmIndex = true
			}
			continue
		}
		defer utils.RemoveAll(rootFs, d)
	}
	if removeHelmIndex {
		defer utils.RemoveAll(rootFs, RepositoryHelmIndexFilepath)
	}
	defer utils.RemoveAll(rootFs, ChartsRepositoryCurrentBranchDirpath)
	defer utils.RemoveAll(rootFs, ChartsRepositoryUpstreamBranchDirpath)
	// Copy current assets to new assets
	packages, err := GetPackages(rootFs.Root(), currentChart)
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", ChartsRepositoryUpstreamBranchDirpath, err)
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			return err
		}
	}
	if err := utils.CopyDir(rootFs, RepositoryAssetsDirpath, newAssets); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", RepositoryAssetsDirpath, newAssets, err)
	}
	if err := utils.CopyDir(rootFs, RepositoryChartsDirpath, newCharts); err != nil {
		return fmt.Errorf("Failed to copy %s into %s: %s", RepositoryChartsDirpath, newCharts, err)
	}
	// Copy upstream assets to original assets
	originalChartsUpstream := GetUpstreamForBranch(upstreamConfig, compareGeneratedAssetsOptions.WithBranch)
	if err := originalChartsUpstream.Pull(rootFs, rootFs, ChartsRepositoryCurrentBranchDirpath); err != nil {
		return fmt.Errorf("Failed to pull upstream configuration pointing to new charts: %s", err)
	}
	packages, err = GetPackages(utils.GetAbsPath(rootFs, ChartsRepositoryCurrentBranchDirpath), "")
	if err != nil {
		return fmt.Errorf("Failed to get packages in %s: %s", ChartsRepositoryCurrentBranchDirpath, err)
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

// CompareGeneratedAssets compares the newCharts against originalCharts and newAssets against originalAssets, while processing dropping release candidate versions if necessary
func CompareGeneratedAssets(rootFs billy.Filesystem, newCharts, newAssets, originalCharts, originalAssets string, dropReleaseCandidates bool, keepNewAssets bool) error {
	// Ensures that any modified files are cleared out, but not added files
	repo, err := utils.GetRepo(rootFs.Root())
	if err != nil {
		return fmt.Errorf("Could not retrieve the repository: %s", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("Could not retrieve current worktree: %s", err)
	}
	currentBranchRefName, err := utils.GetCurrentBranchRefName(repo)
	if err != nil {
		return fmt.Errorf("Could not get the current branch's reference name: %s", err)
	}
	checkCharts := newCharts
	checkAssets := newAssets
	if dropReleaseCandidates {
		newChartsWithoutRC := fmt.Sprintf("%s-without-rc", newCharts)
		newAssetsWithoutRC := fmt.Sprintf("%s-without-rc", newAssets)
		for _, d := range []string{newAssetsWithoutRC, newChartsWithoutRC} {
			if err := rootFs.MkdirAll(d, os.ModePerm); err != nil {
				return fmt.Errorf("Failed to make directory %s: %s", d, err)
			}
			defer utils.PruneEmptyDirsInPath(rootFs, d)
			defer utils.RemoveAll(rootFs, d)
		}
		// Only keep the biggest RC of any packageVersion
		visitedChart := make(map[string]bool)
		latestRC := make(map[string]string)
		err := utils.WalkDir(rootFs, newCharts, func(rootFs billy.Filesystem, path string, isDir bool) error {
			// new-assets/charts/{package}/{chart}
			if strings.Count(path, "/") != 3 {
				return nil
			}
			chart := filepath.Base(path)
			if _, ok := visitedChart[chart]; ok {
				// Already pruned this path
				return nil
			}
			// No need to visit again
			visitedChart[chart] = true
			fileInfos, err := rootFs.ReadDir(path)
			if err != nil {
				return fmt.Errorf("Encountered an error while trying to read directories within %s: %s", path, err)
			}
			for _, f := range fileInfos {
				chartVersion := f.Name()
				splitChartVersion := strings.Split(chartVersion, "-rc")
				chartVersionWithoutRC := strings.Join(splitChartVersion[:len(splitChartVersion)-1], "")
				latestRCSeenSoFar, ok := latestRC[chartVersionWithoutRC]
				if !ok {
					// First time seeing this RC
					latestRC[chartVersionWithoutRC] = chartVersion
					continue
				}
				// Compare with existing value
				if latestRCSeenSoFar >= chartVersion {
					continue
				}
				latestRC[chartVersionWithoutRC] = chartVersion
				if err := utils.RemoveAll(rootFs, filepath.Join(path, latestRCSeenSoFar)); err != nil {
					return fmt.Errorf("Failed to remove older RC %s: %s", filepath.Join(path, latestRCSeenSoFar), err)
				}
				logrus.Infof("Purged old release candidate version: %s", latestRCSeenSoFar)
			}
			return nil
		})
		if err != nil {
			return err
		}
		logrus.Infof("Found the following latest release candidate versions: %s", latestRC)
		// Export each helm chart to newChartsWithoutRC
		err = utils.WalkDir(rootFs, newCharts, func(rootFs billy.Filesystem, path string, isDir bool) error {
			// new-assets/charts/{package}/{chart}/{version}
			if strings.Count(path, "/") != 4 {
				return nil
			}
			if !isDir {
				return fmt.Errorf("Expected chart version to be found at %s, but that path does not represent a directory", path)
			}
			packageName := filepath.Base(filepath.Dir(filepath.Dir(path)))
			err := TrimRCVersionFromHelmMetadataVersion(rootFs, path)
			if err != nil {
				return fmt.Errorf("Encountered error when dropping rc from %s", path)
			}
			err = ExportHelmChart(rootFs, rootFs, path, "", filepath.Join(newAssetsWithoutRC, packageName), filepath.Join(newChartsWithoutRC, packageName))
			if err != nil {
				return fmt.Errorf("Encountered error when re-exporting latest releaseCandidateVersion of package without the version: %s", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		checkCharts = newChartsWithoutRC
		checkAssets = newAssetsWithoutRC
	}
	// level is 4 since the structure is charts/{package}/{chart}/{version}
	if err := DoesNotModifyContentsAtLevel(rootFs, originalCharts, checkCharts, 4); err != nil {
		return err
	}
	if !keepNewAssets {
		return nil
	}
	// Ensure that assets are kept by copying them into the assets and charts directory
	if err := utils.CopyDir(rootFs, checkAssets, RepositoryAssetsDirpath); err != nil {
		return fmt.Errorf("Encountered error while copying over new assets: %s", err)
	}
	if err := utils.CopyDir(rootFs, checkCharts, RepositoryChartsDirpath); err != nil {
		return fmt.Errorf("Encountered error while copying over new charts: %s", err)
	}
	// Ensure that you don't wipe out new assets on a clean
	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("Could not retrieve current git status: %s", err)
	}
	for p, fileStatus := range status {
		if fileStatus.Worktree == git.Untracked && fileStatus.Staging == git.Untracked {
			wt.Excludes = append(wt.Excludes, gitignore.ParsePattern(p, []string{}))
		}
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: currentBranchRefName,
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("Could not clean up current repository to get it ready for a commit: %s", err)
	}
	return nil
}
