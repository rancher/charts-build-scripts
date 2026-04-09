package canary_test

import (
	"testing"

	"github.com/blang/semver"
)

// TestBlangSemverCompat is a canary for github.com/blang/semver (v3.5.1+incompatible).
//
// blang/semver is used throughout the chart generation and version bumping pipeline:
//   - pkg/auto/versioning.go: Make(), MustParse(), Version.Major field comparison
//   - pkg/charts/parse.go:    Make() to parse version from package.yaml
//   - pkg/helm/export.go:     Make() + Build field append to produce <ver>+up<upstream>
//
// If this test fails it means the blang/semver API or serialisation behaviour changed
// in a way that would break chart version generation or the auto-bump pipeline.
func TestBlangSemverCompat(t *testing.T) {
	t.Run("Make parses repo prefix version", func(t *testing.T) {
		v, err := semver.Make("108.0.0")
		if err != nil {
			t.Fatalf("Make(%q) failed: %v", "108.0.0", err)
		}
		if v.Major != 108 {
			t.Errorf("Major: got %d, want 108", v.Major)
		}
		if v.Minor != 0 {
			t.Errorf("Minor: got %d, want 0", v.Minor)
		}
		if v.Patch != 0 {
			t.Errorf("Patch: got %d, want 0", v.Patch)
		}
	})

	t.Run("Make parses upstream app version", func(t *testing.T) {
		v, err := semver.Make("1.2.3")
		if err != nil {
			t.Fatalf("Make(%q) failed: %v", "1.2.3", err)
		}
		if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
			t.Errorf("got %d.%d.%d, want 1.2.3", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("MustParse parses repo prefix version", func(t *testing.T) {
		v := semver.MustParse("105.0.0")
		if v.Major != 105 {
			t.Errorf("Major: got %d, want 105", v.Major)
		}
	})

	t.Run("Build metadata append produces +up chart version", func(t *testing.T) {
		// This mirrors the exact pattern in pkg/helm/export.go:146
		// metadataSemver.Build = append(metadataSemver.Build, fmt.Sprintf("up%s", upstreamChartVersion))
		v, err := semver.Make("108.0.0")
		if err != nil {
			t.Fatalf("Make failed: %v", err)
		}
		v.Build = append(v.Build, "up1.2.3")

		got := v.String()
		want := "108.0.0+up1.2.3"
		if got != want {
			t.Errorf("String() after Build append: got %q, want %q", got, want)
		}
	})

	t.Run("Build metadata append with prerelease produces +up rc version", func(t *testing.T) {
		v, err := semver.Make("108.0.0")
		if err != nil {
			t.Fatalf("Make failed: %v", err)
		}
		v.Pre = []semver.PRVersion{{VersionStr: "rc", IsNum: false}, {VersionNum: 1, IsNum: true}}
		v.Build = append(v.Build, "up0.25.0-rc.1")

		got := v.String()
		want := "108.0.0-rc.1+up0.25.0-rc.1"
		if got != want {
			t.Errorf("String() with pre+build: got %q, want %q", got, want)
		}
	})

	t.Run("Major field used for branch line comparison", func(t *testing.T) {
		// Mirrors applyVersionRules: repoPrefixSemverRule.Major - latestRepoPrefix.svr.Major == 1
		rule, err := semver.Make("108.0.0")
		if err != nil {
			t.Fatalf("Make rule failed: %v", err)
		}
		latest, err := semver.Make("107.0.0")
		if err != nil {
			t.Fatalf("Make latest failed: %v", err)
		}
		diff := rule.Major - latest.Major
		if diff != 1 {
			t.Errorf("Major diff: got %d, want 1", diff)
		}
	})

	t.Run("Patch field manipulation for packageVersion", func(t *testing.T) {
		// Mirrors export.go:141
		// metadataSemver.Patch = patchNumMultiplier*metadataSemver.Patch + uint64(*packageVersion)
		v, err := semver.Make("108.0.2")
		if err != nil {
			t.Fatalf("Make failed: %v", err)
		}
		const patchNumMultiplier = uint64(100)
		packageVersion := uint64(1)
		v.Patch = patchNumMultiplier*v.Patch + packageVersion
		if v.Patch != 201 {
			t.Errorf("Patch after manipulation: got %d, want 201", v.Patch)
		}
		if v.String() != "108.0.201" {
			t.Errorf("String() after patch manipulation: got %q, want %q", v.String(), "108.0.201")
		}
	})
}
