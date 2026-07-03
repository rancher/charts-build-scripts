package options

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadImageVersionCheck(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		wantLen   int
		wantFirst ImageVersionEntry
	}{
		{
			name: "valid config",
			yaml: `images:
  - name: prometheus
    repository: rancher/mirrored-prometheus-prometheus
  - name: node-exporter
    repository: rancher/mirrored-prometheus-node-exporter
`,
			wantLen: 2,
			wantFirst: ImageVersionEntry{
				Name:       "prometheus",
				Repository: "rancher/mirrored-prometheus-prometheus",
			},
		},
		{
			name: "empty images list",
			yaml: `images: []
`,
			wantLen: 0,
		},
		{
			name:    "unknown field rejected by strict unmarshal",
			yaml:    "images:\n  - name: foo\n    repository: bar\n    unknown: baz\n",
			wantErr: true,
		},
		{
			name:    "file not found",
			yaml:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string

			if tt.name == "file not found" {
				configPath = filepath.Join(t.TempDir(), "nonexistent.yaml")
			} else {
				f, err := os.CreateTemp(t.TempDir(), "image-version-check-*.yaml")
				require.NoError(t, err)
				_, err = f.WriteString(tt.yaml)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				configPath = f.Name()
			}

			got, err := LoadImageVersionCheck(configPath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, got.Images, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantFirst, got.Images[0])
			}
		})
	}
}
