package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/github"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

var (
	// Version represents the current version of the chart build scripts
	Version = "v0.0.0-dev"
	// GitCommit represents the latest commit when building this script
	GitCommit = "HEAD"

	// SourceBranchOptions represents the default Options that should be set when viewing packages from the source branch
	SourceBranchOptions = options.BranchOptions{
		ExportOptions: options.ExportOptions{
			PreventOverwrite: false,
		},
		CleanOptions: options.CleanOptions{
			PreventCleanAssets: false,
		},
	}

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
			Required:    true,
			TakesFile:   true,
			Destination: &ChartsRepoConfigFile,
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
				},
			},
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
	if err = validateRepoHasConfigAsRemote(repo, &config.ChartsScriptsRepository); err != nil {
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
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.RepositoryConfiguration); err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoPointingToBranch(repo, chartsRepo.BranchConfiguration.Source); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart, SourceBranchOptions)
	if err != nil {
		logrus.Fatal(err)
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
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.RepositoryConfiguration); err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoPointingToBranch(repo, chartsRepo.BranchConfiguration.Source); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart, SourceBranchOptions)
	if err != nil {
		logrus.Fatal(err)
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
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.RepositoryConfiguration); err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoPointingToBranch(repo, chartsRepo.BranchConfiguration.Source); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart, SourceBranchOptions)
	if err != nil {
		logrus.Fatal(err)
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
	if err = validateRepoHasConfigAsRemote(repo, &chartsRepo.RepositoryConfiguration); err != nil {
		logrus.Fatal(err)
	}
	if err = validateRepoPointingToBranch(repo, chartsRepo.BranchConfiguration.Source); err != nil {
		logrus.Fatal(err)
	}
	packages, err := charts.GetPackages(path, CurrentChart, SourceBranchOptions)
	if err != nil {
		logrus.Fatal(err)
	}
	for _, p := range packages {
		if err = p.Clean(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func parseConfig() *config.ChartsRepositoryConfiguration {
	configYaml, err := ioutil.ReadFile(ChartsRepoConfigFile)
	if err != nil {
		logrus.Fatalf("Unable to find configuration file: %s", err)
	}
	chartsRepo := config.ChartsRepositoryConfiguration{}
	if err := yaml.Unmarshal(configYaml, &chartsRepo); err != nil {
		logrus.Fatalf("Unable to unmarshall configuration file: %s", err)
	}
	return &chartsRepo
}

func validateRepoHasConfigAsRemote(repo *git.Repository, repoConfig *config.RepositoryConfiguration) error {
	_, err := repoConfig.GetRemoteName(repo)
	if err == config.ErrRemoteDoesNotExist {
		// TODO(aiyengar2): need to change to rancher
		err = fmt.Errorf("This command is only intended to be called from a repository pointing to %s", config.ChartsScriptsRepository)
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
		return fmt.Errorf("Cannot execute command on current branch (%s). You must be in the source branch (%s) to run this command", currentBranch, branch)
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
