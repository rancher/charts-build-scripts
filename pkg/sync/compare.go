package sync

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/rancher/charts-build-scripts/pkg/change"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/sirupsen/logrus"
)

const (
	temporaryBranchName = "charts-build-scripts-temporary-branch-012345"
)

// CompareGeneratedAssets compares the newCharts against originalCharts and newAssets against originalAssets
func CompareGeneratedAssets(rootFs billy.Filesystem, newCharts, newAssets, originalCharts, originalAssets string, keepNewAssets bool) error {
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
