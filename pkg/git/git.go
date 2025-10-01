package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// Git struct holds necessary data to work with the current git repository
type Git struct {
	Dir     string
	Branch  string
	Remotes map[string]string
}

// GitRepo uses singleton pattern to access a local git repository
var GitRepo *Git

// CloneAtDir clones a repository at a given directory.
// Equivalent to: git clone <url> <dir>
// It will return a Git struct with the repository's branch and remotes populated.
func CloneAtDir(ctx context.Context, url, dir string) (*Git, error) {
	var err error

	logger.Log(ctx, slog.LevelInfo, "cloning repository", slog.String("url", url), slog.String("dir", dir))

	cmd := exec.Command("git", "clone", "--depth", "1", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.Log(ctx, slog.LevelError, "error while cloning repository", slog.String("url", url), slog.String("dir", dir), logger.Err(err))
		return nil, err
	}

	git := &Git{
		Dir: dir,
	}

	git.Branch, err = git.getGitBranch(ctx)
	if err != nil {
		return nil, err
	}

	git.Remotes, err = git.getGitRemotes(ctx)
	if err != nil {
		return nil, err
	}

	return git, nil
}

// OpenGitRepo will check if the target workingDir exists and is a local git repository.
// will also get the current branch and configured remotes.
func OpenGitRepo(ctx context.Context, workingDir string) (*Git, error) {
	if GitRepo == nil {
		if err := checkGitFolder(ctx, workingDir); err != nil {
			return nil, err
		}

		GitRepo = &Git{
			Dir: workingDir,
		}
	}

	if GitRepo.Dir != workingDir {
		if err := checkGitFolder(ctx, workingDir); err != nil {
			return nil, err
		}
		GitRepo.Dir = workingDir
	}

	var err error

	GitRepo.Branch, err = GitRepo.getGitBranch(ctx)
	if err != nil {
		return nil, err
	}

	GitRepo.Remotes, err = GitRepo.getGitRemotes(ctx)
	if err != nil {
		return nil, err
	}

	logger.Log(ctx, slog.LevelDebug, "git repo", slog.String("branch", GitRepo.Branch), slog.Any("remotes", GitRepo.Remotes))
	return GitRepo, nil
}

func checkGitFolder(ctx context.Context, workingDir string) error {
	logger.Log(ctx, slog.LevelDebug, "opening git repo", slog.String("dir", workingDir))

	gitFolder := workingDir + "/.git"

	_, err := os.Stat(gitFolder)
	if os.IsNotExist(err) {
		return errors.New(workingDir + " is not a git repository")
	}
	if err != nil {
		return fmt.Errorf("error while checking if %s is a git repository: %s", workingDir, err)
	}

	return nil
}

// getGitBranch returns the current branch of the git repository
func (g *Git) getGitBranch(ctx context.Context) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.Dir // Set the working directory

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to get git branch", logger.Err(err))
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

// getGitRemotes returns the remotes of the git repository as a map
func (g *Git) getGitRemotes(ctx context.Context) (map[string]string, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = g.Dir // Set the working directory

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to get git remotes", logger.Err(err))
		return nil, err
	}

	remotes := make(map[string]string)
	for _, line := range strings.Split(out.String(), "\n") {
		if line == "" {
			continue // Skip empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue // Skip if there are not enough fields
		}

		remoteName := fields[0]
		remoteURL := fields[1]
		// Assuming we want to avoid duplicates and only need one URL per remote,
		// we check if the remote is already in the map.
		if _, exists := remotes[remoteName]; !exists {
			remotes[remoteURL] = remoteName
		}
	}

	return remotes, nil
}

// FetchAndPullBranch fetches and pulls a branch
func (g *Git) FetchAndPullBranch(ctx context.Context, branch string) error {
	logger.Log(ctx, slog.LevelInfo, "fetching and pulling branch", slog.String("branch", branch))

	upstreamRemote, err := g.getUpstreamRemote()
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "-C", g.Dir, "fetch", upstreamRemote, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "-C", g.Dir, "pull", upstreamRemote, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// FetchAndCheckoutBranch fetches and checks out a branch
func (g *Git) FetchAndCheckoutBranch(ctx context.Context, branch string) error {
	logger.Log(ctx, slog.LevelInfo, "fetching and checking out branch", slog.String("branch", branch))

	err := g.FetchBranch(branch)
	if err != nil {
		return err
	}
	return g.CheckoutBranch(branch)
}

// FetchBranch fetches a branch
func (g *Git) FetchBranch(branch string) error {
	upstreamRemote, err := g.getUpstreamRemote()
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "-C", g.Dir, "fetch", upstreamRemote, branch+":"+branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckoutBranch checks out a branch
func (g *Git) CheckoutBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "checkout", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckoutFile checks out a file in a branch
// ex: git checkout <remote>/<branch> -- <file>
func (g *Git) CheckoutFile(branch, file string) error {
	upstreamRemote, err := g.getUpstreamRemote()
	if err != nil {
		return err
	}

	targetBranch := upstreamRemote + "/" + branch
	cmd := exec.Command("git", "-C", g.Dir, "checkout", targetBranch, "--", file)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CreateAndCheckoutBranch creates and checks out to a given branch.
// Equivalent to: git checkout -b <branch>
func (g *Git) CreateAndCheckoutBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "checkout", "-b", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsClean checks if the git repository is clean and,
// returns nil if it is clean, throws an error otherwise
func (g *Git) IsClean(ctx context.Context) error {
	clean, err := g.StatusProcelain(ctx)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to check git status", logger.Err(err))
		return err
	}
	if !clean {
		logger.Log(ctx, slog.LevelError, "git repo should be clean")
		return fmt.Errorf("git repo should be clean")
	}
	return nil
}

// StatusProcelain checks if the git repository is clean and,
// returns true if it is clean, false otherwise
func (g *Git) StatusProcelain(ctx context.Context) (bool, error) {
	logger.Log(ctx, slog.LevelDebug, "check if git is clean")

	cmd := exec.Command("git", "-C", g.Dir, "status", "--porcelain")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	if len(output) > 0 {
		logger.Log(ctx, slog.LevelDebug, "git is not clean")
		return false, nil
	}

	logger.Log(ctx, slog.LevelDebug, "git is clean")
	return true, nil
}

// AddAndCommit stages all changes and commits them with a given message,
// equivalent to: git add -A && git commit -m message
func (g *Git) AddAndCommit(message string) error {
	// Stage all changes, including deletions
	cmd := exec.Command("git", "-C", g.Dir, "add", "-A")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Commit the staged changes
	cmd2 := exec.Command("git", "commit", "-m", message)
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	return cmd2.Run()
}

// PushBranch pushes the current branch to a given remote name
func (g *Git) PushBranch(remote, branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "push", remote, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DeleteBranch deletes the given branch
func (g *Git) DeleteBranch(branch string) error {
	return exec.Command("git", "-C", g.Dir, "branch", "-D", branch).Run()
}

// CheckFileExists checks if a file exists in the git repository for a specific branch
func (g *Git) CheckFileExists(file, branch string) error {
	upstreamRemote, err := g.getUpstreamRemote()
	if err != nil {
		return err
	}

	target := upstreamRemote + "/" + branch + ":" + file
	return exec.Command("git", "-C", g.Dir, "cat-file", "-e", target).Run()
}

// FullReset performs a hard reset, cleans the repository and restores it
func (g *Git) FullReset() error {
	if err := g.HardHEADReset(); err != nil {
		return err
	}
	if err := g.ForceClean(); err != nil {
		return err
	}
	return g.Restore()
}

// HardHEADReset = git reset --hard HEAD
func (g *Git) HardHEADReset() error {
	return exec.Command("git", "-C", g.Dir, "reset", "--hard", "HEAD").Run()
}

// ForceClean = git clean -fdx
func (g *Git) ForceClean() error {
	return exec.Command("git", "-C", g.Dir, "clean", "-fdx").Run()
}

// Restore = git restore .
func (g *Git) Restore() error {
	return exec.Command("git", "-C", g.Dir, "restore", ".").Run()
}

// ResetHEAD resets the HEAD of the git repository
// ex: git reset HEAD
func (g *Git) ResetHEAD() error {
	return exec.Command("git", "-C", g.Dir, "reset", "HEAD").Run()
}

func (g *Git) getUpstreamRemote() (string, error) {
	upstreamRemote := g.Remotes["https://github.com/rancher/charts"]
	if upstreamRemote == "" {
		upstreamRemote = g.Remotes["git@github.com:rancher/charts.git"]
	}

	if upstreamRemote == "" {
		return "", errors.New("upstream remote not configured")
	}

	return upstreamRemote, nil
}

// Status prints the status of the git repository
// equivalent to: git status --untracked-files=all --short
func (g *Git) Status(ctx context.Context) error {
	logger.Log(ctx, slog.LevelDebug, "git status -u -s")

	cmd := exec.Command("git", "-C", g.Dir, "status", "--untracked-files=all", "--short")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
