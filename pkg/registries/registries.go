package registries

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// chartsToIgnoreTags defines the charts and system charts in which a specified
// image tag should be ignored.
var chartsToIgnoreTags = map[string]string{
	"rancher-vsphere-csi": "latest",
	"rancher-vsphere-cpi": "latest",
}

// Scan will untar and map all images/tags dependencies, scan the staging registry,
// prime registry and if they are available there, it will create 2 yaml files:
//   - dockerToPrime.yaml
//   - stagingToPrime.yaml
//
// Which will be used by another process to sync images/tags to Prime registry.
func Scan(ctx context.Context, primeRegistry string) error {
	if primeRegistry == "" {
		return errors.New("no Prime URL provided")
	}
	// check the state of current assets and prime/staging registries
	_, dockerToPrime, stagingToPrime, err := checkRegistriesImagesTags(ctx, primeRegistry)
	if err != nil {
		return err
	}
	// sanitize the tags, we don't sync ever RC's, alphas, betas.
	fromDocker, fromStaging := sanitizeTags(dockerToPrime), sanitizeTags(stagingToPrime)

	logger.Log(ctx, slog.LevelInfo, "Docker to Prime registry",
		slog.String("file", config.PathDockerSyncYaml))

	// Create the sync yaml files
	if err := createSyncYamlFile(ctx, fromDocker, config.PathDockerSyncYaml); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "Staging to Prime registry",
		slog.String("file", config.PathStagingSyncYaml))

	if err := createSyncYamlFile(ctx, fromStaging, config.PathStagingSyncYaml); err != nil {
		return err
	}

	// Separate the creation of the files for easier inspection.
	logger.Log(ctx, slog.LevelInfo, "commiting")
	if err := checkStatusAndCommit(ctx, "images/tags (Docker to Prime) and (Staging to Prime)"); err != nil {
		return err
	}

	return nil
}

// checkStatusAndCommit will create a unique commit for inspecting which images/tags will be synced later in the CI process.
func checkStatusAndCommit(ctx context.Context, message string) error {
	logger.Log(ctx, slog.LevelDebug, "getting current working dir")
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelDebug, "opening git repot at: ", slog.String("currentDir", currentDir))
	g, err := git.OpenGitRepo(ctx, ".")
	if err != nil {
		return err
	}

	clean, _ := g.StatusProcelain(ctx)
	logger.Log(ctx, slog.LevelDebug, "git status", slog.Bool("clean", clean))
	if clean {
		logger.Log(ctx, slog.LevelWarn, "nothing to commit")
		return nil
	}

	logger.Log(ctx, slog.LevelDebug, "git add and git commit", slog.String("message", message))
	if err := g.AddAndCommit(message); err != nil {
		return err
	}

	return nil
}

// newTagMap makes a new deep copy of the original map to ensure data independence
func newTagMap(originalMap map[string][]string) map[string][]string {
	if originalMap == nil {
		return nil
	}

	newMap := make(map[string][]string, len(originalMap))
	for key, tags := range originalMap {
		if len(tags) == 0 {
			continue
		}

		newTags := make([]string, len(tags))
		copy(newTags, tags)
		newMap[key] = newTags
	}

	return newMap
}

// createSyncYamlFile will attempt to create or just open the given sync yaml file.
func createSyncYamlFile(ctx context.Context, imageTagMap map[string][]string, path string) error {
	file, err := filesystem.CreateAndOpenYamlFile(ctx, path, true)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := filesystem.UpdateYamlFile(file, imageTagMap); err != nil {
		logger.Log(ctx, slog.LevelError, "update failed", slog.Group("args", path, logger.Err(err)))
	}

	return nil
}

// sanitizeTags filters specific tags that should not bot synced to prime registry
func sanitizeTags(imageTagMap map[string][]string) map[string][]string {
	result := make(map[string][]string)

	for repo, tags := range imageTagMap {
		for _, tag := range tags {
			if strings.Contains(tag, "-rc") ||
				strings.Contains(tag, "-beta") ||
				strings.Contains(tag, "-alpha") ||
				strings.HasPrefix(tag, "sha256-") {
				continue
			}
			result[repo] = append(result[repo], tag)
		}
	}

	return result
}
