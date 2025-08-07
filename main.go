package main

import (
	"context"
	"errors"
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
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/registries"
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
	// defaultOverrideVersionEnvironmentVariable is the default environment variable that indicates the version to override
	defaultOverrideVersionEnvironmentVariable = "OVERRIDE_VERSION"
	// defaultMultiRCEnvironmentVariable is the default environment variable that indicates if the auto-bump should not remove previous RC versions
	defaultMultiRCEnvironmentVariable = "MULTI_RC"
	// Docker Registry authentication
	defaultDockerUserEnvironmentVariable     = "DOCKER_USER"
	defaultDockerPasswordEnvironmentVariable = "DOCKER_PASSWORD"
	// Prime Registry authentication
	defaultPrimeUserEnvironmentVariable     = "PRIME_USER"
	defaultPrimePasswordEnvironmentVariable = "PRIME_PASSWORD"
	defaultPrimeURLEnvironmentVariable      = "PRIME_URL"
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
	Skip = false
	// SoftErrorMode indicates if certain non-fatal errors will be turned into warnings
	SoftErrorMode = false
	// RepoRoot represents the root path of the repository
	RepoRoot string
	// OverrideVersion is the type of version override (patch, minor, auto)
	OverrideVersion string
	// MultiRC indicates if the auto-bump should not remove previous RC versions
	MultiRC bool
	// DockerUser is the username provided by EIO
	DockerUser string
	// DockerPassword is the password provided by EIO
	DockerPassword string
	// PrimeUser is the username provided by EIO
	PrimeUser string
	// PrimePassword is the password provided by EIO
	PrimePassword string
	// PrimeURL of SUSE Prime registry
	PrimeURL string
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
	newLogger := slog.New(tint.NewHandler(os.Stderr, tintOptions))
	slog.SetDefault(newLogger)
	logger.Log(context.Background(), slog.LevelInfo, "charts-build-scripts", slog.String("LOG", os.Getenv("LOG")))
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
	multiRCFlag := cli.BoolFlag{
		Name:        "multi-rc",
		Usage:       "default is false, if passed, auto-bump will not remove previous RC versions",
		Required:    false,
		EnvVar:      defaultMultiRCEnvironmentVariable,
		Destination: &MultiRC,
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
	dockerUserFlag := cli.StringFlag{
		Name:        "docker-user",
		Usage:       "--docker-user=******** || DOCKER_USER=*******",
		Required:    true,
		EnvVar:      defaultDockerUserEnvironmentVariable,
		Destination: &DockerUser,
	}
	dockerPasswordFlag := cli.StringFlag{
		Name:        "docker-password",
		Usage:       "--docker-password=******** || DOCKER_PASSWORD=*******",
		Required:    true,
		EnvVar:      defaultDockerPasswordEnvironmentVariable,
		Destination: &DockerPassword,
	}
	primeUserFlag := cli.StringFlag{
		Name:        "prime-user",
		Usage:       "--prime-user=******** || PRIME_USER=*******",
		Required:    true,
		EnvVar:      defaultPrimeUserEnvironmentVariable,
		Destination: &PrimeUser,
	}
	primePasswordFlag := cli.StringFlag{
		Name:        "prime-password",
		Usage:       "--prime-password=******** || PRIME_PASSWORD=*******",
		Required:    true,
		EnvVar:      defaultPrimePasswordEnvironmentVariable,
		Destination: &PrimePassword,
	}
	primeURLFlag := cli.StringFlag{
		Name:        "prime-url",
		Usage:       "--prime-url=******** || PRIME_URL=*******",
		Required:    true,
		EnvVar:      defaultPrimeURLEnvironmentVariable,
		Destination: &PrimeURL,
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
			Name:   "scan-registries",
			Usage:  "Fetch, list and compare SUSE's registries and create yaml files with what is supposed to be synced from Docker Hub",
			Action: scanRegistries,
			Flags:  []cli.Flag{primeURLFlag, dockerUserFlag, dockerPasswordFlag},
		},
		{
			Name:   "sync-registries",
			Usage:  "Fetch, list and compare SUSE's registries and create yaml files with what is supposed to be synced from Docker Hub",
			Action: syncRegistries,
			Flags:  []cli.Flag{dockerUserFlag, dockerPasswordFlag, primeUserFlag, primePasswordFlag, primeURLFlag},
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
			Flags:  []cli.Flag{packageFlag, configFlag, localModeFlag, remoteModeFlag, skipFlag},
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
			Name:   "chart-bump",
			Usage:  `Generate a new chart bump PR.`,
			Action: chartBump,
			Before: setupCache,
			Flags:  []cli.Flag{packageFlag, branchFlag, overrideVersionFlag, multiRCFlag},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal(context.Background(), err.Error())
	}
}

func listPackages(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	packageList, err := charts.ListPackages(ctx, RepoRoot, CurrentPackage)
	if err != nil {
		logger.Fatal(ctx, err.Error())
	}
	if PorcelainMode {
		logger.Log(ctx, slog.LevelInfo, "", slog.String("packageList", strings.Join(packageList, " ")))
		return

	}

	logger.Log(ctx, slog.LevelInfo, "", slog.Any("packageList", packageList))
}

func prepareCharts(c *cli.Context) {
	ctx := context.Background()

	util.SetSoftErrorMode(SoftErrorMode)
	packages := getPackages()
	if len(packages) == 0 {
		logger.Fatal(ctx, "could not find any packages in packages/ folder")
	}
	for _, p := range packages {
		if err := p.Prepare(ctx); err != nil {
			logger.Fatal(ctx, err.Error())
		}
	}
}

func generatePatch(c *cli.Context) {
	ctx := context.Background()

	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(ctx, slog.LevelInfo, "no packages found")
		return
	}
	if len(packages) != 1 {
		packageNames := make([]string, len(packages))
		for i, pkg := range packages {
			packageNames[i] = pkg.Name
		}
		logger.Fatal(ctx, fmt.Sprintf("PACKAGE=\"%s\"; is wrong, it must be set to point to one package", CurrentPackage))
	}
	if err := packages[0].GeneratePatch(ctx); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func generateCharts(c *cli.Context) {
	ctx := context.Background()

	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(ctx, slog.LevelInfo, "no packages found")
		return
	}

	chartsScriptOptions := parseScriptOptions(ctx)
	for _, p := range packages {
		if p.Auto == false {
			if err := p.GenerateCharts(ctx, chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
				logger.Fatal(ctx, err.Error())
			}
		}
	}
}

func downloadIcon(c *cli.Context) {
	ctx := context.Background()

	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(ctx, slog.LevelInfo, "no packages found")
		return
	}
	for _, p := range packages {
		err := p.DownloadIcon(ctx)
		if err != nil {
			logger.Fatal(ctx, err.Error())
		}
	}
}

func scanRegistries(c *cli.Context) {
	ctx := context.Background()

	if PrimeURL == "" {
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("URL Empty", true))
		logger.Fatal(ctx, errors.New("no Prime URL provided").Error())
	}

	if err := registries.Scan(ctx, PrimeURL+"/"); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func syncRegistries(c *cli.Context) {
	ctx := context.Background()

	emptyUser := PrimeUser == ""
	emptyPass := PrimePassword == ""
	emptyURL := PrimeURL == ""
	emptyDockerUser := DockerUser == ""
	emptyDockerPass := DockerPassword == ""
	if emptyUser || emptyPass || emptyURL || emptyDockerUser || emptyDockerPass {
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("User Empty", emptyUser))
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("Password Empty", emptyPass))
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("URL Empty", emptyURL))
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("Docker User Empty", emptyDockerUser))
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("Docker Pass Empty", emptyDockerPass))
		logger.Fatal(ctx, errors.New("no credentials provided for sync").Error())
	}

	if err := registries.Sync(ctx, PrimeUser, PrimePassword, PrimeURL, DockerUser, DockerPassword); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func createOrUpdateIndex(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	if err := helm.CreateOrUpdateHelmIndex(ctx, filesystem.GetFilesystem(RepoRoot)); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func zipCharts(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	if err := zip.ArchiveCharts(ctx, RepoRoot, CurrentChart); err != nil {
		logger.Fatal(ctx, err.Error())
	}
	createOrUpdateIndex(c)
}

func unzipAssets(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	if err := zip.DumpAssets(ctx, RepoRoot, CurrentAsset); err != nil {
		logger.Fatal(ctx, err.Error())
	}
	createOrUpdateIndex(c)
}

func cleanRepo(c *cli.Context) {
	ctx := context.Background()

	packages := getPackages()
	if len(packages) == 0 {
		logger.Log(ctx, slog.LevelInfo, "no packages found")
		return
	}

	for _, p := range packages {
		if err := p.Clean(ctx); err != nil {
			logger.Fatal(ctx, err.Error())
		}
	}
}

func validateRepo(c *cli.Context) {
	ctx := context.Background()
	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	if LocalMode && RemoteMode {
		logger.Fatal(ctx, "cannot specify both local and remote validation")
	}

	chartsScriptOptions := parseScriptOptions(ctx)

	logger.Log(ctx, slog.LevelInfo, "checking if Git is clean")
	_, _, status := getGitInfo()
	if !status.IsClean() {
		logger.Fatal(ctx, "repository must be clean to run validation")
	}

	// Only skip icon validations for forward-ports
	if !Skip {
		if err := auto.ValidateIcons(ctx, rootFs); err != nil {
			logger.Fatal(ctx, err.Error())
		}
	}

	if RemoteMode {
		logger.Log(ctx, slog.LevelInfo, "remove validation only")
	} else {
		logger.Log(ctx, slog.LevelInfo, "generating charts")
		generateCharts(c)

		logger.Log(ctx, slog.LevelInfo, "checking if Git is clean after generating charts")
		_, _, status = getGitInfo()
		if err := validate.StatusExceptions(ctx, status); err != nil {
			logger.Fatal(ctx, err.Error())
		}

		logger.Log(ctx, slog.LevelInfo, "successfully validated that current charts and assets are up-to-date")
	}

	if chartsScriptOptions.ValidateOptions != nil {
		if LocalMode {
			logger.Log(ctx, slog.LevelInfo, "local validation only")
		} else {
			getRepoRoot()
			repoFs := filesystem.GetFilesystem(RepoRoot)
			releaseOptions, err := options.LoadReleaseOptionsFromFile(ctx, repoFs, "release.yaml")
			if err != nil {
				logger.Fatal(ctx, fmt.Errorf("unable to unmarshall release.yaml: %w", err).Error())
			}
			u := chartsScriptOptions.ValidateOptions.UpstreamOptions
			branch := chartsScriptOptions.ValidateOptions.Branch

			logger.Log(ctx, slog.LevelInfo, "upstream validation against repository", slog.String("url", u.URL), slog.String("branch", branch))
			compareGeneratedAssetsResponse, err := validate.CompareGeneratedAssets(ctx, RepoRoot, repoFs, u, branch, releaseOptions)
			if err != nil {
				logger.Fatal(ctx, err.Error())
			}
			if !compareGeneratedAssetsResponse.PassedValidation() {
				// Output charts that have been modified
				compareGeneratedAssetsResponse.LogDiscrepancies(ctx)

				logger.Log(ctx, slog.LevelInfo, "dumping release.yaml to track changes that have been introduced")
				if err := compareGeneratedAssetsResponse.DumpReleaseYaml(ctx, repoFs); err != nil {
					logger.Log(ctx, slog.LevelError, "unable to dump newly generated release.yaml", logger.Err(err))
				}

				logger.Log(ctx, slog.LevelInfo, "updating index.yaml")
				if err := helm.CreateOrUpdateHelmIndex(ctx, repoFs); err != nil {
					logger.Fatal(ctx, err.Error())
				}

				logger.Fatal(ctx, fmt.Sprintf("validation against upstream repository %s at branch %s failed.", u.URL, branch))
			}
		}
	}

	logger.Log(ctx, slog.LevelInfo, "zipping charts to ensure that contents of assets, charts, and index.yaml are in sync")
	zipCharts(c)

	logger.Log(ctx, slog.LevelInfo, "final check if Git is clean")
	_, _, status = getGitInfo()
	if !status.IsClean() {
		logger.Fatal(ctx, fmt.Sprintf("repository must be clean to pass validation; status: %s", status.String()))
	}

	logger.Log(ctx, slog.LevelInfo, "make validate success")
}

func standardizeRepo(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	repoFs := filesystem.GetFilesystem(RepoRoot)
	if err := standardize.RestructureChartsAndAssets(ctx, repoFs); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func createOrUpdateTemplate(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	repoFs := filesystem.GetFilesystem(RepoRoot)
	chartsScriptOptions := parseScriptOptions(ctx)
	if err := update.ApplyUpstreamTemplate(ctx, repoFs, *chartsScriptOptions); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to update repository based on upstream template: %w", err).Error())
	}

	logger.Log(ctx, slog.LevelInfo, "successfully updated repository based on upstream template")
}

func setupCache(c *cli.Context) error {
	ctx := context.Background()

	getRepoRoot()
	return puller.InitRootCache(ctx, RepoRoot, CacheMode, path.DefaultCachePath)
}

func cleanCache(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	if err := puller.CleanRootCache(ctx, RepoRoot, path.DefaultCachePath); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func parseScriptOptions(ctx context.Context) *options.ChartsScriptOptions {
	configYaml, err := os.ReadFile(ChartsScriptOptionsFile)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("unable to find configuration file: %w", err).Error())
	}
	chartsScriptOptions := options.ChartsScriptOptions{}
	if err := yaml.UnmarshalStrict(configYaml, &chartsScriptOptions); err != nil {
		logger.Fatal(ctx, fmt.Errorf("unable to unmarshall configuration file: %w", err).Error())
	}

	if chartsScriptOptions.ValidateOptions != nil {
		logger.Log(ctx, slog.LevelInfo, "chart script options", slog.Group("opts",
			slog.Group("validate",
				slog.String("branch", chartsScriptOptions.ValidateOptions.Branch),
				slog.Group("upstream",
					slog.String("url", chartsScriptOptions.ValidateOptions.UpstreamOptions.URL),
					slog.Any("commit", chartsScriptOptions.ValidateOptions.UpstreamOptions.Commit),
					slog.Any("subdirectory", chartsScriptOptions.ValidateOptions.UpstreamOptions.Subdirectory),
				),
			),
			slog.Group("helmRepo",
				slog.String("CNAME", chartsScriptOptions.HelmRepoConfiguration.CNAME),
			),
			slog.String("template", chartsScriptOptions.Template),
			slog.Bool("omitBuildMetadata", chartsScriptOptions.OmitBuildMetadataOnExport),
		))
	}

	return &chartsScriptOptions
}

func getRepoRoot() {
	ctx := context.Background()

	RepoRoot = os.Getenv("DEV_REPO_ROOT")
	if RepoRoot != "" {
		logger.Log(ctx, slog.LevelDebug, "using customized repo root: ", slog.String("repoRoot", RepoRoot))
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("unable to get current working directory: %w", err).Error())
	}
	if repoRoot == "" {
		logger.Fatal(ctx, "unable to get current working directory")
	}

	logger.Log(ctx, slog.LevelDebug, "using current working directory as repo root: ", slog.String("repoRoot", repoRoot))
	RepoRoot = repoRoot
}

func getPackages() []*charts.Package {
	ctx := context.Background()

	getRepoRoot()
	packages, err := charts.GetPackages(ctx, RepoRoot, CurrentPackage)
	if err != nil {
		logger.Fatal(ctx, err.Error())
	}
	return packages
}

func getGitInfo() (*git.Repository, *git.Worktree, git.Status) {
	ctx := context.Background()

	getRepoRoot()
	repo, err := repository.GetRepo(RepoRoot)
	if err != nil {
		logger.Fatal(ctx, err.Error())
	}

	// Check if git is clean
	wt, err := repo.Worktree()
	if err != nil {
		logger.Fatal(ctx, err.Error())
	}

	status, err := wt.Status()
	if err != nil {
		logger.Fatal(ctx, err.Error())
	}

	return repo, wt, status
}

func checkImages(c *cli.Context) {
	ctx := context.Background()

	if err := registries.DockerScan(ctx); err != nil {
		logger.Fatal(ctx, err.Error())
	}
}

func checkRCTagsAndVersions(c *cli.Context) {
	ctx := context.Background()

	getRepoRoot()
	// Grab all images that contain RC tags
	rcImageTagMap := registries.DockerCheckRCTags(ctx, RepoRoot)

	// Grab all chart versions that contain RC tags
	rcChartVersionMap, err := charts.CheckRCCharts(ctx, RepoRoot)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("unable to check for RC charts: %w", err).Error())
	}

	// If there are any charts that contains RC version or images that contains RC tags
	// log them and return an error
	if len(rcChartVersionMap) > 0 || len(rcImageTagMap) > 0 {
		logger.Log(ctx, slog.LevelError, "found images with RC tags", slog.Any("rcImageTagMap", rcImageTagMap))
		logger.Log(ctx, slog.LevelError, "found charts with RC version", slog.Any("rcChartVersionMap", rcChartVersionMap))
		logger.Fatal(ctx, "RC check has failed")
	}

	logger.Log(ctx, slog.LevelInfo, "successfully checked RC tags and versions")
}

func lifecycleStatus(c *cli.Context) {
	ctx := context.Background()

	// Initialize dependencies with branch-version and current chart
	logger.Log(ctx, slog.LevelDebug, "initialize lifecycle-status")

	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)
	lifeCycleDep, err := lifecycle.InitDependencies(ctx, RepoRoot, rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	// Execute lifecycle status check and save the logs
	logger.Log(ctx, slog.LevelDebug, "checking lifecycle status and saving logs")
	_, err = lifeCycleDep.CheckLifecycleStatusAndSave(ctx, CurrentChart)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to check lifecycle status: %w", err).Error())
	}
}

func autoForwardPort(c *cli.Context) {
	ctx := context.Background()

	if ForkURL == "" {
		logger.Fatal(ctx, "FORK environment variable must be set to run auto-forward-port")
	}

	// Initialize dependencies with branch-version and current chart
	logger.Log(ctx, slog.LevelDebug, "initialize auto forward port")

	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	lifeCycleDep, err := lifecycle.InitDependencies(ctx, RepoRoot, rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	// Execute lifecycle status check and save the logs
	logger.Log(ctx, slog.LevelInfo, "checking lifecycle status and saving logs")
	status, err := lifeCycleDep.CheckLifecycleStatusAndSave(ctx, CurrentChart)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to check lifecycle status: %w", err).Error())
	}

	// Execute forward port with loaded information from status
	fp, err := auto.CreateForwardPortStructure(ctx, lifeCycleDep, status.AssetsToBeForwardPorted, ForkURL)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to prepare forward port: %w", err).Error())
	}

	err = fp.ExecuteForwardPort(ctx, CurrentChart)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to execute forward port: %w", err).Error())
	}
}

func release(c *cli.Context) {
	ctx := context.Background()

	if ForkURL == "" {
		logger.Fatal(ctx, "FORK environment variable must be set to run release cmd")
	}

	if CurrentChart == "" {
		logger.Fatal(ctx, "CHART environment variable must be set to run release cmd")
	}
	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	dependencies, err := lifecycle.InitDependencies(ctx, RepoRoot, rootFs, c.String("branch-version"), CurrentChart)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	status, err := lifecycle.LoadState(rootFs)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("could not load state; please run lifecycle-status before this command: %w", err).Error())
	}

	release, err := auto.InitRelease(ctx, dependencies, status, ChartVersion, CurrentChart, ForkURL)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to initialize release: %w", err).Error())
	}

	if err := release.PullAsset(); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to execute release: %w", err).Error())
	}

	// Unzip Assets: ASSET=<chart>/<chart>-<version.tgz make unzip
	CurrentAsset = release.Chart + "/" + release.AssetTgz
	unzipAssets(c)

	if err := release.PullIcon(ctx, rootFs); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to pull icon: %w", err).Error())
	}

	// update release.yaml
	if err := release.UpdateReleaseYaml(ctx, true); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to update release.yaml: %w", err).Error())
	}

	// make index
	createOrUpdateIndex(c)
}

func validateRelease(c *cli.Context) {
	ctx := context.Background()

	if Skip {
		logger.Log(ctx, slog.LevelInfo, "skipping release validation")
		return
	}
	if GithubToken == "" {
		logger.Fatal(ctx, "GH_TOKEN environment variable must be set to run validate-release-charts")
	}
	if PullRequest == "" {
		logger.Fatal(ctx, "PR_NUMBER environment variable must be set to run validate-release-charts")
	}
	if Branch == "" {
		logger.Fatal(ctx, "BRANCH environment variable must be set to run validate-release-charts")
	}
	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	if !strings.HasPrefix(Branch, "release-v") {
		logger.Fatal(ctx, "branch must be in the format release-v2.x")
	}

	dependencies, err := lifecycle.InitDependencies(ctx, RepoRoot, rootFs, strings.TrimPrefix(Branch, "release-v"), "")
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("encountered error while initializing dependencies: %w", err).Error())
	}

	if err := auto.ValidatePullRequest(ctx, GithubToken, PullRequest, dependencies); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to validate pull request: %w", err).Error())
	}
}

func compareIndexFiles(c *cli.Context) {
	ctx := context.Background()

	if Branch == "" {
		logger.Fatal(ctx, "BRANCH environment variable must be set to run compare-index-files")
	}

	getRepoRoot()
	rootFs := filesystem.GetFilesystem(RepoRoot)

	if err := auto.CompareIndexFiles(ctx, rootFs); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to compare index files: %w", err).Error())
	}

	logger.Log(ctx, slog.LevelInfo, "index.yaml files are the same at git repository and charts.rancher.io")
}

func chartBump(c *cli.Context) {
	ctx := context.Background()

	logger.Log(ctx, slog.LevelInfo, "received parameters")
	logger.Log(ctx, slog.LevelInfo, "", slog.String("package", CurrentPackage))
	logger.Log(ctx, slog.LevelInfo, "", slog.String("branch", Branch))
	logger.Log(ctx, slog.LevelInfo, "", slog.String("overrideVersion", OverrideVersion))
	logger.Log(ctx, slog.LevelInfo, "", slog.Bool("multi-RC", MultiRC))

	if CurrentPackage == "" || Branch == "" || OverrideVersion == "" {
		logger.Fatal(ctx, fmt.Sprintf("must provide values for CurrentPackage[%s], Branch[%s], and OverrideVersion[%s]",
			CurrentPackage, Branch, OverrideVersion))
	}

	if OverrideVersion != "patch" && OverrideVersion != "minor" && OverrideVersion != "auto" {
		logger.Fatal(ctx, "OverrideVersion must be set to either patch, minor, or auto")
	}

	ChartsScriptOptionsFile = path.ConfigurationYamlFile
	chartsScriptOptions := parseScriptOptions(ctx)

	bump, err := auto.SetupBump(ctx, RepoRoot, CurrentPackage, Branch, chartsScriptOptions)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to setup: %w", err).Error())
	}

	if err := bump.BumpChart(ctx, OverrideVersion, MultiRC); err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to bump: %w", err).Error())
	}
}
