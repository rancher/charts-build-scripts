package registries

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/util"
)

// ImageResult holds the version-check result for one image.
type ImageResult struct {
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

var LoadImageVersionList = config.LoadImageVersionList

// collectChartImages walks the unpacked chart source under <repoRoot>/charts/<chart>/<version>
// and returns a map of repository → []tags found in any values.yaml or values.yml.
func collectChartImages(ctx context.Context, repoRoot, chart, version string) (map[string][]string, error) {
	chartsBase := filepath.Join("charts", chart)
	exists, err := filesystem.PathExists(ctx, filesystem.GetFilesystem(repoRoot), filepath.Join("charts", chart))
	if err != nil || !exists {
		return nil, fmt.Errorf("chart directory not found: %s", chartsBase)
	}

	versionDir := filepath.Join(repoRoot, chartsBase, version)

	repoTagMap := make(map[string][]string)
	err = filepath.WalkDir(versionDir, func(path string, d os.DirEntry, err error) error {
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

	if err != nil {
		return nil, err
	}
	return repoTagMap, nil
}

// ValidateImageVersions checks whether the images listed in configPath are on their
// latest minor/patch version within the chart at <repoRoot>/charts/<chart>/<version>.
func ValidateImageVersions(ctx context.Context, repoRoot, chart, version string) (Report, error) {
	report := Report{Chart: chart, Images: []ImageResult{}}

	cfg, err := LoadImageVersionList(ctx)
	if err != nil {
		return report, fmt.Errorf("loading config: %w", err)
	}

	chartImages, err := collectChartImages(ctx, repoRoot, chart, version)
	if err != nil {
		return report, fmt.Errorf("collecting chart images: %w", err)
	}

	for _, entry := range *cfg {
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
			latestTag, needsUpdate := util.LatestSameMajor(currentTag, availableTags)
			result := ImageResult{
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
