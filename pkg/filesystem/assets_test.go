package filesystem

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeValuesYamlFile(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		assertFn func(t *testing.T, got map[string]interface{})
	}{
		{
			name: "simple flat map",
			yaml: `repository: rancher/mirrored-prometheus
tag: v2.45.0
`,
			assertFn: func(t *testing.T, got map[string]interface{}) {
				assert.Equal(t, "rancher/mirrored-prometheus", got["repository"])
				assert.Equal(t, "v2.45.0", got["tag"])
			},
		},
		{
			name: "nested map with image block",
			yaml: `prometheus:
  image:
    repository: rancher/mirrored-prometheus-prometheus
    tag: v2.45.0
`,
			assertFn: func(t *testing.T, got map[string]interface{}) {
				prom := got["prometheus"].(map[string]interface{})
				img := prom["image"].(map[string]interface{})
				assert.Equal(t, "rancher/mirrored-prometheus-prometheus", img["repository"])
				assert.Equal(t, "v2.45.0", img["tag"])
			},
		},
		{
			name: "numeric values coerced to strings",
			yaml: `replicaCount: 2
timeout: 30.5
`,
			assertFn: func(t *testing.T, got map[string]interface{}) {
				assert.Equal(t, "2", got["replicaCount"])
				assert.Equal(t, "30.5", got["timeout"])
			},
		},
		{
			name: "array of maps",
			yaml: `images:
  - repository: rancher/a
    tag: v1.0.0
  - repository: rancher/b
    tag: v2.0.0
`,
			assertFn: func(t *testing.T, got map[string]interface{}) {
				imgs := got["images"].([]interface{})
				require.Len(t, imgs, 2)
				first := imgs[0].(map[string]interface{})
				assert.Equal(t, "rancher/a", first["repository"])
			},
		},
		{
			name:    "file not found",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.wantErr {
				filePath = "/nonexistent/path/values.yaml"
			} else {
				f, err := os.CreateTemp(t.TempDir(), "values-*.yaml")
				require.NoError(t, err)
				_, err = f.WriteString(tt.yaml)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				filePath = f.Name()
			}

			got, err := DecodeValuesYamlFile(ctx, filePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.assertFn(t, got)
		})
	}
}
