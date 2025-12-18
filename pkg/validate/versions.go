package validate

import (
	"context"
	"strconv"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/config"
)

// FilterPortsAndToRelease will filter out RC's, alphas, betas from to release versions
func FilterPortsAndToRelease(ctx context.Context, indexMap map[string][]string) (map[string][]string, map[string][]string, error) {
	cfg, err := config.FromContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	toRelease := make(map[string][]string)
	toPort := make(map[string][]string)

	for chart, versions := range indexMap {
		for _, version := range versions {
			isCandidate := false
			for _, candidate := range cfg.VersionRules.AllowedCandidates {
				if strings.Contains(version, candidate) {
					isCandidate = true
					break
				}
			}

			// If prefix matches with the version, this is a not a previous line version
			prefixMatched := VersionPrefixMatch(cfg, version)
			switch {
			// can only be forward-ported to dev branches but never to release branches
			case isCandidate:
				toPort[chart] = append(toPort[chart], version)

			// should be released
			case !isCandidate && prefixMatched:
				toRelease[chart] = append(toRelease[chart], version)

			// can only be forward-ported
			case !isCandidate && !prefixMatched:
				toPort[chart] = append(toPort[chart], version)

			}
		}
	}

	return toRelease, toPort, nil
}

// VersionPrefixMatch will check if the passed version belongs to the current branch prefix major version
func VersionPrefixMatch(cfg *config.Config, version string) bool {
	currentPrefix := cfg.VersionRules.Rules[cfg.VersionRules.BranchVersion]
	return strings.HasPrefix(version, strconv.Itoa(currentPrefix))
}

// VersionToRelease will check if the is a version that should be released
func VersionToRelease(cfg *config.Config, version string) bool {
	if !cfg.VersionRules.IsVersionForCurrentLine(version) ||
		cfg.VersionRules.IsVersionCandidate(version) {
		return false
	}

	return true
}
