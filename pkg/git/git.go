package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// Git struct holds necessary data to work with the current git repository
type Git struct {
	Dir     string
	Branch  string
	Remotes map[string]string
}

// CloneAtDir clones a repository at a given directory.
// Equivalent to: git clone <url> <dir>
// It will return a Git struct with the repository's branch and remotes populated.
func CloneAtDir(url, dir string) (*Git, error) {
	var err error
	logrus.Infof("Cloning repository %s into %s", url, dir)
	cmd := exec.Command("git", "clone", "--depth", "1", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while cloning repository: %s; err: %v", url, err)
		return nil, fmt.Errorf("error while cloning repository: %s", err)
	}

	git := &Git{
		Dir: dir,
	}

	git.Branch, err = git.getGitBranch()
	if err != nil {
		return nil, err
	}

	git.Remotes, err = git.getGitRemotes()
	if err != nil {
		return nil, err
	}

	return git, nil
}

// OpenGitRepo TODO: Docs
func OpenGitRepo(workingDir string) (*Git, error) {
	logrus.Debugf("Opening git repository at %s", workingDir)

	gitFolder := fmt.Sprintf("%s/.git", workingDir)
	_, err := os.Stat(gitFolder)
	if os.IsNotExist(err) {
		logrus.Errorf("%s is not a git repository", workingDir)
		return nil, fmt.Errorf("%s is not a git repository", workingDir)
	}
	if err != nil {
		return nil, fmt.Errorf("error while checking if %s is a git repository: %s", workingDir, err)
	}

	git := &Git{
		Dir: workingDir,
	}

	git.Branch, err = git.getGitBranch()
	if err != nil {
		return nil, err
	}

	git.Remotes, err = git.getGitRemotes()
	if err != nil {
		return nil, err
	}

	return git, nil
}

// getGitBranch returns the current branch of the git repository
func (g *Git) getGitBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.Dir // Set the working directory

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		logrus.Errorf("error while getting git branch: %s", err)
		return "", fmt.Errorf("error while getting git branch: %s", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// getGitRemotes returns the remotes of the git repository as a map
func (g *Git) getGitRemotes() (map[string]string, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = g.Dir // Set the working directory

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		logrus.Errorf("error while getting git remotes: %s", err)
		return nil, fmt.Errorf("error while getting git remotes; err: %v", err)
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
func (g *Git) FetchAndPullBranch(branch string) error {
	logrus.Infof("Fetching and pulling branch %s", branch)
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
func (g *Git) FetchAndCheckoutBranch(branch string) error {
	logrus.Infof("Fetching and checking out at: %s", g.Branch)
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
func (g *Git) IsClean() error {
	clean, err := g.StatusProcelain()
	if err != nil {
		logrus.Errorf("error while checking if git is clean: %s", err)
		return fmt.Errorf("error while checking if git is clean: %s", err)
	}
	if !clean {
		logrus.Errorf("git must be clean to forward-port")
		return fmt.Errorf("git repo should be clean")
	}
	return nil
}

// StatusProcelain checks if the git repository is clean and,
// returns true if it is clean, false otherwise
func (g *Git) StatusProcelain() (bool, error) {
	cmd := exec.Command("git", "-C", g.Dir, "status", "--porcelain")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	if len(output) > 0 {
		return false, nil
	}
	return true, nil
}

// AddAndCommit stages all changes and commits them with a given message,
// equivalent to: git add -A && git commit -m message
func (g *Git) AddAndCommit(message string) error {
	// Stage all changes, including deletions
	if err := exec.Command("git", "-C", g.Dir, "add", "-A").Run(); err != nil {
		return err
	}

	// Commit the staged changes
	return exec.Command("git", "commit", "-m", message).Run()
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
