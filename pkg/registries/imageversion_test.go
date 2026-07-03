package registries

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestSameMajor(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		available       []string
		wantLatest      string
		wantNeedsUpdate bool
	}{
		{
			name:            "newer minor available",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "v2.45.1", "v2.46.0"},
			wantLatest:      "v2.46.0",
			wantNeedsUpdate: true,
		},
		{
			name:            "newer patch available",
			current:         "v2.45.0",
			available:       []string{"v2.44.0", "v2.45.0", "v2.45.1"},
			wantLatest:      "v2.45.1",
			wantNeedsUpdate: true,
		},
		{
			name:            "newer major and patch available",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "v2.45.1", "v3.0.0"},
			wantLatest:      "v2.45.1",
			wantNeedsUpdate: true,
		},
		{
			name:            "already on latest",
			current:         "v2.46.0",
			available:       []string{"v2.45.0", "v2.46.0"},
			wantLatest:      "v2.46.0",
			wantNeedsUpdate: false,
		},
		{
			name:            "only higher major available, no update",
			current:         "v1.5.0",
			available:       []string{"v1.5.0", "v2.0.0"},
			wantLatest:      "v1.5.0",
			wantNeedsUpdate: false,
		},
		{
			name:            "pre-release tags skipped",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "v2.46.0-rc.1", "v2.46.0-alpha.1"},
			wantLatest:      "v2.45.0",
			wantNeedsUpdate: false,
		},
		{
			name:            "non-semver tags skipped",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "latest", "edge", "v2.46.0"},
			wantLatest:      "v2.46.0",
			wantNeedsUpdate: true,
		},
		{
			name:            "bare tags without v prefix",
			current:         "2.45.0",
			available:       []string{"2.45.0", "2.45.1"},
			wantLatest:      "2.45.1",
			wantNeedsUpdate: true,
		},
		{
			name:            "empty current returns no update",
			current:         "",
			available:       []string{"v1.0.0"},
			wantLatest:      "",
			wantNeedsUpdate: false,
		},
		{
			name:            "garbage current returns no update",
			current:         "not-a-version",
			available:       []string{"v1.0.0"},
			wantLatest:      "",
			wantNeedsUpdate: false,
		},
		{
			name:            "empty available list",
			current:         "v1.0.0",
			available:       []string{},
			wantLatest:      "",
			wantNeedsUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLatest, gotNeedsUpdate := latestSameMajor(tt.current, tt.available)
			assert.Equal(t, tt.wantNeedsUpdate, gotNeedsUpdate, "needsUpdate mismatch")
			if tt.wantLatest != "" {
				assert.Equal(t, tt.wantLatest, gotLatest, "latest tag mismatch")
			}
		})
	}
}

func TestValidateImageVersions(t *testing.T) {
	ctx := context.Background()

	t.Run("image needs update", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "102.0.0+up45.0.0", `
prometheus:
  image:
    repository: rancher/mirrored-prometheus-prometheus
    tag: v2.45.0
`)
		configPath := writeConfigFile(t, repoRoot, `images:
  - name: prometheus
    repository: rancher/mirrored-prometheus-prometheus
`)

		orig := fetchTagsFromRegistryRepo
		defer func() { fetchTagsFromRegistryRepo = orig }()
		fetchTagsFromRegistryRepo = func(_ context.Context, _, _ string) ([]string, error) {
			return []string{"v2.45.0", "v2.46.0", "v2.47.0"}, nil
		}

		report, err := ValidateImageVersions(ctx, repoRoot, "monitoring", "102.0.0+up45.0.0", configPath)
		require.NoError(t, err)
		assert.True(t, report.NeedsUpdate)
		require.Len(t, report.Images, 1)
		assert.Equal(t, "prometheus", report.Images[0].Name)
		assert.Equal(t, "v2.45.0", report.Images[0].CurrentTag)
		assert.Equal(t, "v2.47.0", report.Images[0].LatestAvailable)
		assert.True(t, report.Images[0].NeedsUpdate)
	})

	t.Run("image already up-to-date", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "102.0.0", `
image:
  repository: rancher/mirrored-prometheus-prometheus
  tag: v2.47.0
`)
		configPath := writeConfigFile(t, repoRoot, `images:
  - name: prometheus
    repository: rancher/mirrored-prometheus-prometheus
`)

		orig := fetchTagsFromRegistryRepo
		defer func() { fetchTagsFromRegistryRepo = orig }()
		fetchTagsFromRegistryRepo = func(_ context.Context, _, _ string) ([]string, error) {
			return []string{"v2.45.0", "v2.47.0"}, nil
		}

		report, err := ValidateImageVersions(ctx, repoRoot, "monitoring", "102.0.0", configPath)
		require.NoError(t, err)
		assert.False(t, report.NeedsUpdate)
		require.Len(t, report.Images, 1)
		assert.False(t, report.Images[0].NeedsUpdate)
	})

	t.Run("config image missing from chart", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "102.0.0", `
image:
  repository: rancher/some-other-image
  tag: v1.0.0
`)
		configPath := writeConfigFile(t, repoRoot, `images:
  - name: prometheus
    repository: rancher/mirrored-prometheus-prometheus
`)

		orig := fetchTagsFromRegistryRepo
		defer func() { fetchTagsFromRegistryRepo = orig }()
		fetchTagsFromRegistryRepo = func(_ context.Context, _, _ string) ([]string, error) {
			return []string{}, nil
		}

		report, err := ValidateImageVersions(ctx, repoRoot, "monitoring", "102.0.0", configPath)
		require.NoError(t, err)
		assert.False(t, report.NeedsUpdate)
		assert.Empty(t, report.Images)
		require.Len(t, report.MissingFromChart, 1)
		assert.Equal(t, "rancher/mirrored-prometheus-prometheus", report.MissingFromChart[0])
	})
}

func writeValuesYaml(t *testing.T, dir, chartName, version, content string) string {
	t.Helper()
	chartDir := filepath.Join(dir, "charts", chartName, version)
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	p := filepath.Join(chartDir, "values.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	return dir
}

func writeConfigFile(t *testing.T, dir, content string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "image-version-check-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}
