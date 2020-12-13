package helm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// CopyCRDsFromChart copies the CRDs from a chart to another chart
func CopyCRDsFromChart(fs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := utils.RemoveAll(fs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	srcCRDsDirpath := filepath.Join(srcHelmChartPath, srcCRDsDir)
	dstCRDsDirpath := filepath.Join(dstHelmChartPath, destCRDsDir)
	logrus.Infof("Copying CRDs from %s to %s", srcCRDsDirpath, dstCRDsDirpath)
	return utils.CopyDir(fs, srcCRDsDirpath, dstCRDsDirpath)
}

// DeleteCRDsFromChart deletes all the CRDs loaded by a chart
func DeleteCRDsFromChart(fs billy.Filesystem, helmChartPath string) error {
	chart, err := helmLoader.Load(utils.GetAbsPath(fs, helmChartPath))
	if err != nil {
		return fmt.Errorf("Could not load Helm chart: %s", err)
	}
	for _, crd := range chart.CRDObjects() {
		crdFilepath := filepath.Join(helmChartPath, crd.File.Name)
		exists, err := utils.PathExists(fs, crdFilepath)
		if err != nil {
			return err
		}
		if exists {
			logrus.Infof("Deleting %s", crdFilepath)
			if err := fs.Remove(crdFilepath); err != nil {
				return err
			}
		}
		if err := utils.PruneEmptyDirsInPath(fs, crdFilepath); err != nil {
			return err
		}
	}
	return nil
}
