package charts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveDependencyURL(t *testing.T) {
	t.Run("OCI repository", func(t *testing.T) {
		tests := []struct {
			name       string
			repository string
			depName    string
			version    string
			expected   string
		}{
			{
				name:       "bitnami postgresql",
				repository: "oci://registry-1.docker.io/bitnamicharts",
				depName:    "postgresql",
				version:    "14.1.10",
				expected:   "oci://registry-1.docker.io/bitnamicharts/postgresql:14.1.10",
			},
			{
				name:       "trailing slash on repository is trimmed",
				repository: "oci://registry-1.docker.io/bitnamicharts/",
				depName:    "redis",
				version:    "18.14.1",
				expected:   "oci://registry-1.docker.io/bitnamicharts/redis:18.14.1",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := resolveDependencyURL(tt.repository, tt.depName, tt.version)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			})
		}
	})

	t.Run("classic HTTP(S) repository still resolves via index.yaml", func(t *testing.T) {
		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		defer server.Close()

		mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/yaml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`apiVersion: v1
entries:
  mychart:
    - name: mychart
      version: 1.0.0
      urls:
        - mychart-1.0.0.tgz
generated: "2023-01-01T00:00:00Z"
`))
		})

		got, err := resolveDependencyURL(server.URL, "mychart", "1.0.0")
		assert.NoError(t, err)
		assert.Equal(t, server.URL+"/mychart-1.0.0.tgz", got)
	})

	t.Run("classic repository that cannot be reached returns an error", func(t *testing.T) {
		_, err := resolveDependencyURL("http://127.0.0.1:0", "mychart", "1.0.0")
		assert.Error(t, err)
	})
}

// TestLoadDependencies_LocalPathDependency whose Chart.yaml/Chart.lock declare a
// dependency with an empty "repository" field to indicate that the subchart is
// already vendored locally under charts/<name>, rather than fetched from anywhere.
func TestLoadDependencies_LocalPathDependency(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	pkgFs := filesystem.GetFilesystem(root)

	packageVersion := 0
	packageOpts := options.PackageOptions{
		PackageVersion: &packageVersion,
		MainChartOptions: options.ChartOptions{
			WorkingDir: "charts",
			UpstreamOptions: options.UpstreamOptions{
				URL: "https://github.com/example/example-repo.git",
			},
		},
	}
	require.NoError(t, packageOpts.WriteToFile(ctx, pkgFs, path.PackageOptionsFile))

	chartYaml := `apiVersion: v2
name: test-chart
version: 0.1.0
type: application
dependencies:
  - name: shared
    version: 1.0.0
    repository: file://../shared
  - name: bundled-sub-chart
    version: 0.0.0
    repository: ""
`
	chartLock := `dependencies:
  - name: shared
    repository: file://../shared
    version: 1.0.0
  - name: bundled-sub-chart
    repository: ""
    version: 0.0.0
digest: sha256:0000000000000000000000000000000000000000000000000000000000000
generated: "2024-01-01T00:00:00Z"
`
	require.NoError(t, os.MkdirAll(filepath.Join(root, "charts"), os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(root, "charts", "Chart.yaml"), []byte(chartYaml), os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(root, "charts", "Chart.lock"), []byte(chartLock), os.ModePerm))

	err := LoadDependencies(ctx, pkgFs, "charts", path.GeneratedChangesDir, map[string]bool{})
	require.NoError(t, err)

	// The "file://" dependency should be resolved relative to the main chart's upstream, unchanged.
	sharedOpts, err := options.LoadChartOptionsFromFile(ctx, pkgFs, filepath.Join(path.GeneratedChangesDir, path.GeneratedChangesDependenciesDir, "shared", path.DependencyOptionsFile))
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/example/example-repo.git", sharedOpts.UpstreamOptions.URL)
	require.NotNil(t, sharedOpts.UpstreamOptions.Subdirectory)
	assert.Equal(t, "../shared", *sharedOpts.UpstreamOptions.Subdirectory)

	// The empty-repository dependency should be treated as vendored under charts/<name>
	// within the main chart itself, not sent to helm's repo index resolver.
	subchartOpts, err := options.LoadChartOptionsFromFile(ctx, pkgFs, filepath.Join(path.GeneratedChangesDir, path.GeneratedChangesDependenciesDir, "bundled-sub-chart", path.DependencyOptionsFile))
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/example/example-repo.git", subchartOpts.UpstreamOptions.URL)
	require.NotNil(t, subchartOpts.UpstreamOptions.Subdirectory)
	assert.Equal(t, filepath.Join("charts", "bundled-sub-chart"), *subchartOpts.UpstreamOptions.Subdirectory)
}
