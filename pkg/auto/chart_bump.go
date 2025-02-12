package auto

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

// Bump TODO: Doc this
type Bump struct {
	configOptions     *options.ChartsScriptOptions
	targetChart       string
	Pkg               *charts.Package
	versions          *versions
	releaseYaml       *Release
	versionRules      *lifecycle.VersionRules
	assetsVersionsMap map[string][]lifecycle.Asset
}

var (
	// Errors
	errNotDevBranch                 = errors.New("a development branch must be provided; (e.g., dev-v2.*)")
	errBadPackage                   = errors.New("unexpected format for PACKAGE env variable")
	errNoPackage                    = errors.New("no package provided")
	errMultiplePackages             = errors.New("multiple packages provided; this is not supported")
	errFalseAuto                    = errors.New("package.yaml must be configured for auto-chart-bump")
	errPackageName                  = errors.New("package name not loaded")
	errPackageChartVersion          = errors.New("package chart version loaded but it should be dinamycally created")
	errPackageVersion               = errors.New("package version loaded but it should be dinamycally created")
	errPackegeDoNotRelease          = errors.New("package is marked as doNotRelease")
	errChartWorkDir                 = errors.New("chart working directory not loaded")
	errChartURL                     = errors.New("chart upstream url field must be a git repository (.git suffix)")
	errChartRepoCommit              = errors.New("chart upstream commit field should not be provided")
	errChartRepoBranch              = errors.New("chart upstream branch field must be provided")
	errChartSubDir                  = errors.New("chart upstream subdirectory field must be provided")
	errAdditionalChartWorkDir       = errors.New("additional chart template directory not loaded")
	errCRDWorkDir                   = errors.New("additional chart CRDs directory not loaded")
	errAdditionalChartCRDValidation = errors.New("additionalCharts.crdOptions.addCRDValidationToMainChart must be true")
	errChartLatestVersion           = errors.New("latest version not found for chart")
	errChartUpstreamVersion         = errors.New("upstream version not found for chart")
	errChartUpstreamVersionWrong    = errors.New("upstream version should not have the repo prefix version already")
	errBumpVersion                  = errors.New("version to bump is not greater than the latest version")
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

	// Check if package.yaml has all the necessary fields for an auto chart bump
	if err := bump.parsePackageYaml(packages); err != nil {
		return bump, err
	}

	//  Load the chart and release.yaml paths
	releaseYamlPath := filesystem.GetAbsPath(dependencies.RootFs, path.RepositoryReleaseYaml)
	if releaseYamlPath == "" {
		return bump, err
	}

	bump.releaseYaml = &Release{
		Chart:           bump.targetChart,
		ReleaseYamlPath: releaseYamlPath,
	}

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

// parsePackageYaml will assign the package.yaml information to the Bump struct
// and check if the package.yaml has all the necessary fields for an auto chart bump
func (b *Bump) parsePackageYaml(packages []*charts.Package) error {
	if len(packages) == 0 {
		return errNoPackage
	} else if len(packages) > 1 {
		return errMultiplePackages
	}

	b.Pkg = packages[0]

	// package root level fields check
	switch {
	case b.Pkg.Auto == false:
		return errFalseAuto
	case b.Pkg.Name == "":
		return errPackageName
	case b.Pkg.Version != nil:
		return errPackageChartVersion
	case b.Pkg.PackageVersion != nil:
		return errPackageVersion
	case b.Pkg.DoNotRelease == true:
		return errPackegeDoNotRelease
	case b.Pkg.Chart.WorkingDir == "":
		return errChartWorkDir
	}

	// Package Upstream fields check
	upstreamOpts := b.Pkg.Chart.Upstream.GetOptions()
	if err := checkUpstreamOptions(&upstreamOpts); err != nil {
		return err
	}

	// Check Chart and Upstream options for any additional Charts like CRDs
	for _, additionalChart := range b.Pkg.AdditionalCharts {
		additionalUpstream := *additionalChart.Upstream
		additionalUpstremOpts := additionalUpstream.GetOptions()
		if err := checkUpstreamOptions(&additionalUpstremOpts); err != nil {
			return err
		}
		if additionalChart.CRDChartOptions != nil {
			switch {
			case additionalChart.CRDChartOptions.TemplateDirectory == "":
				return errAdditionalChartWorkDir
			case additionalChart.CRDChartOptions.CRDDirectory == "":
				return errCRDWorkDir
			}
		}
	}

	return nil
}

// checkUpstreamOptions checks if the UpstreamOptions fields are properly loaded
func checkUpstreamOptions(options *options.UpstreamOptions) error {
	switch {
	case !strings.HasSuffix(options.URL, ".git"):
		return errChartURL
	case options.Commit != nil:
		return errChartRepoCommit
	case options.ChartRepoBranch == nil:
		return errChartRepoBranch
	case options.Subdirectory == nil:
		return errChartSubDir
	}

	return nil
}

// -----------------------------------------------------------

// BumpChart will execute a similar approach as the defined development workflow for chartowners.
// The main difference is that between the steps: (make prepare and make patch) we will calculate the next version to release.
func (b *Bump) BumpChart() error {

	targetCharts, err := chartsTargets(b.targetChart)
	if err != nil {
		return err
	}

	git, err := git.OpenGitRepo(".")
	if err != nil {
		return err
	}

	// make prepare
	if err := b.Pkg.Prepare(); err != nil {
		err = fmt.Errorf("error while preparing package: %w", err)
		return err
	}

	if err := git.AddAndCommit("make prepare"); err != nil {
		return err
	}

	// Download logo at assets/logos (webhook and fleet are exceptions)
	if b.targetChart != "fleet" && b.targetChart != "rancher-webhook" {
		if err := b.Pkg.DownloadIcon(); err != nil {
			err = fmt.Errorf("error while downloading icon: %w", err)
			return err
		}
	}

	if clean, _ := git.StatusProcelain(); !clean {
		if err := git.AddAndCommit("make icon"); err != nil {
			return err
		}
	}

	// Calculate the next version to release
	if err := b.calculateNextVersion(); err != nil {
		return err
	}

	// make patch - overwriting logo path
	if err := b.Pkg.GeneratePatch(); err != nil {
		err = fmt.Errorf("error while patching package: %w", err)
		return err
	}

	if clean, _ := git.StatusProcelain(); !clean {
		if err := git.AddAndCommit("make patch"); err != nil {
			return err
		}
	}

	// make clean
	if err := b.Pkg.Clean(); err != nil {
		err = fmt.Errorf("error while cleaning package: %w", err)
		return err
	}

	// make charts - generate new assets and charts overwriting logo
	if err := b.Pkg.GenerateCharts(b.configOptions.OmitBuildMetadataOnExport); err != nil {
		err = fmt.Errorf("error while generating chart: %w", err)
		return err
	}

	// modify the release.yaml
	if err := b.updateReleaseYaml(); err != nil {
		err = fmt.Errorf("error while updating release.yaml: %w", err)
		return err
	}

	// This will be used by the GHA job to update the auto generated Pull Request
	fmt.Printf("\n%s", b.Pkg.AutoGeneratedBumpVersion.String())

	return nil
}

func (b *Bump) updateReleaseYaml() error {
	// upadte chart release yaml
	if err := b.releaseYaml.UpdateReleaseYaml(); err != nil {
		return err
	}

	// Check if there is a crd and update it.
	if len(b.Pkg.AdditionalCharts) > 0 {
		if b.Pkg.AdditionalCharts[0].CRDChartOptions != nil {
			b.releaseYaml.Chart = b.releaseYaml.Chart + "-crd"
			return b.releaseYaml.UpdateReleaseYaml()
		}
	}

	return nil
}
