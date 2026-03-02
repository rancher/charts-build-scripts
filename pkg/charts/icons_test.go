package charts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
	helmchart "helm.sh/helm/v3/pkg/chart"
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
			handler: func(w http.ResponseWriter, r *http.Request) {
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
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusNotFound)
			},
			expectedErr: "failed to get icon type",
		},
		{
			name: "#4 - unknown Content-Type with no extension",
			handler: func(w http.ResponseWriter, r *http.Request) {
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
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
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

			metadata := &helmchart.Metadata{
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
