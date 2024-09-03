package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

var (
	errorNoBranchVersion         error = errors.New("branch version not provided")
	errorBranchVersionNotInRules error = errors.New("the given branch version is not defined in the rules")
)

// Version holds the maximum and minimum limits allowed for a specific branch version
type Version struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

// VersionRules will hold all the necessary information to check which assets versions are allowed to be in the repository
type VersionRules struct {
	Rules            map[string]Version `json:"rules"`
	BranchVersion    string             `json:"branch-version,omitempty"`
	MinVersion       int                `json:"min-version,omitempty"`
	MaxVersion       int                `json:"max-version,omitempty"`
	DevBranch        string             `json:"dev-branch,omitempty"`
	DevBranchPrefix  string             `json:"dev-branch-prefix"`
	ProdBranch       string             `json:"prod-branch,omitempty"`
	ProdBranchPrefix string             `json:"prod-branch-prefix"`
}

// loadFromJSON will load the version rules from version_rules.json file at charts repository
func loadFromJSON(fs billy.Filesystem) (*VersionRules, error) {
	vr := &VersionRules{
		Rules: make(map[string]Version),
	}

	if exist, err := filesystem.PathExists(fs, path.VersionRulesFile); err != nil || !exist {
		return nil, err
	}

	file, err := os.Open(filesystem.GetAbsPath(fs, path.VersionRulesFile))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&vr); err != nil {
		return nil, err
	}

	return vr, nil
}

type jsonLoader func(fs billy.Filesystem) (*VersionRules, error)

// rules will check and convert the provided branch version,
// create the hard-coded rules for the charts repository and calculate the minimum and maximum versions according to the branch version.
func (d *Dependencies) rules(branchVersion string, jsonLoad jsonLoader) (*VersionRules, error) {
	if branchVersion == "" {
		return nil, errorNoBranchVersion
	}

	v, err := jsonLoad(d.RootFs)
	if err != nil {
		return v, err
	}

	if _, ok := v.Rules[branchVersion]; !ok {
		return nil, errorBranchVersionNotInRules
	}

	v.BranchVersion = branchVersion

	// Calculate the min and maximum versions allowed for the current branch version lifecycle
	if err := v.getMinMaxVersionInts(); err != nil {
		return nil, err
	}

	v.ProdBranch = v.ProdBranchPrefix + v.BranchVersion
	v.DevBranch = v.DevBranchPrefix + v.BranchVersion

	return v, nil
}

// Current lifecycle rules are:
//
//	Branch can only hold until 2 previous versions of the current branch version.
//	Branch cannot hold versions from newer branches, only older ones.
//
// See CheckChartVersionForLifecycle() for more details.
func (v *VersionRules) getMinMaxVersionInts() error {
	// e.g: 2.9 - 0.2 = 2.7
	minVersionStr := v.Rules[(branchVersionMinorSum(v.BranchVersion, -2))].Min
	maxVersionStr := v.Rules[v.BranchVersion].Max

	var err error
	var min, max int = 0, 0

	if minVersionStr != "" {
		min, err = strconv.Atoi(strings.Split(minVersionStr, ".")[0])
		if err != nil {
			return err
		}
	}
	if maxVersionStr != "" {
		max, err = strconv.Atoi(strings.Split(maxVersionStr, ".")[0])
		if err != nil {
			return err
		}
	}

	v.MinVersion = min
	v.MaxVersion = max
	return nil
}

// convertBranchVersion will convert the received string flag into a float32
func convertBranchVersion(branchVersion string) (float32, error) {
	floatVersion, err := strconv.ParseFloat(branchVersion, 32)
	if err != nil {
		return 0.0, err
	}
	return float32(floatVersion), nil
}

// ExtractBranchVersion will extract the branch version from the branch name
func ExtractBranchVersion(branch string) string {
	parts := strings.Split(branch, "-v")
	return parts[len(parts)-1]
}

// CheckChartVersionForLifecycle will
// Check if the chart version is within the range of the current version:
//
//	If the chart version is within the range, return true, otherwise return false
func (v *VersionRules) CheckChartVersionForLifecycle(chartVersion string) bool {
	chartVersionInt, _ := strconv.Atoi(strings.Split(chartVersion, ".")[0])
	/**
	Rule Example:
	Branch version: 2.9
	Min version: 104
	Max version: 105
	Therefore, the chart version must be >= (104 - 2) and < 105
	i.e: 102 <= chartVersion < 105
	*/
	if chartVersionInt >= v.MinVersion && chartVersionInt < v.MaxVersion {
		return true
	}
	return false
}

// CheckChartVersionToRelease will return if the current versyion being analyzed is the one to be released or not
func (v *VersionRules) CheckChartVersionToRelease(chartVersion string) (bool, error) {
	chartVersionInt, err := strconv.Atoi(strings.Split(chartVersion, ".")[0])
	if err != nil {
		logrus.Errorf("failed to check version to release for chartVersion:%s with error:%v", chartVersion, err)
		return false, err
	}
	return chartVersionInt == (v.MaxVersion - 1), nil
}

// CheckForRCVersion checks if the chart version contains the "-rc" string indicating a release candidate version.
func (v *VersionRules) CheckForRCVersion(chartVersion string) bool {
	if strings.Contains(strings.ToLower(chartVersion), "-rc") {
		return true
	}
	return false
}

func branchVersionMinorSum(branchVersion string, number int) string {
	parts := strings.Split(branchVersion, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		// Handle error
		return ""
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		// Handle error
		return ""
	}

	minor += number
	return fmt.Sprintf("%d.%d", major, minor)
}
