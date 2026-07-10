package util

import (
	"strings"

	"github.com/Masterminds/semver"
)

// SortUpstreamAppVersions compares 2 Rancher chart version strings for Descending sort.
// Returns true if vA should come before vB (higher version first).
//
// Version format: <upstream_version>+up<chart_version>
// Examples:
//   - "109.0.1+up0.10.4-rc.1"
//   - "109.0.1+up0.10.1"
//   - "109.0.1"
//
// Comparison logic:
//  1. Split on "+up" delimiter into upstream and chart parts
//  2. Compare upstream versions using semver (descending)
//  3. If upstream equal, compare chart versions using semver (descending)
//  4. Semver automatically handles prereleases (rc, alpha, beta) per spec
//  5. Invalid semver versions pushed to end of sort order
//
// Use with sort.Slice:
//
//	sort.Slice(versions, func(i, j int) bool {
//	    return util.SortUpstreamAppVersions(versions[i], versions[j])
//	})
func SortUpstreamAppVersions(vA, vB string) bool {
	// Split versions into upstream and chart parts
	// Format: <upstream_version>+up<chart_version>
	upstreamA, chartA := splitUpstreamChart(vA)
	upstreamB, chartB := splitUpstreamChart(vB)

	// Parse upstream versions using semver
	semverUpstreamA, errA := semver.NewVersion(upstreamA)
	semverUpstreamB, errB := semver.NewVersion(upstreamB)

	if errA != nil {
		return false // push invalid to end
	}
	if errB != nil {
		return true // push invalid to end
	}

	// If upstream versions are different, use semver comparison (descending)
	if !semverUpstreamA.Equal(semverUpstreamB) {
		return semverUpstreamA.GreaterThan(semverUpstreamB)
	}

	// Same upstream version - compare chart versions (the part after +up)
	if chartA != "" && chartB != "" {
		semverChartA, errA := semver.NewVersion(chartA)
		semverChartB, errB := semver.NewVersion(chartB)

		if errA != nil {
			return false // push invalid to end
		}
		if errB != nil {
			return true // push invalid to end
		}

		// Compare chart versions (descending)
		// This automatically handles prereleases correctly per semver spec
		if !semverChartA.Equal(semverChartB) {
			return semverChartA.GreaterThan(semverChartB)
		}
	}

	// Versions are equal
	return false
}

// splitUpstreamChart splits a version string into upstream and chart parts
// Example: "109.0.1+up0.10.4-rc.1" returns ("109.0.1", "0.10.4-rc.1")
// Example: "109.0.1" returns ("109.0.1", "")
func splitUpstreamChart(version string) (upstream, chart string) {
	parts := strings.Split(version, "+up")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return version, ""
}

// LatestSameMajor returns the highest available tag that shares the same major version
// as current and has no pre-release suffix, along with whether an update is needed.
// Non-semver and pre-release tags in available are skipped.
// Returns ("", false) when current cannot be parsed as semver.
func LatestSameMajor(current string, available []string) (string, bool) {
	currentVer, err := semver.NewVersion(current)
	if err != nil {
		return "", false
	}

	var bestVer *semver.Version
	var bestTag string

	for _, tag := range available {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if v.Prerelease() != "" {
			continue
		}
		if v.Major() != currentVer.Major() {
			continue
		}
		if bestVer == nil || v.GreaterThan(bestVer) {
			bestVer = v
			bestTag = tag
		}
	}

	if bestVer == nil {
		return "", false
	}
	return bestTag, bestVer.GreaterThan(currentVer)
}
