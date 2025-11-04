package git

import (
	"context"
	"fmt"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GetLocalBranchRefName returns the reference name of a given local branch
func GetLocalBranchRefName(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}

// GetGitInfo returns the git repository, worktree, and status for a given repository root path.
// This function is used for validation operations that need to check git status and repository state.
// It uses go-git library instead of exec commands.
func GetGitInfo(ctx context.Context, repoRoot string) (*gogit.Repository, *gogit.Worktree, gogit.Status, error) {
	repo, err := gogit.PlainOpen(repoRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open git repository at %s: %w", repoRoot, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get git status: %w", err)
	}

	return repo, wt, status, nil
}
