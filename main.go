package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/rancher/charts-build-scripts/pkg/actions"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/update"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
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
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func listPackages(c *cli.Context) error {
	return actions.List(CurrentPackage, PorcelainMode)
}

func prepareCharts(c *cli.Context) error {
	return actions.Prepare(CurrentPackage)
}

func generatePatch(c *cli.Context) error {
	return actions.Patch(CurrentPackage)
}

func generateCharts(c *cli.Context) error {
	actions.ChartsScriptOptionsFile = ChartsScriptOptionsFile
	return actions.Charts(CurrentPackage)
}

func createOrUpdateIndex(c *cli.Context) error {
	return actions.Index()
}

func zipCharts(c *cli.Context) error {
	return actions.Zip(CurrentChart)
}

func unzipAssets(c *cli.Context) error {
	return actions.Unzip(CurrentAsset)
}

func cleanRepo(c *cli.Context) error {
	return actions.Clean(CurrentPackage)
}

func validateRepo(c *cli.Context) error {
	actions.ChartsScriptOptionsFile = ChartsScriptOptionsFile
	if LocalMode && RemoteMode {
		return errors.New("cannot specify both local and remote validation")
	}

	if LocalMode {
		return actions.ValidateLocal()
	}

	if RemoteMode {
		return actions.ValidateRemote()
	}

	return actions.Validate()
}

func standardizeRepo(c *cli.Context) error {
	return actions.Standardize()
}

func createOrUpdateTemplate(c *cli.Context) error {
	actions.ChartsScriptOptionsFile = ChartsScriptOptionsFile
	return actions.Template()
}

func setupCache(c *cli.Context) error {
	return puller.InitRootCache(CacheMode, path.DefaultCachePath)
}

func cleanCache(c *cli.Context) {
	if err := puller.CleanRootCache(path.DefaultCachePath); err != nil {
		logrus.Fatal(err)
	}
}
