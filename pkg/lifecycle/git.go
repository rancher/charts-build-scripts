package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
)

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
