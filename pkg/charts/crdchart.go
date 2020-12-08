package charts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// GenerateCRDChartFromTemplate copies templateDir over to dstPath
func GenerateCRDChartFromTemplate(pkgFs billy.Filesystem, dstHelmChartPath, templateDir, crdsDir string) error {
	exists, err := utils.PathExists(pkgFs, templateDir)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Could not find directory for templates: %s", templateDir)
	}
	if err := pkgFs.MkdirAll(filepath.Join(dstHelmChartPath, crdsDir), os.ModePerm); err != nil {
		return err
	}
	if err := utils.CopyDir(pkgFs, templateDir, dstHelmChartPath); err != nil {
		return err
	}
	return nil
}

// DeleteCRDsFromChart deletes all the CRDs loaded by a chart
func DeleteCRDsFromChart(pkgFs billy.Filesystem, helmChartPath string) error {
	chart, err := helmLoader.Load(utils.GetAbsPath(pkgFs, helmChartPath))
	if err != nil {
		return fmt.Errorf("Could not load Helm chart: %s", err)
	}
	for _, crd := range chart.CRDObjects() {
		crdFilepath := filepath.Join(helmChartPath, crd.File.Name)
		logrus.Infof("Deleting %s", crdFilepath)
		if err := pkgFs.Remove(crdFilepath); err != nil {
			return err
		}
		if err := utils.PruneEmptyDirsInPath(pkgFs, filepath.Dir(crdFilepath)); err != nil {
			return err
		}
	}
	return nil
}

// CopyCRDsFromChart copies the CRDs from a chart to another chart
func CopyCRDsFromChart(pkgFs billy.Filesystem, srcHelmChartPath, srcCRDsDir, dstHelmChartPath, destCRDsDir string) error {
	if err := utils.RemoveAll(pkgFs, filepath.Join(dstHelmChartPath, destCRDsDir)); err != nil {
		return err
	}
	if err := pkgFs.MkdirAll(filepath.Join(dstHelmChartPath, destCRDsDir), os.ModePerm); err != nil {
		return err
	}
	return utils.CopyDir(pkgFs, filepath.Join(srcHelmChartPath, srcCRDsDir), filepath.Join(dstHelmChartPath, destCRDsDir))
}
