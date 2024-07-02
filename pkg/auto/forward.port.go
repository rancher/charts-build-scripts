package auto

import (
	"fmt"

	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/sirupsen/logrus"
)

/**
* forward.port.go executes specific methods and functions
* to automate the forward-port process.
**/

// CreateForwardPortStructure will create the ForwardPort struct with access to the necessary dependencies.
// It will also check if yq command is installed on the system.
func CreateForwardPortStructure(ld *lifecycle.Dependencies, assetsToPort map[string][]lifecycle.Asset, forkURL string) (*ForwardPort, error) {
	// is yq installed?
	yqPath, err := whichYQCommand()
	if err != nil {
		logrus.Errorf("yq must be installed on system before this script can run")
		logrus.Errorf("Please install yq and try again")
		logrus.Error("go install github.com/mikefarah/yq/v4@latest")
	}

	_, ok := ld.Git.Remotes[forkURL]
	if !ok {
		logrus.Errorf("Remote %s not found in git remotes, you need to configure your fork on your git remote references", forkURL)
		return nil, fmt.Errorf("Remote %s not found in git remotes, you need to configure your fork on your git remote references", forkURL)
	}

	isForkRemoteConfigured := git.CheckForValidForkRemote(chartsRepoURL, forkURL, chartsRepoName)
	if !isForkRemoteConfigured {
		logrus.Errorf("Remote %s not configured correctly, you need to configure your fork on your git remote references", forkURL)
		return nil, fmt.Errorf("Remote %s not configured correctly, you need to configure your fork on your git remote references", forkURL)
	}

	return &ForwardPort{
		yqPath:                  yqPath,
		git:                     ld.Git,
		VR:                      ld.VR,
		assetsToBeForwardPorted: assetsToPort,
		pullRequests:            make(map[string]PullRequest),
		forkRemoteURL:           forkURL,
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
	// Execute the forward port commands
	return fp.executeForwardPorts()
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

// executeForwardPorts will execute the forward-port commands.
// It will create a new branch to forward-port the assets, clean the release.yaml file and commit.
// After each forward-port execution it will add and commit the changes to the git repository.
// It will push the branch to the remote repository and delete the local branch before moving,
// to the next Pull Request.
func (fp *ForwardPort) executeForwardPorts() error {
	// save the original branch to change back after forward-port
	originalBranch := fp.git.Branch

	// create log file
	fpLogs, err := lifecycle.CreateLogs("forward-ported", "")
	defer fpLogs.File.Close()
	if err != nil {
		return err
	}

	// write log title and header
	fpLogs.WriteHEAD(fp.VR, "Forward-Ported Assets")

	for asset, pr := range fp.pullRequests {
		fpLogs.Write(asset, "INFO")

		// open and check if it is clean the git repo
		err := fp.createNewBranchToForwardPort(pr.branch)
		if err != nil {
			return err
		}

		// clean release.yaml in the new branch
		if err = prepareReleaseYaml(); err != nil {
			return err
		}
		// git add && commit cleaned release.yaml
		if err = fp.git.AddAndCommit("cleaning release.yaml"); err != nil {
			return err
		}

		fpLogs.Write(fmt.Sprintf("Branch: %s", pr.branch), "INFO")

		for _, command := range pr.commands {
			// execute make forward-port
			err := executeCommand(command.Command, fp.yqPath)
			if err != nil {
				return err
			}
			// git add && commit the changes
			msg := fmt.Sprintf("forward-port %s %s", command.Chart, command.Version)
			if err = fp.git.AddAndCommit(msg); err != nil {
				return err
			}
			// Log this so later we can merge the PRs
			fpLogs.Write(msg, "")
		}
		// push branch
		err = fp.git.PushBranch(fp.git.Remotes[fp.forkRemoteURL], pr.branch)
		if err != nil {
			return err
		}
		// save to log file branch
		fpLogs.Write("PUSHED", "INFO")
		// Change back to the original branch to avoid conflicts
		err = fp.git.CheckoutBranch(originalBranch)
		if err != nil {
			return err
		}
		// delete local created and pushed branch
		err = fp.git.DeleteBranch(pr.branch)
		if err != nil {
			return err
		}
	}
	return nil
}
