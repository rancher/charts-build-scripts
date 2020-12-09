package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/github"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

const (
	// DefaultChartsRepositoryConfigurationFile is the default path to look at for the configuration file
	DefaultChartsRepositoryConfigurationFile = "configuration.yaml"
	// DefaultChartEnvironmentVariable is the default environment variable to pull a specific chart from
	DefaultChartEnvironmentVariable = "CHART"
)

var (
	// Version represents the current version of the chart build scripts
	Version = "v0.0.0-dev"
	// GitCommit represents the latest commit when building this script
	GitCommit = "HEAD"

	// ChartsRepoConfigFile represents a file containing the configuration of the charts repository
	ChartsRepoConfigFile string
	// GithubToken represents the Github Auth token
	GithubToken string
	// CurrentChart represents the specific chart within packages/ in the source branch which is being used
	CurrentChart string
)

func main() {
	app := cli.NewApp()
	app.Name = "charts-build-scripts"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "Build scripts used to maintain patches on Helm charts forked from other repositories"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "repo-config,r",
			Usage:       "YAML configuration of the repository that will contain the forked Helm charts",
			TakesFile:   true,
			Destination: &ChartsRepoConfigFile,
			Value:       DefaultChartsRepositoryConfigurationFile,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:   "init",
			Usage:  "Initialize a repository that will contain the forked Helm charts using your Github credentials",
			Action: initializeChartsRepo,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "github-auth-token,g",
					Usage:       "Github Access Token that can be used to make requests to the Github API on your behalf",
					Required:    true,
					EnvVar:      "GITHUB_AUTH_TOKEN",
					Destination: &GithubToken,
				},
			},
		},
		{
			Name:   "prepare",
			Usage:  "Pull in the chart specified from upstream to the charts directory and apply any patch files",
			Action: prepareCharts,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "chart,c",
					Usage:       "A specific chart that you would like to run prepare on",
					Required:    false,
					Destination: &CurrentChart,
					EnvVar:      DefaultChartEnvironmentVariable,
				},
			},
		},
		{
			Name:   "patch",
			Usage:  "Apply a patch between the upstream chart and the current state of the chart in the charts directory",
			Action: generatePatch,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "chart,c",
					Usage:       "A specific chart that you would like to run prepare on",
					Required:    false,
					Destination: &CurrentChart,
					EnvVar:      DefaultChartEnvironmentVariable,
				},
			},
		},
		{
			Name:   "charts",
			Usage:  "Create a local chart archive of your finalized chart for testing",
			Action: generateCharts,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "chart,c",
					Usage:       "A specific chart that you would like to run prepare on",
					Required:    false,
					Destination: &CurrentChart,
					EnvVar:      DefaultChartEnvironmentVariable,
				},
			},
		},
		{
			Name:   "clean",
			Usage:  "Clean up your current repository to get it ready for a PR",
			Action: cleanRepository,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "chart,c",
					Usage:       "A specific chart that you would like to run prepare on",
					Required:    false,
					Destination: &CurrentChart,
					EnvVar:      DefaultChartEnvironmentVariable,
				},
			},
		},
		{
			Name:   "rebase",
			Usage:  "Provide a rebase.yaml to generate drift against your main chart",
			Action: rebaseChart,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "chart,c",
					Usage:       "A specific chart that you would like to run prepare on",
					Required:    false,
					Destination: &CurrentChart,
					EnvVar:      DefaultChartEnvironmentVariable,
				},
			},
		},
		{
			Name:   "sync",
			Usage:  "Pull in new generated assets from branches that the configuration.yaml has set your current branch to sync with",
			Action: synchronizeRepo,
		},
		{
			Name:   "validate",
			Usage:  "Ensure a sync will not overwrite generated assets in branches that the configuration.yaml wants you to validate against",
			Action: validateRepo,
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func initializeChartsRepo(c *cli.Context) {
	chartsRepo := parseConfig()
	ctx := context.Background()
	client := getGithubClient(ctx)

	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &repository.ChartsScriptsRepository); err != nil {
		logrus.Fatal(err)
	}
	if err = chartsRepo.Init(ctx, client); err != nil {
		logrus.Fatal(err)
	}
}

func prepareCharts(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart)
	if err != nil {
		logrus.Fatal(err)
	}
	if len(packages) == 0 {
		logrus.Fatalf("Could not find any packages in packages/")
	}
	for _, p := range packages {
		if err = p.Prepare(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func generatePatch(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart)
	if err != nil {
		logrus.Fatal(err)
	}
	if len(packages) == 0 {
		logrus.Fatalf("Could not find any packages in packages/")
	}
	for _, p := range packages {
		if err = p.GeneratePatch(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func generateCharts(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart)
	if err != nil {
		logrus.Fatal(err)
	}
	if len(packages) == 0 {
		logrus.Fatalf("Could not find any packages in packages/")
	}
	for _, p := range packages {
		if err = p.GenerateCharts(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func cleanRepository(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoPointingToBranch(repo, chartsRepo.BranchesConfiguration.Source.Name); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart)
	if err != nil {
		logrus.Fatal(err)
	}
	if len(packages) == 0 {
		logrus.Fatalf("Could not find any packages in packages/")
	}
	for _, p := range packages {
		if err = p.Clean(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func rebaseChart(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart)
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

func synchronizeRepo(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	// Get the branchConfig of the branch to sync with
	branch, err := utils.GetCurrentBranch(repo)
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
	if !status.IsClean() {
		logrus.Fatalf("Current repository is not clean:\n%s", status)
	}
	var syncOptions repository.SyncOptions
	switch branch {
	case chartsRepo.BranchesConfiguration.Staging.Name:
		syncOptions = chartsRepo.BranchesConfiguration.Staging.Options.SyncOptions
	case chartsRepo.BranchesConfiguration.Live.Name:
		syncOptions = chartsRepo.BranchesConfiguration.Live.Options.SyncOptions
	default:
		logrus.Fatalf("Current branch %s cannot be synced. Please switch to either %s (staging) or %s (live) to sync", branch, chartsRepo.BranchesConfiguration.Staging.Name, chartsRepo.BranchesConfiguration.Live.Name)
	}
	// Synchronize
	for _, compareGeneratedAssetsOptions := range syncOptions {
		logrus.Infof("Synchronizing with charts that will be generated from %s", compareGeneratedAssetsOptions.WithBranch)
		if err := charts.SynchronizeRepository(wt.Filesystem, chartsRepo.GithubConfiguration, compareGeneratedAssetsOptions); err != nil {
			logrus.Fatalf("Failed to synchronize with %s: %s", compareGeneratedAssetsOptions.WithBranch, err)
		}
		logrus.Infof("Successfully synchronized with %s!", compareGeneratedAssetsOptions.WithBranch)
	}
}

func validateRepo(c *cli.Context) {
	chartsRepo := parseConfig()
	path, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	repo, err := utils.GetRepo(path)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.GithubConfiguration); err != nil {
		logrus.Fatal(err)
	}
	// Get the branchConfig of the branch to sync with
	branch, err := utils.GetCurrentBranch(repo)
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
	if !status.IsClean() {
		logrus.Fatalf("Current repository is not clean:\n%s", status)
	}
	var validateOptions repository.ValidateOptions
	switch branch {
	case chartsRepo.BranchesConfiguration.Source.Name:
		validateOptions = chartsRepo.BranchesConfiguration.Source.Options.ValidateOptions
	case chartsRepo.BranchesConfiguration.Staging.Name:
		validateOptions = chartsRepo.BranchesConfiguration.Staging.Options.ValidateOptions
	default:
		logrus.Fatalf("Current branch %s cannot be validated. Please switch to either %s (source) or %s (staging) to sync", branch, chartsRepo.BranchesConfiguration.Source.Name, chartsRepo.BranchesConfiguration.Staging.Name)
	}
	// Synchronize
	for _, compareGeneratedAssetsOptions := range validateOptions {
		logrus.Infof("Validating against released charts in %s", compareGeneratedAssetsOptions.WithBranch)
		if err := charts.ValidateRepository(wt.Filesystem, chartsRepo.GithubConfiguration, compareGeneratedAssetsOptions); err != nil {
			logrus.Fatalf("Failed to validate against %s: %s", compareGeneratedAssetsOptions.WithBranch, err)
		}
		logrus.Infof("Successfully validated against %s!", compareGeneratedAssetsOptions.WithBranch)
	}
}

func parseConfig() *repository.ChartsRepositoryConfiguration {
	configYaml, err := ioutil.ReadFile(ChartsRepoConfigFile)
	if err != nil {
		logrus.Fatalf("Unable to find configuration file: %s", err)
	}
	chartsRepo := repository.ChartsRepositoryConfiguration{}
	if err := yaml.Unmarshal(configYaml, &chartsRepo); err != nil {
		logrus.Fatalf("Unable to unmarshall configuration file: %s", err)
	}
	return &chartsRepo
}

func validateRepoHasConfigAsRemote(repo *git.Repository, repoConfig *repository.GithubConfiguration) error {
	_, err := repoConfig.GetRemoteName(repo)
	if err == repository.ErrRemoteDoesNotExist {
		// TODO(aiyengar2): need to change to rancher
		err = fmt.Errorf("This command is only intended to be called from a repository pointing to %s", repository.ChartsScriptsRepository)
	}
	return err
}

func validateRepoPointingToBranch(repo *git.Repository, branch string) error {
	ref, err := repo.Head()
	if err != nil {
		return err
	}
	refName := ref.Name()
	if !refName.IsBranch() {
		return fmt.Errorf("Unable to find current branch")
	}
	currentBranch := refName.Short()
	if currentBranch != branch {
		return fmt.Errorf("Cannot execute command on current branch (%s). You must be in %s to run this command", currentBranch, branch)
	}
	return nil
}

// getGithubClient creates a client that can make request to the Github API
func getGithubClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: GithubToken,
	})
	return github.NewClient(oauth2.NewClient(ctx, ts))
}
