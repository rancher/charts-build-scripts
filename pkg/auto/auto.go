package auto

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/sirupsen/logrus"
)

/**
* Shared data modeling for automating the,
* forward-port and release process of charts.
**/

// ForwardPort holds the data and methods to forward-port charts
type ForwardPort struct {
	yqPath                  string
	git                     *git.Git
	VR                      *lifecycle.VersionRules
	assetsToBeForwardPorted map[string][]lifecycle.Asset
	pullRequests            map[string]PullRequest
	forkRemoteURL           string
}

// PullRequest represents a pull request to be created for each chart separately
type PullRequest struct {
	branch   string
	commands []Command
}

// Command holds the necessary information to forward-port a chart
type Command struct {
	Chart   string   // The chart to forward-port
	Version string   // The version to forward-port
	Command []string // The command to run to forward-port
}

const (
	chartsRepoURL  = "https://github.com/rancher/charts"
	chartsRepoName = "charts"
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
		errYqPath := fmt.Errorf("error while getting yq path; err: %w", err)
		logrus.Error(errYqPath)
		return "", errYqPath
	}
	yqPath := strings.TrimSpace(string(output)) // Convert output to string and trim whitespace
	if yqPath == "" {
		errYq := errors.New("yq command not found")
		logrus.Error(errYq)
		return "", errYq
	}
	// Extract the directory from the yqPath and append the yq directory to the PATH
	yqDirPath := filepath.Dir(yqPath)
	currentPath := os.Getenv("PATH")
	newPath := yqDirPath + ":" + currentPath
	return newPath, nil
}

// createForwardPortCommands will create the forward port script commands for each asset and version,
// and return a sorted slice of commands
func (f *ForwardPort) createForwardPortCommands(chart string) ([]Command, error) {

	commands := make([]Command, 0)
	for asset, versions := range f.assetsToBeForwardPorted {
		if chart != "" && !strings.HasPrefix(asset, chart) {
			continue
		}
		for _, version := range versions {
			command, err := f.writeMakeCommand(asset, version.Version)
			if err != nil {
				return nil, err
			}
			commands = append(commands, command)
		}
	}
	// Sorting the commands slice by the Chart field in alphabetical order
	// and then by the Version field using semver
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Chart == commands[j].Chart {
			vi, err := semver.NewVersion(commands[i].Version)
			if err != nil {
				logrus.Errorf("Error parsing version '%s': %v", commands[i].Version, err)
				return false
			}
			vj, err := semver.NewVersion(commands[j].Version)
			if err != nil {
				logrus.Errorf("Error parsing version '%s': %v", commands[j].Version, err)
				return false
			}
			return vi.LessThan(vj)
		}
		return commands[i].Chart < commands[j].Chart
	})

	return commands, nil
}

// writeMakeCommand will write the forward-port command for the given asset and version
func (f *ForwardPort) writeMakeCommand(asset, version string) (Command, error) {
	/**
	* make forward-port
	* CHART=rancher-provisioning-capi
	* VERSION=100.0.0+up0.0.1
	* BRANCH=dev-v2.9
	* UPSTREAM=upstream
	 */

	upstreamRemote, ok := f.git.Remotes["https://github.com/rancher/charts"]
	if !ok {
		errNoUpstreamRemote := errors.New("upstream remote not found; you need to have the upstream remote configured in your git repository (https://github.com/rancher/charts)")
		logrus.Error(errNoUpstreamRemote)
		return Command{}, errNoUpstreamRemote
	}
	commands := []string{
		"make",
		"forward-port",
		"CHART=" + asset,
		"VERSION=" + version,
		"BRANCH=" + f.VR.DevBranch,
		"UPSTREAM=" + upstreamRemote,
	}

	return Command{Chart: asset, Command: commands, Version: version}, nil
}

// checkIfChartChanged will check if the chart has changed from the last chart.
// It will return true if the chart has changed, otherwise it will return false.
// If the chart has changed, it will check for special cases like (fleet and neuvector), and CRD dependencies.
func checkIfChartChanged(lastChart, currentChart string) bool {
	// Check if the current chart is different from the last chart
	if lastChart == currentChart {
		return false
	}

	// Check for special edge cases
	return checkEdgeCasesIfChartChanged(lastChart, currentChart)
}

// checkEdgeCasesIfChartChanged will check for special cases like:
//
//	-CRD dependencies
//	-fleet
//	-neuvector
//	-rancher-alerting-driver + rancher-aks-operator
//	-rancher-gke-operator + rancher-gatekeeper
//
// It will return true if the chart changed, false otherwise
func checkEdgeCasesIfChartChanged(lastChart, currentChart string) bool {

	lastParts := strings.Split(lastChart, "-")
	currentParts := strings.Split(currentChart, "-")

	minLength := 0 // compare which chart is shorter in name (last or current)
	if len(currentParts) < len(lastParts) {
		minLength = len(currentParts)
	} else {
		minLength = len(lastParts)
	}

	equalCounter := 0
	for i := 0; i < minLength; i++ {
		if lastParts[i] == currentParts[i] {
			equalCounter++
		}
	}

	if equalCounter == 0 {
		return true
	} else if equalCounter == 1 && strings.HasPrefix("rancher-", lastParts[0]) && minLength < 3 {
		return true
	} else if equalCounter == 1 && minLength >= 3 {
		return true
	}

	// treat operators edge cases
	// rancher-aks-operator && rancher-gke-operator || rancher-aks-operator-crd && rancher-gke-operator-crd
	if (equalCounter == 2 && minLength == 3) || (equalCounter == 3 && minLength == 4) {
		if lastParts[0] == "rancher" && currentParts[0] == "rancher" {
			if lastParts[2] == "operator" && currentParts[2] == "operator" {
				if lastParts[1] != currentParts[1] {
					return true
				}
			}
		}
	}

	return false
}

// createNewBranchToForwardPort will create a new branch to forward-port the assets
func (f *ForwardPort) createNewBranchToForwardPort(branch string) error {
	// check if git is clean and branch is up-to-date
	err := f.git.IsClean()
	if err != nil {
		return err
	}
	// create new branch and checkout
	return f.git.CreateAndCheckoutBranch(branch)
}

// prepareReleaseYaml will prepare the release.yaml file by erasing its content,
// this is a good practice before releasing or forward-porting any charts with it.
func prepareReleaseYaml() error {
	// Check if the file exists
	_, err := os.Stat("release.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			errNoReleaseYaml := fmt.Errorf("release.yaml does not exist; err: %w", err)
			logrus.Error(errNoReleaseYaml)
			return errNoReleaseYaml
		}
		errReleaseYaml := fmt.Errorf("release.yaml failure; err: %w", err)
		logrus.Error(errReleaseYaml)
		return errReleaseYaml
	}

	// File exists, truncate the file to erase its content
	if err := os.Truncate("release.yaml", 0); err != nil {
		errTruncate := fmt.Errorf("release.yaml failure while truncating it; err: %w", err)
		logrus.Error(errTruncate)
		return errTruncate
	}

	logrus.Info("Content of release.yaml erased successfully.")
	return nil
}

// executeCommand will execute the given command using the yqPath if needed
func executeCommand(command []string, yqPath string) error {
	// Prepare the command
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+yqPath)

	// Set the command's stdout and stderr to the current process's stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute it
	if err := cmd.Run(); err != nil {
		errCommand := fmt.Errorf("error while executing command: %s; err: %w", command, err)
		logrus.Error(errCommand)
		return errCommand
	}
	return nil
}
