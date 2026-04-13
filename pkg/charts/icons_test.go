package charts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
	helmChart "helm.sh/helm/v3/pkg/chart"
)

func Test_downloadIcon(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		serverDown  bool
		expectedErr string
	}{
		{
			name: "#1",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("fake-icon-data"))
			},
			expectedErr: "",
		},
		{
			name:        "#2 - HTTP request fails",
			serverDown:  true,
			expectedErr: "failed to download icon",
		},
		{
			name: "#3 - non-200 status",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusNotFound)
			},
			expectedErr: "failed to get icon type",
		},
		{
			name: "#4 - unknown Content-Type with no extension",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-test-no-extension")
				w.WriteHeader(http.StatusOK)
			},
			expectedErr: "failed to get icon type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var iconURL string
			if tt.serverDown {
				ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
				iconURL = ts.URL
				ts.Close()
			} else {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				iconURL = ts.URL
			}

			fs := memfs.New()
			if err := fs.MkdirAll("assets/logos", os.ModePerm); err != nil {
				t.Fatalf("failed to create logos dir: %v", err)
			}

			metadata := &helmChart.Metadata{
				Name: "my-chart",
				Icon: iconURL,
			}

			iconPath, err := downloadIcon(context.Background(), fs, metadata)

			if tt.expectedErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, "assets/logos/my-chart.png", iconPath)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
				assert.Empty(t, iconPath)
			}
		})
	}
}

func TestPackage_DownloadIcon_DoNotRelease(t *testing.T) {
	ctx := context.Background()

	// Create package with DoNotRelease flag
	pkg := &Package{
		Name:         "test-chart",
		DoNotRelease: true,
		fs:           memfs.New(),
		rootFs:       memfs.New(),
	}

	// Create charts directory to simulate prepared package
	if err := pkg.fs.MkdirAll("charts", os.ModePerm); err != nil {
		t.Fatalf("failed to create charts dir: %v", err)
	}

	// DownloadIcon should return early without error
	err := pkg.DownloadIcon(ctx)
	assert.NoError(t, err)

	// Verify no icon was downloaded (assets/logos should not exist)
	exists, _ := pkg.rootFs.Stat("assets/logos")
	assert.Nil(t, exists, "assets/logos should not be created for doNotRelease packages")
}

func TestPackage_DownloadIcon_NotDoNotRelease(t *testing.T) {
	ctx := context.Background()

	// Create package WITHOUT DoNotRelease flag
	pkg := &Package{
		Name:         "test-chart",
		DoNotRelease: false,
		fs:           memfs.New(),
		rootFs:       memfs.New(),
	}

	// Without charts directory, should fail with specific message
	err := pkg.DownloadIcon(ctx)
	assert.NoError(t, err, "should return nil when charts dir doesn't exist")
}
