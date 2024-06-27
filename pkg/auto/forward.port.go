package auto

import (
	"fmt"

	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/sirupsen/logrus"
)

/**
* forward.port.go executes specific methods and functions
* to automate the forward-port process.
**/

// CreateForwardPortStructure will create the ForwardPort struct with access to the necessary dependencies.
// It will also check if yq command is installed on the system.
func CreateForwardPortStructure(ld *lifecycle.Dependencies, assetsToPort map[string][]lifecycle.Asset) (*ForwardPort, error) {
	// is yq installed?
	yqPath, err := whichYQCommand()
	if err != nil {
		logrus.Errorf("yq must be installed on system before this script can run")
		logrus.Errorf("Please install yq and try again")
		logrus.Error("go install github.com/mikefarah/yq/v4@latest")
	}
	return &ForwardPort{
		yqPath:                  yqPath,
		git:                     ld.Git,
		VR:                      ld.VR,
		assetsToBeForwardPorted: assetsToPort,
		pullRequests:            make(map[string]PullRequest),
	}, err
}

// ExecuteForwardPort will execute all steps to organize and create the forward-port PRs
func (fp *ForwardPort) ExecuteForwardPort(chart string) error {
	// Get the forward port script commands
	commands, err := fp.createForwardPortCommands(chart)
	if err != nil {
		return err
	}
	// Organize the commands into pull requests grouping by chart with it's dependencies
	fp.organizePullRequestsByChart(commands)
	return nil
}

// organizePullRequests will organize the commands into pull requests
func (fp *ForwardPort) organizePullRequestsByChart(commands []Command) {
	lastChart := ""

	for _, command := range commands {

		changed := checkIfChartChanged(lastChart, command.Chart)

		// If the chart is the same as the last chart, append the command to the pull request
		if !changed {
			pr := fp.pullRequests[lastChart]           // Extract the struct from the map
			pr.commands = append(pr.commands, command) // Modify the struct's field
			fp.pullRequests[lastChart] = pr            // Put the modified struct back into the map
		}

		// If the chart is different from the last chart, organize a new pull request
		if changed {
			pr := PullRequest{
				branch:   fmt.Sprintf("auto-forward-port-%s-%.1f", command.Chart, fp.VR.BranchVersion),
				commands: []Command{command},
			}
			fp.pullRequests[command.Chart] = pr
			lastChart = command.Chart
		}
	}
}
