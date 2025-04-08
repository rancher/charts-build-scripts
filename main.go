package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/util"

	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/auto"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/images"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/regsync"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/standardize"
	"github.com/rancher/charts-build-scripts/pkg/update"
	"github.com/rancher/charts-build-scripts/pkg/validate"
	"github.com/rancher/charts-build-scripts/pkg/zip"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	// defaultChartsScriptOptionsFile is the default path to look a file containing options for the charts scripts to use for this branch
	defaultChartsScriptOptionsFile = path.ConfigurationYamlFile
	// defaultPackageEnvironmentVariable is the default environment variable for picking a specific package
	defaultPackageEnvironmentVariable = "PACKAGE"
	// defaultChartEnvironmentVariable is the default environment variable for picking a specific chart
	defaultChartEnvironmentVariable = "CHART"
	// defaultAssetEnvironmentVariable is the default environment variable for picking a specific asset
	defaultAssetEnvironmentVariable = "ASSET"
	// defaultPorcelainEnvironmentVariable is the default environment variable that indicates whether we should run on porcelain mode
	defaultPorcelainEnvironmentVariable = "PORCELAIN"
	// defaultCacheEnvironmentVariable is the default environment variable that indicates that a cache should be used on pulls to remotes
	defaultCacheEnvironmentVariable = "USE_CACHE"
	// defaultBranchVersionEnvironmentVariable is the default environment variable that indicates the branch version to compare against
	defaultBranchVersionEnvironmentVariable = "BRANCH_VERSION"
	// defaultBranchEnvironmentVariable is the default environment variable that indicates the branch
	defaultBranchEnvironmentVariable = "BRANCH"
	// defaultForkEnvironmentVariable is the default environment variable that indicates the fork URL
	defaultForkEnvironmentVariable = "FORK"
	// defaultChartVersionEnvironmentVariable is the default environment variable that indicates the version to release
	defaultChartVersionEnvironmentVariable = "CHART_VERSION"
	// defaultGHTokenEnvironmentVariable is the default environment variable that indicates the Github Auth token
	defaultGHTokenEnvironmentVariable = "GH_TOKEN"
	// defaultPRNumberEnvironmentVariable is the default environment variable that indicates the PR number
	defaultPRNumberEnvironmentVariable = "PR_NUMBER"
	// defaultSkipEnvironmentVariable is the default environment variable that indicates whether to skip execution
	defaultSkipEnvironmentVariable = "SKIP"
	// softErrorsEnvironmentVariable is the default environment variable that indicates if soft error mode is enabled
	softErrorsEnvironmentVariable = "SOFT_ERRORS"
	// defaultLogLevelEnvironmentVariable is the default environment variable that indicates the log level
	defaultLogLevelEnvironmentVariable = "LOG"
)

var (
	// Version represents the current version of the chart build scripts
	Version = "v0.0.0-dev"
	// GitCommit represents the latest commit when building this script
	GitCommit = "HEAD"
	// ChartsScriptOptionsFile represents a name of a file that contains options for the charts script to use for this branch
	ChartsScriptOptionsFile string
	// CurrentPackage represents the specific package to apply the scripts to
	CurrentPackage string
	// CurrentChart represents a specific chart to apply the scripts to. Also accepts a specific version.
	CurrentChart string
	// CurrentAsset represents a specific asset to apply the scripts to. Also accepts a specific archive.
	CurrentAsset string
	// PorcelainMode indicates that the output of the scripts should be in an easy-to-parse format for scripts
	PorcelainMode bool
	// LocalMode indicates that only local validation should be run
	LocalMode bool
	// RemoteMode indicates that only remote validation should be run
	RemoteMode bool
	// CacheMode indicates that caching should be used on all remotely pulled resources
	CacheMode = false
	// ForkURL represents the fork URL configured as a remote in your local git repository
	ForkURL = ""
	// ChartVersion of the chart to release
	ChartVersion = ""
	// Branch repsents the branch to compare against
	Branch = ""
	// PullRequest represents the Pull Request identifying number
	PullRequest = ""
	// GithubToken represents the Github Auth token
	GithubToken string
	// Skip indicates whether to skip execution
	Skip bool
	// SoftErrorMode indicates if certain non-fatal errors will be turned into warnings
	SoftErrorMode = false
	// RepoRoot represents the root path of the repository
	RepoRoot string
)

func init() {
	tintOptions := &tint.Options{
		AddSource:  true,
		TimeFormat: "15:04:05",
	}

	// Set the log level based on the LOG environment variable
	lvl := os.Getenv("LOG")
	if lvl != "" {
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
	}
	// Create a new slog logger with tint handler
	logger := slog.New(tint.NewHandler(os.Stderr, tintOptions))
	slog.SetDefault(logger)
}

func main() {
	util.InitSoftErrorMode()

	app := cli.NewApp()
	app.Name = "charts-build-scripts"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Build scripts used to maintain patches on Helm charts forked from other repositories"
	// Flags
	configFlag := cli.StringFlag{
		Name:        "config",
		Usage:       "A configuration file with additional options for allowing this branch to interact with other branches",
		TakesFile:   true,
		Destination: &ChartsScriptOptionsFile,
		Value:       defaultChartsScriptOptionsFile,
	}
	packageFlag := cli.StringFlag{
		Name:        "package,p",
		Usage:       "A package you would like to run the scripts on",
		Required:    false,
		Destination: &CurrentPackage,
		EnvVar:      defaultPackageEnvironmentVariable,
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
	branchVersionFlag := cli.StringFlag{
		Name: "branch-version",
		Usage: `Usage:
			./bin/charts-build-scripts <command> --branch-version="x.y"
			BRANCH_VERSION="x.y" make <command>

		The branch version line to compare against.
		Available inputs: (2.5; 2.6; 2.7; 2.8; 2.9; 2.10; 2.11; 2.12).
		Default Environment Variable:
		`,
		Required: true,
		EnvVar:   defaultBranchVersionEnvironmentVariable,
	}
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
	softErrorsFlag := cli.BoolFlag{
		Name:        "soft-errors",
		Usage:       "Enables soft error mode - some non-fatal errors will become warnings",
		EnvVar:      softErrorsEnvironmentVariable,
		Destination: &SoftErrorMode,
	}

	// Commands
	app.Commands = []cli.Command{
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
			Name:   "regsync",
			Usage:  "Create a regsync config file containing all images used for the particular Rancher version",
			Action: generateRegSyncConfigFile,
			Flags:  []cli.Flag{},
		},
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
			Name:   "clean",
			Usage:  "Clean up your current repository to get it ready for a PR",
			Action: cleanRepo,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "clean-cache",
			Usage:  "Experimental: Clean cache",
			Action: cleanCache,
		},
		{
			Name:   "validate",
			Usage:  "Run validation to ensure that contents of assets and charts won't overwrite released charts",
			Action: validateRepo,
			Flags:  []cli.Flag{packageFlag, configFlag, localModeFlag, remoteModeFlag},
		},
		{
			Name:   "standardize",
			Usage:  "Standardize a Helm repository to the expected assets, charts, and index.yaml structure of these scripts",
			Action: standardizeRepo,
			Flags:  []cli.Flag{packageFlag, configFlag},
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
					Destination: &update.ChartsBuildScriptsRepositoryURL,
					Value:       "https://github.com/rancher/charts-build-scripts.git",
				},
				cli.StringFlag{
					Name:        "branch,b",
					Required:    false,
					Destination: &update.ChartsBuildScriptsRepositoryBranch,
					Value:       "master",
				},
			},
		},
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
			Name:   "icon",
			Usage:  "Download the chart icon locally and use it",
			Action: downloadIcon,
			Flags:  []cli.Flag{packageFlag, configFlag, cacheFlag},
		},
		{
			Name: "lifecycle-status",
			Usage: `Print the status of the current assets and charts based on the branch version and chart version according to the lifecycle rules.
			Saves the logs in the logs/ directory.`,
			Action: lifecycleStatus,
			Flags:  []cli.Flag{branchVersionFlag, chartFlag},
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
			Flags:  []cli.Flag{branchVersionFlag, chartFlag, forkFlag},
		},
		{
			Name: "release",
			Usage: `Execute the release script to release a chart to the production branch.
			`,
			Action: release,
			Flags:  []cli.Flag{branchVersionFlag, chartFlag, chartVersionFlag, forkFlag},
		},
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
		{
			Name: "chart-bump",
			Usage: `Generate a new chart bump PR.
			`,
			Action: chartBump,
			Before: setupCache,
			Flags:  []cli.Flag{packageFlag, branchFlag},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal(err.Error())
	}
}

func listPackages(c *cli.Context) {
	getRepoRoot()
	packageList, err := charts.ListPackages(RepoRoot, CurrentPackage)
	if err != nil {
		logger.Fatal(err.Error())
	}
	if PorcelainMode {
		logger.Log(slog.LevelInfo, "", slog.String("packageList", strings.Join(packageList, " ")))
		return

	}

	logger.Log(slog.LevelInfo, "", slog.Any("packageList", packageList))
}

func prepareCharts(c *cli.Context) {
	util.SetSoftErrorMode(SoftErrorMode)
	packages := getPackages()
	if len(packages) == 0 {
		logger.Fatal("could not find any packages in packages/ folder")
	}
	for _, p := range packages {
		if err := p.Prepare(); err != nil {
			logger.Fatal(err.Error())
		}
	}
}

func generatePatch(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(slog.LevelInfo, "no packages found")
		return
	}
	if len(packages) != 1 {
		packageNames := make([]string, len(packages))
		for i, pkg := range packages {
			packageNames[i] = pkg.Name
		}
		logger.Fatal(fmt.Sprintf("PACKAGE=\"%s\"; is wrong, it must be set to point to one package", CurrentPackage))
	}
	if err := packages[0].GeneratePatch(); err != nil {
		logger.Fatal(err.Error())
	}
}

func generateCharts(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(slog.LevelInfo, "no packages found")
		return
	}
	chartsScriptOptions := parseScriptOptions()
	for _, p := range packages {
		if p.Auto == false {
			if err := p.GenerateCharts(chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
				logger.Fatal(err.Error())
			}
		}
	}
}

func downloadIcon(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(slog.LevelInfo, "no packages found")
		return
	}
	for _, p := range packages {
		err := p.DownloadIcon()
		if err != nil {
			logger.Fatal(err.Error())
		}
	}
}

func generateRegSyncConfigFile(c *cli.Context) {
	if err := regsync.GenerateConfigFile(); err != nil {
		logger.Fatal(err.Error())
	}
}

func createOrUpdateIndex(c *cli.Context) {
	getRepoRoot()
	if err := helm.CreateOrUpdateHelmIndex(filesystem.GetFilesystem(RepoRoot)); err != nil {
		logger.Fatal(err.Error())
	}
}

func zipCharts(c *cli.Context) {
	getRepoRoot()
	if err := zip.ArchiveCharts(RepoRoot, CurrentChart); err != nil {
		logger.Fatal(err.Error())
	}
	createOrUpdateIndex(c)
}

func unzipAssets(c *cli.Context) {
	getRepoRoot()
	if err := zip.DumpAssets(RepoRoot, CurrentAsset); err != nil {
		logger.Fatal(err.Error())
	}
	createOrUpdateIndex(c)
}

func cleanRepo(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(slog.LevelInfo, "no packages found")
		return
	}
	for _, p := range packages {
		if err := p.Clean(); err != nil {
			logger.Fatal(err.Error())
		}
	}
}

func validateRepo(c *cli.Context) {
	if LocalMode && RemoteMode {
		logger.Fatal("cannot specify both local and remote validation")
	}

	chartsScriptOptions := parseScriptOptions()

	logger.Log(slog.LevelInfo, "checking if Git is clean")
	_, _, status := getGitInfo()
	if !status.IsClean() {
		logger.Fatal("repository must be clean to run validation")
	}

	if RemoteMode {
		logger.Log(slog.LevelInfo, "remove validation only")
	} else {
		logger.Log(slog.LevelInfo, "generating charts")
		generateCharts(c)

		logger.Log(slog.LevelInfo, "checking if Git is clean after generating charts")
		_, _, status = getGitInfo()
		if err := validate.StatusExceptions(status); err != nil {
			logger.Fatal(err.Error())
		}

		logger.Log(slog.LevelInfo, "successfully validated that current charts and assets are up-to-date")
	}

	if chartsScriptOptions.ValidateOptions != nil {
		if LocalMode {
			logger.Log(slog.LevelInfo, "local validation only")
		} else {
			getRepoRoot()
			repoFs := filesystem.GetFilesystem(RepoRoot)
			releaseOptions, err := options.LoadReleaseOptionsFromFile(repoFs, "release.yaml")
			if err != nil {
				logger.Fatal(fmt.Errorf("unable to unmarshall release.yaml: %w", err).Error())
			}
			u := chartsScriptOptions.ValidateOptions.UpstreamOptions
			branch := chartsScriptOptions.ValidateOptions.Branch

			logger.Log(slog.LevelInfo, "upstream validation against repository", slog.String("url", u.URL), slog.String("branch", branch))
			compareGeneratedAssetsResponse, err := validate.CompareGeneratedAssets(RepoRoot, repoFs, u, branch, releaseOptions)
			if err != nil {
				logger.Fatal(err.Error())
			}
			if !compareGeneratedAssetsResponse.PassedValidation() {
				// Output charts that have been modified
				compareGeneratedAssetsResponse.LogDiscrepancies()

				logger.Log(slog.LevelInfo, "dumping release.yaml to track changes that have been introduced")
				if err := compareGeneratedAssetsResponse.DumpReleaseYaml(repoFs); err != nil {
					logger.Log(slog.LevelError, "unable to dump newly generated release.yaml", logger.Err(err))
				}

				logger.Log(slog.LevelInfo, "updating index.yaml")
				if err := helm.CreateOrUpdateHelmIndex(repoFs); err != nil {
					logger.Fatal(err.Error())
				}

				logger.Fatal(fmt.Sprintf("validation against upstream repository %s at branch %s failed.", u.URL, branch))
			}
		}
	}

	logger.Log(slog.LevelInfo, "zipping charts to ensure that contents of assets, charts, and index.yaml are in sync")
	zipCharts(c)

	logger.Log(slog.LevelInfo, "final check if Git is clean")
	_, _, status = getGitInfo()
	if !status.IsClean() {
		logger.Fatal(fmt.Sprintf("repository must be clean to pass validation; status: %s", status.String()))
	}

	logger.Log(slog.LevelInfo, "make validate success")
}

func standardizeRepo(c *cli.Context) {
	getRepoRoot()
	repoFs := filesystem.GetFilesystem(RepoRoot)
	if err := standardize.RestructureChartsAndAssets(repoFs); err != nil {
		logger.Fatal(err.Error())
	}
}

func createOrUpdateTemplate(c *cli.Context) {
	getRepoRoot()
	repoFs := filesystem.GetFilesystem(RepoRoot)
	chartsScriptOptions := parseScriptOptions()
	if err := update.ApplyUpstreamTemplate(repoFs, *chartsScriptOptions); err != nil {
		logger.Fatal(fmt.Errorf("failed to update repository based on upstream template: %w", err).Error())
	}

	logger.Log(slog.LevelInfo, "successfully updated repository based on upstream template")
}

func setupCache(c *cli.Context) error {
	getRepoRoot()
	return puller.InitRootCache(RepoRoot, CacheMode, path.DefaultCachePath)
}

func cleanCache(c *cli.Context) {
	getRepoRoot()
	if err := puller.CleanRootCache(RepoRoot, path.DefaultCachePath); err != nil {
		logger.Fatal(err.Error())
	}
}

func parseScriptOptions() *options.ChartsScriptOptions {
	configYaml, err := os.ReadFile(ChartsScriptOptionsFile)
	if err != nil {
		logger.Fatal(fmt.Errorf("unable to find configuration file: %w", err).Error())
	}
	chartsScriptOptions := options.ChartsScriptOptions{}
	if err := yaml.UnmarshalStrict(configYaml, &chartsScriptOptions); err != nil {
		logger.Fatal(fmt.Errorf("unable to unmarshall configuration file: %w", err).Error())
	}
	return &chartsScriptOptions
}

func getRepoRoot() {
	RepoRoot = os.Getenv("DEV_REPO_ROOT")
	if RepoRoot != "" {
		logger.Log(slog.LevelDebug, "using customized repo root: ", slog.String("repoRoot", RepoRoot))
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		logger.Fatal(fmt.Errorf("unable to get current working directory: %w", err).Error())
	}
	if repoRoot == "" {
		logger.Fatal("unable to get current working directory")
	}

	logger.Log(slog.LevelDebug, "using current working directory as repo root: ", slog.String("repoRoot", repoRoot))
	RepoRoot = repoRoot
}

func getPackages() []*charts.Package {
	getRepoRoot()
	packages, err := charts.GetPackages(RepoRoot, CurrentPackage)
	if err != nil {
		logger.Fatal(err.Error())
	}
	return packages
}

func getGitInfo() (*git.Repository, *git.Worktree, git.Status) {
	getRepoRoot()
	repo, err := repository.GetRepo(RepoRoot)
	if err != nil {
		logger.Fatal(err.Error())
	}
	// Check if git is clean
	wt, err := repo.Worktree()
	if err != nil {
		logger.Fatal(err.Error())
	}
	status, err := wt.Status()
	if err != nil {
		logger.Fatal(err.Error())
	}
	return repo, wt, status
}

func checkImages(c *cli.Context) {
	if err := images.CheckImages(); err != nil {
		logger.Fatal(err.Error())
	}
}

func checkRCTagsAndVersions(c *cli.Context) {
	getRepoRoot()
	// Grab all images that contain RC tags
	rcImageTagMap := images.CheckRCTags(RepoRoot)

	// Grab all chart versions that contain RC tags
	rcChartVersionMap, err := charts.CheckRCCharts(RepoRoot)
	if err != nil {
		logger.Fatal(fmt.Errorf("unable to check for RC charts: %w", err).Error())
	}

	// If there are any charts that contains RC version or images that contains RC tags
	// log them and return an error
	if len(rcChartVersionMap) > 0 || len(rcImageTagMap) > 0 {
		logger.Log(slog.LevelError, "found images with RC tags", slog.Any("rcImageTagMap", rcImageTagMap))
		logger.Log(slog.LevelError, "found charts with RC version", slog.Any("rcChartVersionMap", rcChartVersionMap))
		logger.Fatal("RC check has failed")
	}

	logger.Log(slog.LevelInfo, "successfully checked RC tags and versions")
}

func lifecycleStatus(c *cli.Context) {
	// Initialize dependencies with branch-version and current chart
	logger.Log(slog.LevelDebug, "initialize lifecycle-status")

	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)
	lifeCycleDep, err := lifecycle.InitDependencies(RepoRoot, rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logger.Fatal(fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	// Execute lifecycle status check and save the logs
	logger.Log(slog.LevelDebug, "checking lifecycle status and saving logs")
	_, err = lifeCycleDep.CheckLifecycleStatusAndSave(CurrentChart)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to check lifecycle status: %w", err).Error())
	}
}

func autoForwardPort(c *cli.Context) {
	if ForkURL == "" {
		logger.Fatal("FORK environment variable must be set to run auto-forward-port")
	}

	// Initialize dependencies with branch-version and current chart
	logger.Log(slog.LevelDebug, "initialize auto forward port")

	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	lifeCycleDep, err := lifecycle.InitDependencies(RepoRoot, rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logger.Fatal(fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	// Execute lifecycle status check and save the logs
	logger.Log(slog.LevelInfo, "checking lifecycle status and saving logs")
	status, err := lifeCycleDep.CheckLifecycleStatusAndSave(CurrentChart)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to check lifecycle status: %w", err).Error())
	}

	// Execute forward port with loaded information from status
	fp, err := auto.CreateForwardPortStructure(lifeCycleDep, status.AssetsToBeForwardPorted, ForkURL)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to prepare forward port: %w", err).Error())
	}

	err = fp.ExecuteForwardPort(CurrentChart)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to execute forward port: %w", err).Error())
	}
}

func release(c *cli.Context) {
	if ForkURL == "" {
		logger.Fatal("FORK environment variable must be set to run release cmd")
	}

	if CurrentChart == "" {
		logger.Fatal("CHART environment variable must be set to run release cmd")
	}
	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	dependencies, err := lifecycle.InitDependencies(RepoRoot, rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logger.Fatal(fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	status, err := lifecycle.LoadState(rootFs)
	if err != nil {
		logger.Fatal(fmt.Errorf("could not load state; please run lifecycle-status before this command: %w", err).Error())
	}

	release, err := auto.InitRelease(dependencies, status, ChartVersion, CurrentChart, ForkURL)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to initialize release: %w", err).Error())
	}

	if err := release.PullAsset(); err != nil {
		logger.Fatal(fmt.Errorf("failed to execute release: %w", err).Error())
	}

	// Unzip Assets: ASSET=<chart>/<chart>-<version.tgz make unzip
	CurrentAsset = release.Chart + "/" + release.AssetTgz
	unzipAssets(c)

	// update release.yaml
	if err := release.UpdateReleaseYaml(); err != nil {
		logger.Fatal(fmt.Errorf("failed to update release.yaml: %w", err).Error())
	}

	// make index
	createOrUpdateIndex(c)
}

func validateRelease(c *cli.Context) {
	if Skip {
		logger.Log(slog.LevelInfo, "skipping release validation")
		return
	}
	if GithubToken == "" {
		logger.Fatal("GH_TOKEN environment variable must be set to run validate-release-charts")
	}
	if PullRequest == "" {
		logger.Fatal("PR_NUMBER environment variable must be set to run validate-release-charts")
	}
	if Branch == "" {
		logger.Fatal("BRANCH environment variable must be set to run validate-release-charts")
	}
	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	if !strings.HasPrefix(Branch, "release-v") {
		logger.Fatal("branch must be in the format release-v2.x")
	}

	dependencies, err := lifecycle.InitDependencies(RepoRoot, rootFs, strings.TrimPrefix(Branch, "release-v"), "")
	if err != nil {
		logger.Fatal(fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	if err := auto.ValidatePullRequest(GithubToken, PullRequest, dependencies); err != nil {
		logger.Fatal(fmt.Errorf("failed to validate pull request: %w", err).Error())
	}
}

func compareIndexFiles(c *cli.Context) {
	if Branch == "" {
		logger.Fatal("BRANCH environment variable must be set to run compare-index-files")
	}

	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	if err := auto.CompareIndexFiles(rootFs); err != nil {
		logger.Fatal(fmt.Errorf("failed to compare index files: %w", err).Error())
	}

	logger.Log(slog.LevelInfo, "index.yaml files are the same at git repository and charts.rancher.io")
}

func chartBump(c *cli.Context) {
	if CurrentPackage == "" {
		logger.Fatal("CurrentPackage environment variable must be set")
	}
	if Branch == "" {
		logger.Fatal("Branch environment variable must be set")
	}

	ChartsScriptOptionsFile = path.ConfigurationYamlFile
	chartsScriptOptions := parseScriptOptions()

	logger.Log(slog.LevelDebug, "", slog.String("CurrentPackage", CurrentPackage))
	logger.Log(slog.LevelDebug, "", slog.String("Branch", Branch))

	logger.Log(slog.LevelInfo, "setup auto-chart-bump")
	bump, err := auto.SetupBump(RepoRoot, CurrentPackage, Branch, chartsScriptOptions)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to setup: %w", err).Error())
	}

	logger.Log(slog.LevelInfo, "start auto-chart-bump")
	if err := bump.BumpChart(); err != nil {
		logger.Fatal(fmt.Errorf("failed to bump: %w", err).Error())
	}
}
