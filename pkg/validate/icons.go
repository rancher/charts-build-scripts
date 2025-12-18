package validate

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// ValidateIcons will check if the icons are present in the local filesystem and if they are not, it will return an error.
func ValidateIcons(ctx context.Context) error {
	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	releaseOpts, err := options.LoadReleaseOptionsFromFile(ctx, cfg.RootFS, config.PathReleaseYaml)
	if err != nil {
		return err
	}

	releaseOpts.SortBySemver(ctx)

	logger.Log(ctx, slog.LevelInfo, "checking if icons are present in the local filesystem")
	for chart, versions := range releaseOpts {
		isException, err := IsIconException(cfg, chart)
		if err != nil {
			return err
		}
		if isException {
			logger.Log(ctx, slog.LevelDebug, "skipping icon check for:", slog.String("chart", chart))
			continue
		}

		version := versions[len(versions)-1]
		logger.Log(ctx, slog.LevelDebug, "checking chart", slog.String("chart", chart))
		if err := loadAndCheckIconPrefix(ctx, cfg.RootFS, chart, version); err != nil {
			return err
		}
	}

	return nil
}

func IsIconException(cfg *config.Config, chart string) (bool, error) {
	if strings.Contains(chart, "-crd") {
		return true, nil
	}

	trackedChart := cfg.TrackedCharts.GetChartByName(chart)
	if trackedChart == nil {
		err := errors.New("chart is not tracked: " + chart + " at " + config.PathTrackChartsYaml)
		logger.Err(err)
		return false, err
	}

	return !trackedChart.Icon, nil
}

func loadAndCheckIconPrefix(ctx context.Context, rootFs billy.Filesystem, chart string, version string) error {
	metaData, err := helm.LoadChartYaml(rootFs, chart, version)
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelDebug, "checking if chart has downloaded icon")
	iconField := metaData.Icon

	// Check file prefix if it is a URL just skip this process
	if !strings.HasPrefix(iconField, "file://") {
		logger.Log(ctx, slog.LevelError, "icon path is not a file:// prefix")
		return errors.New("icon path is not a file:// prefix, after make prepare, you need to run make icon for chart:" + chart)
	}

	exist, err := filesystem.PathExists(ctx, rootFs, strings.TrimPrefix(iconField, "file://"))
	if err != nil {
		return err
	}

	if !exist {
		return errors.New("icon path is a file:// prefix, but the icon does not exist, after 'make prepare', you need to run 'make icon' for chart:" + chart)
	}

	return nil
}
