package lifecycle

import (
	"fmt"
	"strconv"
	"strings"
)

type version struct {
	min string
	max string
}

// VersionRules will hold all the necessary information to check which assets versions are allowed to be in the repository
type VersionRules struct {
	rules         map[float32]version
	branchVersion float32
	minVersion    int
	maxVersion    int
}

func (vr *VersionRules) log(debug bool) {
	if !debug {
		return
	}

	for key, val := range vr.rules {
		cycleLog(debug, "Branch version", key)
		cycleLog(debug, "|- min version", val.min)
		cycleLog(debug, "|- max version", val.max)
	}
	cycleLog(debug, "Applied rules for branch version", nil)
	cycleLog(debug, "|-- min version", vr.minVersion)
	cycleLog(debug, "|-- max version", vr.maxVersion)
}

// GetVersionRules will check and convert the provided branch version,
// create the hard-coded rules for the charts repository and calculate the minimum and maximum versions according to the branch version.
func GetVersionRules(branchVersion string, debug bool) (*VersionRules, error) {
	if branchVersion == "" {
		return nil, fmt.Errorf("branch version is empty")
	}
	// The rules are defined by the minimum and maximum version that the assets can have
	var VersionRulesMap = map[float32]version{
		2.9: {min: "104.0.0", max: "105.0.0"},
		2.8: {min: "103.0.0", max: "104.0.0"},
		2.7: {min: "101.0.0", max: "103.0.0"}, // 101 and 102, this is the only case like it
		2.6: {min: "100.0.0", max: "101.0.0"},
		2.5: {max: "100.0.0"},
	}
	// Just convert the string provided branch version to a float32
	floatBranchVersion, err := convertBranchVersion(branchVersion)
	if err != nil {
		return nil, err
	}
	// Check if floatBranchVersion is an existing key in VersionRulesMap
	if _, ok := VersionRulesMap[floatBranchVersion]; !ok {
		return nil, fmt.Errorf("branch version %v is not defined in the rules", floatBranchVersion)
	}

	vr := &VersionRules{
		rules:         VersionRulesMap,
		branchVersion: floatBranchVersion,
	}

	// Calculate the min and maximum versions allowed for the current branch version lifecycle
	vr.getMinMaxVersionInts()

	vr.log(debug)

	return vr, err
}

// Current lifecycle rules are:
//
//	Branch can only hold until 2 previous versions of the current branch version.
//	Branch cannot hold versions from newer branches, only older ones.
//
// See checkChartVersionForLifecycle() for more details.
func (vr *VersionRules) getMinMaxVersionInts() {
	// e.g: 2.9 - 0.2 = 2.7
	minVersionStr := vr.rules[(vr.branchVersion - 0.2)].min
	maxVersionStr := vr.rules[vr.branchVersion].max

	// Don't check for errors here, these are hard-coded values
	min, _ := strconv.Atoi(strings.Split(minVersionStr, ".")[0])
	max, _ := strconv.Atoi(strings.Split(maxVersionStr, ".")[0])

	vr.minVersion = min
	vr.maxVersion = max
	return
}

// convertBranchVersion will convert the received string flag into a float32
func convertBranchVersion(branchVersion string) (float32, error) {
	floatVersion, err := strconv.ParseFloat(branchVersion, 32)
	if err != nil {
		return 0.0, err
	}
	return float32(floatVersion), nil
}

// checkChartVersionForLifecycle will
// Check if the chart version is within the range of the current version:
//
//	If the chart version is within the range, return true, otherwise return false
func (vr *VersionRules) checkChartVersionForLifecycle(chartVersion string) bool {
	chartVersionInt, _ := strconv.Atoi(strings.Split(chartVersion, ".")[0])
	/**
	Rule Example:
	Branch version: 2.9
	Min version: 104
	Max version: 105
	Therefore, the chart version must be >= (104 - 2) and < 105
	i.e: 102 <= chartVersion < 105
	*/
	if chartVersionInt >= vr.minVersion && chartVersionInt < vr.maxVersion {
		return true
	}
	return false
}