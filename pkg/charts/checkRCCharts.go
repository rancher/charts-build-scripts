package charts

import (
	"context"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// CheckRCCharts checks for any charts that have RC versions
func CheckRCCharts(ctx context.Context, repoRoot string) (map[string][]string, error) {
	// Get the filesystem on the repo root
	repoFs := filesystem.GetFilesystem(repoRoot)

	// Load the release options from the release.yaml file
	releaseOptions, err := options.LoadReleaseOptionsFromFile(ctx, repoFs, "release.yaml")
	if err != nil {
		return nil, err
	}

	rcChartVersionMap := make(map[string][]string, 0)

	// Grab all charts that contain RC tags
	for chart := range releaseOptions {
		for _, version := range releaseOptions[chart] {
			if strings.Contains(version, "-rc") {
				rcChartVersionMap[chart] = append(rcChartVersionMap[chart], version)
			}
		}
	}

	return rcChartVersionMap, nil
}
