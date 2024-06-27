package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

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
	cmd := exec.Command("git", "-C", g.Dir, "fetch", "origin", branch+":"+branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while fetching branch: %s; err: %v", branch, err)
		return fmt.Errorf("error while fetching branch: %s", err)
	}
	return nil
}

// CheckoutBranch checks out a branch
func (g *Git) CheckoutBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "checkout", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while checking out branch: %s; err: %v", branch, err)
		return fmt.Errorf("error while checking out branch: %s", err)
	}
	return nil
}

// CreateAndCheckoutBranch creates and checks out to a given branch.
// Equivalent to: git checkout -b <branch>
func (g *Git) CreateAndCheckoutBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "checkout", "-b", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while creating and checking out branch: %s; err: %v", branch, err)
		return fmt.Errorf("error while creating and checking out branch: %s", err)
	}
	return nil
}

// IsClean checks if the git repository is clean and,
// returns nil if it is clean, throws an error otherwise
func (g *Git) IsClean() error {
	clean, err := g.StatusProcelain(true)
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
func (g *Git) StatusProcelain(debug bool) (bool, error) {
	cmd := exec.Command("git", "-C", g.Dir, "status", "--porcelain")
	if debug {
		cmd := exec.Command("git", "status")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("error while checking if git is clean: %s", err)
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
	cmd := exec.Command("git", "-C", g.Dir, "add", "-A")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Commit the staged changes
	cmd = exec.Command("git", "commit", "-m", message)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// PushBranch pushes the current branch to a given remote name
func (g *Git) PushBranch(remote, branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "push", g.Remotes[remote], branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while pushing branch: %s; err: %v", branch, err)
		return fmt.Errorf("error while pushing branch: %s", err)
	}
	return nil
}

// DeleteBranch deletes the given branch
func (g *Git) DeleteBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.Dir, "branch", "-D", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while deleting branch: %s; err: %v", g.Branch, err)
		return fmt.Errorf("error while deleting branch: %s", err)
	}
	return nil
}
