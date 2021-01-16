package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/rancher/charts-build-scripts/pkg/change"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/sirupsen/logrus"
)

const (
	temporaryBranchName = "charts-build-scripts-temporary-branch-012345"
)

// CompareGeneratedAssets compares the newCharts against originalCharts and newAssets against originalAssets, while processing dropping release candidate versions if necessary
func CompareGeneratedAssets(rootFs billy.Filesystem, newCharts, newAssets, originalCharts, originalAssets string, dropReleaseCandidates bool, keepNewAssets bool) error {
	// Ensures that any modified files are cleared out, but not added files
	repo, err := repository.GetRepo(rootFs.Root())
	if err != nil {
		return fmt.Errorf("Could not retrieve the repository: %s", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("Could not retrieve current worktree: %s", err)
	}
	currentBranchRefName, err := repository.GetCurrentBranchRefName(repo)
	if err != nil {
		logrus.Warnf("Encountered error while trying to get the current branch's reference name: %s", err)
		logrus.Warnf("Using current head hash to create a brand new branch that will be used for making changes")
		// Operating in detached mode, so use the commit instead
		currentHeadHash, err := repository.GetHead(repo)
		if err != nil {
			return fmt.Errorf("Could not get head hash reference: %s", err)
		}
		if err := repository.CreateBranch(repo, temporaryBranchName, currentHeadHash); err != nil {
			return fmt.Errorf("Could not create new branch from detached head: %s", err)
		}
		defer logrus.Warnf("You must manually clean up the branch %s", temporaryBranchName)
		currentBranchRefName = repository.GetLocalBranchRefName(temporaryBranchName)
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
			defer filesystem.PruneEmptyDirsInPath(rootFs, d)
			defer filesystem.RemoveAll(rootFs, d)
		}
		// Only keep the biggest RC of any packageVersion
		visitedChart := make(map[string]bool)
		latestRC := make(map[string]string)
		err := filesystem.WalkDir(rootFs, newCharts, func(rootFs billy.Filesystem, path string, isDir bool) error {
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
				if err := filesystem.RemoveAll(rootFs, filepath.Join(path, latestRCSeenSoFar)); err != nil {
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
		err = filesystem.WalkDir(rootFs, newCharts, func(rootFs billy.Filesystem, path string, isDir bool) error {
			// new-assets/charts/{package}/{chart}/{version}
			if strings.Count(path, "/") != 4 {
				return nil
			}
			if !isDir {
				return fmt.Errorf("Expected chart version to be found at %s, but that path does not represent a directory", path)
			}
			packageName := filepath.Base(filepath.Dir(filepath.Dir(path)))
			err := helm.TrimRCVersionFromHelmMetadataVersion(rootFs, path)
			if err != nil {
				return fmt.Errorf("Encountered error when dropping rc from %s", path)
			}
			err = helm.ExportHelmChart(rootFs, rootFs, path, "", filepath.Join(newAssetsWithoutRC, packageName), filepath.Join(newChartsWithoutRC, packageName))
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
	if err := change.DoesNotModifyContentsAtLevel(rootFs, originalCharts, checkCharts, 4); err != nil {
		return err
	}
	if !keepNewAssets {
		return nil
	}
	// Ensure that assets are kept by copying them into the assets and charts directory
	if err := filesystem.CopyDir(rootFs, checkAssets, path.RepositoryAssetsDir); err != nil {
		return fmt.Errorf("Encountered error while copying over new assets: %s", err)
	}
	if err := filesystem.CopyDir(rootFs, checkCharts, path.RepositoryChartsDir); err != nil {
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
