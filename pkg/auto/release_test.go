package auto

import (
	"os"
	"testing"
)

func Test_UpdateReleaseYaml(t *testing.T) {
	type input struct {
		ReleaseVersions map[string][]string
		ChartVersion    string
		Chart           string
	}
	type expected struct {
		ReleaseVersions map[string][]string
	}
	type test struct {
		name string
		i    input
		ex   expected
	}
	tests := []test{
		{
			name: "Test #1",
			i: input{
				ReleaseVersions: map[string][]string{},
				ChartVersion:    "1.0.0",
				Chart:           "chart1",
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0"},
				},
			},
		},
		{
			name: "Test #2",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0"},
				},
				ChartVersion: "2.0.0",
				Chart:        "chart1",
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0"},
				},
			},
		},
		{
			name: "Test #3",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0"},
				},
				ChartVersion: "3.0.0",
				Chart:        "chart1",
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "3.0.0"},
				},
			},
		},

		{
			name: "Test #4",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "3.0.0"},
				},
				ChartVersion: "2.0.0",
				Chart:        "chart2",
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "3.0.0"},
					"chart2": {"2.0.0"},
				},
			},
		},
		// Test for duplicate versions
		{
			name: "Test #5",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "3.0.0"},
					"chart2": {"2.0.0"},
				},
				ChartVersion: "2.0.0",
				Chart:        "chart2",
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "3.0.0"},
					"chart2": {"2.0.0"},
				},
			},
		},
	}

	tempDir, err := os.MkdirTemp("", "unit-test-tmp")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a release.yaml file
	if _, err := os.Create(tempDir + "/release.yaml"); err != nil {
		t.Fatalf("failed to create release.yaml file: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			r := &Release{
				ChartVersion:    tt.i.ChartVersion,
				Chart:           tt.i.Chart,
				ReleaseYamlPath: tempDir + "/release.yaml",
			}

			if err := r.UpdateReleaseYaml(); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			releaseVersions, err := r.readReleaseYaml()
			if err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			if len(releaseVersions) != len(tt.ex.ReleaseVersions) {
				t.Errorf("expected %v, got %v", tt.ex.ReleaseVersions, tt.i.ReleaseVersions)
			}

			for k, v := range tt.ex.ReleaseVersions {
				if len(releaseVersions[k]) != len(v) {
					t.Errorf("expected %v, got %v", v, releaseVersions[k])
				}
				for i := range v {
					if v[i] != releaseVersions[k][i] {
						t.Errorf("expected %v, got %v", v, releaseVersions[k])
					}
				}
			}
		})
	}

}

func Test_mountAssetVersionPath(t *testing.T) {
	type input struct {
		chart, version string
	}
	inputs := []input{
		{"chart1", "1.0.0"},
		{"chart2", "100.0.0+up0.0.0"},
	}
	type output struct {
		assetPath, assetTgz string
	}
	outputs := []output{
		{"assets/chart1/chart1-1.0.0.tgz", "chart1-1.0.0.tgz"},
		{"assets/chart2/chart2-100.0.0+up0.0.0.tgz", "chart2-100.0.0+up0.0.0.tgz"},
	}

	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		{"#1", inputs[0], outputs[0]},
		{"#2", inputs[1], outputs[1]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assetPath, assetTgz := mountAssetVersionPath(test.input.chart, test.input.version)
			if assetPath != test.output.assetPath {
				t.Errorf("expected %s, got %s", test.output.assetPath, assetPath)
			}
			if assetTgz != test.output.assetTgz {
				t.Errorf("expected %s, got %s", test.output.assetTgz, assetTgz)
			}
		})
	}
}

func Test_removeDuplicates(t *testing.T) {
	type input struct {
		versions []string
	}
	inputs := []input{
		{[]string{"1.0.0", "1.0.0", "1.0.0"}},
		{[]string{"1.0.0", "1.0.0", "2.0.0", "2.0.0", "3.0.0"}},
	}
	type output struct {
		uniqueVersions []string
	}
	outputs := []output{
		{[]string{"1.0.0"}},
		{[]string{"1.0.0", "2.0.0", "3.0.0"}},
	}

	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{
		{"#1", inputs[0], outputs[0]},
		{"#2", inputs[1], outputs[1]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uniqueVersions := removeDuplicates(test.input.versions)
			if len(uniqueVersions) != len(test.output.uniqueVersions) {
				t.Errorf("expected %v, got %v", test.output.uniqueVersions, uniqueVersions)
			}
			for i := range uniqueVersions {
				if uniqueVersions[i] != test.output.uniqueVersions[i] {
					t.Errorf("expected %v, got %v", test.output.uniqueVersions, uniqueVersions)
				}
			}
		})
	}
}
