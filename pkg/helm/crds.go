package helm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// CopyCRDsFromChart copies the CRDs from a chart to another chart
func CopyCRDsFromChart(ctx context.Context, fs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := filesystem.RemoveAll(fs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	srcCRDsDirpath := filepath.Join(srcHelmChartPath, srcCRDsDir)
	dstCRDsDirpath := filepath.Join(dstHelmChartPath, destCRDsDir)

	exists, err := filesystem.PathExists(ctx, fs, srcCRDsDirpath)
	if err != nil {
		return fmt.Errorf("error checking if CRDs directory exists: %s", err)
	} else if !exists {
		return os.ErrNotExist
	}

	logger.Log(ctx, slog.LevelInfo, "copying CRDs", slog.String("srcCRDsDirpath", srcCRDsDirpath), slog.String("dstCRDsDirpath", dstCRDsDirpath))

	return filesystem.CopyDir(ctx, fs, srcCRDsDirpath, dstCRDsDirpath)
}

// DeleteCRDsFromChart deletes all the CRDs loaded by a chart
func DeleteCRDsFromChart(ctx context.Context, fs billy.Filesystem, helmChartPath string) error {
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, helmChartPath))
	if err != nil {
		return fmt.Errorf("could not load Helm chart: %s", err)
	}
	for _, crd := range chart.CRDObjects() {
		crdFilepath := filepath.Join(helmChartPath, crd.File.Name)
		exists, err := filesystem.PathExists(ctx, fs, crdFilepath)
		if err != nil {
			return err
		}
		if exists {
			logger.Log(ctx, slog.LevelDebug, "deleting CRD", slog.String("crdFilepath", crdFilepath))

			if err := fs.Remove(crdFilepath); err != nil {
				return err
			}
		}
		if err := filesystem.PruneEmptyDirsInPath(ctx, fs, crdFilepath); err != nil {
			return err
		}
	}
	return nil
}

// ArchiveCRDs bundles, compresses and saves the CRD files from the source to the destination
func ArchiveCRDs(ctx context.Context, fs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := filesystem.RemoveAll(fs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	srcCRDsDirPath := filepath.Join(srcHelmChartPath, srcCRDsDir)
	dstFilePath := filepath.Join(dstHelmChartPath, destCRDsDir, path.ChartCRDTgzFilename)

	logger.Log(ctx, slog.LevelDebug, "compressing CRDs", slog.String("srcCRDsDirPath", srcCRDsDirPath), slog.String("dstFilePath", dstFilePath))
	return filesystem.ArchiveDir(ctx, fs, srcCRDsDirPath, dstFilePath)
}
