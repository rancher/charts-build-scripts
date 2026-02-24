package auto

import (
	"context"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/stretchr/testify/assert"
)

func Test_listAssetsToForwardPort(t *testing.T) {
	type input struct {
		source map[string][]string
		target map[string][]string
		cfg    *config.Config
	}
	type expected struct {
		toForwardPort map[string][]string
	}
	type test struct {
		name     string
		input    input
		expected expected
	}

	versionRules := &config.VersionRules{
		AllowedCandidates: []string{"-alpha", "-beta", "-rc"},
		AllowedMetadata:   []string{"-rancher"},
		Rules: map[string]int{
			"2.14": 109,
		},
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				source: map[string][]string{
					"test-chart":     {"108.0.0+up0.0.1"},
					"test-chart-crd": {"108.0.0+up0.0.1"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart", "test-chart-crd"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart":     {"108.0.0+up0.0.1"},
					"test-chart-crd": {"108.0.0+up0.0.1"},
				},
			},
		},

		{
			name: "#2",
			input: input{
				source: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-rc.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-rc.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart", "test-chart-crd"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart":     {"108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
			},
		},

		{
			name: "#3",
			input: input{
				source: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-alpha.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-beta.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart", "test-chart-crd"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart":     {"108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
			},
		},

		{
			name: "#4",
			input: input{
				source: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart", "test-chart-crd"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
			},
		},

		{
			name: "#5",
			input: input{
				source: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart", "test-chart-crd"},
								Status:  "legacy",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
			},
		},

		{
			name: "#6",
			input: input{
				source: map[string][]string{
					"test-chart":     {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-crd": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart", "test-chart-crd"},
								Status:  "deprecated",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{},
			},
		},

		{
			name: "#7",
			input: input{
				source: map[string][]string{
					"test-chart":       {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"deprecated-chart": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart": {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
			},
		},

		{
			name: "#8",
			input: input{
				source: map[string][]string{
					"test-chart":             {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-but-removed": {"108.0.0+up1.0.0-rancher0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart": {
								Targets: []string{"test-chart"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart": {"108.0.0+up1.0.0-rancher.1", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
			},
		},

		{
			name: "#9",
			input: input{
				source: map[string][]string{
					"test-chart-1":     {"109.2.0+up2.0.0-rc.1", "109.2.0+up2.0.0-beta-1", "109.2.0+up2.0.0-alpha1", "109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-1-crd": {"109.2.0+up2.0.0-rc.1", "109.2.0+up2.0.0-beta-1", "109.2.0+up2.0.0-alpha1", "109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart-1": {
								Targets: []string{"test-chart-1", "test-chart-1-crd"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart-1":     {"109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-1-crd": {"109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"}},
			},
		},

		{
			name: "#10",
			input: input{
				source: map[string][]string{
					"test-chart-1":         {"109.2.0+up2.0.0-rc.1", "109.2.0+up2.0.0-beta-1", "109.2.0+up2.0.0-alpha1", "109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-1-crd":     {"109.2.0+up2.0.0-rc.1", "109.2.0+up2.0.0-beta-1", "109.2.0+up2.0.0-alpha1", "109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-2":         {"109.2.0+up2.0.0"},
					"test-chart-3":         {"109.0.0+up1.0.0", "108.0.0+up1.0.0", "109.0.0+up1.0.0-rc.0"},
					"test-chart-3-monitor": {"109.0.0+up1.0.0", "108.0.0+up1.0.0", "109.0.0+up1.0.0-rancher.0"},
				},
				cfg: &config.Config{
					TrackedCharts: &config.TrackedCharts{
						Charts: map[string]config.ChartEntry{
							"test-chart-1": {
								Targets: []string{"test-chart-1", "test-chart-1-crd"},
								Status:  "legacy",
							},
							"test-chart-2": {
								Targets: []string{"test-chart-2"},
								Status:  "deprecated",
							},
							"test-chart-3": {
								Targets: []string{"test-chart-3", "test-chart-3-monitor"},
								Status:  "active",
							},
						},
					},
					VersionRules: versionRules,
				},
			},
			expected: expected{
				toForwardPort: map[string][]string{
					"test-chart-1":         {"109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-1-crd":     {"109.2.0+up2.0.0", "108.0.0+up0.0.1", "107.0.0+up1.0.0"},
					"test-chart-3":         {"109.0.0+up1.0.0", "108.0.0+up1.0.0"},
					"test-chart-3-monitor": {"109.0.0+up1.0.0", "108.0.0+up1.0.0", "109.0.0+up1.0.0-rancher.0"},
				},
			},
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := listAssetsToForwardPort(ctx, tc.input.source, tc.input.target, tc.input.cfg)
			assert.Equal(t, tc.expected.toForwardPort, result)
		})
	}
}
