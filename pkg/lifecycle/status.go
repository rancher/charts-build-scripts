package lifecycle

import "github.com/sirupsen/logrus"

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

	// List the production and development assets versions comparisons from the default branches

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
