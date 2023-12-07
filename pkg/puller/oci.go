package puller

import (
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/getter"
)

type Registry struct {
	// URL represents the link to the chart registry including the chart version
	URL string `yaml:"url"`
}

func (r Registry) Pull(rootFs, fs billy.Filesystem, path string) error {
	logrus.Infof("Pulling %s from upstream into %s", r.URL, path)

	getter, err := getter.NewOCIGetter()
	if err != nil {
		return err
	}

	buffer, err := getter.Get(r.URL)
	if err != nil {
		return err
	}

	tgz, err := filesystem.CreateFileAndDirs(fs, chartArchiveFilepath)
	if err != nil {
		return err
	}
	defer fs.Remove(chartArchiveFilepath)

	if _, err := tgz.Write(buffer.Bytes()); err != nil {
		return err
	}

	if err := fs.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	defer filesystem.PruneEmptyDirsInPath(fs, path)

	if err := filesystem.UnarchiveTgz(fs, chartArchiveFilepath, "", path, true); err != nil {
		return err
	}

	return nil
}

func (r Registry) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: r.URL,
	}
}

func (r Registry) IsWithinPackage() bool {
	return false
}
