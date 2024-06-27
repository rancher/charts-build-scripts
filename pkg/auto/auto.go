package auto

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
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

// createForwardPortCommands will create the forward port script commands for each asset and version,
// and return a sorted slice of commands
func (fp *ForwardPort) createForwardPortCommands(chart string) ([]Command, error) {

	commands := make([]Command, 0)
	for asset, versions := range fp.assetsToBeForwardPorted {
		if chart != "" && !strings.HasPrefix(asset, chart) {
			continue
		}
		for _, version := range versions {
			command, err := fp.writeMakeCommand(asset, version.Version)
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
func (fp *ForwardPort) writeMakeCommand(asset, version string) (Command, error) {
	/**
	* make forward-port
	* CHART=rancher-provisioning-capi
	* VERSION=100.0.0+up0.0.1
	* BRANCH=dev-v2.9
	* UPSTREAM=upstream
	 */

	upstreamRemote, ok := fp.git.Remotes["https://github.com/rancher/charts"]
	if !ok {
		logrus.Error("upstream remote not found; you need to have the upstream remote configured in your git repository (https://github.com/rancher/charts)")

		return Command{}, fmt.Errorf("upstream remote not found; you need to have the upstream remote configured in your git repository (https://github.com/rancher/charts)")
	}
	commands := []string{
		"make",
		"forward-port",
		"CHART=" + asset,
		"VERSION=" + version,
		"BRANCH=" + fp.VR.DevBranch,
		"UPSTREAM=" + upstreamRemote,
	}

	return Command{Chart: asset, Command: commands, Version: version}, nil
}
