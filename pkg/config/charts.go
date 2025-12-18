package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"gopkg.in/yaml.v2"
)

// ChartEntry represents a single chart configuration in trackCharts.yaml
type ChartEntry struct {
	Targets []string `yaml:"targets"`
	Status  string   `yaml:"status"`
	Icon    bool     `yaml:"icon"`
}

// TrackedCharts represents the structure of the trackCharts.yaml file
type TrackedCharts struct {
	Charts map[string]ChartEntry `yaml:"charts"`
}

// loadTrackedCharts loads the trackCharts.yaml file and returns the full TrackedCharts structure
// This function reads from config/trackCharts.yaml in the repository root
func loadTrackedCharts(ctx context.Context, rootFS billy.Filesystem) (*TrackedCharts, error) {
	var trackedCharts TrackedCharts

	// Check if file exists
	exists, err := filesystem.PathExists(ctx, rootFS, PathTrackChartsYaml)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("trackCharts.yaml file does not exist at path: %s", PathTrackChartsYaml)
	}

	// Read file
	fileBytes, err := os.ReadFile(filesystem.GetAbsPath(rootFS, PathTrackChartsYaml))
	if err != nil {
		return nil, fmt.Errorf("failed to read trackCharts.yaml: %w", err)
	}

	// Unmarshal YAML
	if err := yaml.Unmarshal(fileBytes, &trackedCharts); err != nil {
		return nil, fmt.Errorf("failed to parse trackCharts.yaml: %w", err)
	}

	return &trackedCharts, nil
}

// GetChartByName returns a specific chart entry by name or target name
// It intelligently handles both direct chart names (e.g., "fleet") and target names with suffixes (e.g., "fleet-crd")
// Known suffixes: -crd, -monitor, -agent
func (tc *TrackedCharts) GetChartByName(name string) *ChartEntry {
	// First, try direct lookup
	if chart, exists := tc.Charts[name]; exists {
		return &chart
	}

	// If not found, try stripping known suffixes to find parent chart
	knownSuffixes := []string{"-crd", "-monitor", "-agent"}
	for _, suffix := range knownSuffixes {
		if strings.HasSuffix(name, suffix) {
			parentName := strings.TrimSuffix(name, suffix)
			if chart, exists := tc.Charts[parentName]; exists {
				return &chart
			}
		}
	}

	return nil
}

// IsActive checks if a chart or any of its targets is tracked and active
// This leverages GetChartByName which handles both chart names and target names with suffixes
func (tc *TrackedCharts) IsActive(chartOrTargetName string) bool {
	chart := tc.GetChartByName(chartOrTargetName)
	return chart != nil && chart.Status == "active"
}

// IsActiveOrLegacy checks if a chart or any of its targets is tracked and active or legacy
// This leverages GetChartByName which handles both chart names and target names with suffixes
func (tc *TrackedCharts) IsActiveOrLegacy(chartOrTargetName string) bool {
	chart := tc.GetChartByName(chartOrTargetName)
	return chart != nil && (chart.Status == "active" || chart.Status == "legacy")
}
