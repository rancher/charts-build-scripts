package auto

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

/**
* These are the common methods and functions
* to be used by the automations to forward-port and release charts.
**/

// whichYQCommand will return the PATH with the yq directory appended to it,
// if yq command is found in the system, otherwise it will return an error.
func whichYQCommand() (string, error) {
	cmd := exec.Command("which", "yq")
	output, err := cmd.Output() // Capture the output instead of printing it
	if err != nil {
		logrus.Errorf("error while getting yq path; err: %v", err)
		return "", fmt.Errorf("error while executing command: %s", err)
	}
	yqPath := strings.TrimSpace(string(output)) // Convert output to string and trim whitespace
	if yqPath == "" {
		return "", fmt.Errorf("yq command not found")
	}
	// Extract the directory from the yqPath
	yqDirPath := filepath.Dir(yqPath)
	// Append the yq directory to the PATH
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", yqDirPath, currentPath)
	return newPath, nil
}
