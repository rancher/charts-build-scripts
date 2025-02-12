package auto

import (
	"testing"

	"github.com/blang/semver"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/stretchr/testify/assert"
)

func assertError(t *testing.T, err, expectedErr error) {
	if expectedErr != nil {
		assert.EqualError(t, err, expectedErr.Error())
	} else {
		assert.NoError(t, err)
	}
}

func Test_parseBranchVersion(t *testing.T) {
	type expected struct {
		version string
		err     error
	}

	type test struct {
		name  string
		input string
		expected
	}

	tests := []test{
		{
			name:  "#1",
			input: "dev-v2.9",
			expected: expected{
				version: "2.9",
				err:     nil,
			},
		},
		{
			name:  "#2",
			input: "release-v2.9",
			expected: expected{
				version: "",
				err:     errNotDevBranch,
			},
		},
		{
			name:  "#3",
			input: "dev-2.9",
			expected: expected{
				version: "",
				err:     errNotDevBranch,
			},
		},
		{
			name:  "#4",
			input: "something-weird-v2.10",
			expected: expected{
				version: "",
				err:     errNotDevBranch,
			},
		},
		{
			name:  "#5",
			input: "weird_and-wrong",
			expected: expected{
				version: "",
				err:     errNotDevBranch,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			version, err := parseBranchVersion(tc.input)
			assertError(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.version, version)
		})
	}
}

func Test_parseChartFromPackage(t *testing.T) {
	type input struct {
		targetPackage string
		bump          *Bump
	}
	type expected struct {
		err  error
		bump *Bump
	}

	type test struct {
		name     string
		input    input
		expected expected
	}

	tests := []test{
		{
			name: "#1",
			input: input{
				targetPackage: "rancher-chart",
				bump:          &Bump{},
			},
			expected: expected{
				err: nil,
				bump: &Bump{
					targetChart: "rancher-chart",
				},
			},
		},
		{
			name: "#2",
			input: input{
				targetPackage: "rancher-chart/v100/rancher-chart",
				bump:          &Bump{},
			},
			expected: expected{
				err: nil,
				bump: &Bump{
					targetChart: "rancher-chart",
				},
			},
		},
		{
			name: "#3",
			input: input{
				targetPackage: "rancher-chart/v100/rancher-chart/sub-chart",
				bump:          &Bump{},
			},
			expected: expected{
				err: nil,
				bump: &Bump{
					targetChart: "sub-chart",
				},
			},
		},
		{
			name: "#4",
			input: input{
				targetPackage: "rancher-monitoring/rancher-windows-exporter",
				bump:          &Bump{},
			},
			expected: expected{
				err: nil,
				bump: &Bump{
					targetChart: "rancher-windows-exporter",
				},
			},
		},
		{
			name: "#5",
			input: input{
				targetPackage: "rancher-chart/v100/rancher-chart/sub-chart/another-sub-chart?",
				bump:          &Bump{},
			},
			expected: expected{
				err: errBadPackage,
				bump: &Bump{
					targetChart: "",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.bump.parseChartFromPackage(tc.input.targetPackage)
			assertError(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.bump.targetChart, tc.input.bump.targetChart)
		})
	}
}

func Test_parsePackageYaml(t *testing.T) {
	// new pointers to valid values for each test
	newValidPackagesFunc := func() []*charts.Package {
		validSubDir := "charts"
		validRepoBranch := "master"
		validCRDSubDir := "charts-crd"

		validUpstreamOpts := options.UpstreamOptions{
			URL:             "https://github.com/<owner>/<repo>.git",
			Subdirectory:    &validSubDir,
			ChartRepoBranch: &validRepoBranch,
		}
		validCRDUpstreamOpts := options.UpstreamOptions{
			URL:             "https://github.com/<owner>/<repo>.git",
			Subdirectory:    &validCRDSubDir,
			ChartRepoBranch: &validRepoBranch,
		}
		validCRDChartOpts := &options.CRDChartOptions{
			TemplateDirectory:           "crd-template",
			CRDDirectory:                "templates",
			UseTarArchive:               false,
			AddCRDValidationToMainChart: true,
		}

		validUpstream, err := charts.GetUpstream(validUpstreamOpts)
		if err != nil {
			panic(err)
		}
		validCRDUpstream, err := charts.GetUpstream(validCRDUpstreamOpts)
		if err != nil {
			panic(err)
		}

		return []*charts.Package{
			{
				Chart: charts.Chart{
					Upstream:   validUpstream,
					WorkingDir: "charts",
				},
				AdditionalCharts: []*charts.AdditionalChart{
					{
						Upstream:        &validCRDUpstream,
						CRDChartOptions: validCRDChartOpts,
						WorkingDir:      "charts-crd",
					},
				},
				Name: "rancher-chart",
				Auto: true,
			},
		}
	}

	newInvalidUpstreamFunc := func(opts options.UpstreamOptions) puller.Puller {
		invalidUpstream, err := charts.GetUpstream(opts)
		if err != nil {
			panic(err)
		}
		return invalidUpstream
	}

	someIntValue := 1

	type input struct {
		packages []*charts.Package
		b        *Bump
	}
	type expected struct {
		err error
		b   *Bump
	}
	type test struct {
		name     string
		input    input
		expected expected
	}

	tests := []test{
		{
			name: "#0.1",
			input: input{
				packages: []*charts.Package{},
				b: &Bump{
					targetChart: "rancher-chart",
				},
			},
			expected: expected{
				err: errNoPackage,
			},
		},
		{
			name: "#0.2",
			input: input{
				packages: []*charts.Package{{}, {}},
				b: &Bump{
					targetChart: "rancher-chart",
				},
			},
			expected: expected{
				err: errMultiplePackages,
			},
		},
		{
			name: "#1",
			input: input{
				packages: newValidPackagesFunc(),
				b: &Bump{
					targetChart: "rancher-chart",
				},
			},
			expected: expected{
				err: nil,
				b: &Bump{
					targetChart: "rancher-chart",
					Pkg:         newValidPackagesFunc()[0],
				},
			},
		},
		{
			name: "#1.1",
			input: input{
				packages: newValidPackagesFunc(),
				b: &Bump{
					targetChart: "rancher-chart",
				},
			},
			expected: expected{
				err: errFalseAuto,
				b: &Bump{
					targetChart: "rancher-chart",
					Pkg:         newValidPackagesFunc()[0],
				},
			},
		},
		{
			name: "#2",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errPackageName},
		},
		{
			name: "#3",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errPackageChartVersion},
		},
		{
			name: "#4",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errPackageVersion},
		},
		{
			name: "#5",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errPackegeDoNotRelease},
		},
		{
			name: "#6",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errChartWorkDir},
		},
		{
			name: "#7",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errChartURL},
		},
		{
			name: "#8",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errChartRepoCommit},
		},
		{
			name: "#9",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errChartRepoBranch},
		},
		{
			name: "#10",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errChartSubDir},
		},
		{
			name: "#11",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errAdditionalChartWorkDir},
		},
		{
			name: "#12",
			input: input{
				packages: newValidPackagesFunc(),
				b:        &Bump{targetChart: "rancher-chart"},
			},
			expected: expected{err: errCRDWorkDir},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// testing specific error cases
			switch tc.name {
			// chart errors
			case "#1.1":
				tc.input.packages[0].Auto = false
			case "#2":
				tc.input.packages[0].Name = ""
			case "#3":
				tc.input.packages[0].Version = &semver.Version{}
			case "#4":
				tc.input.packages[0].PackageVersion = &someIntValue
			case "#5":
				tc.input.packages[0].DoNotRelease = true
			case "#6":
				tc.input.packages[0].Chart.WorkingDir = ""
			case "#7":
				tc.input.packages[0].Chart.Upstream = newInvalidUpstreamFunc(
					options.UpstreamOptions{
						URL: "https://github.com/<owner/<repo>/release/vX.Y/.tgz",
					},
				)

			// chart upstream options errors
			case "#8":
				someString := "some-string"
				tc.input.packages[0].Chart.Upstream = newInvalidUpstreamFunc(
					options.UpstreamOptions{
						URL:    "https://github.com/<owner/<repo>.git",
						Commit: &someString,
					},
				)
			case "#9":
				tc.input.packages[0].Chart.Upstream = newInvalidUpstreamFunc(
					options.UpstreamOptions{
						URL:             "https://github.com/<owner/<repo>.git",
						ChartRepoBranch: nil,
					},
				)
			case "#10":
				someBranch := "some-branch"
				tc.input.packages[0].Chart.Upstream = newInvalidUpstreamFunc(
					options.UpstreamOptions{
						URL:             "https://github.com/<owner/<repo>.git",
						ChartRepoBranch: &someBranch,
						Subdirectory:    nil,
					},
				)

			// additional chart errors
			case "#11":
				someBranch := "some-branch"
				someSubDir := "some-subdir"
				// use the newInvalidUpstreamFunc but passing valid options; these options are being tested above.
				validUpstreamOpts := newInvalidUpstreamFunc(options.UpstreamOptions{
					URL:             "https://github.com/<owner/<repo>.git",
					ChartRepoBranch: &someBranch,
					Subdirectory:    &someSubDir,
				})
				tc.input.packages[0].AdditionalCharts = []*charts.AdditionalChart{
					{
						Upstream: &validUpstreamOpts,
						CRDChartOptions: &options.CRDChartOptions{
							TemplateDirectory: "",
						},
					},
				}
			case "#12":
				someBranch := "some-branch"
				someSubDir := "some-subdir"
				// use the newInvalidUpstreamFunc but passing valid options; these options are being tested above.
				validUpstreamOpts := newInvalidUpstreamFunc(options.UpstreamOptions{
					URL:             "https://github.com/<owner/<repo>.git",
					ChartRepoBranch: &someBranch,
					Subdirectory:    &someSubDir,
				})
				tc.input.packages[0].AdditionalCharts = []*charts.AdditionalChart{
					{
						Upstream: &validUpstreamOpts,
						CRDChartOptions: &options.CRDChartOptions{
							TemplateDirectory: "crd-template",
							CRDDirectory:      "",
						},
					},
				}
			}

			err := tc.input.b.parsePackageYaml(tc.input.packages)
			assertError(t, err, tc.expected.err)

			if tc.expected.b != nil {
				assert.Equal(t, tc.expected.b.versions, tc.input.b.versions)
				assert.Equal(t, tc.expected.b.targetChart, tc.input.b.targetChart)
				assert.Equal(t, tc.expected.b.Pkg.Chart.WorkingDir, tc.input.b.Pkg.Chart.WorkingDir)
				assert.Equal(t, tc.expected.b.Pkg.Name, tc.input.b.Pkg.Name)
				assert.Equal(t, tc.expected.b.Pkg.Version, tc.input.b.Pkg.Version)
				assert.Equal(t, tc.expected.b.Pkg.PackageVersion, tc.input.b.Pkg.PackageVersion)
				assert.Equal(t, tc.expected.b.Pkg.DoNotRelease, tc.input.b.Pkg.DoNotRelease)
				assert.Equal(
					t,
					tc.expected.b.Pkg.AutoGeneratedBumpVersion,
					tc.input.b.Pkg.AutoGeneratedBumpVersion,
				)
				assert.Equal(
					t,
					tc.expected.b.Pkg.Chart.Upstream.GetOptions(),
					tc.input.b.Pkg.Chart.Upstream.GetOptions(),
				)
				assert.Equal(t, tc.expected.b.Pkg.AdditionalCharts, tc.input.b.Pkg.AdditionalCharts)
			}
		})
	}
}
