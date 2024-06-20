package lifecycle

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Status struct hold the results of the assets versions comparison,
// this data will all be logged and saves into log files for further analysis
type Status struct {
	ld                              *Dependencies
	assetsInLifecycleCurrentBranch  map[string][]Asset
	assetsOutLifecycleCurrentBranch map[string][]Asset
	assetsReleasedInLifecycle       map[string][]Asset // OK if not empty
	assetsNotReleasedOutLifecycle   map[string][]Asset // OK if not empty
	assetsNotReleasedInLifecycle    map[string][]Asset // WARN if not empty
	assetsReleasedOutLifecycle      map[string][]Asset // ERROR if not empty
	assetsToBeReleased              map[string][]Asset
	assetsToBeForwardPorted         map[string][]Asset
}

// getStatus will create the Status object inheriting the Dependencies object and return it after:
//
//	list the current assets versions in the current branch
//	list the production and development assets versions from the default branches
//	separate the assets to be released from the assets to be forward ported
func (ld *Dependencies) getStatus() (*Status, error) {
	status := &Status{ld: ld}

	// List the current assets versions in the current branch
	status.listCurrentAssetsVersionsOnTheCurrentBranch()

	// List the production and development assets versions comparisons from the default branches
	err := status.listProdAndDevAssets()
	if err != nil {
		logrus.Errorf("Error while comparing production and development branches: %s", err)
		return status, err
	}

	// Separate the assets to be released from the assets to be forward ported after the comparison

	return status, nil
}

// CheckLifecycleStatusAndSave checks the lifecycle status of the assets
// at 3 different levels prints to the console and saves to log files at 'logs/' folder.
func (ld *Dependencies) CheckLifecycleStatusAndSave(chart string) error {

	// Get the status of the assets versions
	status, err := ld.getStatus()
	if err != nil {
		logrus.Errorf("Error while getting the status: %s", err)
		return err
	}
	_ = status // This will be removed in the future.

	// Create the logs infrastructure in the filesystem

	// ##############################################################################
	// Save the logs for the current branch

	// ##############################################################################
	// Save the logs for the comparison between production and development branches

	// ##############################################################################
	// Save the logs for the separations of assets to be released and forward ported

	return nil
}

// listCurrentAssetsVersionsOnTheCurrentBranch returns the Status struct by reference
// with 2 maps of assets versions, one for the assets that are in the lifecycle,
// and another for the assets that are outside of the lifecycle.
func (s *Status) listCurrentAssetsVersionsOnTheCurrentBranch() {
	insideLifecycle := make(map[string][]Asset)
	outsideLifecycle := make(map[string][]Asset)

	for asset, versions := range s.ld.assetsVersionsMap {
		for _, version := range versions {
			inLifecycle := s.ld.VR.CheckChartVersionForLifecycle(version.version)
			if inLifecycle {
				insideLifecycle[asset] = append(insideLifecycle[asset], version)
			} else {
				outsideLifecycle[asset] = append(outsideLifecycle[asset], version)
			}
		}
	}

	s.assetsInLifecycleCurrentBranch = insideLifecycle
	s.assetsOutLifecycleCurrentBranch = outsideLifecycle

	return
}

// listProdAndDevAssets will clone the charts repository at a temporary directory,
// fetch and checkout in the production and development branches for the given version,
// get the assets versions from the index.yaml file and compare the assets versions,
// separating into 4 different maps for further analysis.
func (s *Status) listProdAndDevAssets() error {
	// Create and destroy a temporary directory structure
	defaultWorkingDir, tempDir, err := createTemporaryDirStructure()
	if err != nil {
		logrus.Errorf("Error while creating temporary dir structure: %s", err)
		return err
	}
	defer destroyTemporaryDirStructure(defaultWorkingDir, tempDir)

	// Clone the repository at the temporary directory
	git, err := cloneAtDir("https://github.com/rancher/charts", tempDir)
	if err != nil {
		return err
	}

	_ = git // this will be removed in the future

	// Fetch, checkout and map assets versions in the production and development branches

	// Compare the assets versions between the production and development branches
	return nil
}

// createTemporaryDirStructure creates a temporary directory structure and changes the working directory to it returning the path for both folders.
func createTemporaryDirStructure() (string, string, error) {
	// Save the current working directory for changing back to it later
	defaultWorkingDir, err := os.Getwd()
	if err != nil {
		logrus.Errorf("Error while getting the current working directory: %s", err)
		return "", "", err
	}

	// Create the temporary directory
	tempDir, err := os.MkdirTemp("", "temporaryDir")
	if err != nil {
		return "", "", err
	}

	// change the workind directory to the temporary one
	err = os.Chdir(tempDir)
	if err != nil {
		logrus.Errorf("Error while changing working directory to temporary directory: %v", err)
		return defaultWorkingDir, tempDir, err
	}
	return defaultWorkingDir, tempDir, nil
}

// destroyTemporaryDirStructure destroys the temporary directory and changes the working directory back to the default one.
func destroyTemporaryDirStructure(defaultWorkingDir, tempDir string) error {
	// Change the directory back to the default working directory
	err := os.Chdir(defaultWorkingDir)
	if err != nil {
		logrus.Errorf("Error while changing back to default working directory: %v", err)
		return err
	}

	// Remove the temporary directory
	err = os.RemoveAll(tempDir)
	if err != nil {
		logrus.Errorf("Error while removing temporary directory: %v", err)
		return err
	}
	return nil
}