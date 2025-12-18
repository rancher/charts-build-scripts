package config

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
)

// VersionRules contains the version management rules loaded from config/versionRules.yaml
// This struct supports both YAML unmarshaling and runtime-computed fields.
type VersionRules struct {
	// YAML fields (loaded from file)
	BranchVersion        string         `yaml:"branch-version"`             // Current branch version being processed (e.g., "2.13")
	ReleasePrefix        string         `yaml:"release-prefix"`             // Prefix for release branches (e.g., "release-v")
	DevPrefix            string         `yaml:"dev-prefix"`                 // Prefix for development branches (e.g., "dev-v")
	MinorRetentionPolicy int            `yaml:"minor-retention-policy"`     // Number of minor versions to retain (0 = unlimited)
	AllowedCandidates    []string       `yaml:"allowed-pre-release-naming"` // allowed naming conventions for build metadata in pre-release candidates
	AllowedMetadata      []string       `yaml:"allowed-metadata-naming"`    // allowed naming convetions for build metadata in release versions
	Rules                map[string]int `yaml:"version-rules"`              // Map of Rancher version to chart version prefix

	// Runtime fields (computed after loading)
	MinMajor int `yaml:"-"` // Minimum major version prefix to retain (based on retention policy)
}

// versionRules loads the version rules from config/versionRules.yaml and
// calculates runtime values like retention boundaries.
//
// Process:
//  1. Load version rules YAML file
//  2. Validate that the current branch version is defined in rules
//  3. Calculate MinMajor based on retention policy
//
// Example:
//
//	If branchVersion = "2.13" and MinorRetentionPolicy = 5:
//	- Current prefix: Rules["2.13"] = 108
//	- MinMajor = 108 - 5 = 103
//	- Only charts with prefix >= 103 will be processed
//
// This implements the "keep last N minor versions" policy.
func versionRules(ctx context.Context, RootFS billy.Filesystem) (*VersionRules, error) {

	// Load version rules from YAML
	filePath := filesystem.GetAbsPath(RootFS, PathVersionRulesYaml)

	vr, err := filesystem.LoadYamlFile[VersionRules](ctx, filePath, false)
	if err != nil {
		return nil, err
	}

	// Ensure the current branch version is defined in the rules
	if _, exist := vr.Rules[vr.BranchVersion]; !exist {
		return nil, errorBranchVersionNotInRules
	}

	// Calculate minimum major version prefix for retention policy
	// If retention policy is 0, keep all versions (MinMajor stays 0)
	// Otherwise, calculate the cutoff based on the current version minus retention count
	if vr.MinorRetentionPolicy > 0 {
		vr.MinMajor = vr.Rules[vr.BranchVersion] - vr.MinorRetentionPolicy
	}

	return vr, nil
}

// IsVersionCandidate returns if given version is a release candidate
func (vr *VersionRules) IsVersionCandidate(version string) bool {
	for _, name := range vr.AllowedCandidates {
		if strings.Contains(strings.ToLower(version), name) {
			return true
		}
	}
	return false
}

// IsVersionForCurrentLine checks if the given version has the Major prefix for the current branch line
func (vr *VersionRules) IsVersionForCurrentLine(version string) bool {
	return strings.HasPrefix(strings.ToLower(version),
		strconv.Itoa(vr.GetLineMajorPrefix()))
}

// GetLineMajorPrefix will return the current prefix version for the current branch version line
func (vr *VersionRules) GetLineMajorPrefix() int {
	return vr.Rules[vr.BranchVersion]
}

// CheckBranchVersion is a helper function to check if the user target branch is the same as the current local git repository state
func (c *Config) CheckBranchVersion(branchVersion string) error {
	if c.VersionRules.BranchVersion != branchVersion && branchVersion != "" {
		return errors.New("your target branch-version does not match the current branch")
	}
	return nil
}

// GetCurrentMajorPrefix is a helper function to quickly access the current branch verison line expected Major prefix version
func (c *Config) GetCurrentMajorPrefix(branchVersion string) int {
	return c.VersionRules.Rules[branchVersion]
}
