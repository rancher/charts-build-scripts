package auto

import (
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
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
