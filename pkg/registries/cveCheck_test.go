package registries

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckChartCVEs(t *testing.T) {
	ctx := context.Background()

	t.Run("delta against previous version", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "103.1.0+up45.0.1", `
image:
  repository: rancher/prometheus
  tag: v2.47.0
`)
		writeValuesYaml(t, repoRoot, "monitoring", "103.0.0+up45.0.0", `
image:
  repository: rancher/prometheus
  tag: v2.46.0
`)
		writeIndexYaml(t, repoRoot, "monitoring", "103.1.0+up45.0.1", "103.0.0+up45.0.0")

		orig := scanImage
		defer func() { scanImage = orig }()
		scanImage = func(_ context.Context, repository, tag string) (SeverityCounts, error) {
			switch fmt.Sprintf("%s:%s", repository, tag) {
			case "rancher/prometheus:v2.47.0":
				return SeverityCounts{Critical: 1, High: 2, Medium: 3, Low: 4, Unknown: 1}, nil
			case "rancher/prometheus:v2.46.0":
				return SeverityCounts{Critical: 3, High: 1, Medium: 3, Low: 2, Unknown: 2}, nil
			}
			return SeverityCounts{}, fmt.Errorf("unexpected image %s:%s", repository, tag)
		}

		report, err := CheckChartCVEs(ctx, repoRoot, "monitoring", "103.1.0+up45.0.1")
		require.NoError(t, err)

		assert.Equal(t, SeverityCounts{Critical: 1, High: 2, Medium: 3, Low: 4, Unknown: 1}, report.CVECounts)
		require.Len(t, report.Images, 1)
		assert.Equal(t, "rancher/prometheus", report.Images[0].Repository)
		assert.Equal(t, "v2.47.0", report.Images[0].Tag)

		assert.Equal(t, "103.0.0+up45.0.0", report.PreviousVersion)
		require.NotNil(t, report.Delta)
		assert.Equal(t, SeverityCounts{Critical: -2, High: 1, Medium: 0, Low: 2, Unknown: -1}, *report.Delta)
	})

	t.Run("no previous version", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "103.0.0+up45.0.0", `
image:
  repository: rancher/prometheus
  tag: v2.46.0
`)
		writeIndexYaml(t, repoRoot, "monitoring", "103.0.0+up45.0.0")

		orig := scanImage
		defer func() { scanImage = orig }()
		scanImage = func(_ context.Context, _, _ string) (SeverityCounts, error) {
			return SeverityCounts{Critical: 1}, nil
		}

		report, err := CheckChartCVEs(ctx, repoRoot, "monitoring", "103.0.0+up45.0.0")
		require.NoError(t, err)

		assert.Equal(t, SeverityCounts{Critical: 1}, report.CVECounts)
		assert.Empty(t, report.PreviousVersion)
		assert.Nil(t, report.Delta)
	})
}

func TestFindPreviousSameMajorVersion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		chart     string
		version   string
		want      string
		wantFound bool
	}{
		{
			name:      "picks previous patch",
			chart:     "monitoring",
			version:   "103.1.0+up45.0.1",
			want:      "103.0.0+up45.0.0",
			wantFound: true,
		},
		{
			name:      "version not yet in index",
			chart:     "monitoring",
			version:   "103.2.0+up45.0.2",
			want:      "103.1.0+up45.0.1",
			wantFound: true,
		},
		{
			name:      "major bump has nothing to compare against",
			chart:     "monitoring",
			version:   "104.0.0+up46.0.0",
			wantFound: false,
		},
		{
			name:      "prerelease in between is skipped",
			chart:     "rancher-istio",
			version:   "103.1.0",
			want:      "103.0.0",
			wantFound: true,
		},
		{
			name:      "chart not tracked in index",
			chart:     "unknown-chart",
			version:   "1.0.0",
			wantFound: false,
		},
	}

	repoRoot := t.TempDir()
	writeIndexYaml(t, repoRoot, "monitoring", "103.1.0+up45.0.1", "103.0.0+up45.0.0", "102.0.0+up44.0.0")
	writeIndexYaml(t, repoRoot, "rancher-istio", "103.1.0", "103.1.0-rc.1", "103.0.0")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := findPreviousSameMajorVersion(ctx, repoRoot, tt.chart, tt.version)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTrivyReport(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    SeverityCounts
		wantErr bool
	}{
		{
			name: "counts every severity",
			json: `{"Results": [{"Vulnerabilities": [
				{"Severity": "CRITICAL"},
				{"Severity": "HIGH"},
				{"Severity": "HIGH"},
				{"Severity": "MEDIUM"},
				{"Severity": "LOW"},
				{"Severity": "UNKNOWN"}
			]}]}`,
			want: SeverityCounts{Critical: 1, High: 2, Medium: 1, Low: 1, Unknown: 1},
		},
		{
			name: "aggregates across results",
			json: `{"Results": [
				{"Vulnerabilities": [{"Severity": "CRITICAL"}]},
				{"Vulnerabilities": [{"Severity": "CRITICAL"}]}
			]}`,
			want: SeverityCounts{Critical: 2},
		},
		{
			name: "no vulnerabilities",
			json: `{"Results": []}`,
			want: SeverityCounts{},
		},
		{
			name:    "malformed json",
			json:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTrivyReport([]byte(tt.json))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// writeIndexYaml appends a chart's versions to <repoRoot>/index.yaml, creating it on first use.
func writeIndexYaml(t *testing.T, repoRoot, chart string, versions ...string) {
	t.Helper()

	block := fmt.Sprintf("  %s:\n", chart)
	for _, version := range versions {
		block += fmt.Sprintf("  - apiVersion: v2\n    name: %s\n    version: %q\n", chart, version)
	}

	path := filepath.Join(repoRoot, "index.yaml")
	existing, err := os.ReadFile(path)
	if err != nil {
		require.NoError(t, os.WriteFile(path, []byte("apiVersion: v1\nentries:\n"+block), 0644))
		return
	}
	require.NoError(t, os.WriteFile(path, append(existing, []byte(block)...), 0644))
}
