package auto

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/registry"
)

func Test_push(t *testing.T) {
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
			name: "Test #1",
			input: input{
				o: &oci{
					DNS:        "######",
					user:       "######",
					password:   "######",
					helmClient: &registry.Client{},
					loadAsset: func(chart, asset string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
						return false, nil
					},
					push: func(helmClient *registry.Client, data []byte, url string) error {
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
			name: "Test #2",
			input: input{
				o: &oci{
					DNS:        "######",
					user:       "######",
					password:   "######",
					helmClient: &registry.Client{},
					loadAsset: func(chart, asset string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
						return false, nil
					},
					push: func(helmClient *registry.Client, data []byte, url string) error {
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
			name: "Test #3",
			input: input{
				o: &oci{
					DNS:        "######",
					user:       "######",
					password:   "######",
					helmClient: &registry.Client{},
					loadAsset: func(chart, asset string) ([]byte, error) {
						return []byte{}, errors.New("some-error")
					},
					checkAsset: func(regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
						return false, nil
					},
					push: func(helmClient *registry.Client, data []byte, url string) error {
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
			name: "Test #4",
			input: input{
				o: &oci{
					DNS:        "######",
					user:       "######",
					password:   "######",
					helmClient: &registry.Client{},
					loadAsset: func(chart, asset string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
						return false, errors.New("some-error")
					},
					push: func(helmClient *registry.Client, data []byte, url string) error {
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
			name: "Test #5",
			input: input{
				o: &oci{
					DNS:        "######",
					user:       "######",
					password:   "######",
					helmClient: &registry.Client{},
					loadAsset: func(chart, asset string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
						return true, nil
					},
					push: func(helmClient *registry.Client, data []byte, url string) error {
						return nil
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{},
				err:          fmt.Errorf("asset already exists in the OCI registry: %s", "chart1-1.0.0+up0.0.0.tgz"),
			},
		},
		{
			name: "Test #6",
			input: input{
				o: &oci{
					DNS:        "######",
					user:       "######",
					password:   "######",
					helmClient: &registry.Client{},
					loadAsset: func(chart, asset string) ([]byte, error) {
						return []byte{}, nil
					},
					checkAsset: func(regClient *registry.Client, ociDNS, chart, version string) (bool, error) {
						return false, nil
					},
					push: func(helmClient *registry.Client, data []byte, url string) error {
						err := fmt.Errorf("failed to push %s; error: %w ", "chart1-1.0.0+up0.0.0.tgz", errors.New("some-error"))
						return err
					},
				},
				release: options.ReleaseOptions{
					"chart1": {"1.0.0+up0.0.0"},
				},
			},
			expected: expected{
				pushedAssets: []string{},
				err:          fmt.Errorf("failed to push %s; error: %w ", "chart1-1.0.0+up0.0.0.tgz", errors.New("some-error")),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assets, err := test.input.o.update(context.Background(), &test.input.release)
			if test.expected.err == nil {
				if err != nil {
					t.Errorf("Expected no error, got: [%v]", err)
				}
			} else {
				if !strings.Contains(err.Error(), test.expected.err.Error()) {
					t.Errorf("Expected error: [%v], got: [%v]", test.expected.err, err)
				}
			}

			assert.EqualValues(t, len(assets), len(test.expected.pushedAssets))
		})
	}

}
