package auto

import (
	"errors"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// Bump TODO: Doc this
type Bump struct {
	configOptions     *options.ChartsScriptOptions
	releaseYaml       *Release
	versionRules      *lifecycle.VersionRules
	assetsVersionsMap map[string][]lifecycle.Asset
}

var (
	// Errors
	errNotDevBranch = errors.New("a development branch must be provided; (e.g., dev-v2.*)")
)

/*******************************************************
*
* This file can be understood in 2 sections:
* 	- SetupBump and it's functions/methods
* 	- BumpChart and it's functions/methods
*
 */

// SetupBump TODO: add description
func SetupBump(repoRoot, targetPackage, targetBranch string, chScriptOpts *options.ChartsScriptOptions) (*Bump, error) {
	bump := &Bump{
		configOptions: chScriptOpts,
	}

	// Check if the targetBranch has dev-v prefix and extract the branch line (i.e., 2.X)
	branch, err := parseBranchVersion(targetBranch)
	if err != nil {
		return nil, err
	}

	// TODO: Load and check the chart name from the target given package
	// here

	// TODO: We initialize the lifecycle dependencies because of the versioning rules and the index.yaml mapping.
	//

	// TODO: Load object with target package information
	//

	// TODO: Check if package.yaml has all the necessary fields for an auto chart bump
	//

	// TODO: Load the chart and release.yaml paths
	//

	return bump, nil
}

func parseBranchVersion(targetBranch string) (string, error) {
	if !strings.HasPrefix(targetBranch, "dev-v") {
		return "", errNotDevBranch
	}
	return strings.TrimPrefix(targetBranch, "dev-v"), nil
}

// -----------------------------------------------------------

// BumpChart TODO: description
func (b *Bump) BumpChart() error {
	// TODO: make prepare

	// TODO: Calculate the next version to release

	// TODO: make patch

	// TODO: make clean

	// TODO: make charts

	// TODO: modify the release.yaml

	return nil
}
