package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	// defaultChartsScriptOptionsFile is the default path to look a file containing options for the charts scripts to use for this branch
	defaultChartsScriptOptionsFile = "configuration.yaml"
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
)

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		logrus.SetLevel(logrus.DebugLevel)
	}
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
		logrus.Fatal(err)
	}
}

func listPackages(c *cli.Context) {
	repoRoot := getRepoRoot()
	packageList, err := charts.ListPackages(repoRoot, CurrentPackage)
	if err != nil {
		logrus.Fatal(err)
	}
	if PorcelainMode {
		fmt.Println(strings.Join(packageList, " "))
		return

	}
	logrus.Infof("Found the following packages: %v", packageList)
}

func prepareCharts(c *cli.Context) {
	util.SetSoftErrorMode(SoftErrorMode)
	packages := getPackages()
	if len(packages) == 0 {
		logrus.Fatal("Could not find any packages in packages/")
	}
	for _, p := range packages {
		if err := p.Prepare(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func generatePatch(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	if len(packages) != 1 {
		packageNames := make([]string, len(packages))
		for i, pkg := range packages {
			packageNames[i] = pkg.Name
		}
		logrus.Fatalf(
			"PACKAGE=\"%s\" must be set to point to exactly one package. Currently found the following packages: %s",
			CurrentPackage, packageNames,
		)
	}
	if err := packages[0].GeneratePatch(); err != nil {
		logrus.Fatal(err)
	}
}

func generateCharts(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	chartsScriptOptions := parseScriptOptions()
	for _, p := range packages {
		if p.Auto == false {
			if err := p.GenerateCharts(chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
				logrus.Fatal(err)
			}
		}
	}
}

func downloadIcon(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	for _, p := range packages {
		err := p.DownloadIcon()
		if err != nil {
			logrus.Fatal(err)
		}
	}
}

func generateRegSyncConfigFile(c *cli.Context) {
	if err := regsync.GenerateConfigFile(); err != nil {
		logrus.Fatal(err)
	}
}

func createOrUpdateIndex(c *cli.Context) {
	repoRoot := getRepoRoot()
	if err := helm.CreateOrUpdateHelmIndex(filesystem.GetFilesystem(repoRoot)); err != nil {
		logrus.Fatal(err)
	}
}

func zipCharts(c *cli.Context) {
	repoRoot := getRepoRoot()
	if err := zip.ArchiveCharts(repoRoot, CurrentChart); err != nil {
		logrus.Fatal(err)
	}
	createOrUpdateIndex(c)
}

func unzipAssets(c *cli.Context) {
	repoRoot := getRepoRoot()
	if err := zip.DumpAssets(repoRoot, CurrentAsset); err != nil {
		logrus.Fatal(err)
	}
	createOrUpdateIndex(c)
}

func cleanRepo(c *cli.Context) {
	packages := getPackages()
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	for _, p := range packages {
		if err := p.Clean(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func validateRepo(c *cli.Context) {
	if LocalMode && RemoteMode {
		logrus.Fatalf("cannot specify both local and remote validation")
	}

	chartsScriptOptions := parseScriptOptions()

	logrus.Infof("Checking if Git is clean")
	_, _, status := getGitInfo()
	if !status.IsClean() {
		logrus.Warnf("Git is not clean:\n%s", status)
		logrus.Fatal("Repository must be clean to run validation")
	}

	if RemoteMode {
		logrus.Infof("Running remote validation only, skipping generating charts locally")
	} else {
		logrus.Infof("Generating charts")
		generateCharts(c)

		logrus.Infof("Checking if Git is clean after generating charts")
		_, _, status = getGitInfo()
		if !status.IsClean() {
			logrus.Warnf("Generated charts produced the following changes in Git.\n%s", status)
			logrus.Fatalf("Please commit these changes and run validation again.")
		}
		logrus.Infof("Successfully validated that current charts and assets are up to date.")
	}

	if chartsScriptOptions.ValidateOptions != nil {
		if LocalMode {
			logrus.Infof("Running local validation only, skipping pulling upstream")
		} else {
			repoRoot := getRepoRoot()
			repoFs := filesystem.GetFilesystem(repoRoot)
			releaseOptions, err := options.LoadReleaseOptionsFromFile(repoFs, "release.yaml")
			if err != nil {
				logrus.Fatalf("Unable to unmarshall release.yaml: %s", err)
			}
			u := chartsScriptOptions.ValidateOptions.UpstreamOptions
			branch := chartsScriptOptions.ValidateOptions.Branch
			logrus.Infof("Performing upstream validation against repository %s at branch %s", u.URL, branch)
			compareGeneratedAssetsResponse, err := validate.CompareGeneratedAssets(repoFs, u, branch, releaseOptions)
			if err != nil {
				logrus.Fatal(err)
			}
			if !compareGeneratedAssetsResponse.PassedValidation() {
				// Output charts that have been modified
				compareGeneratedAssetsResponse.LogDiscrepancies()
				logrus.Infof("Dumping release.yaml tracking changes that have been introduced")
				if err := compareGeneratedAssetsResponse.DumpReleaseYaml(repoFs); err != nil {
					logrus.Errorf("Unable to dump newly generated release.yaml: %s", err)
				}
				logrus.Infof("Updating index.yaml")
				if err := helm.CreateOrUpdateHelmIndex(repoFs); err != nil {
					logrus.Fatal(err)
				}
				logrus.Fatalf("Validation against upstream repository %s at branch %s failed.", u.URL, branch)
			}
		}
	}

	logrus.Info("Zipping charts to ensure that contents of assets, charts, and index.yaml are in sync.")
	zipCharts(c)

	logrus.Info("Doing a final check to ensure Git is clean")
	_, _, status = getGitInfo()
	if !status.IsClean() {
		logrus.Warnf("Git is not clean:\n%s", status)
		logrus.Fatal("Repository must be clean to pass validation")
	}

	logrus.Info("Successfully validated current repository!")
}

func standardizeRepo(c *cli.Context) {
	repoRoot := getRepoRoot()
	repoFs := filesystem.GetFilesystem(repoRoot)
	if err := standardize.RestructureChartsAndAssets(repoFs); err != nil {
		logrus.Fatal(err)
	}
}

func createOrUpdateTemplate(c *cli.Context) {
	repoRoot := getRepoRoot()
	repoFs := filesystem.GetFilesystem(repoRoot)
	chartsScriptOptions := parseScriptOptions()
	if err := update.ApplyUpstreamTemplate(repoFs, *chartsScriptOptions); err != nil {
		logrus.Fatalf("Failed to update repository based on upstream template: %s", err)
	}
	logrus.Infof("Successfully updated repository based on upstream template.")
}

func setupCache(c *cli.Context) error {
	return puller.InitRootCache(CacheMode, path.DefaultCachePath)
}

func cleanCache(c *cli.Context) {
	if err := puller.CleanRootCache(path.DefaultCachePath); err != nil {
		logrus.Fatal(err)
	}
}

func parseScriptOptions() *options.ChartsScriptOptions {
	configYaml, err := ioutil.ReadFile(defaultChartsScriptOptionsFile)
	if err != nil {
		logrus.Fatalf("Unable to find configuration file: %s", err)
	}
	chartsScriptOptions := options.ChartsScriptOptions{}
	if err := yaml.UnmarshalStrict(configYaml, &chartsScriptOptions); err != nil {
		logrus.Fatalf("Unable to unmarshall configuration file: %s", err)
	}
	return &chartsScriptOptions
}

func getRepoRoot() string {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	return repoRoot
}

func getPackages() []*charts.Package {
	repoRoot := getRepoRoot()
	packages, err := charts.GetPackages(repoRoot, CurrentPackage)
	if err != nil {
		logrus.Fatal(err)
	}
	return packages
}

func getGitInfo() (*git.Repository, *git.Worktree, git.Status) {
	repoRoot := getRepoRoot()
	repo, err := repository.GetRepo(repoRoot)
	if err != nil {
		logrus.Fatal(err)
	}
	// Check if git is clean
	wt, err := repo.Worktree()
	if err != nil {
		logrus.Fatal(err)
	}
	status, err := wt.Status()
	if err != nil {
		logrus.Fatal(err)
	}
	return repo, wt, status
}

func checkImages(c *cli.Context) {
	if err := images.CheckImages(); err != nil {
		logrus.Fatal(err)
	}
}

func checkRCTagsAndVersions(c *cli.Context) {
	// Grab all images that contain RC tags
	rcImageTagMap := images.CheckRCTags()

	// Grab all chart versions that contain RC tags
	rcChartVersionMap := charts.CheckRCCharts()

	// If there are any charts that contains RC version or images that contains RC tags
	// log them and return an error
	if len(rcChartVersionMap) > 0 || len(rcImageTagMap) > 0 {
		logrus.Errorf("found images with RC tags: %v", rcImageTagMap)
		logrus.Errorf("found charts with RC version: %v", rcChartVersionMap)
		logrus.Fatal("RC check has failed")
	}

	logrus.Info("RC check has succeeded")
}

func lifecycleStatus(c *cli.Context) {
	// Initialize dependencies with branch-version and current chart
	logrus.Info("Initializing dependencies for lifecycle-status")
	rootFs := filesystem.GetFilesystem(getRepoRoot())
	lifeCycleDep, err := lifecycle.InitDependencies(rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logrus.Fatalf("encountered error while initializing dependencies: %s", err)
	}

	// Execute lifecycle status check and save the logs
	logrus.Info("Checking lifecycle status and saving logs")
	_, err = lifeCycleDep.CheckLifecycleStatusAndSave(CurrentChart)
	if err != nil {
		logrus.Fatalf("Failed to check lifecycle status: %s", err)
	}
}

func autoForwardPort(c *cli.Context) {
	if ForkURL == "" {
		logrus.Fatal("FORK environment variable must be set to run auto-forward-port")
	}

	// Initialize dependencies with branch-version and current chart
	logrus.Info("Initializing dependencies for auto-forward-port")
	rootFs := filesystem.GetFilesystem(getRepoRoot())
	lifeCycleDep, err := lifecycle.InitDependencies(rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logrus.Fatalf("encountered error while initializing dependencies: %v", err)
	}

	// Execute lifecycle status check and save the logs
	logrus.Info("Checking lifecycle status and saving logs")
	status, err := lifeCycleDep.CheckLifecycleStatusAndSave(CurrentChart)
	if err != nil {
		logrus.Fatalf("Failed to check lifecycle status: %v", err)
	}

	// Execute forward port with loaded information from status
	logrus.Info("Preparing forward port data")
	fp, err := auto.CreateForwardPortStructure(lifeCycleDep, status.AssetsToBeForwardPorted, ForkURL)
	if err != nil {
		logrus.Fatalf("Failed to prepare forward port: %v", err)
	}

	logrus.Info("Starting forward port execution")
	err = fp.ExecuteForwardPort(CurrentChart)
	if err != nil {
		logrus.Fatalf("Failed to execute forward port: %v", err)
	}
}

func release(c *cli.Context) {
	if ForkURL == "" {
		logrus.Fatal("FORK environment variable must be set to run release cmd")
	}

	if CurrentChart == "" {
		logrus.Fatal("CHART environment variable must be set to run release cmd")
	}

	rootFs := filesystem.GetFilesystem(getRepoRoot())

	dependencies, err := lifecycle.InitDependencies(rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logrus.Fatalf("encountered error while initializing dependencies: %v", err)
	}

	status, err := lifecycle.LoadState(rootFs)
	if err != nil {
		logrus.Fatalf("could not load state; please run lifecycle-status before this command: %v", err)
	}

	release, err := auto.InitRelease(dependencies, status, ChartVersion, CurrentChart, ForkURL)
	if err != nil {
		logrus.Fatalf("failed to initialize release: %v", err)
	}

	if err := release.PullAsset(); err != nil {
		logrus.Fatalf("failed to execute release: %v", err)
	}

	// Unzip Assets: ASSET=<chart>/<chart>-<version.tgz make unzip
	CurrentAsset = release.Chart + "/" + release.AssetTgz
	unzipAssets(c)

	// update release.yaml
	if err := release.UpdateReleaseYaml(); err != nil {
		logrus.Fatalf("failed to update release.yaml: %v", err)
	}

	// make index
	createOrUpdateIndex(c)
}

func validateRelease(c *cli.Context) {
	if Skip {
		fmt.Println("skipping execution...")
		return
	}
	if GithubToken == "" {
		fmt.Println("GH_TOKEN environment variable must be set to run validate-release-charts")
		os.Exit(1)
	}
	if PullRequest == "" {
		fmt.Println("PR_NUMBER environment variable must be set to run validate-release-charts")
		os.Exit(1)
	}
	if Branch == "" {
		fmt.Println("BRANCH environment variable must be set to run validate-release-charts")
		os.Exit(1)
	}

	rootFs := filesystem.GetFilesystem(getRepoRoot())

	if !strings.HasPrefix(Branch, "release-v") {
		fmt.Println("Branch must be in the format release-v2.x")
		os.Exit(1)
	}

	dependencies, err := lifecycle.InitDependencies(rootFs, strings.TrimPrefix(Branch, "release-v"), "")
	if err != nil {
		fmt.Printf("encountered error while initializing d: %v \n", err)
		os.Exit(1)
	}

	if err := auto.ValidatePullRequest(GithubToken, PullRequest, dependencies); err != nil {
		fmt.Printf("failed to validate pull request: %v \n", err)
		os.Exit(1)
	}
}

func compareIndexFiles(c *cli.Context) {
	if Branch == "" {
		fmt.Println("BRANCH environment variable must be set to run validate-release-charts")
		os.Exit(1)
	}

	rootFs := filesystem.GetFilesystem(getRepoRoot())

	if err := auto.CompareIndexFiles(rootFs); err != nil {
		fmt.Printf("failed to compare index files: %v \n", err)
		os.Exit(1)
	}
	fmt.Println("index.yaml files are the same at git repository and charts.rancher.io")
}

func chartBump(c *cli.Context) {
	if CurrentPackage == "" {
		fmt.Println("CurrentPackage environment variable must be set")
		os.Exit(1)
	}
	if Branch == "" {
		fmt.Println("Branch environment variable must be set")
		os.Exit(1)
	}

	repoRoot := getRepoRoot()
	chartsScriptOptions := parseScriptOptions()

	bump, err := auto.SetupBump(repoRoot, CurrentPackage, Branch, chartsScriptOptions)
	if err != nil {
		fmt.Printf("failed to initialize the chart bump: %s", err.Error())
		bump.Pkg.Clean()
		os.Exit(1)
	}

	if err := bump.BumpChart(); err != nil {
		fmt.Printf("failed to bump the chart: %s", err.Error())
		bump.Pkg.Clean()
		os.Exit(1)
	}
}
