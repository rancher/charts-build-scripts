package auto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

// ChartTargetsMap represents all current active charts
var ChartTargetsMap = map[string][]string{
	"elemental":                  {"elemental", "elemental-crd"},
	"fleet":                      {"fleet", "fleet-crd", "fleet-agent"},
	"harvester-cloud-provider":   {"harvester-cloud-provider"},
	"harvester-csi-driver":       {"harvester-csi-driver"},
	"longhorn":                   {"longhorn", "longhorn-crd"},
	"neuvector":                  {"neuvector", "neuvector-crd", "neuvector-monitor"},
	"prometheus-federator":       {"prometheus-federator"},
	"rancher-aks-operator":       {"rancher-aks-operator", "rancher-aks-operator-crd"},
	"rancher-alerting-drivers":   {"rancher-alerting-drivers"},
	"rancher-backup":             {"rancher-backup", "rancher-backup-crd"},
	"rancher-cis-benchmark":      {"rancher-cis-benchmark", "rancher-cis-benchmark-crd"},
	"rancher-compliance":         {"rancher-compliance", "rancher-compliance-crd"},
	"rancher-csp-adapter":        {"rancher-csp-adapter"},
	"rancher-eks-operator":       {"rancher-eks-operator", "rancher-eks-operator-crd"},
	"rancher-gatekeeper":         {"rancher-gatekeeper", "rancher-gatekeeper-crd"},
	"rancher-gke-operator":       {"rancher-gke-operator", "rancher-gke-operator-crd"},
	"rancher-istio":              {"rancher-istio"},
	"rancher-logging":            {"rancher-logging", "rancher-logging-crd"},
	"rancher-monitoring":         {"rancher-monitoring", "rancher-monitoring-crd"},
	"rancher-project-monitoring": {"rancher-project-monitoring"},
	"rancher-provisioning-capi":  {"rancher-provisioning-capi"},
	"rancher-pushprox":           {"rancher-pushprox"},
	"rancher-vsphere-csi":        {"rancher-vsphere-csi"},
	"rancher-vsphere-cpi":        {"rancher-vsphere-cpi"},
	"rancher-webhook":            {"rancher-webhook"},
	"rancher-windows-gmsa":       {"rancher-windows-gmsa", "rancher-windows-gmsa-crd"},
	"rancher-wins-upgrader":      {"rancher-wins-upgrader"},
	"sriov":                      {"sriov", "sriov-crd"},
	"system-upgrade-controller":  {"system-upgrade-controller"},
	"ui-plugin-operator":         {"ui-plugin-operator", "ui-plugin-operator-crd"},
}

// Bump represents the chart bump process for a single chart
// (with its CRD and dependencies).
type Bump struct {
	// options provided to the charts scripts for this branch
	configOptions *options.ChartsScriptOptions
	// target chart, CRD and any additional chart
	target target
	// represents package/<target_chart> directory loaded options
	Pkg *charts.Package
	// versions to be calculated
	versions *versions
	// release.yaml file information
	releaseYaml *Release
	// version rules at the current branch which will be applied to versions field
	versionRules *lifecycle.VersionRules
	// all assets versions present
	assetsVersionsMap map[string][]lifecycle.Asset
	// git and filesystem
	repo   *git.Git
	rootFs billy.Filesystem
}

// target chart, CRD and additional chart.
// e.g., [main: fleet; additional:{fleet;fleet-crd;fleet-agent}]
type target struct {
	main       string
	additional []string
}

// BumpOutput defines the structure that will be written to config/bump.json
type BumpOutput struct {
	Charts     []string `json:"charts"`      // List of charts processed
	NewVersion string   `json:"new_version"` // The single version applied
}

var (
	errNotDevBranch                 = errors.New("a development branch must be provided; (e.g., dev-v2.*)")
	errBadPackage                   = errors.New("unexpected format for PACKAGE env variable")
	errChartNotListed               = errors.New("chart not listed")
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
* 	- SetupBump and it's functions/methods, which won't generate any file changes at charts/ local repo.
		It only, loads information about the chart to be bumped
* 	- BumpChart and it's functions/methods, which will execute the bump,
		generate file changes, stage and commit them.
*
*/

// SetupBump will load and parse all related information to the chart that should be bumped.
func SetupBump(ctx context.Context, repoRoot, targetPackage, targetBranch string, chScriptOpts *options.ChartsScriptOptions) (*Bump, error) {
	logger.Log(ctx, slog.LevelInfo, "setup auto-chart-bump")

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
	dependencies, err := lifecycle.InitDependencies(ctx, repoRoot, filesystem.GetFilesystem(repoRoot), branch, bump.target.main)
	if err != nil {
		err = fmt.Errorf("failure at SetupBump: %w ", err)
		return bump, err
	}

	bump.versionRules = dependencies.VR
	bump.assetsVersionsMap = dependencies.AssetsVersionsMap
	bump.repo = dependencies.Git
	bump.rootFs = dependencies.RootFs

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
		return bump, errReleaseYaml
	}

	bump.releaseYaml = &Release{
		Chart:           bump.target.main,
		ReleaseYamlPath: releaseYamlPath,
	}

	// Check and parse upstream chart options
	upstreamSubDir := ""
	if bump.Pkg.Upstream.GetOptions().Subdirectory != nil {
		upstreamSubDir = *bump.Pkg.Upstream.GetOptions().Subdirectory
	}

	upstreamCommit := ""
	if bump.Pkg.Upstream.GetOptions().Commit != nil {
		upstreamCommit = *bump.Pkg.Upstream.GetOptions().Commit
	}

	upstreamChartBranch := ""
	if bump.Pkg.Upstream.GetOptions().ChartRepoBranch != nil {
		upstreamChartBranch = *bump.Pkg.Upstream.GetOptions().ChartRepoBranch
	}

	logger.Log(ctx, slog.LevelInfo, "setup", slog.Group("bump",
		slog.String("targetChart", bump.target.main),
		slog.Group("Pkg",
			slog.Group("Chart",
				slog.Group("Upstream",
					slog.Any("URL", bump.Pkg.Upstream.GetOptions().URL),
					slog.Any("SubDir", upstreamSubDir),
					slog.Any("Commit", upstreamCommit),
					slog.Any("ChartRepoBranch", upstreamChartBranch),
				),
				slog.String("workingDir", bump.Pkg.WorkingDir),
			),
			slog.Any("Version", bump.Pkg.Version),
			slog.Any("Package Version", bump.Pkg.PackageVersion),
			slog.Bool("DoNotRelease", bump.Pkg.DoNotRelease),
			slog.Bool("Auto", bump.Pkg.Auto),
		),
		slog.String("last version", bump.assetsVersionsMap[bump.target.main][0].Version),
	))

	return bump, nil
}

// parseBranchVersion trims the prefix and returns the branch line
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

	switch {
	case len(parts) == 1:
		b.target.main = parts[0]
	case len(parts) > 1 && len(parts) <= 4:
		b.target.main = parts[len(parts)-1]
	default:
		return errBadPackage
	}

	if _, exist := ChartTargetsMap[b.target.main]; !exist {
		return errChartNotListed
	}

	b.target.additional = ChartTargetsMap[b.target.main]
	return nil
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

// BumpChart will execute a similar approach as the defined development workflow for chartowners.
// The main difference is that between the steps: (make prepare and make patch) we will calculate the next version to release.
func (b *Bump) BumpChart(ctx context.Context, versionOverride string, multiRCs bool) error {
	logger.Log(ctx, slog.LevelInfo, "start auto-chart-bump")

	if err := b.prepare(ctx); err != nil {
		return err
	}

	// Calculate the next version to release
	if err := b.calculateNextVersion(ctx, versionOverride); err != nil {
		return err
	}

	if err := b.icon(ctx); err != nil {
		return err
	}

	if err := b.patch(ctx); err != nil {
		return err
	}

	if err := b.clean(ctx); err != nil {
		return err
	}

	if err := b.charts(ctx); err != nil {
		return err
	}

	// check if should remove previous RCs versions
	if !multiRCs {
		logger.Log(ctx, slog.LevelWarn, "removing existing RC's")
		if err := b.checkMultiRC(ctx); err != nil {
			return err
		}
	}

	if err := b.updateReleaseYaml(ctx, b.target.additional, multiRCs); err != nil {
		logger.Log(ctx, slog.LevelError, "error while updating release.yaml", logger.Err(err))
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "bump version",
		slog.String("bumpVersion", b.Pkg.AutoGeneratedBumpVersion.String()))

	return b.writeBumpJSON(ctx, b.target.additional, b.Pkg.AutoGeneratedBumpVersion.String())
}

// prepare = && git status && git add . && git commit -m "make prepare"
func (b *Bump) prepare(ctx context.Context) error {
	if err := b.Pkg.Prepare(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while preparing package", logger.Err(err))
		return err
	}

	if err := b.repo.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	// check if the version to bump does not already exists
	alreadyExist, err := checkBumpAppVersion(ctx, b.Pkg.UpstreamChartVersion, b.assetsVersionsMap[b.target.main])
	if err != nil {
		return err
	}
	if alreadyExist {
		b.repo.FullReset() // quitting the job regardless if this works or not
		return errors.New("version to bump already exists: " + *b.Pkg.UpstreamChartVersion)
	}

	if err := b.repo.AddAndCommit("make prepare"); err != nil {
		logger.Log(ctx, slog.LevelError, "error while adding and committing after make prepare", logger.Err(err))
		return err
	}

	return nil
}

// checkBumpAppVersion checks if the bumpAppVersion already exists in the repository
func checkBumpAppVersion(ctx context.Context, bumpAppVersion *string, versions []lifecycle.Asset) (bool, error) {
	if bumpAppVersion == nil {
		logger.Log(ctx, slog.LevelError, "upstreamVersion is nil for chart, abnormal behavior")
		return false, errors.New("upstreamVersion is nil for chart, abnormal behavior")
	}

	for _, version := range versions {
		parts := strings.Split(version.Version, "+up")
		if len(parts) != 2 {
			continue
		}
		if parts[1] == *bumpAppVersion {
			return true, nil
		}
	}

	return false, nil
}

// icon = make icon && git status && git add . && git commit -m "make icon"
func (b *Bump) icon(ctx context.Context) error {
	// Download logo at assets/logos
	if !isIconException(b.target.main) {
		if err := b.Pkg.DownloadIcon(ctx); err != nil {
			logger.Log(ctx, slog.LevelError, "error while downloading icon", logger.Err(err))
			return err
		}
	}

	if err := b.repo.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := b.repo.StatusProcelain(ctx); !clean {
		logger.Log(ctx, slog.LevelDebug, "git is not clean - icon downloaded")
		if err := b.repo.AddAndCommit("make icon"); err != nil {
			logger.Log(ctx, slog.LevelError, "error while git add && commit icon", logger.Err(err))
			return err
		}
	}

	return nil
}

// patch = make patch && git status && git add . && git commit -m "make patch"
func (b *Bump) patch(ctx context.Context) error {
	// overwriting logo path here also
	if err := b.Pkg.GeneratePatch(ctx); err != nil {
		err = fmt.Errorf("error while patching package: %w", err)
		return err
	}

	if err := b.repo.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status after patch", logger.Err(err))
		return err
	}

	if clean, _ := b.repo.StatusProcelain(ctx); !clean {
		if err := b.repo.AddAndCommit("make patch"); err != nil {
			logger.Log(ctx, slog.LevelError, "error while git add && commit after patch", logger.Err(err))
			return err
		}
	}

	return nil
}

// clean = make clean && git status
func (b *Bump) clean(ctx context.Context) error {
	if err := b.Pkg.Clean(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while cleaning package", logger.Err(err))
		return err
	}

	if err := b.repo.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status after make clean", logger.Err(err))
		return err
	}

	return nil
}

// charts = make charts && git status && git add . && git commit -m "make charts"
func (b *Bump) charts(ctx context.Context) error {
	//  generate new assets and charts overwriting logo
	if err := b.Pkg.GenerateCharts(ctx, b.configOptions.OmitBuildMetadataOnExport); err != nil {
		logger.Log(ctx, slog.LevelError, "error while generating charts", logger.Err(err))
		return err
	}

	if err := b.repo.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := b.repo.StatusProcelain(ctx); clean {
		logger.Log(ctx, slog.LevelError, "make charts did not generate any changes")
		return errors.New("make charts did not generate any changes")
	}

	if err := b.repo.AddAndCommit("make chart"); err != nil {
		logger.Log(ctx, slog.LevelError, "error while adding and committing after make chart", logger.Err(err))
		return err
	}

	return nil
}

// checkMultiRC will remove the current RC versions if chart does not support the feature.
func (b *Bump) checkMultiRC(ctx context.Context) error {
	if len(b.versions.currentRCs) > 0 {

		for _, rcVersion := range b.versions.currentRCs {
			removeMe := rcVersion.repoPrefix.txt + "+up" + rcVersion.appVersion.txt

			logger.Log(ctx, slog.LevelDebug, "removing RC version", slog.Group("charts", b.target.main, removeMe))
			if err := removeCharts(ctx, b.rootFs, b.target.additional, removeMe); err != nil {
				return err
			}

			// Check for changes
			if clean, _ := b.repo.StatusProcelain(ctx); clean {
				logger.Log(ctx, slog.LevelError, "should have removed chart", slog.String("chart", b.target.main))
				return errors.New("remove RC chart version failed")
			}
			// Add && Commit
			commit := "remove " + b.target.main + " " + removeMe
			if err := b.repo.AddAndCommit(commit); err != nil {
				return err
			}
		}

	}

	return nil
}

// updateReleaseYaml will add the bumped versions to the release.yaml
func (b *Bump) updateReleaseYaml(ctx context.Context, targetCharts []string, multiRC bool) error {
	logger.Log(ctx, slog.LevelInfo, "update release.yaml")

	for _, chart := range targetCharts {
		b.releaseYaml.Chart = chart
		if err := b.releaseYaml.UpdateReleaseYaml(ctx, !multiRC); err != nil {
			return err
		}
	}

	if err := b.repo.Status(ctx); err != nil {
		logger.Log(ctx, slog.LevelError, "error while checking git status", logger.Err(err))
		return err
	}

	if clean, _ := b.repo.StatusProcelain(ctx); clean {
		logger.Log(ctx, slog.LevelError, "update release.yaml did not generate any changes")
		return errors.New("update release.yaml did not generate any changes")
	}

	if err := b.repo.AddAndCommit("update release.yaml"); err != nil {
		return err
	}

	return nil
}

// writeBumpJSON will write the bump.json file with the new version auto bumped
func (b *Bump) writeBumpJSON(ctx context.Context, targetCharts []string, bumpVersion string) error {

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

	if clean, _ := b.repo.StatusProcelain(ctx); clean {
		logger.Log(ctx, slog.LevelError, "failed to write bump version", slog.String("version", bumpVersion))
		return errors.New("failed to write bump version")
	}

	return nil
}
