package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/stretchr/testify/assert"
)

// setupChartYaml creates a minimal Chart.yaml at charts/<chart>/<version>/Chart.yaml
// within the given root directory, with the provided icon field value.
func setupChartYaml(t *testing.T, rootDir, chart, version, icon string) {
	t.Helper()
	dir := filepath.Join(rootDir, "charts", chart, version)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("failed to create chart dir: %v", err)
	}
	content := "apiVersion: v2\nname: " + chart + "\nversion: " + version + "\nicon: " + icon + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(content), os.ModePerm); err != nil {
		t.Fatalf("failed to write Chart.yaml: %v", err)
	}
}

// setupIconFile creates an empty file at the given path within rootDir.
func setupIconFile(t *testing.T, rootDir, iconPath string) {
	t.Helper()
	abs := filepath.Join(rootDir, iconPath)
	if err := os.MkdirAll(filepath.Dir(abs), os.ModePerm); err != nil {
		t.Fatalf("failed to create icon dir: %v", err)
	}
	if err := os.WriteFile(abs, []byte{}, os.ModePerm); err != nil {
		t.Fatalf("failed to write icon file: %v", err)
	}
}

func Test_loadAndCheckIconPrefix(t *testing.T) {
	tests := []struct {
		name        string
		iconField   string
		createChart bool
		createIcon  bool
		expectedErr string
	}{
		{
			name:        "#1",
			iconField:   "file://assets/logos/my-chart.png",
			createChart: true,
			createIcon:  true,
			expectedErr: "",
		},
		{
			name:        "#2 - icon field is a remote URL, not a file:// path",
			iconField:   "https://example.com/icon.png",
			createChart: true,
			createIcon:  false,
			expectedErr: "icon path is not a file:// prefix",
		},
		{
			name:        "#3 - file:// prefix but icon file does not exist",
			iconField:   "file://assets/logos/my-chart.png",
			createChart: true,
			createIcon:  false,
			expectedErr: "icon path is a file:// prefix, but the icon does not exist",
		},
		{
			name:        "#4 - Chart.yaml does not exist",
			createChart: false,
			createIcon:  false,
			expectedErr: "could not load",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := t.TempDir()
			rootFs := osfs.New(rootDir)

			if tt.createChart {
				setupChartYaml(t, rootDir, "my-chart", "1.0.0", tt.iconField)
			}
			if tt.createIcon {
				setupIconFile(t, rootDir, "assets/logos/my-chart.png")
			}

			err := loadAndCheckIconPrefix(context.Background(), rootFs, "my-chart", "1.0.0")

			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
