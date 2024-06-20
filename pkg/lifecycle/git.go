package lifecycle

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

type Git struct {
	Dir string
}

// cloneAtDir clones a repository at a given directory
func cloneAtDir(url, dir string) (*Git, error) {
	logrus.Infof("Cloning repository %s into %s", url, dir)
	cmd := exec.Command("git", "clone", "--depth", "1", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error while cloning repository: %s; err: %v", url, err)
		return nil, fmt.Errorf("error while cloning repository: %s", err)
	}
	return &Git{Dir: dir}, nil
}

// checkIfGitIsClean checks if the git repository is clean and,
// returns true if it is clean, false otherwise
func checkIfGitIsClean(debug bool) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
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

// gitAddAndCommit stages all changes and commits them with a given message,
// equivalent to: git add -A && git commit -m message
func gitAddAndCommit(message string) error {
	// Stage all changes, including deletions
	cmd := exec.Command("git", "add", "-A")
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
