package lifecycle

import (
	"fmt"
	"os"

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
