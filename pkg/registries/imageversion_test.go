package registries

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateImageVersions(t *testing.T) {
	ctx := context.Background()
	LoadImageVersionList = func(_ context.Context) (*config.ImageVersionCheckOptions, error) {
		return &config.ImageVersionCheckOptions{
			"kuberlr": {
				Repository: "rancher/kuberlr-kubectl",
			},
		}, nil
	}
	t.Run("image needs update", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "102.0.0+up45.0.0", `
prometheus:
  image:
    repository: rancher/kuberlr-kubectl
    tag: v2.45.0
`)

		orig := fetchTagsFromRegistryRepo
		defer func() { fetchTagsFromRegistryRepo = orig }()
		fetchTagsFromRegistryRepo = func(_ context.Context, _, _ string) ([]string, error) {
			return []string{"v2.45.0", "v2.46.0", "v2.47.0"}, nil
		}

		report, err := ValidateImageVersions(ctx, repoRoot, "monitoring", "102.0.0+up45.0.0")
		require.NoError(t, err)
		assert.True(t, report.NeedsUpdate)
		require.Len(t, report.Images, 1)
		assert.Equal(t, "v2.45.0", report.Images[0].CurrentTag)
		assert.Equal(t, "v2.47.0", report.Images[0].LatestAvailable)
		assert.True(t, report.Images[0].NeedsUpdate)
	})

	t.Run("image already up-to-date", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeValuesYaml(t, repoRoot, "monitoring", "102.0.0", `
image:
  repository: rancher/kuberlr-kubectl
  tag: v2.47.0
`)
		orig := fetchTagsFromRegistryRepo
		defer func() { fetchTagsFromRegistryRepo = orig }()
		fetchTagsFromRegistryRepo = func(_ context.Context, _, _ string) ([]string, error) {
			return []string{"v2.45.0", "v2.47.0"}, nil
		}

		report, err := ValidateImageVersions(ctx, repoRoot, "monitoring", "102.0.0")
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

		orig := fetchTagsFromRegistryRepo
		defer func() { fetchTagsFromRegistryRepo = orig }()
		fetchTagsFromRegistryRepo = func(_ context.Context, _, _ string) ([]string, error) {
			return []string{}, nil
		}

		report, err := ValidateImageVersions(ctx, repoRoot, "monitoring", "102.0.0")
		require.NoError(t, err)
		assert.False(t, report.NeedsUpdate)
		assert.Empty(t, report.Images)
		require.Len(t, report.MissingFromChart, 1)
		assert.Equal(t, "rancher/kuberlr-kubectl", report.MissingFromChart[0])
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
