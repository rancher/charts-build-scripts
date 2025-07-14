package auto

import (
	"context"
	"log"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_UpdateReleaseYaml(t *testing.T) {
	ctx := context.Background()

	type input struct {
		ReleaseVersions map[string][]string
		ChartVersion    string
		Chart           string
		OverWrite       bool
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
				OverWrite:       true,
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
				OverWrite:    true,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"2.0.0"},
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
				OverWrite:    true,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"3.0.0"},
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
				OverWrite:    true,
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
				OverWrite:    true,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "3.0.0"},
					"chart2": {"2.0.0"},
				},
			},
		},
		// Test for RC versions
		{
			name: "Test #6",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0-rc.1", "2.0.0"},
				},
				ChartVersion: "2.0.0",
				Chart:        "chart1",
				OverWrite:    true,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"2.0.0"},
				},
			},
		},
		{
			name: "Test #7",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0", "2.0.1"},
				},
				ChartVersion: "2.0.1-rc.1",
				Chart:        "chart1",
				OverWrite:    true,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"2.0.1-rc.1"},
				},
			},
		},
		// Tests with Overwrite false
		{
			name: "Test #8",
			i: input{
				ReleaseVersions: map[string][]string{"chart1": {"1.0.0"}},
				ChartVersion:    "2.0.0",
				Chart:           "chart1",
				OverWrite:       false,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0"},
				},
			},
		},
		{
			name: "Test #9",
			i: input{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0"},
					"chart2": {"1.0.0"},
				},
				ChartVersion: "2.0.0",
				Chart:        "chart1",
				OverWrite:    false,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"1.0.0", "2.0.0"},
					"chart2": {"1.0.0"},
				},
			},
		},
		{
			name: "Test #10",
			i: input{
				ReleaseVersions: map[string][]string{},
				ChartVersion:    "2.0.0",
				Chart:           "chart1",
				OverWrite:       false,
			},
			ex: expected{
				ReleaseVersions: map[string][]string{
					"chart1": {"2.0.0"},
				},
			},
		},
	}

	tempReleaseYamlFunc := func(releaseVersions map[string][]string) string {
		tempDir, err := os.MkdirTemp("", "unit-test-tmp")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		// Create a release.yaml file
		output, err := os.Create(tempDir + "/release.yaml")
		defer output.Close()

		if err != nil {
			t.Fatalf("failed to create release.yaml file: %v", err)
		}

		encoder := yaml.NewEncoder(output)
		encoder.SetIndent(2)
		if err := encoder.Encode(releaseVersions); err != nil {
			log.Fatalf("failed to encode releaseVersions: %v", err)
		}

		if err := encoder.Close(); err != nil {
			t.Fatalf("failed to close yaml encoder: %v", err)
		}

		return tempDir
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tempDir := tempReleaseYamlFunc(tt.i.ReleaseVersions)

			r := &Release{
				ChartVersion:    tt.i.ChartVersion,
				Chart:           tt.i.Chart,
				ReleaseYamlPath: tempDir + "/release.yaml",
			}

			if err := r.UpdateReleaseYaml(ctx, tt.i.OverWrite); err != nil {
				t.Fatalf("expected nil, got %v", err)
			}

			releaseVersions, err := readReleaseYaml(ctx, r.ReleaseYamlPath)
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
			// reset before next test
			os.RemoveAll(tempDir)
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
