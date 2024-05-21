package lifecycle

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

// Asset represents an asset with its version and path in the repository
type Asset struct {
	version string
	path    string
}

// Dependencies struct holds the necessary filesystem,
// assets versions map, version rules and methods
// to apply the lifecycle rules in the target branch
type Dependencies struct {
	rootFs            billy.Filesystem
	assetsVersionsMap map[string][]Asset
	vr                *VersionRules
	// These wrappers are used to mock the filesystem and git status in the tests
	walkDirWrapper           WalkDirFunc
	makeRemoveWrapper        MakeRemoveFunc
	checkIfGitIsCleanWrapper CheckIfGitIsCleanFunc
	gitAddAndCommitWrapper   GitAddAndCommitFunc
}

// WalkDirFunc is a function type that will be used to walk through the filesystem
type WalkDirFunc func(fs billy.Filesystem, dirPath string, doFunc filesystem.RelativePathFunc) error

// MakeRemoveFunc is a function type that will be used to execute make remove
type MakeRemoveFunc func(chart, version string, debug bool) error

// CheckIfGitIsCleanFunc is a function type that will be used to check if the git tree is clean
type CheckIfGitIsCleanFunc func(debug bool) (bool, error)

// GitAddAndCommitFunc is a function type that will be used to add and commit changes in the git tree
type GitAddAndCommitFunc func(message string) error

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
func InitDependencies(repoRoot, branchVersion string, currentChart string, debug bool) (*Dependencies, error) {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: true,
	})
	var err error
	// Create a new Dependencies struct
	dep := &Dependencies{
		walkDirWrapper:           filesystem.WalkDir, // Assign the WalkDir function to the wrapper
		makeRemoveWrapper:        makeRemove,         // Assign the makeRemove function to the wrapper
		checkIfGitIsCleanWrapper: checkIfGitIsClean,  // Assign the checkIfGitIsClean function to the wrapper
		gitAddAndCommitWrapper:   gitAddAndCommit,    // Assign the gitAddAndCommit function to the wrapper
	}

	cycleLog(debug, "Getting branch version rules for: ", branchVersion)
	// Initialize and check version rules for the current branch
	dep.vr, err = GetVersionRules(branchVersion, debug)
	if err != nil {
		return nil, fmt.Errorf("encountered error while getting current branch version: %s", err)
	}

	// Get the filesystem and index.yaml path for the repository
	dep.rootFs = filesystem.GetFilesystem(repoRoot)

	// Check if the assets folder and Helm index file exists in the repository
	exists, err := filesystem.PathExists(dep.rootFs, path.RepositoryAssetsDir)
	if err != nil || !exists {
		return nil, fmt.Errorf("encountered error while checking if assets folder already exists in repository: %s", err)
	}
	exists, err = filesystem.PathExists(dep.rootFs, path.RepositoryHelmIndexFile)
	if err != nil || !exists {
		return nil, fmt.Errorf("encountered error while checking if Helm index file already exists in repository: %s", err)
	}

	// Get the absolute path of the Helm index file and assets versions map to apply rules
	helmIndexPath := filesystem.GetAbsPath(dep.rootFs, path.RepositoryHelmIndexFile)
	dep.assetsVersionsMap, err = getAssetsMapFromIndex(helmIndexPath, currentChart, debug)
	if len(dep.assetsVersionsMap) == 0 {
		return nil, fmt.Errorf("no assets found in the repository")
	}
	if err != nil {
		return nil, err // Abort and return error if the assets map is empty
	}

	// Git tree must be clean before proceeding with removing charts
	clean, err := dep.checkIfGitIsCleanWrapper(debug)
	if !clean {
		return nil, fmt.Errorf("git is not clean, it must be clean before proceeding with removing charts")
	}
	if err != nil {
		return nil, err
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

	// Execute make remove for each chart and version that is not in the lifecycle
	// Commit after each chart removal...
	removedAssetsVersions, err := ld.removeVersionsAssets(debug)
	if err != nil {
		return err
	}

	logrus.Infof("Removed a total of %d assets", len(removedAssetsVersions))
	cycleLog(debug, "Removed assets", removedAssetsVersions)

	return nil
}

// removeVersionsAssets will iterate through assetsVersionsMap and remove the versions that are not in the lifecycle commiting the changes
func (ld *Dependencies) removeVersionsAssets(debug bool) (map[string][]Asset, error) {
	logrus.Info("Executing make remove")

	// Save what was removed for validation
	var removedAssetsVersionsMap map[string][]Asset = make(map[string][]Asset)
	var removedAssetsVersion []Asset = make([]Asset, 0)

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

		// Loop through the versions of the chart and remove the ones that are not in the lifecycle
		for _, asset := range assetsVersionsMap {
			isVersionInLifecycle := ld.vr.checkChartVersionForLifecycle(asset.version)
			if isVersionInLifecycle {
				logrus.Debugf("Version %s is in lifecycle for %s", asset.version, chartName)
				continue // Skipping version in lifecycle
			} else {
				err := ld.makeRemoveWrapper(chartName, asset.version, debug)
				if err != nil {
					logrus.Errorf("Error while removing %s version %s: %s", chartName, asset.version, err)
					return nil, err // Abort and return error if the removal fails
				}
				// Saving removed asset version
				removedAssetsVersion = append(removedAssetsVersion, asset)
			}
		}

		// If no versions were removed from the existing ones, do not commit.
		clean, err := ld.checkIfGitIsCleanWrapper(debug)
		if err != nil {
			return nil, err
		}
		if clean {
			logrus.Infof("No versions were removed for %s", chartName)
			continue // Skipping
		}

		// Commit each chart removal versions in a single commit
		err = ld.gitAddAndCommitWrapper(fmt.Sprintf("Remove %s versions", chartName))
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
