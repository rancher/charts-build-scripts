package helm

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
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
		return fmt.Errorf("encountered error while checking if Helm index file already exists in repository: %s", err)
	} else if exists {
		helmIndexFile, err = helmRepo.LoadIndexFile(absRepositoryHelmIndexFile)
		if err != nil {
			return fmt.Errorf("encountered error while trying to load existing index file: %s", err)
		}
	} else {
		helmIndexFile = helmRepo.NewIndexFile()
	}

	// Generate the current index file from the assets/ directory
	newHelmIndexFile, err := helmRepo.IndexDirectory(absRepositoryAssetsDir, path.RepositoryAssetsDir)
	if err != nil {
		return fmt.Errorf("encountered error while trying to generate new Helm index: %s", err)
	}

	// Update index
	helmIndexFile, upToDate := UpdateIndex(helmIndexFile, newHelmIndexFile)

	if upToDate {
		logrus.Info("index.yaml is up-to-date")
		return nil
	}

	// Write new index to disk
	err = helmIndexFile.WriteFile(absRepositoryHelmIndexFile, os.ModePerm)
	if err != nil {
		return fmt.Errorf("encountered error while trying to write updated Helm index into index.yaml: %s", err)
	}
	logrus.Info("Generated index.yaml")
	return nil
}

// UpdateIndex updates the original index with the new contents
func UpdateIndex(original, new *helmRepo.IndexFile) (*helmRepo.IndexFile, bool) {
	// Create a copy of new to return
	updatedIndex := helmRepo.NewIndexFile()
	updatedIndex.Merge(new)
	upToDate := true

	// Preserve generated timestamp
	updatedIndex.Generated = original.Generated

	// Ensure newer version of chart is used if it has been updated
	for chartName, chartVersions := range updatedIndex.Entries {
		for i, chartVersion := range chartVersions {
			version := chartVersion.Version
			if !original.Has(chartName, version) {
				// Keep the newly generated chart version as-is
				upToDate = false
				logrus.Debugf("Chart %s has introduced a new version %s: %v", chartName, chartVersion.Version, *chartVersion)
				continue
			}
			// Get original chart version
			var originalChartVersion *helmRepo.ChartVersion
			for _, originalChartVersion = range original.Entries[chartName] {
				if originalChartVersion.Version == chartVersion.Version {
					// found originalChartVersion, which must exist since we checked that the original has it
					break
				}
			}
			// Try to preserve it only if nothing has changed.
			if originalChartVersion.Digest == chartVersion.Digest {
				// Don't modify created timestamp
				updatedIndex.Entries[chartName][i].Created = originalChartVersion.Created
			} else {
				upToDate = false
				logrus.Debugf("Chart %s at version %s has been modified", chartName, originalChartVersion.Version)
			}
		}
	}

	for chartName, chartVersions := range original.Entries {
		for _, chartVersion := range chartVersions {
			if !updatedIndex.Has(chartName, chartVersion.Version) {
				// Chart was removed
				upToDate = false
				logrus.Debugf("Chart %s at version %s has been removed: %v", chartName, chartVersion.Version, *chartVersion)
				continue
			}
		}
	}

	// Sort and return entries
	updatedIndex.SortEntries()
	return updatedIndex, upToDate
}

// OpenIndexYaml will check and open the index.yaml file in the local repository at the default file path
func OpenIndexYaml(rootFs billy.Filesystem) (*helmRepo.IndexFile, error) {
	helmIndexFilePath := filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile)

	exists, err := filesystem.PathExists(rootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("index.yaml file does not exist in the local repository")
	}

	return helmRepo.LoadIndexFile(helmIndexFilePath)
}
