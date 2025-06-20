package registries

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rancher/charts-build-scripts/pkg/logger"

	name "github.com/google/go-containerregistry/pkg/name"
	remote "github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	// PrimeURL of SUSE Prime registry
	PrimeURL string = "registry.suse.com/"
	// StagingURL of SUSE Staging registry
	StagingURL string = "stgregistry.suse.com/"
	// DockerURL of images
	DockerURL string = "docker.io/"

	loginURL = "https://hub.docker.com/v2/users/login/"
)

// checkRegistriesImagesTags will check and split which repository images/tags must be synced
// to the prime registry from the DockerHub and Staging registry.
//
//  1. Load all values.yaml files repositories/tags latest values
//  2. List all the repositories/tags from Prime Registry
//  3. Filter what is present only on DockerHub but not in the Prime Registry
//  4. From the list only present on Docker, list what is present in the Staging Registry
//  5. Split the difference (Docker only images/tags and Staging Also images/tags)
func checkRegistriesImagesTags(ctx context.Context) (map[string][]string, map[string][]string, map[string][]string, error) {
	logger.Log(ctx, slog.LevelInfo, "checking registries images and tags")

	// List all repository tags on Docker Hub by walking the entire image dependencies across all charts
	assetsImageTagMap, err := createAssetValuesRepoTagMap(ctx)
	fmt.Println(assetsImageTagMap["rancher/fleet-agent"])
	if err != nil {
		return nil, nil, nil, err
	}

	// Prime registry
	primeImgTags, err := listRegistryImageTags(ctx, assetsImageTagMap, PrimeURL)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to check prime image tags", logger.Err(err))
		return nil, nil, nil, err
	}

	// docker repository tags that are not present in the prime registry
	dockerNotPrimeImgTags := filterDockerNotPrimeTags(ctx, newTagMap(assetsImageTagMap), newTagMap(primeImgTags))

	// Staging registry
	stagingImgTags, err := listRegistryImageTags(ctx, dockerNotPrimeImgTags, StagingURL)
	if err != nil {
		return nil, nil, nil, err
	}

	/* Compare stgRegistry with primeImgTags, split bewtween
		Docker Image Tags Not In the Prime Registry and Not in the Staging Registry;
	    Staging Image Tags also In the Docker Hub and Not in the Prime registry
	*/
	dockerToPrime, stagingToPrime := splitDockerOnlyAndStgImgTags(ctx, newTagMap(dockerNotPrimeImgTags), newTagMap(stagingImgTags))
	return assetsImageTagMap, dockerToPrime, stagingToPrime, nil
}

// ListRegistryImageTags checks images and its tags on a given registry.
// this function is mockable for unit-testing.
var listRegistryImageTags = func(ctx context.Context, imageTagMap map[string][]string, registry string) (map[string][]string, error) {
	logger.Log(ctx, slog.LevelInfo, "listing registry images/tags", slog.String("remote", registry))

	remoteImgTagMap := make(map[string][]string)

	for asset := range imageTagMap {
		if asset == "" {
			continue
		}

		logger.Log(ctx, slog.LevelDebug, "listing...", slog.String(registry, asset))
		tags, err := fetchTagsFromRegistryRepo(ctx, registry+asset)
		if err != nil {
			logger.Log(ctx, slog.LevelError, "remote fetch failure", slog.Group(asset, logger.Err(err)))
			return nil, err
		}

		logger.Log(ctx, slog.LevelDebug, "", slog.Any("tags", tags))
		remoteImgTagMap[asset] = tags
	}

	return remoteImgTagMap, nil
}

// fetchTagsFromRegistryRepo will check a remote registry repository image for its tags.
func fetchTagsFromRegistryRepo(ctx context.Context, remoteTarget string) ([]string, error) {
	repo, err := name.NewRepository(remoteTarget)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "remote repository failure", logger.Err(err))
		return nil, err
	}

	options := []remote.Option{}

	tags, err := remote.List(repo, options...)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "list failure", logger.Err(err))
		return nil, err
	}

	return tags, nil
}

// filterDockerNotPrimeTags will only allow the tags that are not present in the prime registry but are present on Docker Hub
func filterDockerNotPrimeTags(ctx context.Context, dockerImgTags, primeImgTags map[string][]string) map[string][]string {
	logger.Log(ctx, slog.LevelInfo, "filter docker hub only tags from prime")

	dockerOnlyImgTags := make(map[string][]string)

	// Loop Fetched Docker Hub Image Tags
	for dockerImg, dockerTags := range dockerImgTags {
		if dockerImg == "" {
			continue
		}

		// Check for the first time the image is synced
		if _, exist := primeImgTags[dockerImg]; !exist {
			dockerOnlyImgTags[dockerImg] = dockerImgTags[dockerImg]
			continue
		}

		// Temporary hash set for prime tags
		primeTagHashSet := make(map[string]struct{})
		for _, primeTag := range primeImgTags[dockerImg] {
			primeTagHashSet[primeTag] = struct{}{}
		}

		// Filter for docker img tags not present at the prime registry
		for _, dockerTag := range dockerTags {
			if _, exist := primeTagHashSet[dockerTag]; !exist {
				dockerOnlyImgTags[dockerImg] = append(dockerOnlyImgTags[dockerImg], dockerTag)
			}
		}

		// Logs for later inspection, the registries sync is critical
		if len(dockerOnlyImgTags[dockerImg]) > 0 {
			logger.Log(ctx, slog.LevelDebug, "DockerHub only tags", slog.Any(dockerImg, dockerOnlyImgTags[dockerImg]))
		} else {
			logger.Log(ctx, slog.LevelDebug, "Docker/Prime tags already synced", slog.String("repo", dockerImg))
		}
	}

	return dockerOnlyImgTags
}

// splitDockerOnlyAndStgImgTags will split the given image tags lists in:
//   - dockerOnly -> images that are present at docker hub only (these are never signed)
//   - stagingAlso -> images that are present in staging and also in docker (these can be signed)
//
// stagingAlso should be synced from Staging Registry and not from Docker.
func splitDockerOnlyAndStgImgTags(ctx context.Context, dockerImgTags, stgImgTags map[string][]string) (map[string][]string, map[string][]string) {
	logger.Log(ctx, slog.LevelInfo, "splitting image tags between docker only and staging also image tags")

	dockerOnly := make(map[string][]string)
	stgAlso := make(map[string][]string)

	for dockerImg, dockerTags := range dockerImgTags {
		if _, exist := stgImgTags[dockerImg]; !exist {
			dockerOnly[dockerImg] = dockerTags
			continue
		}

		dockerOnlyTags, stgAlsoTags := splitTags(dockerTags, stgImgTags[dockerImg])
		if len(dockerOnlyTags) > 0 {
			dockerOnly[dockerImg] = dockerOnlyTags
			logger.Log(ctx, slog.LevelDebug, "DOCKER", slog.Any(dockerImg, dockerOnlyTags))

		}
		if len(stgAlsoTags) > 0 {
			stgAlso[dockerImg] = stgAlsoTags
			logger.Log(ctx, slog.LevelDebug, "STAGING", slog.Any(dockerImg, stgAlsoTags))
		}
	}

	return dockerOnly, stgAlso
}

// splitTags creates a hash set to iterate faster while separating the tags
func splitTags(dockerTags, stgTags []string) ([]string, []string) {
	dockerOnlyTags := make([]string, 0)
	stgAlsoTags := make([]string, 0)

	stgTagHashSet := make(map[string]struct{}, len(stgTags))
	// make hashsets
	for _, stgTag := range stgTags {
		stgTagHashSet[stgTag] = struct{}{}
	}

	for _, tag := range dockerTags {
		if _, exist := stgTagHashSet[tag]; exist {
			stgAlsoTags = append(stgAlsoTags, tag)
		} else {
			dockerOnlyTags = append(dockerOnlyTags, tag)
		}
	}

	return dockerOnlyTags, stgAlsoTags
}
