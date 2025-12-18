package puller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	git "github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

var fullCloneMutex sync.Mutex

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

// CacheKey returns the key to use for caching
func (r GithubRepository) CacheKey() string {
	if !r.IsCacheable() {
		return ""
	}
	return filepath.Join(".gitrepos", r.String())
}

// IsCacheable returns whether this repository can be cached
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
func (r GithubRepository) Pull(ctx context.Context, rootFs, fs billy.Filesystem, path string) error {
	cloneURL := r.GetHTTPSURL()
	isCacheable := r.IsCacheable()
	logger.Log(ctx, slog.LevelInfo, "pulling from upstream", slog.String("url", cloneURL), slog.Bool("isCacheable", isCacheable))

	if isCacheable {
		pulledFromCache, err := RootCache.Get(ctx, r.CacheKey(), fs, path)
		if err != nil {
			return err
		}
		if pulledFromCache {
			logger.Log(ctx, slog.LevelInfo, "pulled from cache", slog.String("repo", r.name), slog.String("path", path))
			return nil
		}
	}

	switch {
	case r.Commit == nil && r.branch == nil:
		logger.Log(ctx, slog.LevelError, "Git Repo pull; a commit or a branch is required in the package.yaml")
		return errors.New("no commit or branch specified")
	case r.branch != nil:
		logger.Log(ctx, slog.LevelDebug, "", slog.String("branch", *r.branch), slog.String("url", cloneURL))
		_, err := gogit.PlainClone(filesystem.GetAbsPath(fs, path), false, &gogit.CloneOptions{
			URL:           cloneURL,
			ReferenceName: git.GetLocalBranchRefName(*r.branch),
			SingleBranch:  true,
			Depth:         1,
		})
		if err != nil {
			return err
		}
	case r.Commit != nil && r.Subdirectory == nil:
		logger.Log(ctx, slog.LevelDebug, "SLOW", slog.String("commit", *r.Commit), slog.String("url", cloneURL))

		fullCloneMutex.Lock()
		repo, err := gogit.PlainClone(filesystem.GetAbsPath(fs, path), false, &gogit.CloneOptions{
			URL: cloneURL,
		})
		if err != nil {
			return err
		}
		fullCloneMutex.Unlock()

		wt, err := repo.Worktree()
		if err != nil {
			return err
		}
		err = wt.Checkout(&gogit.CheckoutOptions{
			Hash: plumbing.NewHash(*r.Commit),
		})
		if err != nil {
			return err
		}
		head, err := repo.Head()
		if err != nil {
			return errors.New("unable to confirm if checkout was successful: " + err.Error())
		}
		if head.Hash().String() != *r.Commit {
			return errors.New("unable to checkout commit %s, may not be a valid commit hash from upstream: " + *r.Commit)
		}
	case r.Commit != nil && r.Subdirectory != nil:
		if err := git.SparseCloneSubdirectory(ctx, r.GetHTTPSURL(), *r.Commit, *r.Subdirectory, fs, path); err != nil {
			return err
		}
	}

	if err := filesystem.RemoveAll(fs, filepath.Join(path, ".git")); err != nil {
		return err
	}

	if r.Subdirectory != nil {
		logger.Log(ctx, slog.LevelDebug, "", slog.String("subdirectory", *r.Subdirectory))
		if len(*r.Subdirectory) > 0 {
			if err := filesystem.MakeSubdirectoryRoot(ctx, fs, path, *r.Subdirectory, config.IsSoftError(ctx)); err != nil {
				return err
			}
		}
	}

	if isCacheable {
		addedToCache, err := RootCache.Add(ctx, r.CacheKey(), fs, path)
		if err != nil {
			return err
		}
		if addedToCache {
			logger.Log(ctx, slog.LevelInfo, "cached", slog.String("repo", r.name), slog.String("path", path))
		}
	}

	return nil
}

// GetOptions returns the path used to construct this upstream
func (r GithubRepository) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL:             r.GetHTTPSURL(),
		Subdirectory:    r.Subdirectory,
		Commit:          r.Commit,
		ChartRepoBranch: r.branch,
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
