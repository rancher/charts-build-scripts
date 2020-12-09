package repository

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/github"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ChartsRepositoryConfiguration represents the configuration of a repository that contains forked Helm charts
type ChartsRepositoryConfiguration struct {
	// Repository configuration represents the configuration of the charts repository
	GithubConfiguration `yaml:"repository"`
	// BranchesConfiguration represents any special roles that certain branches hold
	BranchesConfiguration `yaml:",inline"`
	// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
	HelmRepoConfiguration `yaml:"helmRepo"`
}

func (c ChartsRepositoryConfiguration) String() string {
	return fmt.Sprintf("%s[%s]", c.GithubConfiguration, c.BranchesConfiguration)
}

// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
type HelmRepoConfiguration struct {
	URL string `yaml:"url"`
}

// Init checks whether a charts repository exists and is valid or creates it with a local copy
func (c ChartsRepositoryConfiguration) Init(ctx context.Context, client *github.Client) error {
	logrus.Infof("Checking if the repository exists: %s", c.GithubConfiguration)
	exists, err := c.GithubConfiguration.Exists(ctx, client)
	if err != nil {
		return err
	}
	if !exists {
		logrus.Infof("Repository does not exist on Github. Initializing the repository using Github credentials")
		if err = c.GithubConfiguration.Create(ctx, client); err != nil {
			return err
		}
		// Make changes to a local copy in a temporary directory to initialize the branches
		tempDir, err := ioutil.TempDir("", c.GithubConfiguration.Name)
		defer os.RemoveAll(tempDir)
		if err != nil {
			return err
		}
		repo, err := utils.CreateRepo(tempDir)
		if err != nil {
			return err
		}
		// Create an initial commit, add the remote, and make the first push
		if err = utils.CreateInitialCommit(repo); err != nil {
			return err
		}
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "upstream",
			URLs: []string{c.GithubConfiguration.GetHTTPSURL(), c.GithubConfiguration.GetSSHURL()},
		})
		if err != nil {
			return err
		}
		err = repo.Push(&git.PushOptions{
			RemoteName: "upstream",
		})
		if err != nil {
			return err
		}
		// Initialize the remaining branches
		if err = c.initBranches(repo); err != nil {
			return err
		}
		logrus.Infof("Successfully initialized repository based on configuration provided: %s", c)
		return nil
	}
	logrus.Infof("Repository already exists on Github.")
	return nil
}

// initBranches initializes a local repository with the expected branch configuration
func (c ChartsRepositoryConfiguration) initBranches(repo *git.Repository) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	hash, err := utils.GetHead(repo)
	if err != nil {
		return err
	}

	logrus.Infof("Initializing the source branch %s", c.Source)
	if err = c.prepareBranch(repo, hash, c.Source.Name, path.Join(cwd, "branch-templates/source")); err != nil {
		return err
	}

	logrus.Infof("Initializing the staging branch %s", c.Staging)
	if err = c.prepareBranch(repo, hash, c.Staging.Name, path.Join(cwd, "branch-templates/staging")); err != nil {
		return err
	}

	logrus.Infof("Initializing the live branch %s", c.Live)
	if err = c.prepareBranch(repo, hash, c.Live.Name, path.Join(cwd, "branch-templates/live")); err != nil {
		return err
	}
	return nil
}

// prepareBranch extracts the repo from repoPath, creates the branch from the parentHash, checks it out, applies the template found in templateDir, and commits the changes
func (c ChartsRepositoryConfiguration) prepareBranch(repo *git.Repository, parentHash plumbing.Hash, branchName, templateDir string) error {
	var err error
	if err = utils.CreateBranch(repo, branchName, parentHash); err != nil {
		return err
	}
	if err = utils.CheckoutBranch(repo, branchName); err != nil {
		return err
	}
	if err = c.applyTemplate(repo, templateDir); err != nil {
		return err
	}
	commitMessage := fmt.Sprintf("Initialize %s", branchName)
	if err = utils.CommitAll(repo, commitMessage); err != nil {
		return err
	}
	return repo.Push(&git.PushOptions{RemoteName: "upstream"})
}

// applyTemplate applies the configuration to a template located in templateDir and adds it to the repo
func (c ChartsRepositoryConfiguration) applyTemplate(repo *git.Repository, templateDir string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	applyTemplate := func(fs billy.Filesystem, path string, isDir bool) error {
		if isDir {
			logrus.Infof("Creating directory at %s/", path)
			return fs.MkdirAll(path, os.ModePerm)
		}
		fileName := filepath.Base(path)
		absPath := utils.GetAbsPath(fs, path)
		// Parse template
		t := template.Must(template.New(fileName).ParseFiles(absPath))
		// Create file
		logrus.Infof("Creating file at %s", path)
		f, err := utils.CreateFileAndDirs(fs, path)
		if err != nil {
			return err
		}
		defer f.Close()
		return t.Execute(f, c)
	}
	return utils.WalkDir(wt.Filesystem, templateDir, applyTemplate)
}
