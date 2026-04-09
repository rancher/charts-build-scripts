package charts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/go-git/go-billy/v5"
	helmChart "helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmChartUtil "helm.sh/helm/v3/pkg/chartutil"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

// DownloadIcon downloads the icon from the charts.yaml file to the assets/logos folder
// and changes the chart.yaml file to use it
func (p *Package) DownloadIcon(ctx context.Context) error {
	logger.Log(ctx, slog.LevelInfo, "make icon")

	exists, err := filesystem.PathExists(ctx, p.fs, path.RepositoryChartsDir)
	if err != nil {
		return fmt.Errorf("failed to check for charts dir: %w", err)
	}
	if !exists {
		logger.Log(ctx, slog.LevelError, "charts dir does not exist, run make prepare first", slog.String("path", path.RepositoryChartsDir))
		return nil
	}

	absHelmChartPath := filesystem.GetAbsPath(p.fs, path.RepositoryChartsDir)
	chart, err := helmLoader.Load(absHelmChartPath)
	if err != nil {
		return fmt.Errorf("could not load Helm chart: %w", err)
	}

	if !strings.HasPrefix(chart.Metadata.Icon, "file://") {
		logger.Log(ctx, slog.LevelDebug, "chart icon is pointing to a remote url", slog.String("url", chart.Metadata.Icon))

		// download icon and change the icon property to point to it
		p, err := downloadIcon(ctx, p.rootFs, chart.Metadata)
		if err == nil { // managed to download the icon and save it locally
			chart.Metadata.Icon = fmt.Sprintf("file://%s", p)
		} else {
			logger.Log(ctx, slog.LevelError, "failed to download icon", logger.Err(err))
		}

		chartYamlPath := fmt.Sprintf("%s/Chart.yaml", absHelmChartPath)
		err = helmChartUtil.SaveChartfile(chartYamlPath, chart.Metadata)
		if err != nil {
			return fmt.Errorf("failed to save chart.yaml file: %w", err)
		}
	}

	exists, err = filesystem.PathExists(ctx, p.rootFs, strings.TrimPrefix(chart.Metadata.Icon, "file://"))
	if err != nil {
		return err
	}

	if !exists {
		return errors.New("icon path is a file:// prefix, but the icon does not exist, you will need to manually download it at assets/logos dir")
	}

	return nil
}

// downloadIcon receives a chart metadata and the filesystem pointing to the root of the project.
// From the metadata, gets the icon and name of the chart.
// It downloads the icon, infers the type using the content-type header from the response
// and saves the file locally to path.RepositoryLogosDir using the name of the chart as the file name.
func downloadIcon(_ context.Context, rootFs billy.Filesystem, metadata *helmChart.Metadata) (string, error) {
	icon, err := http.Get(metadata.Icon)
	if err != nil {
		return "", fmt.Errorf("failed to download icon %s: %w", metadata.Icon, err)
	}
	defer icon.Body.Close()

	byType, err := mime.ExtensionsByType(icon.Header.Get("Content-Type"))
	if err != nil || len(byType) == 0 || icon.StatusCode != http.StatusOK {
		return "", errors.New("failed to get icon type")
	}

	iconPath := fmt.Sprintf("%s/%s%s", path.RepositoryLogosDir, metadata.Name, byType[0])
	create, err := rootFs.Create(iconPath)
	if err != nil {
		return "", fmt.Errorf("failed to create icon %s: %w", metadata.Icon, err)
	}
	defer create.Close()

	_, err = io.Copy(create, icon.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write icon %s: %w", metadata.Icon, err)
	}
	return iconPath, nil
}
