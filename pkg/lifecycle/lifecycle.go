package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

// Asset represents an asset with its version and path in the repository
type Asset struct {
	Version string
	path    string
}

// Dependencies holds the necessary filesystem,
// assets versions map, version rules and methods to apply the lifecycle rules in the target branch
type Dependencies struct {
	RootFs            billy.Filesystem
	assetsVersionsMap map[string][]Asset
	VR                *VersionRules
	Git               *git.Git
	// These wrappers are used to mock the filesystem and git status in the tests
	walkDirWrapper         WalkDirFunc
	makeRemoveWrapper      MakeRemoveFunc
	statusPorceLainWrapper StatusPorcelainFunc
	addAndCommitWrapper    AddAndCommitFunc
}

// Function types to be mocked in the tests and used on Dependencies struct methods

// WalkDirFunc is a function type that will be used to walk through the filesystem
type WalkDirFunc func(fs billy.Filesystem, dirPath string, doFunc filesystem.RelativePathFunc) error

// MakeRemoveFunc is a function type that will be used to execute make remove
type MakeRemoveFunc func(chart, version string, debug bool) error

// StatusPorcelainFunc is a function type that will be used to check if the git tree is clean
type StatusPorcelainFunc func(debug bool) (bool, error)

// AddAndCommitFunc is a function type that will be used to add and commit changes in the git tree
type AddAndCommitFunc func(message string) error

// cycleLog is a function to log debug messages if debug mode is enabled
func cycleLog(debugMode bool, msg string, data interface{}) {
	if debugMode {
		if data != nil {
			logrus.Debugf("%s: %#+v \n", msg, data)
		} else {
			logrus.Debug(msg)
		}
	}
}

// InitDependencies will check the filesystem, branch version,
// git status, initialize the Dependencies struct and populate it.
// If anything fails the operation will be aborted.
func InitDependencies(rootFs billy.Filesystem, branchVersion string, currentChart string, debug bool) (*Dependencies, error) {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: true,
	})
	var err error

	workDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	git, err := git.OpenGitRepo(workDir)
	if err != nil {
		return nil, err
	}

	// Create the Dependencies struct which will be used for the entire process
	dep := &Dependencies{
		walkDirWrapper:         filesystem.WalkDir,  // Assign the WalkDir function to the wrapper
		makeRemoveWrapper:      makeRemove,          // Assign the makeRemove function to the wrapper
		statusPorceLainWrapper: git.StatusProcelain, // Assign the IsCleanGitRepo function to the wrapper
		addAndCommitWrapper:    git.AddAndCommit,    // Assign the gitAddAndCommit function to the wrapper
		Git:                    git,
	}

	// Git tree must be clean before proceeding with removing charts
	clean, err := dep.statusPorceLainWrapper(debug)
	if err != nil {
		return nil, err
	}
	if !clean {
		return nil, errors.New("git repo should be clean")
	}

	cycleLog(debug, "Getting branch version rules for: ", branchVersion)
	// Initialize and check version rules for the current branch
	dep.VR, err = GetVersionRules(branchVersion, debug)
	if err != nil {
		return nil, fmt.Errorf("encountered error while getting current branch version: %s", err)
	}

	// Get the filesystem and index.yaml path for the repository
	dep.RootFs = rootFs

	// Check if the assets folder and Helm index file exists in the repository
	exists, err := filesystem.PathExists(dep.RootFs, path.RepositoryAssetsDir)
	if err != nil {
		return nil, fmt.Errorf("encountered error while checking if assets folder already exists in repository: %s", err)
	}
	if !exists {
		return nil, fmt.Errorf("assets folder does not exist in the repository")
	}
	exists, err = filesystem.PathExists(dep.RootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return nil, fmt.Errorf("encountered error while checking if Helm index file already exists in repository: %s", err)
	}
	if !exists {
		return nil, fmt.Errorf("Helm index file does not exist in the repository")
	}

	// Get the absolute path of the Helm index file and assets versions map to apply rules
	helmIndexPath := filesystem.GetAbsPath(dep.RootFs, path.RepositoryHelmIndexFile)
	dep.assetsVersionsMap, err = getAssetsMapFromIndex(helmIndexPath, currentChart, debug)
	if len(dep.assetsVersionsMap) == 0 {
		return nil, fmt.Errorf("no assets found in the repository")
	}
	if err != nil {
		return nil, err // Abort and return error if the assets map is empty
	}

	return dep, nil
}

// ApplyRules will populate all assets versions and paths, sort the versions,
// and execute make remove for each chart and version.
// After each chart removal, it will commit the changes in a single commit
// for all versions of that chart.
func (ld *Dependencies) ApplyRules(currentChart string, debug bool) error {
	// Populate the assets versions and paths combining the index.yaml and assets folder
	err := ld.populateAssetsVersionsPath(debug)
	if err != nil {
		return err
	}
	// Sort the versions before removing
	ld.sortAssetsVersions()

	// Execute make remove for each chart and version that is not in the lifecycle.
	// Commit after each chart removal.
	removedAssetsVersions, err := ld.removeAssetsVersions(debug)
	if err != nil {
		return err
	}

	if len(removedAssetsVersions) == 0 {
		logrus.Infof("No assets were removed")
	}

	logrus.Infof("Removed a total of %d assets", len(removedAssetsVersions))
	cycleLog(debug, "Removed assets", removedAssetsVersions)

	return nil
}

// removeAssetsVersions will iterate through assetsVersionsMap and remove the versions that are not in the lifecycle committing the changes
func (ld *Dependencies) removeAssetsVersions(debug bool) (map[string][]Asset, error) {
	logrus.Info("Executing make remove")

	// Save what was removed for validation
	var removedAssetsVersionsMap map[string][]Asset = make(map[string][]Asset)
	var removedAssetsVersion []Asset

	// Loop through the assetsVersionsMap, i.e: entries in the index.yaml
	for chartName, assetsVersionsMap := range ld.assetsVersionsMap {
		cycleLog(debug, "Chart name", chartName)

		// Reset the slice for the next iteration
		removedAssetsVersion = nil

		// Skip if there are no versions to remove
		if len(assetsVersionsMap) == 0 {
			cycleLog(debug, "Skipping... no versions found for", chartName)
			continue
		}

		// Loop through the versions of the asset and remove the ones that are not in the lifecycle
		for _, asset := range assetsVersionsMap {
			isVersionInLifecycle := ld.VR.CheckChartVersionForLifecycle(asset.Version)
			if isVersionInLifecycle {
				logrus.Debugf("Version %s is in lifecycle for %s", asset.Version, chartName)
				continue // Skipping version in lifecycle
			} else {
				err := ld.makeRemoveWrapper(chartName, asset.Version, debug)
				if err != nil {
					logrus.Errorf("Error while removing %s version %s: %s", chartName, asset.Version, err)
					return nil, err // Abort and return error if the removal fails
				}
				// Saving removed asset version
				removedAssetsVersion = append(removedAssetsVersion, asset)
			}
		}

		// If no versions were removed from the existing ones, do not commit.
		clean, err := ld.statusPorceLainWrapper(debug)
		if err != nil {
			return nil, err
		}
		if clean {
			logrus.Infof("No versions were removed for %s", chartName)
			continue // Skipping
		}

		// Commit each chart removal versions in a single commit
		err = ld.addAndCommitWrapper(fmt.Sprintf("Remove %s versions", chartName))
		if err != nil {
			logrus.Errorf("Error while committing the removal of %s versions: %s", chartName, err)
			return nil, err // Abort and return error if the commit fails
		}

		// Saving removed asset versions
		removedAssetsVersionsMap[chartName] = removedAssetsVersion
	}

	logrus.Info("lifecycle-assets-clean is Done!")
	return removedAssetsVersionsMap, nil
}

// makeRemove will execute make remove script to a specific chart and version
func makeRemove(chart, version string, debug bool) error {

	chartArg := fmt.Sprintf("CHART=%s", chart)
	versionArg := fmt.Sprintf("VERSION=%s", version)
	logrus.Infof("Executing > make remove %s %s \n", chartArg, versionArg)
	cmd := exec.Command("make", "remove", chartArg, versionArg)

	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Run() failed for chart:%s and version:%s with error:%w",
			chart, version, err)
	}
	return nil
}
