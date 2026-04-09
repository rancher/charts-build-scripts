package canary_test

import (
	"strings"
	"testing"

	semverV1 "github.com/Masterminds/semver"
)

// TestMastermindsV1SemverCompat is a canary for github.com/Masterminds/semver (v1.5.0).
//
// Masterminds/semver v1 is used in:
//   - pkg/helm/helm.go:           NewVersion() for base version comparison in SortVersions
//   - pkg/options/validate.go:    NewVersion() to sort release versions; Equal() + Metadata()
//     to compare +up metadata separately when base versions match
//   - pkg/validate/pull_requests.go: NewVersion() + Minor()/Major()/Patch() for PR validation
//
// Critical behaviour: build metadata (the +up... part) is IGNORED by Equal(), LessThan(),
// and GreaterThan() per the semver spec. This is why validate.go extracts Metadata()
// separately and re-parses it for ordering. If this behaviour changes, the release
// version sort will silently produce wrong results.
//
// If this test fails it means v1 changed its parsing or comparison semantics.
func TestMastermindsV1SemverCompat(t *testing.T) {
	t.Run("NewVersion parses stable +up chart version", func(t *testing.T) {
		v, err := semverV1.NewVersion("108.0.0+up1.0.0")
		if err != nil {
			t.Fatalf("NewVersion(%q) failed: %v", "108.0.0+up1.0.0", err)
		}
		if v.Major() != 108 {
			t.Errorf("Major: got %d, want 108", v.Major())
		}
		if v.Minor() != 0 {
			t.Errorf("Minor: got %d, want 0", v.Minor())
		}
		if v.Patch() != 0 {
			t.Errorf("Patch: got %d, want 0", v.Patch())
		}
	})

	t.Run("NewVersion parses rc prerelease +up chart version", func(t *testing.T) {
		v, err := semverV1.NewVersion("108.0.0+up0.25.0-rc.1")
		if err != nil {
			t.Fatalf("NewVersion(%q) failed: %v", "108.0.0+up0.25.0-rc.1", err)
		}
		if v.Major() != 108 || v.Minor() != 0 || v.Patch() != 0 {
			t.Errorf("got %d.%d.%d, want 108.0.0", v.Major(), v.Minor(), v.Patch())
		}
	})

	t.Run("Metadata returns +up string without leading +", func(t *testing.T) {
		v, err := semverV1.NewVersion("108.0.0+up0.9.0")
		if err != nil {
			t.Fatalf("NewVersion failed: %v", err)
		}
		got := v.Metadata()
		want := "up0.9.0"
		if got != want {
			t.Errorf("Metadata(): got %q, want %q", got, want)
		}
	})

	t.Run("Equal ignores build metadata — critical for validate.go sort logic", func(t *testing.T) {
		// 108.0.0+up0.9.0 and 108.0.0+up0.9.1 must be Equal() because semver
		// ignores build metadata in comparisons. validate.go depends on this
		// to detect when it needs to fall through to metadata comparison.
		a, _ := semverV1.NewVersion("108.0.0+up0.9.0")
		b, _ := semverV1.NewVersion("108.0.0+up0.9.1")
		if !a.Equal(b) {
			t.Errorf("Equal(): 108.0.0+up0.9.0 and 108.0.0+up0.9.1 should be equal (build metadata ignored)")
		}
	})

	t.Run("LessThan compares base version, ignores metadata", func(t *testing.T) {
		a, _ := semverV1.NewVersion("108.0.0+up0.9.0")
		b, _ := semverV1.NewVersion("108.0.1+up0.9.1")
		if !a.LessThan(b) {
			t.Errorf("LessThan(): 108.0.0 should be less than 108.0.1")
		}
	})

	t.Run("Metadata strip and re-parse mirrors validate.go ordering logic", func(t *testing.T) {
		// Mirrors validate.go:118-132
		// viMetadata, _ := strings.CutPrefix(vi.Metadata(), "up")
		// mi, err := semver.NewVersion(viMetadata)
		// return mi.LessThan(mj)
		vi, _ := semverV1.NewVersion("108.0.0+up0.9.0")
		vj, _ := semverV1.NewVersion("108.0.0+up0.9.1")

		// Versions are equal at the base level
		if !vi.Equal(vj) {
			t.Fatalf("precondition: versions should be equal at base level")
		}

		miStr, _ := strings.CutPrefix(vi.Metadata(), "up")
		mjStr, _ := strings.CutPrefix(vj.Metadata(), "up")

		mi, err := semverV1.NewVersion(miStr)
		if err != nil {
			t.Fatalf("NewVersion(%q) failed: %v", miStr, err)
		}
		mj, err := semverV1.NewVersion(mjStr)
		if err != nil {
			t.Fatalf("NewVersion(%q) failed: %v", mjStr, err)
		}

		if !mi.LessThan(mj) {
			t.Errorf("metadata 0.9.0 should be less than 0.9.1")
		}
	})

	t.Run("Minor accessor used in PR validation", func(t *testing.T) {
		v, _ := semverV1.NewVersion("108.3.0+up1.5.0")
		if v.Minor() != 3 {
			t.Errorf("Minor(): got %d, want 3", v.Minor())
		}
		if v.Patch() != 0 {
			t.Errorf("Patch(): got %d, want 0", v.Patch())
		}
	})
}
