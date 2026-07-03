package registries

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	semver "github.com/Masterminds/semver/v3"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// ImageResult holds the version-check result for one image.
type ImageResult struct {
	Name            string `json:"name"`
	Repository      string `json:"repository"`
	CurrentTag      string `json:"currentTag"`
	LatestAvailable string `json:"latestAvailable"`
	NeedsUpdate     bool   `json:"needsUpdate"`
}

// Report is the top-level output of ValidateImageVersions.
type Report struct {
	Chart              string        `json:"chart"`
	NeedsUpdate        bool          `json:"needsUpdate"`
	Images             []ImageResult `json:"images"`
	MissingFromChart   []string      `json:"missingFromChart,omitempty"`
	SkippedUnsupported []string      `json:"skippedUnsupported,omitempty"`
}

// latestSameMajor returns the highest available tag that shares the same major version
// as current and has no pre-release suffix, along with whether an update is needed.
// Non-semver and pre-release tags in available are skipped.
// Returns ("", false) when current cannot be parsed as semver.
func latestSameMajor(current string, available []string) (string, bool) {
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

// collectChartImages walks the unpacked chart source under <repoRoot>/charts/<chart>/<version>
// and returns a map of repository → []tags found in any values.yaml or values.yml.
func collectChartImages(ctx context.Context, repoRoot, chart, version string) (map[string][]string, error) {
	chartsBase := filepath.Join(repoRoot, "charts", chart)
	if _, err := os.Stat(chartsBase); os.IsNotExist(err) {
		return nil, fmt.Errorf("chart directory not found: %s", chartsBase)
	}

	versionDir := filepath.Join(chartsBase, version)

	repoTagMap := make(map[string][]string)
	if err := walkValuesFiles(ctx, versionDir, repoTagMap); err != nil {
		return nil, err
	}
	return repoTagMap, nil
}

// walkValuesFiles walks a chart directory tree looking for values.yaml / values.yml files
// (including nested charts/ subcharts) and feeds each into traverseRepoTags.
func walkValuesFiles(ctx context.Context, root string, repoTagMap map[string][]string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base != "values.yaml" && base != "values.yml" {
			return nil
		}
		data, err := filesystem.DecodeValuesYamlFile(ctx, path)
		if err != nil {
			logger.Log(ctx, slog.LevelWarn, "skipping values file", slog.String("path", path), logger.Err(err))
			return nil
		}
		traverseRepoTags(ctx, data, repoTagMap, "")
		return nil
	})
}

// ValidateImageVersions checks whether the images listed in configPath are on their
// latest minor/patch version within the chart at <repoRoot>/charts/<chart>/<version>.
func ValidateImageVersions(ctx context.Context, repoRoot, chart, version, configPath string) (Report, error) {
	report := Report{Chart: chart, Images: []ImageResult{}}

	cfg, err := options.LoadImageVersionCheck(configPath)
	if err != nil {
		return report, fmt.Errorf("loading config: %w", err)
	}

	chartImages, err := collectChartImages(ctx, repoRoot, chart, version)
	if err != nil {
		return report, fmt.Errorf("collecting chart images: %w", err)
	}

	for _, entry := range cfg.Images {
		currentTags, found := chartImages[entry.Repository]
		// check if the chart uses the image
		if !found || len(currentTags) == 0 {
			logger.Log(ctx, slog.LevelWarn, "image not found in chart", slog.String("repository", entry.Repository))
			report.MissingFromChart = append(report.MissingFromChart, entry.Repository)
			continue
		}

		// only dockerhub for now
		registryURL := DockerURL
		availableTags, err := fetchTagsFromRegistryRepo(ctx, registryURL, entry.Repository)
		if err != nil {
			return report, fmt.Errorf("fetching tags for %s: %w", entry.Repository, err)
		}

		// There may be multiple current tags; check each.
		for _, currentTag := range currentTags {
			latestTag, needsUpdate := latestSameMajor(currentTag, availableTags)
			result := ImageResult{
				Name:            entry.Name,
				Repository:      entry.Repository,
				CurrentTag:      currentTag,
				LatestAvailable: latestTag,
				NeedsUpdate:     needsUpdate,
			}
			report.Images = append(report.Images, result)
			if needsUpdate {
				report.NeedsUpdate = true
			}
		}
	}

	return report, nil
}
