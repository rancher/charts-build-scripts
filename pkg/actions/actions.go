package actions

import (
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

func List(currentPackage string, porcelainMode bool) {
	repoRoot := getRepoRoot()
	packageList, err := charts.ListPackages(repoRoot, currentPackage)
	if err != nil {
		logrus.Fatal(err)
	}
	if porcelainMode {
		fmt.Println(strings.Join(packageList, " "))
		return

	}
	logrus.Infof("Found the following packages: %v", packageList)
}

func Prepare(currentPackage string) {
	packages := getPackages(currentPackage)
	if len(packages) == 0 {
		logrus.Fatal("Could not find any packages in packages/")
	}
	for _, p := range packages {
		if err := p.Prepare(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func Patch(currentPackage string) {
	packages := getPackages(currentPackage)
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	if len(packages) != 1 {
		packageNames := make([]string, len(packages))
		for i, pkg := range packages {
			packageNames[i] = pkg.Name
		}
		logrus.Fatalf(
			"PACKAGE=\"%s\" must be set to point to exactly one package. Currently found the following packages: %s",
			currentPackage, packageNames,
		)
	}
	if err := packages[0].GeneratePatch(); err != nil {
		logrus.Fatal(err)
	}
}

func Charts(currentPackage string) {
	packages := getPackages(currentPackage)
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	chartsScriptOptions := parseScriptOptions()
	for _, p := range packages {
		if err := p.GenerateCharts(chartsScriptOptions.OmitBuildMetadataOnExport); err != nil {
			logrus.Fatal(err)
		}
	}
}

func Index() {
	repoRoot := getRepoRoot()
	if err := helm.CreateOrUpdateHelmIndex(filesystem.GetFilesystem(repoRoot)); err != nil {
		logrus.Fatal(err)
	}
}

func Zip(currentChart string) {
	repoRoot := getRepoRoot()
	if err := zip.ArchiveCharts(repoRoot, currentChart); err != nil {
		logrus.Fatal(err)
	}
	Index()
}

func Unzip(currentAsset string) {
	repoRoot := getRepoRoot()
	if err := zip.DumpAssets(repoRoot, currentAsset); err != nil {
		logrus.Fatal(err)
	}
	Index()
}

func Clean(currentPackage string) {
	packages := getPackages(currentPackage)
	if len(packages) == 0 {
		logrus.Infof("No packages found.")
		return
	}
	for _, p := range packages {
		if err := p.Clean(); err != nil {
			logrus.Fatal(err)
		}
	}
}

func Validate() {
	chartsScriptOptions := parseScriptOptions()

	logrus.Infof("Checking if Git is clean")
	checkGit()

	logrus.Infof("Generating charts")
	Charts("")

	logrus.Infof("Checking if Git is clean after generating charts")
	checkGit()
	logrus.Infof("Successfully validated that current charts and assets are up to date.")

	pullUpstream(chartsScriptOptions)

	logrus.Info("Zipping charts to ensure that contents of assets, charts, and index.yaml are in sync.")
	Zip("")

	logrus.Info("Doing a final check to ensure Git is clean")
	checkGit()

	logrus.Info("Successfully validated current repository!")
}

func ValidateLocal() {
	logrus.Infof("Checking if Git is clean")
	checkGit()

	logrus.Infof("Generating charts")
	Charts("")

	logrus.Infof("Checking if Git is clean after generating charts")
	checkGit()
	logrus.Infof("Successfully validated that current charts and assets are up to date.")

	logrus.Infof("Running local validation only, skipping pulling upstream")

	logrus.Info("Zipping charts to ensure that contents of assets, charts, and index.yaml are in sync.")
	Zip("")

	logrus.Info("Doing a final check to ensure Git is clean")
	checkGit()

	logrus.Info("Successfully validated current repository!")
}

func ValidateRemote() {
	chartsScriptOptions := parseScriptOptions()

	logrus.Infof("Checking if Git is clean")
	checkGit()

	logrus.Infof("Running remote validation only, skipping generating charts locally")

	pullUpstream(chartsScriptOptions)

	logrus.Info("Zipping charts to ensure that contents of assets, charts, and index.yaml are in sync.")
	Zip("")

	logrus.Info("Doing a final check to ensure Git is clean")
	checkGit()

	logrus.Info("Successfully validated current repository!")
}

func Standardize() {
	repoRoot := getRepoRoot()
	repoFs := filesystem.GetFilesystem(repoRoot)
	if err := standardize.RestructureChartsAndAssets(repoFs); err != nil {
		logrus.Fatal(err)
	}
}

func Template() {
	repoRoot := getRepoRoot()
	repoFs := filesystem.GetFilesystem(repoRoot)
	chartsScriptOptions := parseScriptOptions()
	if err := update.ApplyUpstreamTemplate(repoFs, *chartsScriptOptions); err != nil {
		logrus.Fatalf("Failed to update repository based on upstream template: %s", err)
	}
	logrus.Infof("Successfully updated repository based on upstream template.")
}

func getRepoRoot() string {
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	return repoRoot
}

func getPackages(currentPackage string) []*charts.Package {
	repoRoot := getRepoRoot()
	packages, err := charts.GetPackages(repoRoot, currentPackage)
	if err != nil {
		logrus.Fatal(err)
	}
	return packages
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

func checkGit() {
	_, _, status := getGitInfo()
	if !status.IsClean() {
		logrus.Warnf("Git is not clean:\n%s", status)
		logrus.Fatal("Repository must be clean to run validation")
	}
}

func pullUpstream(chartsScriptOptions *options.ChartsScriptOptions) {
	if chartsScriptOptions.ValidateOptions != nil {
		repoRoot := getRepoRoot()
		repoFs := filesystem.GetFilesystem(repoRoot)
		releaseOptions, err := options.LoadReleaseOptionsFromFile(repoFs, "release.yaml")
		if err != nil {
			logrus.Fatalf("Unable to unmarshall release.yaml: %s", err)
		}
		u := chartsScriptOptions.ValidateOptions.UpstreamOptions
		branch := chartsScriptOptions.ValidateOptions.Branch
		logrus.Infof("Performing upstream validation against repository %s at branch %s", u.URL, branch)
		compareGeneratedAssetsResponse, err := validate.CompareGeneratedAssets(repoFs, u, branch, releaseOptions)
		if err != nil {
			logrus.Fatal(err)
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
				logrus.Fatal(err)
			}
			logrus.Fatalf("Validation against upstream repository %s at branch %s failed.", u.URL, branch)
		}
	}
}

func getGitInfo() (*git.Repository, *git.Worktree, git.Status) {
	repoRoot := getRepoRoot()
	repo, err := repository.GetRepo(repoRoot)
	if err != nil {
		logrus.Fatal(err)
	}
	// Check if git is clean
	wt, err := repo.Worktree()
	if err != nil {
		logrus.Fatal(err)
	}
	status, err := wt.Status()
	if err != nil {
		logrus.Fatal(err)
	}
	return repo, wt, status
}
