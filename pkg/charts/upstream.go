package charts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
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
	GetOptions() options.UpstreamOptions
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
	_, err := git.PlainClone(utils.GetAbsPath(fs, path), false, &git.CloneOptions{
		URL: u.GetHTTPSURL(),
	})
	if err != nil {
		return err
	}
	if err := utils.RemoveAll(fs, filepath.Join(path, ".git")); err != nil {
		return err
	}
	if u.Subdirectory != nil && len(*u.Subdirectory) > 0 {
		absTempDir, err := ioutil.TempDir(fs.Root(), "pull-from-upstream")
		if err != nil {
			return err
		}
		defer os.RemoveAll(absTempDir)
		tempDir, err := utils.GetRelativePath(fs, absTempDir)
		if err != nil {
			return err
		}
		if err := utils.CopyDir(fs, path, tempDir); err != nil {
			return err
		}
		if err := utils.RemoveAll(fs, path); err != nil {
			return nil
		}
		if err := utils.CopyDir(fs, filepath.Join(tempDir, *u.Subdirectory), path); err != nil {
			return err
		}
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamRepository) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL:          u.GetHTTPSURL(),
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
	if err := utils.GetChartArchive(fs, u.URL, chartArchiveFilepath); err != nil {
		return err
	}
	defer fs.Remove(chartArchiveFilepath)
	if err := fs.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	defer utils.PruneEmptyDirsInPath(fs, path)
	var subdirectory string
	if u.Subdirectory != nil {
		subdirectory = *u.Subdirectory
	}
	if err := utils.UnarchiveTgz(fs, chartArchiveFilepath, subdirectory, path, true); err != nil {
		return err
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamChartArchive) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: u.URL,
	}
}

func (u UpstreamChartArchive) String() string {
	repoStr := u.URL
	if u.Subdirectory != nil {
		repoStr = fmt.Sprintf("%s[path=%s]", repoStr, *u.Subdirectory)
	}
	return repoStr
}

// UpstreamLocal represents a Helm chart that exists within the charts repo itself
type UpstreamLocal struct {
	// Name represents the name of the package
	Name string
}

// Pull grabs the Helm chart by preparing the package itself
func (u UpstreamLocal) Pull(fs billy.Filesystem, path string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Encountered error while trying to get repository path: %s", err)
	}
	repoFs := utils.GetFilesystem(repoRoot)
	pkg, err := GetPackage(repoFs, u.Name, options.BranchOptions{})
	if err != nil {
		return err
	}
	if err := pkg.Prepare(); err != nil {
		return err
	}
	return os.Rename(utils.GetAbsPath(pkg.fs, pkg.WorkingDir), utils.GetAbsPath(fs, path))
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamLocal) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: fmt.Sprintf("packages/%s", u.Name),
	}
}

func (u UpstreamLocal) String() string {
	return fmt.Sprintf("packages/%s", u.Name)
}
