package util

import (
	"strings"

	"github.com/Masterminds/semver"
)

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
