package puller

import (
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/sirupsen/logrus"
)

const chartArchiveFilepath = "chart.tgz"

// Archive represents a URL pointing to a .tgz file
type Archive struct {
	// URL represents a download link for an archive
	URL string `yaml:"url"`
	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory"`
}

// Pull grabs the archive
func (u Archive) Pull(_ /*rootFs*/, fs billy.Filesystem, path string) error {
	logrus.Infof("Pulling %s from upstream into %s", u, path)
	if err := filesystem.GetChartArchive(fs, u.URL, chartArchiveFilepath); err != nil {
		return err
	}
	defer fs.Remove(chartArchiveFilepath)
	if err := fs.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	defer filesystem.PruneEmptyDirsInPath(fs, path)
	var subdirectory string
	if u.Subdirectory != nil {
		subdirectory = *u.Subdirectory
	}
	if err := filesystem.UnarchiveTgz(fs, chartArchiveFilepath, subdirectory, path, true); err != nil {
		return err
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u Archive) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: u.URL,
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u Archive) IsWithinPackage() bool {
	return false
}

func (u Archive) String() string {
	repoStr := u.URL
	if u.Subdirectory != nil {
		repoStr = fmt.Sprintf("%s[path=%s]", repoStr, *u.Subdirectory)
	}
	return repoStr
}
