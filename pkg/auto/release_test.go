package auto

import "testing"

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
