package lifecycle

import (
	"fmt"

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
	checkIfGitIsCleanWrapper CheckIfGitIsCleanFunc
}

// CheckIfGitIsCleanFunc is a function type that will be used to check if the git tree is clean
type CheckIfGitIsCleanFunc func(debug bool) (bool, error)

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
		checkIfGitIsCleanWrapper: checkIfGitIsClean, // Assign the checkIfGitIsClean function to the wrapper

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
