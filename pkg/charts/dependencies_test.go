package charts

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
