package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
)

// GetRepo returns an existing GitRepository at the path provided
func GetRepo(repoPath string) (*git.Repository, error) {
	return git.PlainOpen(repoPath)
}

// CreateRepo returns a newly generated GitRepository at the path provided
func CreateRepo(repoPath string) (*git.Repository, error) {
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("Encountered error while trying to create directory for new repo: %s", err)
	}
	return git.PlainInit(repoPath, false)
}

// CreateBranch creates a new branch within the given repository or returns an error
func CreateBranch(repo *git.Repository, branch string, hash plumbing.Hash) error {
	hashRef := plumbing.NewHashReference(GetLocalBranchRefName(branch), hash)
	return repo.Storer.SetReference(hashRef)
}

// GetCurrentBranch gets the current branch of the repositry or returns an error
func GetCurrentBranch(repo *git.Repository) (string, error) {
	branchRefName, err := GetCurrentBranchRefName(repo)
	return branchRefName.Short(), err
}

// GetCurrentBranchRefName gets the current branch of the repositry or returns an error
func GetCurrentBranchRefName(repo *git.Repository) (plumbing.ReferenceName, error) {
	headRef, err := repo.Head()
	if err != nil {
		return "", err
	}
	branchRefName := headRef.Name()
	if !branchRefName.IsBranch() {
		return "", fmt.Errorf("head does not point to branch")
	}
	return branchRefName, nil
}

// CheckoutBranch checks out the branch provided in the given repository or returns an error
func CheckoutBranch(repo *git.Repository, branch string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: GetLocalBranchRefName(branch),
	})
	return err
}

// CommitAll commits all changes, whether or not they have been staged, and creates a commit
func CommitAll(repo *git.Repository, commitMessage string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	status, err := wt.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		return fmt.Errorf("Cannot create commit since there are no files to be committed")
	}
	logrus.Infof("Committing the following modified files:\n%s", status)
	if _, err = wt.Add("."); err != nil {
		return err
	}
	_, err = wt.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name: "charts-build-scripts",
			When: time.Now(),
		},
	})
	return err
}

// GetHead returns the HEAD hash of the current branch
func GetHead(repo *git.Repository) (plumbing.Hash, error) {
	headRef, err := repo.Head()
	if err != nil {
		return plumbing.Hash{}, err
	}
	return headRef.Hash(), nil
}

// GetRepoPath returns the path to the repo in the local filesystem
func GetRepoPath(repo *git.Repository) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	return wt.Filesystem.Root(), nil
}

// CreateInitialCommit creates an initial commit for a Github repository at the repoPath provided
func CreateInitialCommit(repo *git.Repository) error {
	repoPath, err := GetRepoPath(repo)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(path.Join(repoPath, "README.md"), []byte{}, 0644); err != nil {
		return err
	}
	return CommitAll(repo, "Create initial commit")
}

// GetLocalBranchRefName returns the reference name of a given local branch
func GetLocalBranchRefName(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}

// GetRemoteBranchRefName returns the reference name of a given remote branch
func GetRemoteBranchRefName(branch, remote string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/remote/%s/%s", remote, branch))
}
