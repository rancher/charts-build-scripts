package helm

import (
	"testing"

	helmRepo "helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/chart"
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
				"108.0.0+up0.14.0",        // stable first
				"108.0.0+up0.14.0-rc.2",   // rc descending
				"108.0.0+up0.14.0-rc.1",
				"108.0.0+up0.14.0-beta.1", // beta
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
