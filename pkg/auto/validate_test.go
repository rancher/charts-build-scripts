package auto

import (
	"fmt"
	"testing"

	"github.com/google/go-github/v69/github"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/stretchr/testify/assert"
)

func Test_validateReleaseYaml(t *testing.T) {

	buildExpectedError := func(assetFilePathErrors map[string]error) error {
		return fmt.Errorf("%w: %v", errReleaseYaml, assetFilePathErrors)
	}

	vr := &lifecycle.VersionRules{
		Rules: map[string]lifecycle.Version{
			"2.10": {Min: "105.0.0", Max: "106.0.0"},
			"2.9":  {Min: "104.0.0", Max: "105.0.0"},
			"2.8":  {Min: "103.0.0", Max: "104.0.0"},
		},
		BranchVersion: "2.9",
		MinVersion:    104,
		MaxVersion:    105,
	}

	type input struct {
		releaseOpts options.ReleaseOptions
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
			name: "Test #1 - [1 chart; 1 version] : Expected NIL",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
				},
				validation: validation{
					files: []*github.CommitFile{
						{
							Filename: github.String("assets/chart-1/chart-1-104.0.0.tgz"),
							Status:   github.String("added"),
						},
					},
					dep: &lifecycle.Dependencies{
						AssetsVersionsMap: map[string][]lifecycle.Asset{},
					},
				},
			},
			ex: expected{
				err: nil,
			},
		},

		// #2
		{
			name: "Test #2 - [1 chart; 1 version; N files; minor update] : Expected NIL",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.1"},
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
					dep: &lifecycle.Dependencies{
						AssetsVersionsMap: map[string][]lifecycle.Asset{
							"chart-1": []lifecycle.Asset{
								{
									Version: "104.0.1",
								},
								{
									Version: "104.0.0",
								},
							},
						},
						VR: vr,
					},
				},
			},
			ex: expected{
				err: nil,
			},
		},

		// #3
		{
			name: "Test #3 - [2 charts; 1 version; N files; 2 new charts] : Expected NIL",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
					"chart-2": {"104.0.0"},
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
					dep: &lifecycle.Dependencies{
						AssetsVersionsMap: map[string][]lifecycle.Asset{
							"chart-1": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
							"chart-2": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
						},
						VR: vr,
					},
				},
			},
			ex: expected{
				err: nil,
			},
		},

		// #4
		{
			name: "Test #4 - [1 chart; 1 version; N files] : Expected ERROR",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
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
					dep: &lifecycle.Dependencies{
						AssetsVersionsMap: map[string][]lifecycle.Asset{
							"chart-1": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
						},
						VR: vr,
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
					dep: &lifecycle.Dependencies{
						AssetsVersionsMap: map[string][]lifecycle.Asset{
							"chart-1": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
							"chart-2": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
						},
						VR: vr,
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
			name: "Test #6 - [2 chart; 1 version; N files] : Expected >1 ERRORs",
			i: input{
				releaseOpts: options.ReleaseOptions{
					"chart-1": {"104.0.0"},
					"chart-2": {"104.0.0"},
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
					dep: &lifecycle.Dependencies{
						AssetsVersionsMap: map[string][]lifecycle.Asset{
							"chart-1": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
							"chart-2": []lifecycle.Asset{
								{
									Version: "104.0.0",
								},
							},
						},
						VR: vr,
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
			err := tt.i.validation.validateReleaseYaml(tt.i.releaseOpts)
			if tt.ex.err == nil {
				assert.Nil(t, err, "expected nil error")
			} else {
				assert.EqualError(t, err, tt.ex.err.Error(), "unexpected error")
			}
		})
	}
}

func Test_checkMinorPatchVersion(t *testing.T) {
	vr := &lifecycle.VersionRules{
		Rules: map[string]lifecycle.Version{
			"2.10": {Min: "105.0.0", Max: "106.0.0"},
			"2.9":  {Min: "104.0.0", Max: "105.0.0"},
			"2.8":  {Min: "103.0.0", Max: "104.0.0"},
		},
		BranchVersion: "2.9",
		MinVersion:    104,
		MaxVersion:    105,
	}

	type input struct {
		validation       validation
		version          string
		releasedVersions []lifecycle.Asset
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
			name: "Test #1.0 [patch update] : Expected NIL",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.0.1",
				releasedVersions: []lifecycle.Asset{{Version: "104.0.0"}},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #1.1 [minor update] : Expected NIL",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.1.0",
				releasedVersions: []lifecycle.Asset{{Version: "104.0.0"}},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #1.2 [minor update && > 1 released chart] : Expected NIL",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.2.0",
				releasedVersions: []lifecycle.Asset{{Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #1.3 [patch update && > 1 released chart] : Expected NIL",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.2.1",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #1.4 [patch update (old chart) && > 1 released chart] : Expected NIL",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.1.1",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #1.5 [patch update (old chart) && > 1 released chart] : Expected NIL",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.0.1",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: nil,
			},
		},
		{
			name: "Test #2.0 [patch update (old chart) && > 1 released chart] : Expected ERR",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.0.2",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.0.2"),
			},
		},
		{
			name: "Test #2.1 [patch update (old chart) && > 1 released chart] : Expected ERR",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.1.2",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.1.2"),
			},
		},
		{
			name: "Test #2.2 [patch update (new chart) && > 1 released chart] : Expected ERR",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.2.2",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.2.2"),
			},
		},
		{
			name: "Test #2.3 [minor/patch update (new chart) && > 1 released chart] : Expected ERR",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.3.1",
				releasedVersions: []lifecycle.Asset{{Version: "104.2.0"}, {Version: "104.1.0"}, {Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.3.1"),
			},
		},
		{
			name: "Test #3 [minor/patch update] : Expected Error",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.1.1",
				releasedVersions: []lifecycle.Asset{{Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.1.1"),
			},
		},
		{
			name: "Test #4 [patch update > 1] : Expected Error",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.0.2",
				releasedVersions: []lifecycle.Asset{{Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.0.2"),
			},
		},
		{
			name: "Test #5 [minor update > 1] : Expected Error",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "104.3.0",
				releasedVersions: []lifecycle.Asset{{Version: "104.0.0"}},
			},
			ex: expected{
				err: fmt.Errorf("%w: version: %s", errMinorPatchVersion, "104.3.0"),
			},
		},
		{
			name: "Test #6 [forward-ported chart] : Expected nil",
			i: input{
				validation: validation{
					dep: &lifecycle.Dependencies{
						VR: vr,
					},
				},
				version:          "103.1.5",
				releasedVersions: []lifecycle.Asset{{Version: "104.1.2"}},
			},
			ex: expected{
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.i.validation.checkMinorPatchVersion(tt.i.version, tt.i.releasedVersions)
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
