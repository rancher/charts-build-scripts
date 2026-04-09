package registries

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/stretchr/testify/assert"
	helmRegistry "helm.sh/helm/v3/pkg/registry"
)

func TestBuildPushURL(t *testing.T) {
	tests := []struct {
		name       string
		ociDNS     string
		customPath string
		chart      string
		version    string
		expected   string
	}{
		{
			name:       "default path",
			ociDNS:     "registry.example.com",
			customPath: "",
			chart:      "rancher-turtles",
			version:    "1.0.0+up0.1.0",
			expected:   "registry.example.com/rancher/charts/rancher-turtles:1.0.0+up0.1.0",
		},
		{
			name:       "custom path",
			ociDNS:     "registry.example.com",
			customPath: "myorg/charts",
			chart:      "rancher-turtles",
			version:    "1.0.0+up0.1.0",
			expected:   "registry.example.com/myorg/charts/rancher-turtles:1.0.0+up0.1.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := buildPushURL(test.ociDNS, test.customPath, test.chart, test.version)
			assert.Equal(t, test.expected, got)
		})
	}
}

func TestBuildTagsURL(t *testing.T) {
	tests := []struct {
		name       string
		ociDNS     string
		customPath string
		chart      string
		expected   string
	}{
		{
			name:       "default path",
			ociDNS:     "registry.example.com",
			customPath: "",
			chart:      "rancher-turtles",
			expected:   "registry.example.com/rancher/charts/rancher-turtles",
		},
		{
			name:       "custom path",
			ociDNS:     "registry.example.com",
			customPath: "myorg/charts",
			chart:      "rancher-turtles",
			expected:   "registry.example.com/myorg/charts/rancher-turtles",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := buildTagsURL(test.ociDNS, test.customPath, test.chart)
			assert.Equal(t, test.expected, got)
		})
	}
}

func Test_checkAndPush(t *testing.T) {
	type input struct {
		o       *oci
		release options.ReleaseOptions
	}
	type expected struct {
		pushedAssets []string
		err          error
	}

	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			// single chart, single version: happy path push
			name: "#1",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{"chart1-1.0.0.tgz"},
				err:          nil,
			},
		},
		{
			// multiple charts, single version each: all pushed
			name: "#2",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
					"chart2": {"1.0.0+up0.0.0"},
					"chart3": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{
					"chart1-1.0.0+up0.0.0.tgz",
					"chart2-1.0.0+up0.0.0.tgz",
					"chart3-1.0.0+up0.0.0.tgz",
				},
				err: nil,
			},
		},
		{
			// loadAsset error: phase 1 fails fast, nothing pushed
			name: "#3",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, errors.New("some-error")
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{},
				err:          errors.New("some-error"),
			},
		},
		{
			// checkAsset error: phase 1 fails fast, nothing pushed
			name: "#4",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, errors.New("some-error")
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{},
				err:          errors.New("some-error"),
			},
		},
		{
			// all charts already exist in registry: all skipped, no error
			name: "#5",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return true, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{},
				err:          nil,
			},
		},
		{
			// push error: all charts fail, error returned
			name: "#6",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						err := errors.New("some assets failed, please fix and retry only these assets")
						return err
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{},
				err:          errors.New("some assets failed, please fix and retry only these assets"),
			},
		},
		{
			// multiple versions of same chart: all versions are pushed
			name: "#7",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.1.0", "2.0.0+up0.2.0", "3.0.0+up0.3.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{
					"chart1-1.0.0+up0.1.0.tgz",
					"chart1-2.0.0+up0.2.0.tgz",
					"chart1-3.0.0+up0.3.0.tgz",
				},
				err: nil,
			},
		},
		{
			// mixed: chart2 already exists, chart1 and chart3 are new and get pushed
			name: "#8",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, chart, _ string) (bool, error) {
						return chart == "chart2", nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, _ string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
					"chart2": {"1.0.0+up0.0.0"},
					"chart3": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{
					"chart1-1.0.0+up0.0.0.tgz",
					"chart3-1.0.0+up0.0.0.tgz",
				},
				err: nil,
			},
		},
		{
			// partial push failure: chart1 and chart3 succeed, chart2 fails, error returned with partial results
			name: "#9",
			input: input{
				o: &oci{
					dns:        "######",
					user:       "######",
					password:   "######",
					helmClient: &helmRegistry.Client{},
					loadAsset: func(_, _ string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(_ context.Context, _ *helmRegistry.Client, _, _, _, _ string) (bool, error) {
						return false, nil
					},
					push: func(_ *helmRegistry.Client, _ []byte, url string) error {
						if strings.Contains(url, "chart2") {
							return errors.New("push failed")
						}
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
					"chart2": {"1.0.0+up0.0.0"},
					"chart3": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{
					"chart1-1.0.0+up0.0.0.tgz",
					"chart3-1.0.0+up0.0.0.tgz",
				},
				err: errors.New("some assets failed, please fix and retry only these assets"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assets, err := test.input.o.checkAndPush(context.Background(), &test.input.release)
			if test.expected.err == nil {
				if err != nil {
					t.Errorf("Expected no error, got: [%v]", err)
				}
			} else {
				if !strings.Contains(err.Error(), test.expected.err.Error()) {
					t.Errorf("Expected error: [%v], got: [%v]", test.expected.err, err)
				}
			}

			assert.ElementsMatch(t, test.expected.pushedAssets, assets)
		})
	}

}
