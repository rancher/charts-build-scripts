package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/logger"

	"github.com/rancher/charts-build-scripts/pkg/auto"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/registries"
	"github.com/rancher/charts-build-scripts/pkg/validate"
	"github.com/urfave/cli"
)

// Configuration global variables
var (
	// Ctx is the root context used to store standard configuration of cli-app
	Ctx context.Context
	// RepoRoot represents the root path of the repository
	RepoRoot string
	// Version represents the current version of the chart build scripts
	Version = "v0.0.0-dev"
	// GitCommit represents the latest commit when building this script
	GitCommit = "HEAD"
	// ChartsScriptOptionsFile represents a file that contains options for the charts script to use for this branch
	ChartsScriptOptionsFile string
)

// Chart owners global variables
var (
	// CurrentPackage represents the specific package to apply the scripts to
	CurrentPackage string
	// SoftErrorMode indicates if certain non-fatal errors will be turned into warnings
	SoftErrorMode = false
	// CurrentChart represents a specific chart to apply the scripts to. Also accepts a specific version.
	CurrentChart string
	// CurrentAsset represents a specific asset to apply the scripts to. Also accepts a specific archive.
	CurrentAsset string
	// PorcelainMode indicates that the output of the scripts should be in an easy-to-parse format for scripts
	PorcelainMode bool
	// CacheMode indicates that caching should be used on all remotely pulled resources
	CacheMode = false
)

// Release Team global variables
var (
	// ForkURL represents the fork URL configured as a remote in your local git repository
	ForkURL = ""
	// ChartVersion of the chart to release
	ChartVersion = ""
	// MultiRC indicates if the auto-bump should not remove previous RC versions
	MultiRC bool
	// OverrideVersion is the type of version override (patch, minor, auto)
	OverrideVersion string
	// NewChart boolean option for creating a net-new chart with auto-bump
	NewChart bool
	// IsPrimeChart boolean option
	IsPrimeChart bool
)

// Registries Global Variables
var (
	// GithubToken represents the Github Auth token
	GithubToken string
	// OciDNS represents the DNS of the OCI Registry
	OciDNS string
	// CustomOCIPAth represents a custom override for the OCI Registry
	CustomOCIPAth string
	// OciUser represents the user of the OCI Registry
	OciUser string
	// OciPassword represents the password of the OCI Registry
	OciPassword string
	// PrimeURL of SUSE Prime registry
	PrimeURL string
	// DebugMode indicates debug mode
	DebugMode bool
)

// Branch global variables
var (
	// BranchVersion is the branch line we are in (e.g., 2.12 for dev-v2.12 or release-v2.12)
	BranchVersion = ""
	// Branch repsents the branch to compare against
	Branch = ""
)

// Validation global variables
var (
	// LocalMode indicates that only local validation should be run
	LocalMode bool
	// RemoteMode indicates that only remote validation should be run
	RemoteMode bool
	// PullRequest represents the Pull Request identifying number
	PullRequest = ""
	// Skip indicates whether to skip execution
	Skip = false
)

// Environment variables
const (
	// Configuration Environment Variables
	defaultLogLevelEnvironmentVariable  = "LOG"                 // indicates the log level
	defaultChartsScriptOptionsFile      = config.PathConfigYaml // default path to look a file containing options for the charts scripts to use for this branch
	defaultPorcelainEnvironmentVariable = "PORCELAIN"           // indicates if git commands should run on porcelain mode
	defaultCacheEnvironmentVariable     = "USE_CACHE"           // indicates that a cache should be used on pulls to remotes

	// Chart Owners Environment Variables
	defaultPackageEnvironmentVariable = "PACKAGE"     // for picking a specific package
	softErrorsEnvironmentVariable     = "SOFT_ERRORS" // indicates if soft error mode is enabled
	defaultChartEnvironmentVariable   = "CHART"       // for picking a specific chart
	defaultAssetEnvironmentVariable   = "ASSET"       // for picking a specific asset

	// Release Team Environment variables
	defaultForkEnvironmentVariable            = "FORK"
	defaultChartVersionEnvironmentVariable    = "CHART_VERSION"    // indicates the version to release
	defaultMultiRCEnvironmentVariable         = "MULTI_RC"         // indicates if the auto-bump should not remove previous RC versions
	defaultOverrideVersionEnvironmentVariable = "OVERRIDE_VERSION" // indicates the version to override
	defaultNewChartVariable                   = "NEW_CHART"        // Net new chart option for auto-bump
	defaultIsPrimeChartVariable               = "IS_PRIME"         // indicates prime charts

	// Registries Environment Variables
	defaultGHTokenEnvironmentVariable  = "GH_TOKEN"
	defaultOciDNS                      = "OCI_DNS"
	defaultCustomOCIPAth               = "CUSTOM_OCI_PATH"
	defaultOciUser                     = "OCI_USER"
	defaultOciPassword                 = "OCI_PASS"
	defaultPrimeURLEnvironmentVariable = "PRIME_URL"

	// Branch environment variables
	defaultBranchEnvironmentVariable        = "BRANCH"         // indicates the branch
	defaultBranchVersionEnvironmentVariable = "BRANCH_VERSION" // indicates the branch version line to compare against

	// Validation environment variables
	defaultPRNumberEnvironmentVariable = "PR_NUMBER"
	defaultSkipEnvironmentVariable     = "SKIP" // indicates whether to skip execution
)

func init() {
	ctx := context.Background()

	tintOptions := &tint.Options{
		AddSource:  true,
		TimeFormat: "15:04:05",
	}

	// Set the log level based on the LOG environment variable
	lvl := os.Getenv(defaultLogLevelEnvironmentVariable)
	switch lvl {
	case "DEBUG":
		tintOptions.Level = slog.LevelDebug
	case "INFO":
		tintOptions.Level = slog.LevelInfo
	case "WARN":
		tintOptions.Level = slog.LevelWarn
	case "ERROR":
		tintOptions.Level = slog.LevelError
	default:
		tintOptions.Level = slog.LevelInfo
	}

	// Create a new slog logger with tint handler
	newLogger := slog.New(tint.NewHandler(os.Stderr, tintOptions))
	slog.SetDefault(newLogger)
	logger.Log(ctx, slog.LevelInfo, "charts-build-scripts",
		slog.String(defaultLogLevelEnvironmentVariable, os.Getenv(defaultLogLevelEnvironmentVariable)))

	// Initialize standard config for cli app
	repoRoot, err := os.Getwd()
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("unable to get current working directory: %w", err).Error())
	}
	if repoRoot == "" {
		logger.Fatal(ctx, "unable to get current working directory")
	}

	RepoRoot = os.Getenv("DEV_REPO_ROOT")
	if RepoRoot != "" {
		logger.Log(ctx, slog.LevelDebug, "using customized repo root: ", slog.String("repoRoot", RepoRoot))
	} else {
		RepoRoot = repoRoot
		logger.Log(ctx, slog.LevelDebug, "using default repo root: ", slog.String("repoRoot", RepoRoot))
	}

	logger.Log(ctx, slog.LevelDebug, "using current working directory as repo root: ", slog.String("repoRoot", repoRoot))

	// initialize global context configuration
	cfg, err := config.Init(ctx, RepoRoot, filesystem.GetFilesystem(RepoRoot))
	if err != nil {
		logger.Fatal(ctx, err.Error())
	}

	Ctx = config.WithConfig(ctx, cfg)
}

func main() {
	app := cli.NewApp()
	app.Name = "charts-build-scripts"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Build scripts used to maintain patches on Helm charts forked from other repositories"

	// Configuration Flags
	debugFlag := cli.BoolFlag{
		Name:        "debug,d",
		Usage:       "Debug mode",
		Required:    false,
		Destination: &DebugMode,
	}
	configFlag := cli.StringFlag{
		Name:        "config",
		Usage:       "A configuration file with additional options for allowing this branch to interact with other branches",
		TakesFile:   true,
		Destination: &ChartsScriptOptionsFile,
		Value:       defaultChartsScriptOptionsFile,
	}
	porcelainFlag := cli.BoolFlag{
		Name:        "porcelain",
		Usage:       "Print the output of the command in a easy-to-parse format for scripts",
		Required:    false,
		Destination: &PorcelainMode,
		EnvVar:      defaultPorcelainEnvironmentVariable,
	}
	cacheFlag := cli.BoolFlag{
		Name:        "useCache",
		Usage:       "Experimental: use a cache to speed up scripts",
		Required:    false,
		Destination: &CacheMode,
		EnvVar:      defaultCacheEnvironmentVariable,
	}

	// Chart owners target flags
	packageFlag := cli.StringFlag{
		Name:        "package,p",
		Usage:       "A package you would like to run the scripts on",
		Required:    false,
		Destination: &CurrentPackage,
		EnvVar:      defaultPackageEnvironmentVariable,
	}
	softErrorsFlag := cli.BoolFlag{
		Name:        "soft-errors",
		Usage:       "Enables soft error mode - some non-fatal errors will become warnings",
		EnvVar:      softErrorsEnvironmentVariable,
		Destination: &SoftErrorMode,
	}
	chartFlag := cli.StringFlag{
		Name: "chart,c",
		Usage: `Usage:
			./bin/charts-build-scripts <some_command> --chart="chart-name"
			CHART=<chart_name> make <some_command>

		A chart you would like to run the scripts on. Can include version.
		Default Environment Variable:
		`,
		Required:    false,
		Destination: &CurrentChart,
		EnvVar:      defaultChartEnvironmentVariable,
	}
	assetFlag := cli.StringFlag{
		Name:        "asset,a",
		Usage:       "An asset you would like to run the scripts on. Can directly point to archive.",
		Required:    false,
		Destination: &CurrentAsset,
		EnvVar:      defaultAssetEnvironmentVariable,
	}

	// Release Team Flags
	forkFlag := cli.StringFlag{
		Name: "fork",
		Usage: `Usage:
			./bin/charts-build-scripts <command> --fork="<fork-URL>"
			FORK="<fork-URL>" make <command>

		Your fork URL configured as a remote in your local git repository.
		`,
		Required:    true,
		Destination: &ForkURL,
		EnvVar:      defaultForkEnvironmentVariable,
	}
	chartVersionFlag := cli.StringFlag{
		Name: "version",
		Usage: `Usage:
			./bin/charts-build-scripts <command> --version="<chart_version>"
			CHART_VERSION="<chart_version>" make <command>

		Target version of chart to release.
		`,
		Required:    true,
		Destination: &ChartVersion,
		EnvVar:      defaultChartVersionEnvironmentVariable,
	}
	multiRCFlag := cli.BoolFlag{
		Name:        "multi-rc",
		Usage:       "default is false, if passed, auto-bump will not remove previous RC versions",
		Required:    false,
		EnvVar:      defaultMultiRCEnvironmentVariable,
		Destination: &MultiRC,
	}
	overrideVersionFlag := cli.StringFlag{
		Name: "override",
		Usage: `Usage:
			- "patch"
			- "minor"
			- "auto"
			- ""
		`,
		Required:    false,
		Destination: &OverrideVersion,
		EnvVar:      defaultOverrideVersionEnvironmentVariable,
	}
	newChartFlag := cli.BoolFlag{
		Name: "new-chart",
		Usage: `Usage:
			--new-chart=<false or true>
		`,
		Required:    false,
		Destination: &NewChart,
		EnvVar:      defaultNewChartVariable,
	}
	isPrimeChartFlag := cli.BoolFlag{
		Name: "is-prime",
		Usage: `Usage:
			--is-prime=<false or true>
		`,
		Required:    false,
		Destination: &IsPrimeChart,
		EnvVar:      defaultIsPrimeChartVariable,
	}

	// Registries Flags
	ghTokenFlag := cli.StringFlag{
		Name: "gh_token",
		Usage: `Usage:
					./bin/charts-build-scripts <command> --gh_token="********"
					GH_TOKEN="*********" make <command>

					Github Auth Token provided by Github Actions job
					`,
		Required:    true,
		EnvVar:      defaultGHTokenEnvironmentVariable,
		Destination: &GithubToken,
	}
	ociDNS := cli.StringFlag{
		Name: "oci-dns",
		Usage: `Usage:
			Provided OCI registry DNS.
		`,
		Required:    true,
		Destination: &OciDNS,
		EnvVar:      defaultOciDNS,
	}
	customOciPath := cli.StringFlag{
		Name: "custom-oci-path",
		Usage: `Usage:
			Provided OCI registry custom URL PATH.
		`,
		Required:    false,
		Destination: &CustomOCIPAth,
		EnvVar:      defaultCustomOCIPAth,
	}
	ociUser := cli.StringFlag{
		Name: "oci-user",
		Usage: `Usage:
			Provided OCI registry User.
		`,
		Required:    true,
		Destination: &OciUser,
		EnvVar:      defaultOciUser,
	}
	ociPass := cli.StringFlag{
		Name: "oci-pass",
		Usage: `Usage:
			Provided OCI registry Password.
		`,
		Required:    true,
		Destination: &OciPassword,
		EnvVar:      defaultOciPassword,
	}
	primeURLFlag := cli.StringFlag{
		Name:        "prime-url",
		Usage:       "--prime-url=******** || PRIME_URL=*******",
		Required:    true,
		EnvVar:      defaultPrimeURLEnvironmentVariable,
		Destination: &PrimeURL,
	}

	// branch specific flags
	branchVersionFlag := cli.StringFlag{
		Name: "branch-version",
		Usage: `Usage:
			./bin/charts-build-scripts <command> --branch-version="x.y"
			BRANCH_VERSION="x.y" make <command>

		The branch version line to compare against.
		Available inputs: (2.5; 2.6; 2.7; 2.8; 2.9; 2.10; 2.11; 2.12).
		Default Environment Variable:
		`,
		Required:    true,
		Destination: &BranchVersion,
		EnvVar:      defaultBranchVersionEnvironmentVariable,
	}
	branchFlag := cli.StringFlag{
		Name: "branch,b",
		Usage: `Usage:
					./bin/charts-build-scripts <command> --branch="release-v2.y" OR
					BRANCH="dev-v2.y" make <command>
					Available branches: (release-v2.8; dev-v2.9; release-v2.10.)
					`,
		Required:    true,
		EnvVar:      defaultBranchEnvironmentVariable,
		Destination: &Branch,
	}

	// Validation Flags
	localModeFlag := cli.BoolFlag{
		Name:        "local,l",
		Usage:       "Only perform local validation of the contents of assets and charts",
		Required:    false,
		Destination: &LocalMode,
	}
	remoteModeFlag := cli.BoolFlag{
		Name:        "remote,r",
		Usage:       "Only perform upstream validation of the contents of assets and charts",
		Required:    false,
		Destination: &RemoteMode,
	}
	prNumberFlag := cli.StringFlag{
		Name: "pr_number",
		Usage: `Usage:
					./bin/charts-build-scripts <command> --pr_number="****"
					PR_NUMBER="****" make <command>

					Pull Request identifying number provided by Github Actions job
					`,
		Required:    true,
		EnvVar:      defaultPRNumberEnvironmentVariable,
		Destination: &PullRequest,
	}
	skipFlag := cli.BoolFlag{
		Name:        "skip",
		Usage:       "Skip the execution and return success",
		EnvVar:      defaultSkipEnvironmentVariable,
		Destination: &Skip,
	}

	// Commands
	app.Commands = []cli.Command{
		// chart owners commands
		{
			Name:   "list",
			Usage:  "Print a list of all packages tracked in the current repository",
			Action: listPackages,
			Flags:  []cli.Flag{packageFlag, porcelainFlag},
		},
		{
			Name:   "prepare",
			Usage:  "Pull in the chart specified from upstream to the charts directory and apply any patch files",
			Action: prepareCharts,
			Before: setupCache,
			Flags:  []cli.Flag{packageFlag, cacheFlag, softErrorsFlag},
		},
		{
			Name:   "icon",
			Usage:  "Download the chart icon locally and use it",
			Action: downloadIcon,
			Flags:  []cli.Flag{packageFlag, configFlag, cacheFlag},
		},
		{
			Name:   "patch",
			Usage:  "Apply a patch between the upstream chart and the current state of the chart in the charts directory",
			Action: generatePatch,
			Before: setupCache,
			Flags:  []cli.Flag{packageFlag, cacheFlag},
		},
		{
			Name:   "charts",
			Usage:  "Create a local chart archive of your finalized chart for testing",
			Action: generateCharts,
			Before: setupCache,
			Flags:  []cli.Flag{packageFlag, configFlag, cacheFlag},
		},
		{
			Name:   "clean",
			Usage:  "Clean up your current repository to get it ready for a PR",
			Action: cleanRepo,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "validate",
			Usage:  "Run validation to ensure that contents of assets and charts won't overwrite released charts",
			Action: validateRepository,
			Flags:  []cli.Flag{packageFlag, configFlag, localModeFlag, remoteModeFlag, skipFlag},
		},
		// inner processes chart owners commands
		{
			Name:   "index",
			Usage:  "Create or update the existing Helm index.yaml at the repository root",
			Action: createOrUpdateIndex,
		},
		{
			Name:   "zip",
			Usage:  "Take the contents of a chart under charts/ and rezip the asset if it has been changed",
			Action: zipCharts,
			Flags:  []cli.Flag{chartFlag},
		},
		{
			Name:   "unzip",
			Usage:  "Take the contents of an asset under assets/ and unzip the chart",
			Action: unzipAssets,
			Flags:  []cli.Flag{assetFlag},
		},
		{
			Name:   "standardize",
			Usage:  "Standardize a Helm repository to the expected assets, charts, and index.yaml structure of these scripts",
			Action: standardizeRepo,
			Flags:  []cli.Flag{packageFlag, configFlag},
		},
		// validation commands
		{
			Name: "validate-release-charts",
			Usage: `Check charts to release in PR.
			`,
			Action: validateRelease,
			Flags:  []cli.Flag{branchFlag, ghTokenFlag, prNumberFlag, skipFlag},
		},
		{
			Name: "compare-index-files",
			Usage: `Compare the index.yaml between github repository and charts.rancher.io.
			`,
			Action: compareIndexFiles,
			Flags:  []cli.Flag{branchFlag},
		},

		// configuration commands
		{
			Name:   "clean-cache",
			Usage:  "Experimental: Clean cache",
			Action: cleanCache,
		},
		{
			Usage:  "Updates the current directory by applying the configuration.yaml on upstream Go templates to pull in the most up-to-date docs, scripts, etc.",
			Name:   "template",
			Action: createOrUpdateTemplate,
			Flags: []cli.Flag{
				// TODO: verify if this is the correct way to pass the variables
				configFlag,
				cli.StringFlag{
					Name:        "repositoryUrl,r",
					Required:    false,
					Destination: &puller.ChartsBuildScriptsRepositoryURL,
					Value:       "https://github.com/rancher/charts-build-scripts.git",
				},
				cli.StringFlag{
					Name:        "branch,b",
					Required:    false,
					Destination: &puller.ChartsBuildScriptsRepositoryBranch,
					Value:       "master",
				},
			},
		},

		// registries commands
		{
			Name:   "check-images",
			Usage:  "Checks all container images used in the charts repository",
			Action: checkImages,
		},
		{
			Name:   "check-rc",
			Usage:  "Checks if there are any images with RC tags or charts with RC versions in the charts repository",
			Action: checkRCTagsAndVersions,
		},
		{
			Name: "update-oci-registry",
			Usage: `Push helm charts to an OCI registry.
			`,
			Action: pushToOCI,
			Flags: []cli.Flag{
				debugFlag, ociDNS, ociUser, ociPass, customOciPath,
			},
		},
		{
			Name:   "scan-registries",
			Usage:  "Fetch, list and compare SUSE's registries and create yaml files with what is supposed to be synced from Docker Hub",
			Action: scanRegistries,
			Flags:  []cli.Flag{primeURLFlag},
		},
		{
			Name:   "sync-registries",
			Usage:  "Fetch, list and compare SUSE's registries and create yaml files with what is supposed to be synced from Docker Hub",
			Action: syncRegistries,
			Flags:  []cli.Flag{primeURLFlag, customOciPath},
		},

		// Release team commands
		{
			Name: "lifecycle-status",
			Usage: `Print the status of the current assets and charts based on the branch version and chart version according to the lifecycle rules.
			Saves the logs in the logs/ directory.`,
			Action: lifecycleStatus,
			Flags:  []cli.Flag{branchVersionFlag},
		},
		{
			Name: "release",
			Usage: `Execute the release script to release a chart to the production branch.
			`,
			Action: release,
			Flags:  []cli.Flag{branchVersionFlag, chartFlag, chartVersionFlag, forkFlag},
		},
		{
			Name: "auto-forward-port",
			Usage: `Execute the forward-port script to forward port a chart or all to the production branch.
				The charts to be forward ported are listed in the result of lifecycle-status command.
				It is advised to run make lifecycle-status before running this command.
				At the end of the execution, the script will create a PR with the changes to each chart.
				At the end of the execution, the script will save the logs in the logs directory,
				with all assets versions and branches that were pushed to the upstream repository.
			`,
			Action: autoForwardPort,
			Flags:  []cli.Flag{chartFlag, branchFlag},
		},

		// automatic processes for chart owners
		{
			Name:   "chart-bump",
			Usage:  `Generate a new chart bump PR.`,
			Action: chartBump,
			Before: setupCache,
			Flags:  []cli.Flag{packageFlag, branchVersionFlag, overrideVersionFlag, multiRCFlag, newChartFlag, isPrimeChartFlag},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal(context.Background(), err.Error())
	}
}

func listPackages(c *cli.Context) {
	packageList, err := charts.ListPackages(Ctx, CurrentPackage)
	if err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if PorcelainMode {
		logger.Log(Ctx, slog.LevelInfo, "", slog.String("packageList", strings.Join(packageList, " ")))
		return
	}
	logger.Log(Ctx, slog.LevelInfo, "", slog.Any("packageList", packageList))
}

func prepareCharts(c *cli.Context) {
	config.SetSoftError(Ctx, SoftErrorMode)

	packages, err := charts.GetPackages(Ctx, CurrentPackage)
	if err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if len(packages) == 0 {
		logger.Fatal(Ctx, "could not find any packages in packages/ folder")
	}

	for _, p := range packages {
		if err := p.Prepare(Ctx); err != nil {
			logger.Fatal(Ctx, err.Error())
		}
	}
}

func downloadIcon(c *cli.Context) {
	packages, err := charts.GetPackages(Ctx, CurrentPackage)
	if err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if len(packages) == 0 {
		logger.Fatal(Ctx, "could not find any packages in packages/ folder")
	}

	for _, p := range packages {
		err := p.DownloadIcon(Ctx)
		if err != nil {
			logger.Fatal(Ctx, err.Error())
		}
	}
}

func generatePatch(c *cli.Context) {
	packages, err := charts.GetPackages(Ctx, CurrentPackage)
	if err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if len(packages) != 1 {
		packageNames := make([]string, len(packages))
		for i, pkg := range packages {
			packageNames[i] = pkg.Name
		}
		logger.Fatal(Ctx, fmt.Sprintf("PACKAGE=\"%s\"; is wrong, it must be set to point to one package", CurrentPackage))
	}

	if err := packages[0].GeneratePatch(Ctx); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func generateCharts(c *cli.Context) {
	packages, err := charts.GetPackages(Ctx, CurrentPackage)
	if err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if len(packages) == 0 {
		logger.Fatal(Ctx, "could not find any packages in packages/ folder")
	}

	for _, p := range packages {
		if p.Auto == false {
			if err := p.GenerateCharts(Ctx); err != nil {
				logger.Fatal(Ctx, err.Error())
			}
		}
	}
}

func cleanRepo(c *cli.Context) {
	packages, err := charts.GetPackages(Ctx, CurrentPackage)
	if err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if len(packages) == 0 {
		logger.Fatal(Ctx, "could not find any packages in packages/ folder")
	}

	for _, p := range packages {
		if err := p.Clean(Ctx); err != nil {
			logger.Fatal(Ctx, err.Error())
		}
	}
}

func validateRepository(c *cli.Context) {
	if err := validate.ChartsRepository(Ctx, Skip, RemoteMode, LocalMode, CurrentPackage); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func createOrUpdateIndex(c *cli.Context) {
	if err := helm.CreateOrUpdateHelmIndex(Ctx); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func zipCharts(c *cli.Context) {
	if err := helm.ArchiveCharts(Ctx, CurrentChart); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if err := helm.CreateOrUpdateHelmIndex(Ctx); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func unzipAssets(c *cli.Context) {
	if err := helm.DumpAssets(Ctx, CurrentAsset); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
	if err := helm.CreateOrUpdateHelmIndex(Ctx); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func standardizeRepo(c *cli.Context) {
	if err := helm.RestructureChartsAndAssets(Ctx, nil); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func validateRelease(c *cli.Context) {
	if err := validate.PullRequestCheckPoints(Ctx, GithubToken, PullRequest, Branch, Skip); err != nil {
		logger.Fatal(Ctx, fmt.Errorf("failed to validate pull request: %w", err).Error())
	}
}

func compareIndexFiles(c *cli.Context) {
	if err := validate.CompareIndexFiles(Ctx, Branch); err != nil {
		logger.Fatal(Ctx, fmt.Errorf("failed to compare index files: %w", err).Error())
	}
}

func setupCache(c *cli.Context) error {
	return puller.InitRootCache(Ctx, CacheMode, config.PathCache)
}

func cleanCache(c *cli.Context) {
	if err := puller.CleanRootCache(Ctx, config.PathCache); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func createOrUpdateTemplate(c *cli.Context) {
	if err := puller.ApplyUpstreamTemplate(Ctx); err != nil {
		logger.Fatal(Ctx, fmt.Errorf("failed to update repository based on upstream template: %w", err).Error())
	}
}

func checkImages(c *cli.Context) {
	if err := registries.DockerScan(Ctx); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func checkRCTagsAndVersions(c *cli.Context) {
	// Grab all images that contain RC tags
	if err := registries.DockerCheckRCTags(Ctx); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func pushToOCI(c *cli.Context) {
	if err := registries.PusthToOci(Ctx, OciDNS, CustomOCIPAth, OciUser, OciPassword, DebugMode); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func scanRegistries(c *cli.Context) {
	if err := registries.Scan(Ctx, PrimeURL+"/"); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func syncRegistries(c *cli.Context) {
	if err := registries.Sync(Ctx, PrimeURL, CustomOCIPAth); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func lifecycleStatus(c *cli.Context) {
	if _, err := validate.LifecycleStatus(Ctx, BranchVersion); err != nil {
		logger.Fatal(Ctx, fmt.Errorf("failed to check lifecycle status: %w", err).Error())
	}
}

func release(c *cli.Context) {
	if err := auto.Release(Ctx, ChartVersion, CurrentChart); err != nil {
		logger.Fatal(Ctx, fmt.Errorf("failed to release: %w", err).Error())
	}
}

func autoForwardPort(c *cli.Context) {
	if err := auto.ForwardPort(Ctx, Branch); err != nil {
		logger.Fatal(Ctx, err.Error())
	}
}

func chartBump(c *cli.Context) {
	if err := auto.BumpChart(Ctx, CurrentPackage, BranchVersion, OverrideVersion, MultiRC, NewChart, IsPrimeChart); err != nil {
		logger.Fatal(Ctx, fmt.Errorf("failed to bump: %w", err).Error())
	}
}
