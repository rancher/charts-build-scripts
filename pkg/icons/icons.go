package icons

import (
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/rancher/charts-build-scripts/pkg/path"

	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart"
)

// Download receives a chart metadata and the filesystem pointing to the root of the project.
// From the metadata, gets the icon and name of the chart.
// It downloads the icon, infers the type using the content-type header from the response
// and saves the file locally to path.RepositoryLogosDir using the name of the chart as the file name.
func Download(rootFs billy.Filesystem, metadata *chart.Metadata) (string, error) {
	icon, err := http.Get(metadata.Icon)
	if err != nil {
		logrus.Errorf(err.Error())
		return "", fmt.Errorf("err: %w", err)
	}

	byType, err := mime.ExtensionsByType(icon.Header.Get("Content-Type"))
	if err != nil || len(byType) == 0 || icon.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid icon")
	}
	path := fmt.Sprintf("%s/%s%s", path.RepositoryLogosDir, metadata.Name, byType[0])
	create, err := rootFs.Create(path)
	if err != nil {
		logrus.Errorf(err.Error())
		return "", fmt.Errorf("err: %w", err)
	}
	defer create.Close()
	_, err = io.Copy(create, icon.Body)
	defer icon.Body.Close()
	if err != nil {
		logrus.Errorf(err.Error())
		return "", fmt.Errorf("err: %w", err)
	}
	return path, nil
}
