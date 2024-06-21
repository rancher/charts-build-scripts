package lifecycle

import (
	"fmt"
	"os"
	"time"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
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
	status.separateReleaseFromForwardPort()

	return status, nil
}

// createLogFiles will create the log files for the current branch, production and development branches
// and the assets to be released and forward ported, returning the logs objects for each file.
func createLogFiles(chart string) (*logs, *logs, *logs, error) {
	// get a timestamp
	currentTime := time.Now()
	now := currentTime.Format("2006-01-02T15:04")

	// create the log file names with the timestamp and chart name if any
	cbLogFile := fmt.Sprintf("%s_%s_current-branch.log", now, chart)
	pdLogFile := fmt.Sprintf("%s_%s_production-compare-development.log", now, chart)
	rfLogFile := fmt.Sprintf("%s_%s_released-forward-ported.log", now, chart)

	// Create the logs infrastructure in the filesystem for:
	// current branch logs
	cbLogs, err := createLogs(cbLogFile)
	if err != nil {
		logrus.Errorf("Error while creating logs: %s", err)
		return nil, nil, nil, err
	}

	// production and development branches logs
	pdLogs, err := createLogs(pdLogFile)
	if err != nil {
		logrus.Errorf("Error while creating logs: %s", err)
		return nil, nil, nil, err
	}

	// released and forward ported logs
	rfLogs, err := createLogs(rfLogFile)
	if err != nil {
		logrus.Errorf("Error while creating logs: %s", err)
		return nil, nil, nil, err
	}

	return cbLogs, pdLogs, rfLogs, nil
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
	// Create the logs infrastructure in the filesystem and close them once the function ends
	cbLogs, pdLogs, rfLogs, err := createLogFiles(chart)
	if err != nil {
		return err
	}
	defer cbLogs.file.Close()
	defer pdLogs.file.Close()
	defer rfLogs.file.Close()

	// optional filter logs by specific chart
	if chart != "" {
		status.assetsInLifecycleCurrentBranch = map[string][]Asset{chart: status.assetsInLifecycleCurrentBranch[chart]}
		status.assetsOutLifecycleCurrentBranch = map[string][]Asset{chart: status.assetsOutLifecycleCurrentBranch[chart]}
		status.assetsReleasedInLifecycle = map[string][]Asset{chart: status.assetsReleasedInLifecycle[chart]}
		status.assetsNotReleasedOutLifecycle = map[string][]Asset{chart: status.assetsNotReleasedOutLifecycle[chart]}
		status.assetsNotReleasedInLifecycle = map[string][]Asset{chart: status.assetsNotReleasedInLifecycle[chart]}
		status.assetsReleasedOutLifecycle = map[string][]Asset{chart: status.assetsReleasedOutLifecycle[chart]}
	}

	// ##############################################################################
	// Save the logs for the current branch
	cbLogs.writeHEAD(status.ld.VR, "Assets versions vs the lifecycle rules in the current branch")
	cbLogs.write("Versions INSIDE the lifecycle in the current branch", "INFO")
	cbLogs.writeVersions(status.assetsInLifecycleCurrentBranch, "INFO")
	cbLogs.write("", "END")
	cbLogs.write("Versions OUTSIDE the lifecycle in the current branch", "WARN")
	cbLogs.writeVersions(status.assetsOutLifecycleCurrentBranch, "WARN")
	cbLogs.write("", "END")
	// ##############################################################################
	// Save the logs for the comparison between production and development branches
	pdLogs.writeHEAD(status.ld.VR, "Released assets vs development assets with lifecycle rules")
	pdLogs.write("Assets RELEASED and Inside the lifecycle", "INFO")
	pdLogs.write(fmt.Sprintf("At the production branch: %s", status.ld.VR.prodBranch), "INFO")
	pdLogs.writeVersions(status.assetsReleasedInLifecycle, "INFO")
	pdLogs.write("", "END")

	pdLogs.write("Assets NOT released and Out of the lifecycle", "INFO")
	pdLogs.write(fmt.Sprintf("At the development branch: %s", status.ld.VR.devBranch), "INFO")
	pdLogs.writeVersions(status.assetsNotReleasedOutLifecycle, "INFO")
	pdLogs.write("", "END")

	pdLogs.write("Assets NOT released and Inside the lifecycle", "WARN")
	pdLogs.write(fmt.Sprintf("At the development branch: %s", status.ld.VR.devBranch), "WARN")
	pdLogs.writeVersions(status.assetsNotReleasedInLifecycle, "WARN")
	pdLogs.write("", "END")

	pdLogs.write("Assets released and Out of the lifecycle", "ERROR")
	pdLogs.write(fmt.Sprintf("At the production branch: %s", status.ld.VR.prodBranch), "ERROR")
	pdLogs.writeVersions(status.assetsReleasedOutLifecycle, "ERROR")
	pdLogs.write("", "END")
	// ##############################################################################
	// Save the logs for the separations of assets to be released and forward ported
	rfLogs.writeHEAD(status.ld.VR, "Assets to be released vs forward ported")
	rfLogs.write("Assets to be RELEASED", "INFO")
	rfLogs.writeVersions(status.assetsToBeReleased, "INFO")
	rfLogs.write("", "END")
	rfLogs.write("Assets to be FORWARD-PORTED", "INFO")
	rfLogs.writeVersions(status.assetsToBeForwardPorted, "INFO")

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

	// Fetch, checkout and map assets versions in the production and development branches
	releasedAssets, devAssets, err := s.getProdAndDevAssetsFromGit(git, tempDir)
	if err != nil {
		logrus.Errorf("Error while getting assets from production and development branches: %s", err)
		return err
	}

	// Compare the assets versions between the production and development branches
	s.compareReleasedAndDevAssets(releasedAssets, devAssets)
	logrus.Info("Comparison ended and logs saved in the logs directory")
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

// getProdAndDevAssetsFromGit will fetch and checkout the production and development branches,
// get the assets versions from the index.yaml file and return the maps for the assets versions.
func (s *Status) getProdAndDevAssetsFromGit(git *Git, tempDir string) (map[string][]Asset, map[string][]Asset, error) {
	// get filesystem and index file at the temporary directory
	tempDirRootFs := filesystem.GetFilesystem(tempDir)
	tempHelmIndexPath := filesystem.GetAbsPath(tempDirRootFs, path.RepositoryHelmIndexFile)

	// Fetch and checkout to the production branch
	err := git.fetchAndCheckoutBranch(s.ld.VR.prodBranch)
	if err != nil {
		logrus.Errorf("Error while fetching and checking out the production branch at: %s", err)
		return nil, nil, err
	}

	// Get the map for the released assets versions on the production branch
	releasedAssets, err := getAssetsMapFromIndex(tempHelmIndexPath, "", false)
	if err != nil {
		logrus.Errorf("Error while getting assets map from index: %s", err)
		return nil, nil, err
	}

	// Fetch and checkout to the development branch
	err = git.fetchAndCheckoutBranch(s.ld.VR.devBranch)
	if err != nil {
		logrus.Errorf("Error while fetching and checking out the development branch at: %s", err)
		return nil, nil, err
	}

	// Get the map for the development assets versions on the development branch
	devAssets, err := getAssetsMapFromIndex(tempHelmIndexPath, "", false)
	if err != nil {
		logrus.Errorf("Error while getting assets map from index: %s", err)
		return nil, nil, err
	}
	return releasedAssets, devAssets, nil
}

// compareReleasedAndDevAssets will compare the assets versions between
// the production and development branches, returning 4 different maps for further analysis.
func (s *Status) compareReleasedAndDevAssets(releasedAssets, developmentAssets map[string][]Asset) {

	releaseInLifecycle := make(map[string][]Asset)
	noReleaseOutLifecycle := make(map[string][]Asset)
	noReleaseInLifecycle := make(map[string][]Asset)
	releasedOutLifecycle := make(map[string][]Asset)
	/** Compare the assets versions between the production and development branches
	* assets released and in the lifecycle; therefore ok
	* assets not released and out of the lifecycle; therefore ok
	* assets not released and in the lifecycle; therefore it should be released...WARN
	* assets released and not in the lifecycle; therefore it should not be released...ERROR
	**/

	for devAsset, devVersions := range developmentAssets {

		// released assets versions to compare with
		releasedVersions := releasedAssets[devAsset]

		for _, devVersion := range devVersions {
			// check if the version is already released
			released := checkIfVersionIsReleased(devVersion.version, releasedVersions)
			// check if the version is in the lifecycle
			inLifecycle := s.ld.VR.CheckChartVersionForLifecycle(devVersion.version)

			switch {
			case released && inLifecycle:
				releaseInLifecycle[devAsset] = append(releaseInLifecycle[devAsset], devVersion)
			case !released && !inLifecycle:
				noReleaseOutLifecycle[devAsset] = append(noReleaseOutLifecycle[devAsset], devVersion)
			case !released && inLifecycle:
				noReleaseInLifecycle[devAsset] = append(noReleaseInLifecycle[devAsset], devVersion)
			case released && !inLifecycle:
				releasedOutLifecycle[devAsset] = append(releasedOutLifecycle[devAsset], devVersion)
			}
		}
	}

	s.assetsReleasedInLifecycle = releaseInLifecycle
	s.assetsNotReleasedOutLifecycle = noReleaseOutLifecycle
	s.assetsNotReleasedInLifecycle = noReleaseInLifecycle
	s.assetsReleasedOutLifecycle = releasedOutLifecycle
	return
}

// checkIfVersionIsReleased iterates a given version against the list of released versions
// and returns true if the version is found in the list of released versions.
func checkIfVersionIsReleased(version string, releasedVersions []Asset) bool {
	for _, releasedVersion := range releasedVersions {
		if version == releasedVersion.version {
			return true
		}
	}
	return false
}

// separateReleaseFromForwardPort will separate the assets to be released from the assets to be forward ported, the assets were loaded previously by listProdAndDevAssets function.
func (s *Status) separateReleaseFromForwardPort() {
	assetsToBeReleased := make(map[string][]Asset)
	assetsToBeForwardPorted := make(map[string][]Asset)

	for asset, versions := range s.assetsNotReleasedInLifecycle {
		for _, version := range versions {
			if toRelease := s.ld.VR.CheckChartVersionToRelease(version.version); toRelease {
				assetsToBeReleased[asset] = append(assetsToBeReleased[asset], version)
			} else {
				assetsToBeForwardPorted[asset] = append(assetsToBeForwardPorted[asset], version)
			}
		}
	}

	s.assetsToBeReleased = assetsToBeReleased
	s.assetsToBeForwardPorted = assetsToBeForwardPorted

	return
}
