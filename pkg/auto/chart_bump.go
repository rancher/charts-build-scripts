package auto

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// Bump TODO: Doc this
type Bump struct {
	configOptions     *options.ChartsScriptOptions
	targetChart       string
	releaseYaml       *Release
	versionRules      *lifecycle.VersionRules
	assetsVersionsMap map[string][]lifecycle.Asset
}

var (
	// Errors
	errNotDevBranch = errors.New("a development branch must be provided; (e.g., dev-v2.*)")
	errBadPackage   = errors.New("unexpected format for PACKAGE env variable")
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

	// Load and check the chart name from the target given package
	if err := bump.parseChartFromPackage(targetPackage); err != nil {
		return bump, err
	}

	//Initialize the lifecycle dependencies because of the versioning rules and the index.yaml mapping.
	dependencies, err := lifecycle.InitDependencies(filesystem.GetFilesystem(repoRoot), branch, bump.targetChart)
	if err != nil {
		err = fmt.Errorf("failure at SetupBump: %w ", err)
		return bump, err
	}

	bump.versionRules = dependencies.VR
	bump.assetsVersionsMap = dependencies.AssetsVersionsMap

	// Load object with target package information
	packages, err := charts.GetPackages(repoRoot, targetPackage)
	if err != nil {
		return nil, err
	}

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

// parseChartFromPackage extracts the chart name from the targetPackage
// targetPackage is in the format "<chart>/<some_number>/<chart>"
// (e.g., "rancher-istio/1.22/rancher-istio")
// or just <chart>
func (b *Bump) parseChartFromPackage(targetPackage string) error {
	parts := strings.Split(targetPackage, "/")
	if len(parts) == 1 {
		b.targetChart = parts[0]
		return nil
	} else if len(parts) > 1 && len(parts) <= 4 {
		b.targetChart = parts[len(parts)-1]
		return nil
	}
	return errBadPackage
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
