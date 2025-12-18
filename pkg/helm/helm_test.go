package helm

import (
	"context"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"helm.sh/helm/v3/pkg/chart"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

func TestSortVersions(t *testing.T) {
	tests := []struct {
		name     string
		input    helmRepo.ChartVersions
		expected []string // expected order of versions
	}{
		{
			name: "stable versions only - should sort descending",
			input: helmRepo.ChartVersions{
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0"}},
				{Metadata: &chart.Metadata{Version: "108.0.2+up0.9.2"}},
				{Metadata: &chart.Metadata{Version: "108.0.1+up0.9.1"}},
			},
			expected: []string{
				"108.0.2+up0.9.2",
				"108.0.1+up0.9.1",
				"108.0.0+up0.9.0",
			},
		},
		{
			name: "stable + RCs with same base - stable first, then RCs descending",
			input: helmRepo.ChartVersions{
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0-rc.1"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0-rc.2"}},
			},
			expected: []string{
				"108.0.0+up0.9.0",
				"108.0.0+up0.9.0-rc.2",
				"108.0.0+up0.9.0-rc.1",
			},
		},
		{
			name: "RCs only - should sort descending by RC number",
			input: helmRepo.ChartVersions{
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0-rc.1"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0-rc.3"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0-rc.2"}},
			},
			expected: []string{
				"108.0.0+up0.9.0-rc.3",
				"108.0.0+up0.9.0-rc.2",
				"108.0.0+up0.9.0-rc.1",
			},
		},
		{
			name: "mixed base versions with RCs - sort by semver first",
			input: helmRepo.ChartVersions{
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0-rc.1"}},
				{Metadata: &chart.Metadata{Version: "109.0.0+up0.10.0"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.9.0"}},
				{Metadata: &chart.Metadata{Version: "109.0.0+up0.10.0-rc.1"}},
			},
			expected: []string{
				"109.0.0+up0.10.0",
				"109.0.0+up0.10.0-rc.1",
				"108.0.0+up0.9.0",
				"108.0.0+up0.9.0-rc.1",
			},
		},
		{
			name: "alpha/beta/rc/stable - full prerelease hierarchy",
			input: helmRepo.ChartVersions{
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-alpha.2"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-rc.1"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-alpha.5"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-beta.1"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-alpha.1"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-rc.2"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-alpha.3"}},
				{Metadata: &chart.Metadata{Version: "108.0.0+up0.14.0-alpha.4"}},
			},
			expected: []string{
				"108.0.0+up0.14.0",      // stable first
				"108.0.0+up0.14.0-rc.2", // rc descending
				"108.0.0+up0.14.0-rc.1",
				"108.0.0+up0.14.0-beta.1",  // beta
				"108.0.0+up0.14.0-alpha.5", // alpha descending
				"108.0.0+up0.14.0-alpha.4",
				"108.0.0+up0.14.0-alpha.3",
				"108.0.0+up0.14.0-alpha.2",
				"108.0.0+up0.14.0-alpha.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock IndexFile
			index := &helmRepo.IndexFile{
				Entries: map[string]helmRepo.ChartVersions{
					"test-chart": tt.input,
				},
			}

			// Run the sort function
			SortVersions(index)

			// Check the order
			sorted := index.Entries["test-chart"]
			if len(sorted) != len(tt.expected) {
				t.Fatalf("expected %d versions, got %d", len(tt.expected), len(sorted))
			}

			for i, expectedVersion := range tt.expected {
				if sorted[i].Version != expectedVersion {
					t.Errorf("at index %d: expected %s, got %s", i, expectedVersion, sorted[i].Version)
				}
			}
		})
	}
}

func Test_CheckVersionStandards(t *testing.T) {
	cfg := &config.Config{
		VersionRules: &config.VersionRules{
			AllowedCandidates: []string{"-alpha", "-beta", "-rc"},
			AllowedMetadata:   []string{"-rancher"},
		},
	}
	ctx := config.WithConfig(context.Background(), cfg)

	tests := []struct {
		name        string
		input       map[string][]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "#1 - all allowed prerelease types",
			input: map[string][]string{
				"test-chart": {
					"108.0.0+up0.14.0",
					"108.0.0+up0.14.0-alpha.1",
					"108.0.0+up0.14.0-beta.2",
					"108.0.0+up0.14.0-rc.3",
					"108.0.0+up0.14.0-rancher.5",
				},
			},
			expectError: false,
		},
		{
			name: "#2 - stable versions only",
			input: map[string][]string{
				"test-chart": {
					"108.0.0+up0.14.0",
					"108.0.1+up0.14.1",
					"108.0.2+up0.14.2",
				},
			},
			expectError: false,
		},
		{
			name: "#3 - versions without build metadata",
			input: map[string][]string{
				"test-chart": {
					"1.0.0",
					"1.0.1",
				},
			},
			expectError: false,
		},
		{
			name: "#4 - dev prerelease",
			input: map[string][]string{
				"test-chart": {
					"108.0.0+up0.14.0-dev.1",
				},
			},
			expectError: true,
			errorMsg:    "contains invalid build metadata identifier.",
		},
		{
			name: "#5 - snapshot prerelease",
			input: map[string][]string{
				"test-chart": {
					"108.0.0+up0.14.0-snapshot.1",
				},
			},
			expectError: true,
			errorMsg:    "contains invalid build metadata identifier.",
		},
		{
			name: "#6 - preview prerelease",
			input: map[string][]string{
				"test-chart": {
					"108.0.0+up0.14.0-preview.1",
				},
			},
			expectError: true,
			errorMsg:    "contains invalid build metadata identifier.",
		},
		{
			name: "#7 - mixed valid and invalid",
			input: map[string][]string{
				"test-chart": {
					"108.0.0+up0.14.0-alpha.1",
					"108.0.0+up0.14.0-custom.1",
				},
			},
			expectError: true,
			errorMsg:    "contains invalid build metadata identifier.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run the validation function
			err := CheckVersionStandards(ctx, tt.input)

			// Check if error expectation matches
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %s", err.Error())
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
