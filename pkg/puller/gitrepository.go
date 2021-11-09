package puller

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/sirupsen/logrus"
)

const (
	httpsURLFmt = "https://github.com/%s/%s.git"
	sshURLFmt   = "git@github.com:%s/%s.git"
)

// GetGithubRepository gets a GitHub repository from options
func GetGithubRepository(upstreamOptions options.UpstreamOptions, branch *string) (GithubRepository, error) {
	var githubRepo GithubRepository
	if !strings.HasSuffix(upstreamOptions.URL, ".git") {
		return githubRepo, fmt.Errorf("URL does not seem to point to a Git repository: %s", upstreamOptions.URL)
	}
	splitURL := strings.Split(strings.TrimSuffix(upstreamOptions.URL, ".git"), "/")
	if len(splitURL) < 2 {
		return githubRepo, fmt.Errorf("URL does not seem to be valid for a Git repository: %s", upstreamOptions.URL)
	}
	return GithubRepository{
		Subdirectory: upstreamOptions.Subdirectory,
		Commit:       upstreamOptions.Commit,
		owner:        splitURL[len(splitURL)-2],
		name:         splitURL[len(splitURL)-1],
		branch:       branch,
	}, nil
}

// GithubRepository represents a repository hosted on Github
type GithubRepository struct {
	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory"`
	// Commit represents a specific commit hash to treat as the head
	Commit *string `yaml:"commit"`

	// owner represents the account that owns the repo, e.g. rancher
	owner string `yaml:"owner"`
	// name represents the name of the repo, e.g. charts
	name string `yaml:"name"`
	// Branch represents a specific branch to pull from
	branch *string `yaml:"branch"`
}

func (r GithubRepository) CacheKey() string {
	if !r.IsCacheable() {
		return ""
	}
	return filepath.Join(".gitrepos", r.String())
}

func (r GithubRepository) IsCacheable() bool {
	return r.Commit != nil
}

// GetHTTPSURL returns the HTTPS URL of the repository
func (r GithubRepository) GetHTTPSURL() string {
	return fmt.Sprintf(httpsURLFmt, r.owner, r.name)
}

// GetSSHURL returns the SSH URL of the repository
func (r GithubRepository) GetSSHURL() string {
	return fmt.Sprintf(sshURLFmt, r.owner, r.name)
}

// Pull grabs the repository
func (r GithubRepository) Pull(rootFs, fs billy.Filesystem, path string) error {
	if r.IsCacheable() {
		pulledFromCache, err := RootCache.Get(r.CacheKey(), fs, path)
		if err != nil {
			return err
		}
		if pulledFromCache {
			logrus.Infof("Pulled %s from cache into %s", r, path)
			return nil
		}
	}
	logrus.Infof("Pulling %s from upstream into %s", r, path)
	if r.Commit == nil && r.branch == nil {
		return fmt.Errorf("if you are pulling from a Git repository, a commit is required in the package.yaml")
	}
	cloneOptions := git.CloneOptions{
		URL: r.GetHTTPSURL(),
	}
	if r.branch != nil {
		cloneOptions.ReferenceName = repository.GetLocalBranchRefName(*r.branch)
		cloneOptions.SingleBranch = true
	}
	repo, err := git.PlainClone(filesystem.GetAbsPath(fs, path), false, &cloneOptions)
	if err != nil {
		return err
	}
	if r.Commit != nil {
		wt, err := repo.Worktree()
		if err != nil {
			return err
		}
		err = wt.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(*r.Commit),
		})
		if err != nil {
			return err
		}
		head, err := repo.Head()
		if err != nil {
			return fmt.Errorf("unable to confirm if checkout was successful: %s", err)
		}
		if head.Hash().String() != *r.Commit {
			return fmt.Errorf("unable to checkout commit %s, may not be a valid commit hash from upstream", *r.Commit)
		}
	}
	if err := filesystem.RemoveAll(fs, filepath.Join(path, ".git")); err != nil {
		return err
	}
	if r.Subdirectory != nil && len(*r.Subdirectory) > 0 {
		if err := filesystem.MakeSubdirectoryRoot(fs, path, *r.Subdirectory); err != nil {
			return err
		}
	}
	if r.IsCacheable() {
		addedToCache, err := RootCache.Add(r.CacheKey(), fs, path)
		if err != nil {
			return err
		}
		if addedToCache {
			logrus.Infof("Cached %s", r)
		}
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (r GithubRepository) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL:          r.GetHTTPSURL(),
		Subdirectory: r.Subdirectory,
		Commit:       r.Commit,
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (r GithubRepository) IsWithinPackage() bool {
	return false
}

func (r GithubRepository) String() string {
	repoStr := fmt.Sprintf("%s/%s", r.owner, r.name)
	if r.Commit != nil {
		repoStr = fmt.Sprintf("%s@%s", repoStr, *r.Commit)
	}
	if r.Subdirectory != nil {
		repoStr = fmt.Sprintf("%s/%s", repoStr, *r.Subdirectory)
	}
	return repoStr
}
