package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/sync"
	"github.com/rancher/charts-build-scripts/pkg/update"
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
	// CurrentPackage represents the specific chart within packages/ in the source branch which is being used
	CurrentPackage string
)

func main() {
	app := cli.NewApp()
	app.Name = "charts-build-scripts"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Build scripts used to maintain patches on Helm charts forked from other repositories"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config,c",
			Usage:       "A configuration file with additional options for allowing this branch to interact with other branches",
			TakesFile:   true,
			Destination: &ChartsScriptOptionsFile,
			Value:       DefaultChartsScriptOptionsFile,
		},
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
			Name:   "rebase",
			Usage:  "Provide a rebase.yaml to generate drift against your main chart",
			Action: rebaseChart,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "validate",
			Usage:  "Ensure a sync will not overwrite generated assets in branches that the configuration.yaml wants you to validate against",
			Action: validateRepo,
			Flags:  []cli.Flag{packageFlag},
		},
		{
			Name:   "sync",
			Usage:  "Pull in new generated assets from branches that the configuration.yaml has set your current branch to sync with",
			Action: synchronizeRepo,
		},
		{
			Usage:  "Pulls in the latest docs to this repository",
			Name:   "docs",
			Action: getDocs,
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

func rebaseChart(c *cli.Context) {
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
	if len(packages) > 1 {
		logrus.Fatalf("Can only run rebase on exactly one package")
	}
	p := packages[0]
	if err = p.GenerateRebasePatch(); err != nil {
		logrus.Fatal(err)
	}
}

func validateRepo(c *cli.Context) {
	wt, status := getGitInfo()
	if !status.IsClean() {
		logrus.Fatalf("Current repository is not clean:\n%s", status)
	}
	chartsScriptOptions := parseScriptOptions()
	// Validate
	for _, compareGeneratedAssetsOptions := range chartsScriptOptions.ValidateOptions {
		logrus.Infof("Validating against released charts in %s", compareGeneratedAssetsOptions.Branch)
		if err := sync.ValidateRepository(wt.Filesystem, compareGeneratedAssetsOptions, CurrentPackage); err != nil {
			logrus.Fatalf("Failed to validate against %s: %s", compareGeneratedAssetsOptions.Branch, err)
		}
		logrus.Infof("Successfully validated against %s!", compareGeneratedAssetsOptions.Branch)
	}
}

func synchronizeRepo(c *cli.Context) {
	wt, status := getGitInfo()
	if !status.IsClean() {
		logrus.Fatalf("Current repository is not clean:\n%s", status)
	}
	chartsScriptOptions := parseScriptOptions()
	// Synchronize
	for _, compareGeneratedAssetsOptions := range chartsScriptOptions.SyncOptions {
		logrus.Infof("Synchronizing with charts that will be generated from %s", compareGeneratedAssetsOptions.Branch)
		if err := sync.SynchronizeRepository(wt.Filesystem, compareGeneratedAssetsOptions); err != nil {
			logrus.Fatalf("Failed to synchronize with %s: %s", compareGeneratedAssetsOptions.Branch, err)
		}
		logrus.Infof("Successfully synchronized with %s!", compareGeneratedAssetsOptions.Branch)
	}
	logrus.Infof("Creating or updating the Helm index with the newly added assets...")
	// Delete the Helm index if it was the only thing updated, whether or not changes failed
	wt, status = getGitInfo()
	chartsIntroduced := false
	for p, fileStatus := range status {
		if p == path.RepositoryHelmIndexFile {
			continue
		}
		if fileStatus.Worktree == git.Untracked && fileStatus.Staging == git.Untracked {
			// Some charts were added
			if err := helm.CreateOrUpdateHelmIndex(wt.Filesystem); err != nil {
				logrus.Fatalf("Sync was successful but was unable to update the Helm index: %s", err)
			}
			chartsIntroduced = true
			break
		}
	}
	if chartsIntroduced {
		logrus.Infof("Your working directory is ready for a commit.")
	} else {
		logrus.Infof("Nothing to sync. Working directory is up to date.")
	}
}

func getDocs(c *cli.Context) {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repoFs := filesystem.GetFilesystem(repoRoot)
	chartsScriptOptions := parseScriptOptions()
	if err := update.GetDocumentation(repoFs, *chartsScriptOptions); err != nil {
		logrus.Fatalf("Failed to update docs: %s", err)
	}
	logrus.Infof("Successfully pulled new updated docs into working directory.")
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
