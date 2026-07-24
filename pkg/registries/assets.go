package registries

import (
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// createAssetValuesRepoTagMap maps all the latest repository/tags in charts repository.
//  1. walk through all assets folders .tgz files
//  2. untar all values.yaml files into memory and make a map from it.
//  3. traverse all 'repository' and 'tag' fields, filtering the chartsToIgnoreTags.
//
// this function is mocked for unit-testing
var createAssetValuesRepoTagMap = func(ctx context.Context) (map[string][]string, error) {
	logger.Log(ctx, slog.LevelInfo, "map all repository and tags from all values.yaml files on assets/<*>/<*>.tgz files")

	repoTagMap := make(map[string][]string)

	// iterate all folders inside assets/ dir
	logger.Log(ctx, slog.LevelInfo, "walking assets folders .tgz files")
	assetsTgzs, err := filesystem.WalkAssetsFolderTgzFiles(ctx)
	if err != nil {
		return nil, err
	}

	// load blocklist to skip checking images for blocklisted chart versions
	blocklist, err := config.LoadBlockList(ctx)
	if err != nil {
		return nil, err
	}

	// filter out blocklisted chart versions
	assetsTgzs = filterBlocklistedAssets(ctx, assetsTgzs, blocklist)

	// decode .tgz values.yaml files into-memory
	logger.Log(ctx, slog.LevelInfo, "decoding .tgz(values.yaml) in-memory")
	valuesYamlsMap, err := filesystem.DecodeTgzValuesYamlMap(ctx, assetsTgzs)
	if err != nil {
		return nil, err
	}

	logger.Log(ctx, slog.LevelInfo, "", slog.Any("chartsToIgnoreTags", chartsToIgnoreTags))

	// iterate through each values.yaml file and traverse through all fields looking for
	// every value of a 'repository' and 'tag' field.
	for tgz, yamls := range valuesYamlsMap {
		// 1 asset (.tgz file) can have 1 or more values.(yaml||yml) files.
		for _, data := range yamls {
			// check ignored .tgz files
			ignore := false
			for ignoreChart, ignoreTag := range chartsToIgnoreTags {
				if strings.Contains(tgz, ignoreChart) {
					logger.Log(ctx, slog.LevelDebug, "must ignore", slog.String(".tgz", tgz), slog.String("tag", ignoreTag))
					traverseRepoTags(ctx, data, repoTagMap, ignoreTag)
					ignore = true
				}
			}

			// do not traverse twice the same values.yaml
			if ignore {
				continue
			}

			// traverse without a tag to filter
			traverseRepoTags(ctx, data, repoTagMap, "")
		}
	}

	return repoTagMap, nil
}

// filterBlocklistedAssets removes blocklisted chart versions from tgz list.
// Expected tgz path format: assets/{chart}/{chart}-{version}.tgz
func filterBlocklistedAssets(ctx context.Context, tgzPaths []string, blocklist *config.Blocklist) []string {
	filtered := make([]string, 0, len(tgzPaths))

	for _, tgzPath := range tgzPaths {
		// extract chart name and version from path
		// assets/rancher-monitoring/rancher-monitoring-109.0.1+up80.9.1.tgz
		base := filepath.Base(tgzPath)
		chartDir := filepath.Base(filepath.Dir(tgzPath))

		// remove .tgz extension
		nameVersion := strings.TrimSuffix(base, ".tgz")

		// remove chart name prefix to get version
		// rancher-monitoring-109.0.1+up80.9.1 -> 109.0.1+up80.9.1
		version := strings.TrimPrefix(nameVersion, chartDir+"-")

		if blocklist.IsBlocked(chartDir, version) {
			logger.Log(ctx, slog.LevelWarn, "skipping blocklisted chart version",
				slog.String("chart", chartDir),
				slog.String("version", version),
				slog.String("path", tgzPath))
			continue
		}

		filtered = append(filtered, tgzPath)
	}

	logger.Log(ctx, slog.LevelInfo, "filtered blocklisted assets",
		slog.Int("total", len(tgzPaths)),
		slog.Int("filtered", len(filtered)),
		slog.Int("skipped", len(tgzPaths)-len(filtered)))

	return filtered
}

// traverseRepoTags will traverse across 'data' whihc should be nesteds map[string]interface and []interface.
// it will look for 'repository' and 'tag' fields to save these values at 'repoTagMap' and return.
// if 'ignoreTag' is != "", the tag will not be appended to 'repoTagMap'.
func traverseRepoTags(ctx context.Context, data interface{}, repoTagMap map[string][]string, ignoreTag string) {
	// check for duplicate or to be filtered tags before appending below
	isDuplicateOrIgnoredTag := func(tag, repo string, tags []string) bool {
		if !slices.Contains(tags, tag) {
			if ignoreTag == "" {
				return false
			} else if ignoreTag != tag {
				return false
			}
			logger.Log(ctx, slog.LevelWarn, "ignoring", slog.String("repository", repo), slog.String("tag", tag))
		}
		return true
	}

	switch value := data.(type) {
	case map[string]interface{}:
		repo, repoExist := value["repository"]
		tag, tagExist := value["tag"]

		// there can be nil repository and empty tag fields
		if (repoExist && repo != nil) && (tagExist && tag.(string) != "") {
			if !isDuplicateOrIgnoredTag(tag.(string), repo.(string), repoTagMap[repo.(string)]) {
				repoTagMap[repo.(string)] = append(repoTagMap[repo.(string)], tag.(string))
				return // stop traversing this is the last child
			}
		}
		// keep traversing child nodes
		for _, value := range value {
			traverseRepoTags(ctx, value, repoTagMap, ignoreTag)
		}

	// []interface should contain maps inside, keep traversing
	case []interface{}:
		for _, value := range value {
			traverseRepoTags(ctx, value, repoTagMap, ignoreTag)
		}
	}
	return
}
