package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	chartArchiveFilepath = "chart.tgz"
)

// Upstream represents a particular Helm chart pulled from an upstream location
type Upstream interface {
	// Pull grabs the Helm chart and places it on a path in the filesystem
	Pull(rootFs, fs billy.Filesystem, path string) error
	// GetOptions returns the options used to construct this Upstream
	GetOptions() options.UpstreamOptions
	// IsWithinPackage returns whether this upstream already exists within the package
	IsWithinPackage() bool
}

// UpstreamRepository represents a particular Helm chart within a Git repository
type UpstreamRepository struct {
	repository.GithubConfiguration `yaml:",inline"`

	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory"`
	// Commit represents a specific commit hash to treat as the head
	Commit *string `yaml:"commit"`

	// branch represents a specific branch to pull from
	branch *string `yaml:"branch"`
}

// Pull grabs the Helm chart from the upstream repository
func (u UpstreamRepository) Pull(rootFs, fs billy.Filesystem, path string) error {
	logrus.Infof("Pulling %s from upstream into %s", u, path)
	if u.Commit == nil && u.branch == nil {
		return fmt.Errorf("If you are pulling from a Git repository, a commit is required in the package.yaml")
	}
	cloneOptions := git.CloneOptions{
		URL: u.GetHTTPSURL(),
	}
	if u.branch != nil {
		cloneOptions.ReferenceName = utils.GetLocalBranchRefName(*u.branch)
		cloneOptions.SingleBranch = true
	}
	repo, err := git.PlainClone(utils.GetAbsPath(fs, path), false, &cloneOptions)
	if err != nil {
		return err
	}
	if u.Commit != nil {
		wt, err := repo.Worktree()
		if err != nil {
			return err
		}
		err = wt.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(*u.Commit),
		})
		if err != nil {
			return err
		}
	}
	if err := utils.RemoveAll(fs, filepath.Join(path, ".git")); err != nil {
		return err
	}
	if u.Subdirectory != nil && len(*u.Subdirectory) > 0 {
		if err := utils.MakeSubdirectoryRoot(fs, path, *u.Subdirectory); err != nil {
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

// IsWithinPackage returns whether this upstream already exists within the package
func (u UpstreamRepository) IsWithinPackage() bool {
	return false
}

func (u UpstreamRepository) String() string {
	repoStr := u.GithubConfiguration.String()
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
func (u UpstreamChartArchive) Pull(rootFs, fs billy.Filesystem, path string) error {
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

// IsWithinPackage returns whether this upstream already exists within the package
func (u UpstreamChartArchive) IsWithinPackage() bool {
	return false
}

func (u UpstreamChartArchive) String() string {
	repoStr := u.URL
	if u.Subdirectory != nil {
		repoStr = fmt.Sprintf("%s[path=%s]", repoStr, *u.Subdirectory)
	}
	return repoStr
}

// UpstreamPackage represents a Helm chart that exists within the charts repo itself
type UpstreamPackage struct {
	// Name represents the name of the package
	Name         string
	Subdirectory *string
}

// Pull grabs the Helm chart by preparing the package itself
func (u UpstreamPackage) Pull(rootFs, fs billy.Filesystem, path string) error {
	if strings.HasPrefix(path, u.Name) {
		return fmt.Errorf("Cannot add package to itself")
	}
	pkg, err := GetPackage(rootFs, u.Name)
	if err != nil {
		return err
	}
	if err := pkg.Prepare(); err != nil {
		return err
	}
	defer pkg.Clean()
	if pkg.Chart.Upstream.IsWithinPackage() {
		// Copy
		repositoryPackageWorkingDir, err := utils.GetRelativePath(rootFs, utils.GetAbsPath(pkg.fs, pkg.WorkingDir))
		if err != nil {
			return err
		}
		repositoryPath, err := utils.GetRelativePath(rootFs, utils.GetAbsPath(fs, path))
		if err != nil {
			return err
		}
		if err := utils.CopyDir(rootFs, repositoryPackageWorkingDir, repositoryPath); err != nil {
			return fmt.Errorf("Encountered error while moving prepared package into path: %s", err)
		}
	} else {
		// Move
		if err := os.Rename(utils.GetAbsPath(pkg.fs, pkg.WorkingDir), utils.GetAbsPath(fs, path)); err != nil {
			return fmt.Errorf("Encountered error while renaming prepared package into path: %s", err)
		}
	}
	if u.Subdirectory != nil && len(*u.Subdirectory) > 0 {
		if err := utils.MakeSubdirectoryRoot(fs, path, *u.Subdirectory); err != nil {
			return err
		}
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamPackage) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: fmt.Sprintf("packages/%s", u.Name),
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u UpstreamPackage) IsWithinPackage() bool {
	return false
}

func (u UpstreamPackage) String() string {
	return fmt.Sprintf("packages/%s", u.Name)
}

// UpstreamLocal represents a local chart that exists within the package itself
type UpstreamLocal struct{}

// Pull grabs the Helm chart by preparing the package itself
func (u UpstreamLocal) Pull(rootFs, fs billy.Filesystem, path string) error {
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u UpstreamLocal) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: "local",
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u UpstreamLocal) IsWithinPackage() bool {
	return true
}

func (u UpstreamLocal) String() string {
	return "local"
}
