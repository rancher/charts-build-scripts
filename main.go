package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
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
	// DefaultPorcelainModeVariable is the default environment variable that indicates whether we should run on porcelain mode
	DefaultPorcelainEnvironmentVariable = "PORCELAIN"
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
	// PorcelainMode indicates that the output of the scripts should be in an easy-to-parse format for scripts
	PorcelainMode bool
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
	porcelainFlag := cli.BoolFlag{
		Name:        "porcelain",
		Usage:       "Print the output of the command in a easy-to-parse format for scripts",
		Required:    false,
		Destination: &PorcelainMode,
		EnvVar:      DefaultPorcelainEnvironmentVariable,
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
			Flags:  []cli.Flag{packageFlag, configFlag},
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

func listPackages(c *cli.Context) {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
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
	for _, p := range packages {
		if err := p.Prepare(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func generatePatch(c *cli.Context) {
	packages := getPackages()
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
	repo, _, status := getGitInfo()
	if !status.IsClean() {
		logrus.Warnf("Git is not clean:\n%s", status)
		logrus.Fatal("Repository must be clean to generate charts")
	}
	currentBranchRefName, err := repository.GetCurrentBranchRefName(repo)
	if err != nil {
		logrus.Warn("Due to limitations in the Git library used for the scripts, we cannot generate charts in a detached HEAD state.")
		logrus.Fatalf("Could not get reference to current branch: %s", err)
	}
	// Generate charts
	packages := getPackages()
	chartsScriptOptions := parseScriptOptions()
	for _, p := range packages {
		if err := p.GenerateCharts(chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
			logrus.Fatal(err)
		}
	}
	// Copy in only assets from charts that have changed
	_, wt, status := getGitInfo()
	modifiedAssets := make(map[string]bool)
	for p := range status {
		if p == path.RepositoryHelmIndexFile {
			wt.Excludes = append(wt.Excludes, gitignore.ParsePattern(p, []string{}))
			logrus.Infof("Outputted %s", p)
			continue
		}
		if !strings.HasPrefix(p, path.RepositoryChartsDir) {
			continue
		}
		var modifiedAssetPath string
		splitPath := strings.Split(p, "/")
		switch len(splitPath) {
		case 1:
			// charts were generated for the first time
			modifiedAssetPath = fmt.Sprintf("%s/*", path.RepositoryAssetsDir)
		case 2:
			// New package was introduced
			modifiedAssetPath = fmt.Sprintf("%s/*", filepath.Join(path.RepositoryAssetsDir, splitPath[1]))
		case 3:
			// New chart was introduced
			modifiedAssetPath = fmt.Sprintf("%s/*", filepath.Join(path.RepositoryAssetsDir, splitPath[1], splitPath[2]))
		default:
			// Existing chart was modified
			modifiedAssetPath = fmt.Sprintf("%s-%s.tgz", filepath.Join(path.RepositoryAssetsDir, splitPath[1], splitPath[2]), splitPath[3])
		}
		// Add chart
		wt.Excludes = append(wt.Excludes, gitignore.ParsePattern(p, []string{}))
		logrus.Infof("Outputted %s", p)
		// Add asset, but only once
		if added, ok := modifiedAssets[modifiedAssetPath]; ok && added {
			continue
		}
		modifiedAssets[modifiedAssetPath] = true
		wt.Excludes = append(wt.Excludes, gitignore.ParsePattern(modifiedAssetPath, []string{}))
		logrus.Infof("Outputted %s", modifiedAssetPath)
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: currentBranchRefName,
		Force:  true,
	})
	if err != nil {
		logrus.Fatalf("Failed to clean up unchanged assets: %s", err)
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
	_, _, status := getGitInfo()
	if !status.IsClean() {
		logrus.Warnf("Git is not clean:\n%s", status)
		logrus.Fatal("Repository must be clean to run validation")
	}

	logrus.Infof("Generating charts and checking if Git is clean")
	CurrentPackage = "" // Validate always runs on all packages
	generateCharts(c)
	_, wt, status := getGitInfo()
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
		_, _, status := getGitInfo()
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
		logrus.Fatal("Could not find any packages in packages/")
	}
	return packages
}

func getGitInfo() (*git.Repository, *git.Worktree, git.Status) {
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
	return repo, wt, status
}
