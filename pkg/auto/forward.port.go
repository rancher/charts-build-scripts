package auto

import (
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
