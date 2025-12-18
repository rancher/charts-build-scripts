package auto

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"

	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// removeCharts, iterate and remove assets/<chart>; charts/<chart>; remove entry from index.yaml
func removeCharts(ctx context.Context, rootFs billy.Filesystem, charts []string, version string) error {

	for _, chart := range charts {
		logger.Log(ctx, slog.LevelDebug, "removing", slog.Group("chart", chart, version))

		if err := remove(ctx, rootFs, chart, version); err != nil {
			return err
		}
	}

	logger.Log(ctx, slog.LevelDebug, "removing from index", slog.Any("charts", charts))
	if err := removeIndexCharts(ctx, rootFs, charts, version); err != nil {
		return err
	}

	return nil
}

// remove will check and remove assets/<chart>; charts/<chart>
func remove(ctx context.Context, rootFs billy.Filesystem, chart, version string) error {
	if chart == "" || version == "" {
		return errors.New("create error this here")
	}

	// check if the charts dir exists
	chartPath := "charts/" + chart + "/" + version
	if exist, err := filesystem.PathExists(ctx, rootFs, chartPath); !exist || err != nil {
		return errors.New("some error here")
	}
	// check if the assets dir exists
	assetPath := "assets/" + chart + "/" + chart + "-" + version + ".tgz"
	if exist, err := filesystem.PathExists(ctx, rootFs, assetPath); !exist || err != nil {
		return errors.New("some error here")
	}

	// remove charts dir
	if err := filesystem.RemoveAll(rootFs, chartPath); err != nil {
		return err
	}
	// remove assets dir
	if err := filesystem.RemoveAll(rootFs, assetPath); err != nil {
		return err
	}

	return nil
}

// removeIndexCharts seeks a chart at index.yaml and remove it's entry version
func removeIndexCharts(ctx context.Context, rootFs billy.Filesystem, charts []string, version string) error {
	index, err := helm.OpenIndexYaml(ctx, rootFs)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to open index.yaml", logger.Err(err))
		return err
	}

	for _, chart := range charts {
		removeEntryVersionFromIndex(ctx, index, chart, version)
	}

	idxAbsPath := filesystem.GetAbsPath(rootFs, config.PathIndexYaml)
	if err := index.WriteFile(idxAbsPath, os.ModePerm); err != nil {
		logger.Log(ctx, slog.LevelError, "failed to remove charts from index.yaml", logger.Err(err))
		return err
	}

	return nil
}

// removeEntryVersionFromIndex will make a new map without the given version at target chart
func removeEntryVersionFromIndex(ctx context.Context, index *helmRepo.IndexFile, chart, version string) {
	new := make(map[string]helmRepo.ChartVersions, 0)

	for _, chartVersion := range index.Entries[chart] {
		if chartVersion.Metadata.Version == version {
			continue
		}
		new[chart] = append(new[chart], chartVersion)
	}

	index.Entries[chart] = new[chart]
}
