package helm

import (
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// CreateOrUpdateHelmIndex either creates or updates the index.yaml for the repository this package is within
func CreateOrUpdateHelmIndex(rootFs billy.Filesystem) error {
	absRepositoryAssetsDir := filesystem.GetAbsPath(rootFs, path.RepositoryAssetsDir)
	absRepositoryHelmIndexFile := filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile)

	var helmIndexFile *helmRepo.IndexFile

	// Load index file from disk if it exists
	exists, err := filesystem.PathExists(rootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return fmt.Errorf("Encountered error while checking if Helm index file already exists in repository: %s", err)
	} else if exists {
		helmIndexFile, err = helmRepo.LoadIndexFile(absRepositoryHelmIndexFile)
		if err != nil {
			return fmt.Errorf("Encountered error while trying to load existing index file: %s", err)
		}
	} else {
		helmIndexFile = helmRepo.NewIndexFile()
	}

	// Generate the current index file from the assets/ directory
	newHelmIndexFile, err := helmRepo.IndexDirectory(absRepositoryAssetsDir, path.RepositoryAssetsDir)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to generate new Helm index: %s", err)
	}

	// Merge the indices and sort them
	helmIndexFile.Merge(newHelmIndexFile)
	helmIndexFile.SortEntries()

	// Write new index to disk
	err = helmIndexFile.WriteFile(absRepositoryHelmIndexFile, os.ModePerm)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to write updated Helm index into index.yaml: %s", err)
	}
	return nil
}
