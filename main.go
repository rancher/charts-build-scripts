package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/images"
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
	// DefaultChartsScriptOptionsFile is the default path to look a file containing options for the charts scripts to use for this branch
	DefaultChartsScriptOptionsFile = "configuration.yaml"
	// DefaultPackageEnvironmentVariable is the default environment variable for picking a specific package
	DefaultPackageEnvironmentVariable = "PACKAGE"
	// DefaultChartEnvironmentVariable is the default environment variable for picking a specific chart
	DefaultChartEnvironmentVariable = "CHART"
	// DefaultAssetEnvironmentVariable is the default environment variable for picking a specific asset
	DefaultAssetEnvironmentVariable = "ASSET"
	// DefaultPorcelainEnvironmentVariable is the default environment variable that indicates whether we should run on porcelain mode
	DefaultPorcelainEnvironmentVariable = "PORCELAIN"
	// DefaultCacheEnvironmentVariable is the default environment variable that indicates that a cache should be used on pulls to remotes
	DefaultCacheEnvironmentVariable = "USE_CACHE"
)

var (
	// Version represents the current version of the chart build scripts
	Version = "v0.0.0-dev"
	// GitCommit represents the latest commit when building this script
	GitCommit = "HEAD"

	// ChartsScriptOptionsFile represents a name of a file that contains options for the charts script to use for this branch
	ChartsScriptOptionsFile string
	// GithubToken represents the Github Auth token; currently not used
	GithubToken string
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
)

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		logrus.SetLevel(logrus.DebugLevel)
	}
	app := cli.NewApp()
	app.Name = "charts-build-scripts"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Build scripts used to maintain patches on Helm charts forked from other repositories"
	configFlag := cli.StringFlag{
		Name:        "config",
		Usage:       "A configuration file with additional options for allowing this branch to interact with other branches",
		TakesFile:   true,
		Destination: &ChartsScriptOptionsFile,
		Value:       DefaultChartsScriptOptionsFile,
	}
	packageFlag := cli.StringFlag{
		Name:        "package,p",
		Usage:       "A package you would like to run the scripts on",
		Required:    false,
		Destination: &CurrentPackage,
		EnvVar:      DefaultPackageEnvironmentVariable,
	}
	chartFlag := cli.StringFlag{
		Name:        "chart,c",
		Usage:       "A chart you would like to run the scripts on. Can include version.",
		Required:    false,
		Destination: &CurrentChart,
		EnvVar:      DefaultChartEnvironmentVariable,
	}
	assetFlag := cli.StringFlag{
		Name:        "asset,a",
		Usage:       "An asset you would like to run the scripts on. Can directly point to archive.",
		Required:    false,
		Destination: &CurrentAsset,
		EnvVar:      DefaultAssetEnvironmentVariable,
	}
	porcelainFlag := cli.BoolFlag{
		Name:        "porcelain",
		Usage:       "Print the output of the command in a easy-to-parse format for scripts",
		Required:    false,
		Destination: &PorcelainMode,
		EnvVar:      DefaultPorcelainEnvironmentVariable,
	}
	cacheFlag := cli.BoolFlag{
		Name:        "useCache",
		Usage:       "Experimental: use a cache to speed up scripts",
		Required:    false,
		Destination: &CacheMode,
		EnvVar:      DefaultCacheEnvironmentVariable,
	}
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
			Flags:  []cli.Flag{packageFlag, cacheFlag},
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
			Flags: []cli.Flag{packageFlag, configFlag, cli.BoolFlag{
				Name:        "local,l",
				Usage:       "Only perform local validation of the contents of assets and charts",
				Required:    false,
				Destination: &LocalMode,
			}, cli.BoolFlag{
				Name:        "remote,r",
				Usage:       "Only perform upstream validation of the contents of assets and charts",
				Required:    false,
				Destination: &RemoteMode,
			},
			},
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
		if err := p.GenerateCharts(chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
			logrus.Fatal(err)
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

	CurrentPackage = "" // Validate always runs on all packages
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
	configYaml, err := ioutil.ReadFile(ChartsScriptOptionsFile)
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
