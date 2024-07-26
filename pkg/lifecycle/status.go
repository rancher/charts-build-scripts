package lifecycle

import (
	"fmt"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

// Status struct hold the results of the assets versions comparison,
// this data will all be logged and saves into log files for further analysis
type Status struct {
	ld                              *Dependencies      `json:"-"`
	StateFile                       string             `json:"state_file"`
	AssetsInLifecycleCurrentBranch  map[string][]Asset `json:"in_lifecycle_current_branch"`
	AssetsOutLifecycleCurrentBranch map[string][]Asset `json:"out_lifecycle_current_branch"`
	AssetsReleasedInLifecycle       map[string][]Asset `json:"released_in_lifecycle"`      // OK if not empty
	AssetsNotReleasedOutLifecycle   map[string][]Asset `json:"not_released_out_lifecycle"` // OK if not empty
	AssetsNotReleasedInLifecycle    map[string][]Asset `json:"not_released_in_lifecycle"`  // WARN if not empty
	AssetsReleasedOutLifecycle      map[string][]Asset `json:"released_out_lifecycle"`     // ERROR if not empty
	AssetsToBeReleased              map[string][]Asset `json:"to_be_released"`
	AssetsToBeForwardPorted         map[string][]Asset `json:"to_be_forward_ported"`
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
		errList := fmt.Errorf("Error while comparing production and development branches: %s", err)
		logrus.Error(errList)
		return status, errList
	}

	// Separate the assets to be released from the assets to be forward ported after the comparison
	err = status.separateReleaseFromForwardPort()
	if err != nil {
		errSeparating := fmt.Errorf("failed to separate releases from forward-ports: %v", err)
		logrus.Error(errSeparating)
		return status, errSeparating
	}

	return status, nil
}

// createLogFiles will create the log files for the current branch, production and development branches
// and the assets to be released and forward ported, returning the logs objects for each file.
func createLogFiles(chart string) (*Logs, *Logs, *Logs, error) {
	// Create the logs infrastructure in the filesystem for:
	// current branch logs
	cbLogs, err := CreateLogs("current-branch.log", chart)
	if err != nil {
		logrus.Errorf("Error while creating logs: %s", err)
		return nil, nil, nil, err
	}

	// production and development branches logs
	pdLogs, err := CreateLogs("production-x-development.log", chart)
	if err != nil {
		logrus.Errorf("Error while creating logs: %s", err)
		return nil, nil, nil, err
	}

	// released and forward ported logs
	rfLogs, err := CreateLogs("released-x-forward-ported.log", chart)
	if err != nil {
		logrus.Errorf("Error while creating logs: %s", err)
		return nil, nil, nil, err
	}

	return cbLogs, pdLogs, rfLogs, nil
}

// CheckLifecycleStatusAndSave checks the lifecycle status of the assets
// at 3 different levels prints to the console and saves to log files at 'logs/' folder.
func (ld *Dependencies) CheckLifecycleStatusAndSave(chart string) (*Status, error) {

	// Get the status of the assets versions
	status, err := ld.getStatus()
	if err != nil {
		logrus.Errorf("Error while getting the status: %s", err)
		return nil, err
	}
	// Create the logs infrastructure in the filesystem and close them once the function ends
	cbLogs, pdLogs, rfLogs, err := createLogFiles(chart)
	if err != nil {
		return status, err
	}
	defer cbLogs.File.Close()
	defer pdLogs.File.Close()
	defer rfLogs.File.Close()

	// optional filter logs by specific chart
	if chart != "" {
		status.AssetsInLifecycleCurrentBranch = map[string][]Asset{chart: status.AssetsInLifecycleCurrentBranch[chart]}
		status.AssetsOutLifecycleCurrentBranch = map[string][]Asset{chart: status.AssetsOutLifecycleCurrentBranch[chart]}
		status.AssetsReleasedInLifecycle = map[string][]Asset{chart: status.AssetsReleasedInLifecycle[chart]}
		status.AssetsNotReleasedOutLifecycle = map[string][]Asset{chart: status.AssetsNotReleasedOutLifecycle[chart]}
		status.AssetsNotReleasedInLifecycle = map[string][]Asset{chart: status.AssetsNotReleasedInLifecycle[chart]}
		status.AssetsReleasedOutLifecycle = map[string][]Asset{chart: status.AssetsReleasedOutLifecycle[chart]}
		status.AssetsToBeReleased = map[string][]Asset{chart: status.AssetsToBeReleased[chart]}
		status.AssetsToBeForwardPorted = map[string][]Asset{chart: status.AssetsToBeForwardPorted[chart]}
	}

	// ##############################################################################
	// Save the logs for the current branch
	cbLogs.WriteHEAD(status.ld.VR, "Assets versions vs the lifecycle rules in the current branch")
	cbLogs.Write("Versions INSIDE the lifecycle in the current branch", "INFO")
	cbLogs.WriteVersions(status.AssetsInLifecycleCurrentBranch, "INFO")
	cbLogs.Write("", "END")
	cbLogs.Write("Versions OUTSIDE the lifecycle in the current branch", "WARN")
	cbLogs.WriteVersions(status.AssetsOutLifecycleCurrentBranch, "WARN")
	cbLogs.Write("", "END")

	// ##############################################################################
	// Save the logs for the comparison between production and development branches
	pdLogs.WriteHEAD(status.ld.VR, "Released assets vs development assets with lifecycle rules")
	pdLogs.Write("Assets RELEASED and Inside the lifecycle", "INFO")
	pdLogs.Write("At the production branch: "+status.ld.VR.ProdBranch, "INFO")
	pdLogs.WriteVersions(status.AssetsReleasedInLifecycle, "INFO")
	pdLogs.Write("", "END")

	pdLogs.Write("Assets NOT released and Out of the lifecycle", "INFO")
	pdLogs.Write("At the development branch: "+status.ld.VR.DevBranch, "INFO")
	pdLogs.WriteVersions(status.AssetsNotReleasedOutLifecycle, "INFO")
	pdLogs.Write("", "END")

	pdLogs.Write("Assets NOT released and Inside the lifecycle", "WARN")
	pdLogs.Write("At the development branch: "+status.ld.VR.DevBranch, "WARN")
	pdLogs.WriteVersions(status.AssetsNotReleasedInLifecycle, "WARN")
	pdLogs.Write("", "END")

	pdLogs.Write("Assets released and Out of the lifecycle", "ERROR")
	pdLogs.Write("At the production branch: "+status.ld.VR.ProdBranch, "ERROR")
	pdLogs.WriteVersions(status.AssetsReleasedOutLifecycle, "ERROR")
	pdLogs.Write("", "END")

	// ##############################################################################
	// Save the logs for the separations of assets to be released and forward ported
	rfLogs.WriteHEAD(status.ld.VR, "Assets to be released vs forward ported")
	rfLogs.Write("Assets to be RELEASED", "INFO")
	rfLogs.WriteVersions(status.AssetsToBeReleased, "INFO")
	rfLogs.Write("", "END")
	rfLogs.Write("Assets to be FORWARD-PORTED", "INFO")
	rfLogs.WriteVersions(status.AssetsToBeForwardPorted, "INFO")

	err = status.initState()
	if err != nil {
		logrus.Errorf("Error while initializing the state: %s", err)
		return status, err
	}

	return status, nil
}

// listCurrentAssetsVersionsOnTheCurrentBranch returns the Status struct by reference
// with 2 maps of assets versions, one for the assets that are in the lifecycle,
// and another for the assets that are outside of the lifecycle.
func (s *Status) listCurrentAssetsVersionsOnTheCurrentBranch() {
	insideLifecycle := make(map[string][]Asset)
	outsideLifecycle := make(map[string][]Asset)

	for asset, versions := range s.ld.assetsVersionsMap {
		for _, version := range versions {
			inLifecycle := s.ld.VR.CheckChartVersionForLifecycle(version.Version)
			if inLifecycle {
				insideLifecycle[asset] = append(insideLifecycle[asset], version)
			} else {
				outsideLifecycle[asset] = append(outsideLifecycle[asset], version)
			}
		}
	}

	s.AssetsInLifecycleCurrentBranch = insideLifecycle
	s.AssetsOutLifecycleCurrentBranch = outsideLifecycle

	return
}

// listProdAndDevAssets will clone the charts repository at a temporary directory,
// fetch and checkout in the production and development branches for the given version,
// get the assets versions from the index.yaml file and compare the assets versions,
// separating into 4 different maps for further analysis.
func (s *Status) listProdAndDevAssets() error {

	// Open current charts git repository
	git, err := git.OpenGitRepo(s.ld.Git.Dir)
	if err != nil {
		return err
	}

	oldCurrentBranch := git.Branch

	// Fetch, checkout and map assets versions in the production and development branches
	releasedAssets, devAssets, err := s.getProdAndDevAssetsFromGit(git)
	if err != nil {
		logrus.Errorf("Error while getting assets from production and development branches: %s", err)
		return err
	}

	// Compare the assets versions between the production and development branches
	s.compareReleasedAndDevAssets(releasedAssets, devAssets)
	logrus.Info("Comparison ended and logs saved in the logs directory")

	return git.CheckoutBranch(oldCurrentBranch)
}

// getProdAndDevAssetsFromGit will fetch and checkout the production and development branches,
// get the assets versions from the index.yaml file and return the maps for the assets versions.
func (s *Status) getProdAndDevAssetsFromGit(git *git.Git) (map[string][]Asset, map[string][]Asset, error) {
	// get filesystem and index file at the temporary directory
	rootFs := filesystem.GetFilesystem(s.ld.Git.Dir)
	helmIndexPath := filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile)

	// Fetch and checkout to the production branch
	err := git.FetchAndCheckoutBranch(s.ld.VR.ProdBranch)
	if err != nil {
		logrus.Errorf("Error while fetching and checking out the production branch at: %s", err)
		return nil, nil, err
	}

	// Get the map for the released assets versions on the production branch
	releasedAssets, err := getAssetsMapFromIndex(helmIndexPath, "", false)
	if err != nil {
		logrus.Errorf("Error while getting assets map from index: %s", err)
		return nil, nil, err
	}

	// Fetch and checkout to the development branch
	err = git.FetchAndCheckoutBranch(s.ld.VR.DevBranch)
	if err != nil {
		logrus.Errorf("Error while fetching and checking out the development branch at: %s", err)
		return nil, nil, err
	}

	// Get the map for the development assets versions on the development branch
	devAssets, err := getAssetsMapFromIndex(helmIndexPath, "", false)
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
			released := checkIfVersionIsReleased(devVersion.Version, releasedVersions)
			// check if the version is in the lifecycle
			inLifecycle := s.ld.VR.CheckChartVersionForLifecycle(devVersion.Version)

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

	s.AssetsReleasedInLifecycle = releaseInLifecycle
	s.AssetsNotReleasedOutLifecycle = noReleaseOutLifecycle
	s.AssetsNotReleasedInLifecycle = noReleaseInLifecycle
	s.AssetsReleasedOutLifecycle = releasedOutLifecycle
	return
}

// checkIfVersionIsReleased iterates a given version against the list of released versions
// and returns true if the version is found in the list of released versions.
func checkIfVersionIsReleased(version string, releasedVersions []Asset) bool {
	for _, releasedVersion := range releasedVersions {
		if version == releasedVersion.Version {
			return true
		}
	}
	return false
}

// separateReleaseFromForwardPort will separate the assets to be released from the assets to be forward ported, the assets were loaded previously by listProdAndDevAssets function.
func (s *Status) separateReleaseFromForwardPort() error {
	assetsToBeReleased := make(map[string][]Asset)
	assetsToBeForwardPorted := make(map[string][]Asset)

	for asset, versions := range s.AssetsNotReleasedInLifecycle {
		for _, version := range versions {
			toRelease, err := s.ld.VR.CheckChartVersionToRelease(version.Version)
			if err != nil {
				return err
			}
			isRCVersion := s.ld.VR.CheckForRCVersion(version.Version)
			if isRCVersion {
				continue
			}
			if toRelease {
				assetsToBeReleased[asset] = append(assetsToBeReleased[asset], version)
			} else {
				assetsToBeForwardPorted[asset] = append(assetsToBeForwardPorted[asset], version)
			}
		}
	}

	s.AssetsToBeReleased = assetsToBeReleased
	s.AssetsToBeForwardPorted = assetsToBeForwardPorted

	return nil
}
