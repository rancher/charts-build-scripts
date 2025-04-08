package auto

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

/**
* forward.port.go executes specific methods and functions
* to automate the forward-port process.
**/

// CreateForwardPortStructure will create the ForwardPort struct with access to the necessary dependencies.
// It will also check if yq command is installed on the system.
func CreateForwardPortStructure(ctx context.Context, ld *lifecycle.Dependencies, assetsToPort map[string][]lifecycle.Asset, forkURL string) (*ForwardPort, error) {
	logger.Log(ctx, slog.LevelInfo, "preparing forward port data")

	// is yq installed?
	yqPath, err := whichYQCommand(ctx)
	if err != nil {
		return nil, err
	}

	_, ok := ld.Git.Remotes[forkURL]
	if !ok {
		errGitRemote := fmt.Errorf("Remote %s not found in git remotes, you need to configure your fork on your git remote references", forkURL)
		return nil, errGitRemote
	}

	isForkRemoteConfigured := git.CheckForValidForkRemote(chartsRepoURL, forkURL, chartsRepoName)
	if !isForkRemoteConfigured {
		errRemoteConfig := fmt.Errorf("Remote %s not configured correctly, you need to configure your fork on your git remote references", forkURL)
		return nil, errRemoteConfig
	}

	return &ForwardPort{
		yqPath:                  yqPath,
		git:                     ld.Git,
		VR:                      ld.VR,
		assetsToBeForwardPorted: assetsToPort,
		pullRequests:            make(map[string]PullRequest),
		forkRemoteURL:           forkURL,
	}, nil
}

// ExecuteForwardPort will execute all steps to organize and create the forward-port PRs
func (f *ForwardPort) ExecuteForwardPort(ctx context.Context, chart string) error {
	logger.Log(ctx, slog.LevelInfo, "starting forward port")

	// Get the forward port script commands
	commands, err := f.createForwardPortCommands(ctx, chart)
	if err != nil {
		return err
	}
	// Organize the commands into pull requests grouping by chart with it's dependencies
	f.organizePullRequestsByChart(commands)
	// Execute the forward port commands
	return f.executeForwardPorts(ctx)
}

// organizePullRequests will organize the commands into pull requests
func (f *ForwardPort) organizePullRequestsByChart(commands []Command) {
	lastChart := ""

	for _, command := range commands {

		changed := checkIfChartChanged(lastChart, command.Chart)

		// If the chart is the same as the last chart, append the command to the pull request
		if !changed {
			pr := f.pullRequests[lastChart]            // Extract the struct from the map
			pr.commands = append(pr.commands, command) // Modify the struct's field
			f.pullRequests[lastChart] = pr             // Put the modified struct back into the map
		}

		// If the chart is different from the last chart, organize a new pull request
		if changed {
			pr := PullRequest{
				branch:   fmt.Sprintf("auto-forward-port-%s-%s", command.Chart, f.VR.BranchVersion),
				commands: []Command{command},
			}
			f.pullRequests[command.Chart] = pr
			lastChart = command.Chart
		}
	}
}

// executeForwardPorts will execute the forward-port commands.
// It will create a new branch to forward-port the assets, clean the release.yaml file and commit.
// After each forward-port execution it will add and commit the changes to the git repository.
// It will push the branch to the remote repository and delete the local branch before moving,
// to the next Pull Request.
func (f *ForwardPort) executeForwardPorts(ctx context.Context) error {
	// save the original branch to change back after forward-port
	originalBranch := f.git.Branch

	// create log file
	fpLogs, err := lifecycle.CreateLogs(ctx, "forward-ported", "")
	if err != nil {
		return err
	}
	defer fpLogs.File.Close()

	// write log title and header
	fpLogs.WriteHEAD(ctx, f.VR, "Forward-Ported Assets")

	for asset, pr := range f.pullRequests {
		fpLogs.Write(ctx, asset, "INFO")

		// open and check if it is clean the git repo
		if err := f.createNewBranchToForwardPort(ctx, pr.branch); err != nil {
			return err
		}
		// clean release.yaml in the new branch
		if err := prepareReleaseYaml(ctx); err != nil {
			return err
		}
		// git add && commit cleaned release.yaml
		if err := f.git.AddAndCommit("cleaning release.yaml"); err != nil {
			return err
		}

		fpLogs.Write(ctx, "Branch: "+pr.branch, "INFO")

		for _, command := range pr.commands {
			// execute make forward-port
			if err := executeCommand(ctx, command.Command, f.yqPath); err != nil {
				return err
			}
			// git add && commit the changes
			msg := "forward-port " + command.Chart + " " + command.Version
			if err := f.git.AddAndCommit(msg); err != nil {
				return err
			}
			// Log this so later we can merge the PRs
			fpLogs.Write(ctx, msg, "")
		}
		// push branch
		if err := f.git.PushBranch(f.git.Remotes[f.forkRemoteURL], pr.branch); err != nil {
			return err
		}
		// save to log file branch
		fpLogs.Write(ctx, "PUSHED", "INFO")
		// Change back to the original branch to avoid conflicts
		if err := f.git.CheckoutBranch(originalBranch); err != nil {
			return err
		}
		// delete local created and pushed branch
		if err := f.git.DeleteBranch(pr.branch); err != nil {
			return err
		}
	}
	return nil
}
