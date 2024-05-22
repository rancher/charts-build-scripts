package lifecycle

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// getAssetsMapFromIndex returns a map of assets with their version and
// an empty path that will be populated later by populateAssetsVersionsPath()
func getAssetsMapFromIndex(absRepositoryHelmIndexFile, currentChart string, debug bool) (map[string][]Asset, error) {
	fmt.Println(os.Getwd())
	helmIndexFile, err := helmRepo.LoadIndexFile(absRepositoryHelmIndexFile)
	if err != nil {
		return nil, fmt.Errorf("encountered error while trying to load existing index file: %s", err)
	}

	var assetsMap = make(map[string][]Asset)
	var annotatedVersions []Asset

	switch {
	case currentChart == "":
		cycleLog(debug, "Current chart is empty, getting all charts", nil)
		for _, entry := range helmIndexFile.Entries {
			for _, chartVersion := range entry {
				annotatedVersions = append(annotatedVersions, Asset{
					version: chartVersion.Version,
				})
			}
			assetsMap[entry[0].Name] = annotatedVersions
			annotatedVersions = nil // Reset the slice for the next iteration
		}

	case currentChart != "":
		cycleLog(debug, "Target chart is", currentChart)
		if _, ok := helmIndexFile.Entries[currentChart]; !ok {
			return nil, fmt.Errorf("chart %s not found in the index file", currentChart)
		}
		for _, chartVersion := range helmIndexFile.Entries[currentChart] {
			annotatedVersions = append(annotatedVersions, Asset{
				version: chartVersion.Version,
				// path:    chartVersion.URLs[0], we can't trust this field
			})
		}
		assetsMap[currentChart] = annotatedVersions
	}

	return assetsMap, nil
}

// populateAssetsVersionsPath will combine the information from the index.yaml file and the assets directory to get the path of each asset version for each chart.
// It will populate the assetsVersionsMap with the path of the assets.
// It walks through the assets directory and compares the version of the assets with the version of the assets in the index.yaml file.
func (ld *Dependencies) populateAssetsVersionsPath(debug bool) error {
	// Return a complet map of assets with their version and path in the repository
	var assetsVersionsMap = make(map[string][]Asset)

	// Lets see what we have on assets/<chart> dir
	// doFunc is callback function passed as an argument to the WalkDir function
	// WalkDir is expected to call doFunc for each file or directory it encounters
	// during its traversal of the directory specified by dirPath
	// All results will be appended fo filePaths
	var filePaths []string
	doFunc := func(fs billy.Filesystem, path string, isDir bool) error {
		if !isDir {
			filePaths = append(filePaths, path)
		}
		return nil
	}

	// Range through the assetsMap and get the path of the assets
	for chart, assets := range ld.assetsVersionsMap {

		dirPath := fmt.Sprintf("assets/%s", chart)
		cycleLog(debug, "Getting assets at path", dirPath)

		err := ld.walkDirWrapper(ld.rootFs, dirPath, doFunc)
		if err != nil {
			return fmt.Errorf("encountered error while walking through the assets directory: %w", err)
		}

		// Now we have the path of the assets, at filePaths slice
		for _, asset := range assets {
			for _, filePath := range filePaths {
				// Ranging through assets and filePaths to get the version of the asset
				version := strings.TrimPrefix(filePath, dirPath+"/"+chart+"-")
				version = strings.TrimSuffix(version, ".tgz")
				// Compare the received slice of paths with the current versions in assets
				// lets append the path to the assetsVersionsMap
				if asset.version == version {
					cycleLog(debug, "adding asset to map", filePath)
					asset.path = filePath
					assetsVersionsMap[chart] = append(assetsVersionsMap[chart], asset)
				}
			}
		}
		// Reset filePaths slice to be used again in the next iteration through the next asset
		filePaths = nil
	}
	// Reset and assign new assetsVersionsMap to the struct
	ld.assetsVersionsMap = nil
	ld.assetsVersionsMap = assetsVersionsMap

	// Now fileNames slice contains the names of all files in the directories
	return nil
}

// sortAssetsVersions will convert to semver and
// sort the assets for each key in the assetsVersionsMap
func (ld *Dependencies) sortAssetsVersions() {
	// Iterate over the map and sort the assets for each key
	for k, assets := range ld.assetsVersionsMap {
		sort.Slice(assets, func(i, j int) bool {
			vi, _ := semver.NewVersion(assets[i].version)
			vj, _ := semver.NewVersion(assets[j].version)
			return vi.LessThan(vj)
		})
		ld.assetsVersionsMap[k] = assets
	}

	return
}
