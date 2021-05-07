package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/update"
	"github.com/rancher/charts-build-scripts/pkg/validate"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	// DefaultChartsScriptOptionsFile is the default path to look a file containing options for the charts scripts to use for this branch
	DefaultChartsScriptOptionsFile = "configuration.yaml"
	// DefaultPackageEnvironmentVariable is the default environment variable for picking a specific package
	DefaultPackageEnvironmentVariable = "PACKAGE"
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
	// CurrentPackage represents the specific chart within packages/ in the Staging branch which is being used
	CurrentPackage string
)

func main() {
	app := cli.NewApp()
	app.Name = "charts-build-scripts"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Build scripts used to maintain patches on Helm charts forked from other repositories"
	configFlag := cli.StringFlag{
		Name:        "config,c",
		Usage:       "A configuration file with additional options for allowing this branch to interact with other branches",
		TakesFile:   true,
		Destination: &ChartsScriptOptionsFile,
		Value:       DefaultChartsScriptOptionsFile,
	}
	packageFlag := cli.StringFlag{
		Name:        "package,p",
		Usage:       "A package you would like to run prepare on",
		Required:    false,
		Destination: &CurrentPackage,
		EnvVar:      DefaultPackageEnvironmentVariable,
	}
	app.Commands = []cli.Command{
		{
			Name:   "prepare",
			Usage:  "Pull in the chart specified from upstream to the charts directory and apply any patch files",
			Action: prepareCharts,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "patch",
			Usage:  "Apply a patch between the upstream chart and the current state of the chart in the charts directory",
			Action: generatePatch,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "charts",
			Usage:  "Create a local chart archive of your finalized chart for testing",
			Action: generateCharts,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "clean",
			Usage:  "Clean up your current repository to get it ready for a PR",
			Action: cleanRepository,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "validate",
			Usage:  "Run validation to ensure that contents of assets and charts won't overwrite released charts",
			Action: validateRepo,
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

func prepareCharts(c *cli.Context) {
	packages := getPackages()
	for _, p := range packages {
		if err := p.Prepare(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func generatePatch(c *cli.Context) {
	packages := getPackages()
	for _, p := range packages {
		if err := p.GeneratePatch(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func generateCharts(c *cli.Context) {
	packages := getPackages()
	for _, p := range packages {
		if err := p.GenerateCharts(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func cleanRepository(c *cli.Context) {
	packages := getPackages()
	for _, p := range packages {
		if err := p.Clean(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func validateRepo(c *cli.Context) {
	_, status := getGitInfo()
	if !status.IsClean() {
		logrus.Fatalf("Repository must be clean to run validation:\n%s", status)
	}

	logrus.Infof("Generating charts and checking if Git is clean")
	CurrentPackage = "" // Validate always runs on all packages
	generateCharts(c)
	wt, status := getGitInfo()
	if !status.IsClean() {
		logrus.Warnf("Generated charts produced the following changes in Git.\n%s", status)
		logrus.Fatalf("Please commit these changes and run validation again.")
	}
	logrus.Infof("Successfully validated that current charts and assets are up to date.")

	chartsScriptOptions := parseScriptOptions()
	for _, compareGeneratedAssetsOptions := range chartsScriptOptions.ValidateOptions {
		logrus.Infof("Validating against released charts in %s", compareGeneratedAssetsOptions.Branch)
		notSubset, err := validate.CompareGeneratedAssets(wt, compareGeneratedAssetsOptions)
		if err != nil {
			logrus.Fatalf("Failed to validate against %s: %s", compareGeneratedAssetsOptions.Branch, err)
		}
		_, status := getGitInfo()
		if notSubset {
			logrus.Warnf("The following charts and assets exist in %s but do not exist in your current branch.\n%s", compareGeneratedAssetsOptions.Branch, status)
			logrus.Fatalf("Please commit these changes and run validation again.")
		}
		logrus.Infof("Successfully validated against %s!", compareGeneratedAssetsOptions.Branch)
	}
}

func createOrUpdateTemplate(c *cli.Context) {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repoFs := filesystem.GetFilesystem(repoRoot)
	chartsScriptOptions := parseScriptOptions()
	if err := update.ApplyUpstreamTemplate(repoFs, *chartsScriptOptions); err != nil {
		logrus.Fatalf("Failed to update repository based on upstream template: %s", err)
	}
	logrus.Infof("Successfully updated repository based on upstream template.")
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

func getPackages() []*charts.Package {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	packages, err := charts.GetPackages(repoRoot, CurrentPackage)
	if err != nil {
		logrus.Fatal(err)
	}
	if len(packages) == 0 {
		logrus.Fatalf("Could not find any packages in packages/")
	}
	return packages
}

func getGitInfo() (*git.Worktree, git.Status) {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
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
	return wt, status
}
