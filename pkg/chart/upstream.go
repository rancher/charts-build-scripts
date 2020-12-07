package chart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/local"
	"github.com/sirupsen/logrus"
)

const (
	chartArchiveFilepath = "chart.tgz"
)

// Upstream represents a particular Helm chart pulled from an upstream location
type Upstream interface {
	// Pull grabs the Helm chart and places it on a path in the filesystem
	Pull(fs billy.Filesystem, path string) error
	// GetOptions returns the options used to construct this Upstream
	GetOptions() UpstreamOptions
}

// UpstreamRepository represents a particular Helm chart within a Git repository
type UpstreamRepository struct {
	config.RepositoryConfiguration `yaml:",inline"`

	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory"`
	// Commit represents a specific commit hash to treat as the head
	Commit *string `yaml:"commit"`
}

// Pull grabs the Helm chart from the upstream repository
func (u UpstreamRepository) Pull(fs billy.Filesystem, path string) error {
	logrus.Infof("Pulling %s from upstream into %s", u, path)
	if u.Commit == nil {
		return fmt.Errorf("If you are pulling from a Git repository, a commit is required in the package.yaml")
	}
	_, err := git.PlainClone(local.GetAbsPath(fs, path), false, &git.CloneOptions{
		URL: u.GetHTTPSURL(),
	})
	if err != nil {
		return err
	}
	if err := local.RemoveAll(fs, filepath.Join(path, ".git")); err != nil {
		return err
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamRepository) GetOptions() UpstreamOptions {
	url := u.GetHTTPSURL()
	return UpstreamOptions{
		URL:          &url,
		Subdirectory: u.Subdirectory,
		Commit:       u.Commit,
	}
}

func (u UpstreamRepository) String() string {
	repoStr := u.RepositoryConfiguration.String()
	if u.Commit != nil {
		repoStr = fmt.Sprintf("%s@%s", repoStr, *u.Commit)
	}
	if u.Subdirectory != nil {
		repoStr = fmt.Sprintf("%s[path=%s]", repoStr, *u.Subdirectory)
	}
	return repoStr
}

// UpstreamChartArchive represents a particular Helm chart contained in a chart archive
type UpstreamChartArchive struct {
	// URL represents a download link for an archive
	URL string `yaml:"url"`
	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory"`
}

// Pull grabs the Helm chart from the chart archive at the URL specified
func (u UpstreamChartArchive) Pull(fs billy.Filesystem, path string) error {
	logrus.Infof("Pulling %s from upstream into %s", u, path)
	if err := local.GetChartArchive(fs, u.URL, chartArchiveFilepath); err != nil {
		return err
	}
	defer fs.Remove(chartArchiveFilepath)
	if err := fs.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	defer local.PruneEmptyDirsInPath(fs, path)
	var subdirectory string
	if u.Subdirectory != nil {
		subdirectory = *u.Subdirectory
	}
	if err := local.UnarchiveTgz(fs, chartArchiveFilepath, subdirectory, path, true); err != nil {
		return err
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamChartArchive) GetOptions() UpstreamOptions {
	return UpstreamOptions{
		URL: &u.URL,
	}
}

func (u UpstreamChartArchive) String() string {
	repoStr := u.URL
	if u.Subdirectory != nil {
		repoStr = fmt.Sprintf("%s[path=%s]", repoStr, *u.Subdirectory)
	}
	return repoStr
}
