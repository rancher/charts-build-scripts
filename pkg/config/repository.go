package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

const (
	httpsURLFmt = "https://github.com/%s/%s.git"
	sshURLFmt   = "git@github.com:%s/%s.git"
)

var (
	// ErrRemoteDoesNotExist indicates that the remote does not exist in the current repository
	ErrRemoteDoesNotExist = errors.New("Repository does not have any matching remotes")
	// ChartsScriptsRepository represents the configuration of the repository containing these scripts
	ChartsScriptsRepository = RepositoryConfiguration{
		Owner: "aiyengar2", // TODO(aiyengar2): need to change to rancher
		Name:  "charts-build-scripts",
	}
)

// GetRepositoryConfiguration parses the provided URL and returns the Repository Configuration if possible
func GetRepositoryConfiguration(url string) (RepositoryConfiguration, error) {
	if !strings.HasSuffix(url, ".git") {
		return RepositoryConfiguration{}, fmt.Errorf("URL does not seem to point to a Git repository: %s", url)
	}
	splitURL := strings.Split(strings.TrimSuffix(url, ".git"), "/")
	if len(splitURL) < 2 {
		return RepositoryConfiguration{}, fmt.Errorf("URL does not seem to be valid for a Git repository: %s", url)
	}
	return RepositoryConfiguration{
		Owner: splitURL[len(splitURL)-2],
		Name:  splitURL[len(splitURL)-1],
	}, nil
}

// RepositoryConfiguration represents the configuration of a specific repository
type RepositoryConfiguration struct {
	// Owner represents the account that owns the repo, e.g. rancher
	Owner string `yaml:"owner"`
	// Name represents the name of the repo, e.g. charts
	Name string `yaml:"name"`
}

// Create pushes a new Repository onto Github
func (r RepositoryConfiguration) Create(ctx context.Context, client *github.Client) error {
	// Create the repository
	logrus.Infof("Creating repository from configuration: %s", r)
	user, resp, err := client.Users.Get(ctx, r.Owner)
	if resp.StatusCode == 404 {
		return fmt.Errorf("User %s cannot be found on Github", r.Owner)
	}
	if err != nil {
		return err
	}
	_, _, err = client.Repositories.Create(ctx, "", &github.Repository{
		Owner: user,
		Name:  github.String(r.Name),
	})
	if err != nil {
		log.Fatalf("Unable to initialize a Github Repository for your charts: %s", err)
	}
	logrus.Infof("Repository %s has been created on Github", r)
	return nil
}

// Exists checks to see if the repository specified exists on Github
func (r RepositoryConfiguration) Exists(ctx context.Context, client *github.Client) (bool, error) {
	_, resp, err := client.Repositories.Get(ctx, r.Owner, r.Name)
	if resp.StatusCode == 200 {
		return true, nil
	}
	if resp.StatusCode == 404 {
		return false, nil
	}
	return false, err
}

// GetRemoteName gets name of the remote within the repo pointing to the RepositoryConfiguration
func (r RepositoryConfiguration) GetRemoteName(repo *git.Repository) (string, error) {
	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("Unable to list remotes: %s", err)
	}
	sshURL := r.GetSSHURL()
	httpsURL := r.GetHTTPSURL()
	for _, remote := range remotes {
		for _, url := range remote.Config().URLs {
			if url == sshURL || url == httpsURL {
				return remote.Config().Name, nil
			}
		}
	}
	return "", ErrRemoteDoesNotExist
}

// GetHTTPSURL returns the HTTPS URL of the repository
func (r RepositoryConfiguration) GetHTTPSURL() string {
	return fmt.Sprintf(httpsURLFmt, r.Owner, r.Name)
}

// GetSSHURL returns the SSH URL of the repository
func (r RepositoryConfiguration) GetSSHURL() string {
	return fmt.Sprintf(sshURLFmt, r.Owner, r.Name)
}

// String returns a string representation of the RepositoryConfiguration
func (r RepositoryConfiguration) String() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Name)
}
