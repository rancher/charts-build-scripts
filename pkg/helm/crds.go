package helm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// CopyCRDsFromChart copies the CRDs from a chart to another chart
func CopyCRDsFromChart(fs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := filesystem.RemoveAll(fs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	srcCRDsDirpath := filepath.Join(srcHelmChartPath, srcCRDsDir)
	dstCRDsDirpath := filepath.Join(dstHelmChartPath, destCRDsDir)
	logrus.Infof("Copying CRDs from %s to %s", srcCRDsDirpath, dstCRDsDirpath)
	return filesystem.CopyDir(fs, srcCRDsDirpath, dstCRDsDirpath)
}

// DeleteCRDsFromChart deletes all the CRDs loaded by a chart
func DeleteCRDsFromChart(fs billy.Filesystem, helmChartPath string) error {
	chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, helmChartPath))
	if err != nil {
		return fmt.Errorf("Could not load Helm chart: %s", err)
	}
	for _, crd := range chart.CRDObjects() {
		crdFilepath := filepath.Join(helmChartPath, crd.File.Name)
		exists, err := filesystem.PathExists(fs, crdFilepath)
		if err != nil {
			return err
		}
		if exists {
			logrus.Infof("Deleting %s", crdFilepath)
			if err := fs.Remove(crdFilepath); err != nil {
				return err
			}
		}
		if err := filesystem.PruneEmptyDirsInPath(fs, crdFilepath); err != nil {
			return err
		}
	}
	return nil
}

// ArchiveCRDs bundles, compresses and saves the CRD files from the source to the destination
func ArchiveCRDs(fs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := filesystem.RemoveAll(fs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	srcCRDsDirPath := filepath.Join(srcHelmChartPath, srcCRDsDir)
	dstFilePath := filepath.Join(dstHelmChartPath, destCRDsDir, path.ChartCRDTgzFilename)
	logrus.Infof("Compressing CRDs from %s to %s", srcCRDsDirPath, dstFilePath)
	return filesystem.ArchiveDir(fs, srcCRDsDirPath, dstFilePath)
}
