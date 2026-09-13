package canary_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	yamlV2 "go.yaml.in/yaml/v3"
	yamlV3 "go.yaml.in/yaml/v3"
)

// TestYamlV3Compat is a canary for gopkg.in/yaml.v3.
//
// Used in: pkg/filesystem/yaml.go (streaming decode/encode, TypeError handling),
// pkg/config/blocklist.go (Unmarshal), pkg/auto/release_test.go (encoder).
//
// Pins: NewDecoder, Decoder.Decode, NewEncoder, Encoder.Encode, Encoder.SetIndent,
// Encoder.Close, Unmarshal, TypeError, TypeError.Errors.
func TestYamlV3Compat(t *testing.T) {
	t.Run("NewDecoder and Decode parse blocklist YAML", func(t *testing.T) {
		// Mirrors pkg/config/blocklist.go parsing pattern
		input := `rancher-monitoring:
  - 100.0.0+up19.0.3
  - 100.0.1+up19.0.3
rancher-alerting-drivers:
  - 102.0.0+up0.1.0
`
		decoder := yamlV3.NewDecoder(strings.NewReader(input))
		var charts map[string][]string
		if err := decoder.Decode(&charts); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if len(charts) != 2 {
			t.Errorf("expected 2 charts, got %d", len(charts))
		}
		if len(charts["rancher-monitoring"]) != 2 {
			t.Errorf("expected 2 versions for rancher-monitoring, got %d", len(charts["rancher-monitoring"]))
		}
		if charts["rancher-monitoring"][0] != "100.0.0+up19.0.3" {
			t.Errorf("version mismatch: got %q", charts["rancher-monitoring"][0])
		}
	})

	t.Run("Unmarshal parses blocklist YAML directly", func(t *testing.T) {
		// Mirrors pkg/config/blocklist.go: yaml.Unmarshal(data, &charts)
		input := []byte(`fleet:
  - 109.0.0+up0.14.0
  - 109.0.1+up0.15.1-beta.2
`)
		var charts map[string][]string
		if err := yamlV3.Unmarshal(input, &charts); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if charts["fleet"][1] != "109.0.1+up0.15.1-beta.2" {
			t.Errorf("version mismatch: got %q", charts["fleet"][1])
		}
	})

	t.Run("NewEncoder with SetIndent produces release.yaml format", func(t *testing.T) {
		// Mirrors pkg/auto/release_test.go and pkg/filesystem/yaml.go
		releaseVersions := map[string][]string{
			"rancher-monitoring": {"108.0.0+up0.9.0"},
			"fleet":              {"109.0.1+up0.15.1-beta.2"},
		}

		var buf bytes.Buffer
		encoder := yamlV3.NewEncoder(&buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(releaseVersions); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		if err := encoder.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		output := buf.String()
		// Verify indentation (2 spaces)
		if !strings.Contains(output, "  - 108.0.0+up0.9.0") {
			t.Errorf("indentation incorrect, got:\n%s", output)
		}
		// Verify both charts present
		if !strings.Contains(output, "rancher-monitoring:") || !strings.Contains(output, "fleet:") {
			t.Errorf("missing expected keys, got:\n%s", output)
		}
	})

	t.Run("TypeError captures duplicate key errors", func(t *testing.T) {
		// Mirrors pkg/filesystem/yaml.go: errors.As(err, &yamlV3TypeError)
		input := `key: value1
key: value2
`
		var data map[string]string
		err := yamlV3.Unmarshal([]byte(input), &data)

		var typeErr *yamlV3.TypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("expected TypeError, got %T: %v", err, err)
		}

		if len(typeErr.Errors) == 0 {
			t.Errorf("expected Errors slice populated, got empty")
		}

		// Verify error message contains "already defined"
		foundDuplicateError := false
		for _, errMsg := range typeErr.Errors {
			if strings.Contains(errMsg, "mapping key") && strings.Contains(errMsg, "already defined") {
				foundDuplicateError = true
				break
			}
		}
		if !foundDuplicateError {
			t.Errorf("expected duplicate key error in Errors, got: %v", typeErr.Errors)
		}
	})

	t.Run("round-trip preserves chart versions with build metadata", func(t *testing.T) {
		// Critical: versions like "108.0.0+up0.9.0" must survive marshal→unmarshal
		original := map[string][]string{
			"rancher-webhook": {"108.0.0+up0.9.0", "108.0.0-rc.1+up0.9.0-rc.1"},
		}

		data, err := yamlV3.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded map[string][]string
		if err := yamlV3.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded["rancher-webhook"][0] != "108.0.0+up0.9.0" {
			t.Errorf("build metadata lost: got %q", decoded["rancher-webhook"][0])
		}
		if decoded["rancher-webhook"][1] != "108.0.0-rc.1+up0.9.0-rc.1" {
			t.Errorf("prerelease+build lost: got %q", decoded["rancher-webhook"][1])
		}
	})
}

// TestYamlV2Compat is a canary for gopkg.in/yaml.v2.
//
// Used in: main.go (UnmarshalStrict), pkg/charts/crdchart.go (NewDecoder for CRDs),
// pkg/options/package.go (Unmarshal/Marshal for package.yaml),
// pkg/config/imageVersion.go (UnmarshalStrict), pkg/filesystem/yaml.go (fallback decode).
//
// Pins: NewDecoder, Decoder.Decode, Unmarshal, UnmarshalStrict, Marshal.
func TestYamlV2Compat(t *testing.T) {
	t.Run("NewDecoder and Decode parse multi-document CRD YAML", func(t *testing.T) {
		// Mirrors pkg/charts/crdchart.go parsing CRDs with multiple documents
		input := `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.management.cattle.io
spec:
  group: management.cattle.io
  names:
    kind: Cluster
  version: v3
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: settings.management.cattle.io
spec:
  group: management.cattle.io
  names:
    kind: Setting
  version: v3
`
		type k8sCRD struct {
			APIVersion string `yaml:"apiVersion"`
			Spec       struct {
				Group string `yaml:"group"`
				Names struct {
					Kind string `yaml:"kind"`
				} `yaml:"names"`
				Version string `yaml:"version"`
			} `yaml:"spec"`
		}

		decoder := yamlV2.NewDecoder(strings.NewReader(input))
		var crds []k8sCRD
		for {
			var crd k8sCRD
			err := decoder.Decode(&crd)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			crds = append(crds, crd)
		}

		if len(crds) != 2 {
			t.Fatalf("expected 2 CRDs, got %d", len(crds))
		}
		if crds[0].Spec.Names.Kind != "Cluster" {
			t.Errorf("first CRD kind: got %q, want Cluster", crds[0].Spec.Names.Kind)
		}
		if crds[1].Spec.Names.Kind != "Setting" {
			t.Errorf("second CRD kind: got %q, want Setting", crds[1].Spec.Names.Kind)
		}
	})

	t.Run("Unmarshal parses package.yaml", func(t *testing.T) {
		// Mirrors pkg/options/package.go: yaml.Unmarshal(chartOptionsBytes, &packageOptions)
		input := []byte(`packageVersion: 1
workingDir: charts
url: https://github.com/rancher/rancher-monitoring.git
commit: abc123
`)
		type PackageOptions struct {
			PackageVersion int     `yaml:"packageVersion"`
			WorkingDir     string  `yaml:"workingDir"`
			URL            string  `yaml:"url"`
			Commit         *string `yaml:"commit,omitempty"`
		}

		var opts PackageOptions
		if err := yamlV2.Unmarshal(input, &opts); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if opts.PackageVersion != 1 {
			t.Errorf("packageVersion: got %d, want 1", opts.PackageVersion)
		}
		if opts.WorkingDir != "charts" {
			t.Errorf("workingDir: got %q, want charts", opts.WorkingDir)
		}
		if opts.Commit == nil || *opts.Commit != "abc123" {
			t.Errorf("commit: got %v, want abc123", opts.Commit)
		}
	})

	t.Run("UnmarshalStrict rejects unknown fields", func(t *testing.T) {
		// Mirrors main.go:786 and pkg/config/imageVersion.go:55
		input := []byte(`packageVersion: 1
workingDir: charts
unknownField: should-fail
`)
		type PackageOptions struct {
			PackageVersion int    `yaml:"packageVersion"`
			WorkingDir     string `yaml:"workingDir"`
		}

		var opts PackageOptions
		err := yamlV2.UnmarshalStrict(input, &opts)
		if err == nil {
			t.Fatal("UnmarshalStrict should reject unknown fields")
		}
		if !strings.Contains(err.Error(), "unknownField") {
			t.Errorf("error should mention unknownField, got: %v", err)
		}
	})

	t.Run("Marshal produces valid package.yaml", func(t *testing.T) {
		// Mirrors pkg/options/package.go: yaml.Marshal(p)
		type ChartOptions struct {
			WorkingDir string `yaml:"workingDir"`
			URL        string `yaml:"url"`
		}
		opts := ChartOptions{
			WorkingDir: "charts",
			URL:        "https://github.com/rancher/fleet.git",
		}

		data, err := yamlV2.Marshal(opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		output := string(data)
		if !strings.Contains(output, "workingDir: charts") {
			t.Errorf("missing workingDir, got:\n%s", output)
		}
		if !strings.Contains(output, "url: https://github.com/rancher/fleet.git") {
			t.Errorf("missing url, got:\n%s", output)
		}
	})

	t.Run("round-trip preserves package options", func(t *testing.T) {
		type PackageOptions struct {
			PackageVersion int    `yaml:"packageVersion"`
			WorkingDir     string `yaml:"workingDir"`
			DoNotRelease   bool   `yaml:"doNotRelease"`
		}

		original := PackageOptions{
			PackageVersion: 5,
			WorkingDir:     "charts",
			DoNotRelease:   true,
		}

		data, err := yamlV2.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded PackageOptions
		if err := yamlV2.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.PackageVersion != 5 {
			t.Errorf("packageVersion: got %d, want 5", decoded.PackageVersion)
		}
		if decoded.WorkingDir != "charts" {
			t.Errorf("workingDir: got %q, want charts", decoded.WorkingDir)
		}
		if !decoded.DoNotRelease {
			t.Errorf("doNotRelease: got false, want true")
		}
	})
}
