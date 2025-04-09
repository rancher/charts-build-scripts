package auto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/logger"
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

// BumpOutput defines the structure that will be written to bump.json
type BumpOutput struct {
	Charts     []string `json:"charts"`      // List of charts processed
	NewVersion string   `json:"new_version"` // The single version applied
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
func SetupBump(ctx context.Context, repoRoot, targetPackage, targetBranch string, chScriptOpts *options.ChartsScriptOptions) (*Bump, error) {
	bump := &Bump{
		configOptions: chScriptOpts,
	}

	// Check if the targetBranch has dev-v prefix and extract the branch line (i.e., 2.X)
	branch, err := parseBranchVersion(targetBranch)
	if err != nil {
		return nil, err
	}
	logger.Log(ctx, slog.LevelDebug, "", slog.String("branch-line", branch))

	// Load and check the chart name from the target given package
	if err := bump.parseChartFromPackage(targetPackage); err != nil {
		return bump, err
	}

	//Initialize the lifecycle dependencies because of the versioning rules and the index.yaml mapping.
	dependencies, err := lifecycle.InitDependencies(ctx, repoRoot, filesystem.GetFilesystem(repoRoot), branch, bump.targetChart)
	if err != nil {
		err = fmt.Errorf("failure at SetupBump: %w ", err)
		return bump, err
	}

	bump.versionRules = dependencies.VR
	bump.assetsVersionsMap = dependencies.AssetsVersionsMap

	// Load object with target package information
	packages, err := charts.GetPackages(ctx, repoRoot, targetPackage)
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
func (b *Bump) BumpChart(ctx context.Context, versionOverride string) error {
	// List the possible target charts
	targetCharts, err := chartsTargets(b.targetChart)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "error while getting target charts", slog.String("targetChart", b.targetChart))
		return err
	}
	logger.Log(ctx, slog.LevelInfo, "", slog.Any("targetCharts", targetCharts))

	// Open local git repository
	git, err := git.OpenGitRepo(ctx, ".")
	if err != nil {
		logger.Log(ctx, slog.LevelError, "error while opening git repository", logger.Err(err))
		return err
	}

	// make prepare
	if err := b.Pkg.Prepare(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while preparing package", logger.Err(err))
		return err
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if err := git.AddAndCommit("make prepare"); err != nil {
		logger.Log(ctx, slog.LevelError, "error while adding and committing after make prepare", logger.Err(err))
		return err
	}

	// Download logo at assets/logos (webhook and fleet are exceptions)
	if b.targetChart != "fleet" && b.targetChart != "rancher-webhook" {
		if err := b.Pkg.DownloadIcon(ctx); err != nil {
			logger.Log(ctx, slog.LevelError, "error while downloading icon", logger.Err(err))
			return err
		}
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := git.StatusProcelain(ctx); !clean {
		logger.Log(ctx, slog.LevelDebug, "git is not clean - icon downloaded")
		if err := git.AddAndCommit("make icon"); err != nil {
			logger.Log(ctx, slog.LevelError, "error while add/commit icon", logger.Err(err))
			return err
		}
	}

	// Calculate the next version to release
	if err := b.calculateNextVersion(ctx, versionOverride); err != nil {
		return err
	}

	// make patch - overwriting logo path
	if err := b.Pkg.GeneratePatch(ctx); err != nil {
		err = fmt.Errorf("error while patching package: %w", err)
		return err
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := git.StatusProcelain(ctx); !clean {
		if err := git.AddAndCommit("make patch"); err != nil {
			return err
		}
	}

	// make clean
	if err := b.Pkg.Clean(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while cleaning package", logger.Err(err))
		return err
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	// make charts - generate new assets and charts overwriting logo
	if err := b.Pkg.GenerateCharts(ctx, b.configOptions.OmitBuildMetadataOnExport); err != nil {
		logger.Log(ctx, slog.LevelError, "error while generating charts", logger.Err(err))
		return err
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := git.StatusProcelain(ctx); clean {
		logger.Log(ctx, slog.LevelError, "make charts did not generate any changes")
		return errors.New("make charts did not generate any changes")
	}

	if err := git.AddAndCommit("make chart"); err != nil {
		logger.Log(ctx, slog.LevelError, "error while adding and committing after make chart", logger.Err(err))
		return err
	}

	bumpVersion := b.Pkg.AutoGeneratedBumpVersion.String()
	newBranch := "auto-bump-" + b.targetChart + "-" + bumpVersion
	logger.Log(ctx, slog.LevelInfo, "", slog.String("newBranch", newBranch))

	if err := git.CreateAndCheckoutBranch(newBranch); err != nil {
		logger.Log(ctx, slog.LevelError, "error while creating and checking out new branch", logger.Err(err))
		return err
	}

	// TODO: Create option to skip removal of -RC (rancher-webhook for example)
	if strings.Contains(b.versions.latest.txt, "-rc") {
		logger.Log(ctx, slog.LevelInfo, "removing last -RC version", slog.String("latestVersion", b.versions.latest.txt))
		if err := b.makeRemove(targetCharts, git); err != nil {
			logger.Log(ctx, slog.LevelError, "error while removing -RC version", logger.Err(err))
			return err
		}
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	// modify the release.yaml
	if err := b.updateReleaseYaml(ctx, targetCharts); err != nil {
		logger.Log(ctx, slog.LevelError, "error while updating release.yaml", logger.Err(err))
		return err
	}

	if err := git.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := git.StatusProcelain(ctx); clean {
		logger.Log(ctx, slog.LevelError, "update release.yaml did not generate any changes")
		return errors.New("update release.yaml did not generate any changes")
	}

	if err := git.AddAndCommit("update release.yaml"); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "bump version", slog.String("bumpVersion", bumpVersion))
	return writeBumpJSON(targetCharts, bumpVersion)
}

func (b *Bump) updateReleaseYaml(ctx context.Context, targetCharts []string) error {
	logger.Log(ctx, slog.LevelInfo, "update release.yaml")

	for _, chart := range targetCharts {
		b.releaseYaml.Chart = chart
		if err := b.releaseYaml.UpdateReleaseYaml(); err != nil {
			return err
		}
	}
	return nil
}

func chartsTargets(targetChart string) ([]string, error) {

	switch targetChart {
	case "elemental":
		return []string{"elemental", "elemental-crd"}, nil

	case "fleet":
		return []string{"fleet", "fleet-crd", "fleet-agent"}, nil

	case "harvester-cloud-provider":
		return []string{"harvester-cloud-provider"}, nil

	case "harvester-csi-driver":
		return []string{"harvester-csi-driver"}, nil

	case "longhorn":
		return []string{"longhorn", "longhorn-crd"}, nil

	case "neuvector":
		return []string{"neuvector", "neuvector-crd", "neuvector-monitor"}, nil

	case "prometheus-federator":
		return []string{"prometheus-federator"}, nil

	case "rancher-aks-operator":
		return []string{"rancher-aks-operator", "rancher-aks-operator-crd"}, nil

	case "rancher-alerting-drivers":
		return []string{"rancher-alerting-drivers"}, nil

	case "rancher-backup":
		return []string{"rancher-backup", "rancher-backup-crd"}, nil

	case "rancher-cis-benchmark":
		return []string{"rancher-cis-benchmark", "rancher-cis-benchmark-crd"}, nil

	case "rancher-csp-adapter":
		return []string{"rancher-csp-adapter"}, nil

	case "rancher-eks-operator":
		return []string{"rancher-eks-operator", "rancher-eks-operator-crd"}, nil

	case "rancher-gatekeeper":
		return []string{"rancher-gatekeeper", "rancher-gatekeeper-crd"}, nil

	case "rancher-gke-operator":
		return []string{"rancher-gke-operator", "rancher-gke-operator-crd"}, nil

	case "rancher-istio":
		return []string{"rancher-istio"}, nil

	case "rancher-logging":
		return []string{"rancher-logging", "rancher-logging-crd"}, nil

	case "rancher-monitoring":
		return []string{"rancher-monitoring", "rancher-monitoring-crd"}, nil

	case "rancher-project-monitoring":
		return []string{"rancher-project-monitoring"}, nil

	case "rancher-provisioning-capi":
		return []string{"rancher-provisioning-capi"}, nil

	case "rancher-pushprox":
		return []string{"rancher-pushprox"}, nil

	case "rancher-vsphere-csi":
		return []string{"rancher-vsphere-csi"}, nil

	case "rancher-vsphere-cpi":
		return []string{"rancher-vsphere-cpi"}, nil

	case "rancher-webhook":
		return []string{"rancher-webhook"}, nil

	case "rancher-windows-gmsa":
		return []string{"rancher-windows-gmsa", "rancher-windows-gmsa-crd"}, nil

	case "rancher-wins-upgrader":
		return []string{"rancher-wins-upgrader"}, nil

	case "sriov":
		return []string{"sriov", "sriov-crd"}, nil

	case "system-upgrade-controller":
		return []string{"system-upgrade-controller"}, nil

	case "ui-plugin-operator":
		return []string{"ui-plugin-operator", "ui-plugin-operator-crd"}, nil
	}

	return nil, fmt.Errorf("chart %s not listed", targetChart)
}

func (b *Bump) makeRemove(targetCharts []string, g *git.Git) error {
	version := b.versions.latestRepoPrefix.txt + "+up" + b.versions.latest.txt

	for _, chart := range targetCharts {
		cmd := exec.Command("make", "remove", fmt.Sprintf("CHART=%s", chart), fmt.Sprintf("VERSION=%s", version))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to execute remove command for chart %s: %w", chart, err)
		}
		if err := g.AddAndCommit(fmt.Sprintf("remove RC of: %s", chart)); err != nil {
			return err
		}
	}
	return nil
}

// writeBumpJSON will write the bump.json file with the new version auto bumped
func writeBumpJSON(targetCharts []string, bumpVersion string) error {

	dataToWrite := BumpOutput{
		Charts:     targetCharts,
		NewVersion: bumpVersion,
	}

	jsonData, err := json.MarshalIndent(dataToWrite, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path.BumpVersionFile, jsonData, 0644); err != nil {
		return err
	}

	return nil
}
