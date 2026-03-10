package git

import (
	"context"
	"fmt"
	"log/slog"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rancher/charts-build-scripts/pkg/logger"
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

// GetChangedFiles opens the git repository at repoRoot and returns all files
// that were added or modified between the merge base of HEAD and origin/baseBranch.
//
// Equivalent to: git diff origin/<baseBranch>...HEAD
// Correctly handles PRs with any number of commits via merge base resolution.
func GetChangedFiles(ctx context.Context, repoPath, baseBranch string) (object.Changes, error) {
	logger.Log(ctx, slog.LevelInfo, "getting changed files against", slog.String("baseBranch", baseBranch))
	repo, err := gogit.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository at %s: %w", repoPath, err)
	}

	// HEAD commit
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HEAD: %w", err)
	}
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// origin/baseBranch commit
	baseRefName := plumbing.NewRemoteReferenceName("origin", baseBranch)
	baseRef, err := repo.Reference(baseRefName, true)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve origin/%s: %w", baseBranch, err)
	}
	baseCommit, err := repo.CommitObject(baseRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get base commit for origin/%s: %w", baseBranch, err)
	}

	// Find merge base (common ancestor) — handles PRs with any number of commits
	bases, err := headCommit.MergeBase(baseCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to find merge base with origin/%s: %w", baseBranch, err)
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("no common ancestor found between HEAD and origin/%s", baseBranch)
	}

	// Tree diff: merge base → HEAD
	baseTree, err := bases[0].Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get merge base tree: %w", err)
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD tree: %w", err)
	}

	changes, err := object.DiffTree(baseTree, headTree)
	if err != nil {
		return nil, fmt.Errorf("failed to diff trees: %w", err)
	}

	logger.Log(ctx, slog.LevelInfo, "changed files", slog.Int("count", len(changes)))
	for _, c := range changes {
		action, err := c.Action()
		if err != nil {
			continue
		}
		name := c.To.Name
		if name == "" {
			name = c.From.Name
		}
		logger.Log(ctx, slog.LevelDebug, "changed file", slog.String("path", name), slog.String("action", action.String()))
	}
	return changes, nil
}
