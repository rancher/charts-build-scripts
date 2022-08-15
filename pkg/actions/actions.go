package actions

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/repository"
	"github.com/rancher/charts-build-scripts/pkg/standardize"
	"github.com/rancher/charts-build-scripts/pkg/update"
	"github.com/rancher/charts-build-scripts/pkg/validate"
	"github.com/rancher/charts-build-scripts/pkg/zip"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	ChartsScriptOptionsFile = "configuration.yaml"
)

func List(currentPackage string, porcelainMode bool) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	packageList, err := charts.ListPackages(repoRoot, currentPackage)
	if err != nil {
		return err
	}
	if porcelainMode {
		fmt.Println(strings.Join(packageList, " "))
		return nil

	}
	logrus.Infof("Found the following packages: %v", packageList)
	return nil
}

func Prepare(currentPackage string) error {
	packages, err := getPackages(currentPackage)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return errors.New("could not find any packages in package")
	}
	for _, p := range packages {
		if err := p.Prepare(); err != nil {
			return err
		}
	}

	return nil
}

func Patch(currentPackage string) error {
	packages, err := getPackages(currentPackage)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return nil
	}
	if len(packages) != 1 {
		packageNames := make([]string, len(packages))
		for i, pkg := range packages {
			packageNames[i] = pkg.Name
		}
		return fmt.Errorf(
			"PACKAGE=\"%s\" must be set to point to exactly one package. Currently found the following packages: %s",
			currentPackage, packageNames,
		)
	}
	return packages[0].GeneratePatch()
}

func Charts(currentPackage string) error {
	packages, err := getPackages(currentPackage)
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return nil
	}
	chartsScriptOptions := parseScriptOptions()
	for _, p := range packages {
		if err := p.GenerateCharts(chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
			return err
		}
	}

	return nil
}

func Index() error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	return helm.CreateOrUpdateHelmIndex(filesystem.GetFilesystem(repoRoot))
}

func Zip(currentChart string) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	if err := zip.ArchiveCharts(repoRoot, currentChart); err != nil {
		return err
	}

	return Index()
}

func Unzip(currentAsset string) error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}

	if err := zip.DumpAssets(repoRoot, currentAsset); err != nil {
		return err
	}

	return Index()
}

func Clean(currentPackage string) error {
	packages, err := getPackages(currentPackage)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return nil
	}
	for _, p := range packages {
		if err := p.Clean(); err != nil {
			return err
		}
	}
	return nil
}

func Validate() error {
	chartsScriptOptions := parseScriptOptions()

	if err := checkGitIsClean(); err != nil {
		return err
	}
	if err := Charts(""); err != nil {
		return err
	}
	if err := checkGitIsClean(); err != nil {
		return err
	}
	if err := pullUpstream(chartsScriptOptions); err != nil {
		return err
	}
	if err := Zip(""); err != nil {
		return err
	}
	return checkGitIsClean()
}

func ValidateLocal() error {
	if err := checkGitIsClean(); err != nil {
		return err
	}
	if err := Charts(""); err != nil {
		return err
	}
	if err := checkGitIsClean(); err != nil {
		return err
	}
	if err := Zip(""); err != nil {
		return err
	}
	return checkGitIsClean()
}

func ValidateRemote() error {
	chartsScriptOptions := parseScriptOptions()

	if err := checkGitIsClean(); err != nil {
		return err
	}
	if err := pullUpstream(chartsScriptOptions); err != nil {
		return err
	}
	if err := Zip(""); err != nil {
		return err
	}
	return checkGitIsClean()
}

func Standardize() error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}
	repoFs := filesystem.GetFilesystem(repoRoot)
	return standardize.RestructureChartsAndAssets(repoFs)
}

func Template() error {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return err
	}
	repoFs := filesystem.GetFilesystem(repoRoot)
	chartsScriptOptions := parseScriptOptions()
	return update.ApplyUpstreamTemplate(repoFs, *chartsScriptOptions)
}

func getRepoRoot() (string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return repoRoot, nil
}

func getPackages(currentPackage string) ([]*charts.Package, error) {
	var packages []*charts.Package
	repoRoot, err := getRepoRoot()
	if err != nil {
		return packages, err
	}
	return charts.GetPackages(repoRoot, currentPackage)
}

func parseScriptOptions() *options.ChartsScriptOptions {
	configYaml, err := ioutil.ReadFile(ChartsScriptOptionsFile)
	if err != nil {
		logrus.Fatalf("Unable to find configuration file: %s", err)
	}
	chartsScriptOptions := options.ChartsScriptOptions{}
	if err := yaml.UnmarshalStrict(configYaml, &chartsScriptOptions); err != nil {
		logrus.Fatalf("Unable to unmarshall configuration file: %s", err)
	}
	return &chartsScriptOptions
}

func checkGitIsClean() error {
	logrus.Infof("Checking if Git is clean")
	_, _, status, err := getGitInfo()
	if err != nil {
		return err
	}
	if !status.IsClean() {
		logrus.Warnf("Git is not clean:\n%s", status)
		return errors.New("repository must be clean to run validation")
	}
	return nil
}

func pullUpstream(chartsScriptOptions *options.ChartsScriptOptions) error {
	if chartsScriptOptions.ValidateOptions != nil {
		repoRoot, err := getRepoRoot()
		if err != nil {
			return err
		}
		repoFs := filesystem.GetFilesystem(repoRoot)
		releaseOptions, err := options.LoadReleaseOptionsFromFile(repoFs, "release.yaml")
		if err != nil {
			return err
		}
		u := chartsScriptOptions.ValidateOptions.UpstreamOptions
		branch := chartsScriptOptions.ValidateOptions.Branch
		logrus.Infof("Performing upstream validation against repository %s at branch %s", u.URL, branch)
		compareGeneratedAssetsResponse, err := validate.CompareGeneratedAssets(repoFs, u, branch, releaseOptions)
		if err != nil {
			return err
		}
		if !compareGeneratedAssetsResponse.PassedValidation() {
			// Output charts that have been modified
			compareGeneratedAssetsResponse.LogDiscrepancies()
			logrus.Infof("Dumping release.yaml tracking changes that have been introduced")
			if err := compareGeneratedAssetsResponse.DumpReleaseYaml(repoFs); err != nil {
				logrus.Errorf("Unable to dump newly generated release.yaml: %s", err)
			}
			logrus.Infof("Updating index.yaml")
			if err := helm.CreateOrUpdateHelmIndex(repoFs); err != nil {
				return err
			}
			return fmt.Errorf("validation against upstream repository %s at branch %s failed", u.URL, branch)
		}
	}
	return nil
}

func getGitInfo() (*git.Repository, *git.Worktree, git.Status, error) {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return nil, nil, nil, err
	}
	repo, err := repository.GetRepo(repoRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	// Check if git is clean
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, nil, nil, err
	}
	return repo, wt, status, nil
}
