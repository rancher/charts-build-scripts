package validate

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

// Icons checks that every chart listed in release.yaml has a local icon present
// under a file:// path. Charts matching IsIconException are skipped.
func Icons(ctx context.Context, rootFs billy.Filesystem) error {
	releaseOpts, err := options.LoadReleaseOptionsFromFile(ctx, rootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	releaseOpts.SortBySemver(ctx)

	logger.Log(ctx, slog.LevelInfo, "checking if icons are present in the local filesystem")
	for chart, versions := range releaseOpts {
		if IsIconException(chart) {
			logger.Log(ctx, slog.LevelDebug, "skipping icon check for:", slog.String("chart", chart))
			continue
		}

		version := versions[len(versions)-1]
		logger.Log(ctx, slog.LevelDebug, "checking chart", slog.String("chart", chart))
		if err := loadAndCheckIconPrefix(ctx, rootFs, chart, version); err != nil {
			return err
		}
	}

	return nil
}

// IsIconException reports whether the given chart name is exempt from icon validation.
func IsIconException(chart string) bool {
	return strings.Contains(chart, "-crd") ||
		strings.Contains(chart, "fleet") ||
		strings.Contains(chart, "harvester") ||
		chart == "rancher-webhook" ||
		chart == "rancher-aks-operator" ||
		chart == "rancher-eks-operator" ||
		chart == "rancher-gke-operator" ||
		chart == "rancher-provisioning-capi" ||
		chart == "rancher-pushprox" ||
		chart == "rancher-wins-upgrader" ||
		chart == "remotedialer-proxy" ||
		chart == "system-upgrade-controller" ||
		chart == "ui-plugin-operator" ||
		chart == "rancher-csp-adapter" ||
		chart == "rancher-ali-operator"
}

// loadAndCheckIconPrefix loads Chart.yaml for the given chart version and verifies
// that the icon field uses a local file:// path that actually exists on the filesystem.
func loadAndCheckIconPrefix(ctx context.Context, rootFs billy.Filesystem, chart string, chartVersion string) error {
	metaData, err := helm.LoadChartYaml(rootFs, chart, chartVersion)
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
