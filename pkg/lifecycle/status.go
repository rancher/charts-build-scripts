package lifecycle

// CheckLifecycleStatusAndSave checks the lifecycle status of the assets at 3 different levels:
// 1. The current branch
// 2. The comparison between production and development branches
// 3. The separations of assets to be released and forward ported after the comparison in 2.
func (ld *Dependencies) CheckLifecycleStatusAndSave(chart string) error {

	// Get the status of the assets versions

	// Create the logs infrastructure in the filesystem

	// ##############################################################################
	// Save the logs for the current branch

	// ##############################################################################
	// Save the logs for the comparison between production and development branches

	// ##############################################################################
	// Save the logs for the separations of assets to be released and forward ported

	return nil
}
