package validate

import (
	"fmt"
	"testing"

	"github.com/google/go-github/v41/github"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/stretchr/testify/assert"
)

func Test_validateReleaseYaml(t *testing.T) {

	buildExpectedError := func(assetFilePathErrors map[string]error) error {
		return fmt.Errorf("%w: %v", errReleaseYaml, assetFilePathErrors)
	}

	vr := &config.VersionRules{
		BranchVersion: "2.9",
		Rules: map[string]int{
			"2.10": 105,
			"2.9":  104,
			"2.8":  103,
		},
	}

	type input struct {
		releaseOpts options.ReleaseOptions
		cfg         *config.Config
		validation  validation
	}
	type expected struct {
		err error
	}
	type test struct {
		name string
		i    input
		ex   expected
	}

	tests := []test{
		// #1
		{
			name: "#1",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
				},
				cfg: &config.Config{
					VersionRules:      vr,
					AssetsVersionsMap: map[string][]string{},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
					},
				},
			},
			ex: expected{
				err: nil,
			},
		},

		// #2
		{
			name: "#2 - [1 chart; 1 version; N files; minor update]",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.1"},
				},
				cfg: &config.Config{
					VersionRules: vr,
					AssetsVersionsMap: map[string][]string{
						"chart-1": {"104.0.1", "104.0.0"},
					},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.1.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
			},
			ex: expected{
				err: nil,
			},
		},

		// #3
		{
			name: "#3 - [2 charts; 1 version; N files; 2 new charts]",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
					"chart-2": {"104.0.0"},
				},
				cfg: &config.Config{
					VersionRules: vr,
					AssetsVersionsMap: map[string][]string{
						"chart-1": {"104.0.1", "104.0.0"},
					},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("assets/chart-2/chart-2-104.0.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
			},
			ex: expected{
				err: nil,
			},
		},

		// #4
		{
			name: "#4 - [1 chart; 1 version; N files]",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
				},
				cfg: &config.Config{
					VersionRules: vr,
					AssetsVersionsMap: map[string][]string{
						"chart-1": {"104.0.0"},
					},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
			},
			ex: expected{
				err: buildExpectedError(map[string]error{
					"assets/chart-1/chart-1-104.0.0.tgz": errModifiedChart,
				}),
			},
		},

		// #5
		{
			name: "Test #5 - [2 chart; 1 version; N files] : Expected ERROR",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
					"chart-2": {"104.0.0"},
				},
				cfg: &config.Config{
					VersionRules: vr,
					AssetsVersionsMap: map[string][]string{
						"chart-1": {"104.0.0"},
						"chart-2": {"104.0.0"},
					},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("assets/chart-2/chart-2-104.0.0.tgz"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
			},
			ex: expected{
				err: buildExpectedError(map[string]error{
					"assets/chart-2/chart-2-104.0.0.tgz": errModifiedChart,
				}),
			},
		},

		// #6
		{
			name: "#6 - [2 chart; 1 version; N files]",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
					"chart-2": {"104.0.0"},
				},
				cfg: &config.Config{
					VersionRules: vr,
					AssetsVersionsMap: map[string][]string{
						"chart-1": {"104.0.0"},
						"chart-2": {"104.0.0"},
					},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("modfied"),
						},
						{
							Filename: github.String("assets/chart-2/chart-2-104.0.0.tgz"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
			},
			ex: expected{
				err: buildExpectedError(map[string]error{
					"assets/chart-1/chart-1-104.0.0.tgz": errModifiedChart,
					"assets/chart-2/chart-2-104.0.0.tgz": errModifiedChart,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.i.validation.validateReleaseYaml(tt.i.cfg, tt.i.releaseOpts)
			if tt.ex.err == nil {
				assert.Nil(t, err, "expected nil error")
			} else {
				assert.EqualError(t, err, tt.ex.err.Error(), "unexpected error")
			}
		})
	}
}

func Test_checkMinorPatchVersion(t *testing.T) {
	cfg := &config.Config{
		VersionRules: &config.VersionRules{
			BranchVersion: "2.9",
			Rules: map[string]int{
				"2.10": 105,
				"2.9":  104,
				"2.8":  103,
			},
		},
	}

	type input struct {
		validation       validation
		cfg              *config.Config
		version          string
		releasedVersions []string
	}
	type expected struct {
		err error
	}
	type test struct {
		name string
		i    input
		ex   expected
	}

	tests := []test{
		{
			name: "#1.0 [patch update]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.0.1",
				releasedVersions: []string{"104.0.0"},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "#1.1 [minor update]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.1.0",
				releasedVersions: []string{"104.0.0"},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "#1.2 [minor update && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.2.0",
				releasedVersions: []string{"104.1.0", "104.0.0"},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "#1.3 [patch update && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.2.1",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "#1.4 [patch update (old chart) && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.1.1",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "#1.5 [patch update (old chart) && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.0.1",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "#2.0 [patch update (old chart) && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.0.2",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.0.2"),
			},
		},
		{
			name: "#2.1 [patch update (old chart) && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.1.2",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.1.2"),
			},
		},
		{
			name: "#2.2 [patch update (new chart) && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.2.2",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.2.2"),
			},
		},
		{
			name: "#2.3 [minor/patch update (new chart) && > 1 released chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.3.1",
				releasedVersions: []string{"104.2.0", "104.1.0", "104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.3.1"),
			},
		},
		{
			name: "#3 [minor/patch update]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.1.1",
				releasedVersions: []string{"104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.1.1"),
			},
		},
		{
			name: "#4 [patch update > 1]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.0.2",
				releasedVersions: []string{"104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.0.2"),
			},
		},
		{
			name: "#5 [minor update > 1]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "104.3.0",
				releasedVersions: []string{"104.0.0"},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.3.0"),
			},
		},
		{
			name: "#6 [forward-ported chart]",
			i: input{
				validation:       validation{},
				cfg:              cfg,
				version:          "103.1.5",
				releasedVersions: []string{"104.1.2"},
			},
			ex: expected{
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.i.validation.checkMinorPatchVersion(tt.i.cfg, tt.i.version, tt.i.releasedVersions)
			if tt.ex.err == nil {
				assert.Nil(t, err, "expected nil error")
			} else {
				assert.EqualError(t, err, tt.ex.err.Error(), "unexpected error")
			}
		})
	}
}

func Test_checkNeverModifyReleasedChart(t *testing.T) {

	buildExpectedError := func(assetFilePathErrors map[string]error) error {
		return fmt.Errorf("%w: %v", errReleaseYaml, assetFilePathErrors)
	}

	type input struct {
		validation     validation
		assetFilePaths map[string]string
	}
	type expected struct {
		err error
	}
	type test struct {
		name string
		i    input
		ex   expected
	}

	tests := []test{
		{
			name: "Test #1 [1 filed added] : Expected NIL",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz": "104.0.0",
				},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #2 [1 filed removed] : Expected NIL",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("removed"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz": "104.0.0",
				},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #3 [1 filed modified] : Expected Error",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("modified"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz": "104.0.0",
				},
			},
			ex: expected{
				err: buildExpectedError(map[string]error{
					"assets/chart-1/chart-1-104.0.0.tgz": errModifiedChart,
				}),
			},
		},
		{
			name: "Test #4 [1 filed any wrong state] : Expected Error",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("xxxxxx"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz": "104.0.0",
				},
			},
			ex: expected{
				err: buildExpectedError(map[string]error{
					"assets/chart-1/chart-1-104.0.0.tgz": errModifiedChart,
				}),
			},
		},
		{
			name: "Test #5 [1 filed added; several others modified] : Expected NIL",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz": "104.0.0",
				},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #6 [Several files added/removed] : Expected Nil",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("assets/chart-2/chart-2-104.1.0.tgz"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("assets/chart-3/chart-2-104.2.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-2/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("charts/chart-2/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-2/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("charts/chart-3/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("charts/chart-3/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-3/value.yaml"),
							Status:   github.String("removed"),
						},

						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz": "104.0.0",
					"assets/chart-2/chart-2-104.1.0.tgz": "104.1.0",
					"assets/chart-3/chart-3-104.2.0.tgz": "104.2.0",
				},
			},
			ex: expected{
				err: nil,
			},
		},

		{
			name: "Test #7 [Several files added/removed and several modified] : Expected Error",
			i: input{
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("assets/chart-2/chart-2-104.1.0.tgz"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("assets/chart-3/chart-3-104.2.0.tgz"),
							Status:   github.String("added"),
						},
						{
							Filename: github.String("assets/chart-4/chart-4-104.0.0.tgz"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("assets/chart-4-crd/chart-4-crd-104.0.0.tgz"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-1/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-2/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("charts/chart-2/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-2/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("charts/chart-3/value.yaml"),
							Status:   github.String("removed"),
						},
						{
							Filename: github.String("charts/chart-3/Chart.yaml"),
							Status:   github.String("modified"),
						},
						{
							Filename: github.String("charts/chart-3/value.yaml"),
							Status:   github.String("removed"),
						},

						{
							Filename: github.String("release.yaml"),
							Status:   github.String("modified"),
						},
					},
				},
				assetFilePaths: map[string]string{
					"assets/chart-1/chart-1-104.0.0.tgz":         "104.0.0",
					"assets/chart-2/chart-1-104.1.0.tgz":         "104.1.0",
					"assets/chart-3/chart-1-104.2.0.tgz":         "104.2.0",
					"assets/chart-4/chart-4-104.0.0.tgz":         "104.0.0",
					"assets/chart-4-crd/chart-4-crd-104.0.0.tgz": "104.0.0",
				},
			},
			ex: expected{
				err: buildExpectedError(map[string]error{
					"assets/chart-4/chart-4-104.0.0.tgz":         errModifiedChart,
					"assets/chart-4-crd/chart-4-crd-104.0.0.tgz": errModifiedChart,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.i.validation.checkNeverModifyReleasedChart(tt.i.assetFilePaths)
			if tt.ex.err == nil {
				assert.Nil(t, err, "expected nil error")
			} else {
				assert.EqualError(t, err, tt.ex.err.Error(), "unexpected error")
			}
		})
	}
}
