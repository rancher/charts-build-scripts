package canary_test

import (
	"testing"

	semverV3 "github.com/Masterminds/semver/v3"
)

// TestSemverV3ChartVersionCompat is a canary for the Masterminds/semver/v3
// replace directive in go.mod (pinned to v3.3.0).
//
// Our chart versions use the form <repo-prefix>+up<upstream> with optional
// pre-release suffixes (e.g. 108.0.0+up1.0.0-rc.1). semver/v3 is pulled in
// transitively by helm.sh/helm/v3, and v3.4.0+ changed constraint evaluation
// behaviour for pre-release versions in a way that breaks chart version
// validation. The replace directive in go.mod keeps it at v3.3.0.
//
// If this test fails it means either:
//   - the replace directive was removed from go.mod, or
//   - a semver/v3 version beyond v3.3.0 changed parsing/comparison behaviour.
//
// Re-examine the replace directive before merging any semver/v3 or helm bump.
func TestSemverV3ChartVersionCompat(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		expectedMajor uint64
		expectedMinor uint64
		expectedPatch uint64
	}{
		{
			name:          "stable +up version",
			version:       "108.0.0+up1.0.0",
			expectedMajor: 108,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "rc prerelease with +up metadata",
			version:       "108.0.0+up0.25.0-rc.1",
			expectedMajor: 108,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "alpha prerelease with +up metadata",
			version:       "108.0.0+up0.14.0-alpha.5",
			expectedMajor: 108,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "beta prerelease with +up metadata",
			version:       "108.0.0+up0.14.0-beta.1",
			expectedMajor: 108,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "patch release with +up metadata",
			version:       "108.0.2+up1.2.3",
			expectedMajor: 108,
			expectedMinor: 0,
			expectedPatch: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := semverV3.NewVersion(tt.version)
			if err != nil {
				t.Fatalf("semver/v3 failed to parse chart version %q: %v\n"+
					"Check the replace directive for github.com/Masterminds/semver/v3 in go.mod",
					tt.version, err)
			}
			if v.Major() != tt.expectedMajor {
				t.Errorf("major: got %d, want %d", v.Major(), tt.expectedMajor)
			}
			if v.Minor() != tt.expectedMinor {
				t.Errorf("minor: got %d, want %d", v.Minor(), tt.expectedMinor)
			}
			if v.Patch() != tt.expectedPatch {
				t.Errorf("patch: got %d, want %d", v.Patch(), tt.expectedPatch)
			}
		})
	}
}
