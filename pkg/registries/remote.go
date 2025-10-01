package registries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"

	authn "github.com/google/go-containerregistry/pkg/authn"
	name "github.com/google/go-containerregistry/pkg/name"
	remote "github.com/google/go-containerregistry/pkg/v1/remote"
	transport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

const (
	// StagingURL of SUSE Staging registry
	StagingURL string = "stgregistry.suse.com/"
	// DockerURL of images
	DockerURL string = "docker.io/"

	loginURL = "https://hub.docker.com/v2/users/login/"
)

var (
	once          sync.Once
	authenticator authn.Authenticator
)

// checkRegistriesImagesTags will check and split which repository images/tags must be synced
// to the prime registry from the DockerHub and Staging registry.
//
//  1. Load all values.yaml files repositories/tags latest values
//  2. List all the repositories/tags from Prime Registry
//  3. Filter what is present only on DockerHub but not in the Prime Registry
//  4. From the list only present on Docker, list what is present in the Staging Registry
//  5. Split the difference (Docker only images/tags and Staging Also images/tags)
func checkRegistriesImagesTags(ctx context.Context, primeRegistry string) (map[string][]string, map[string][]string, map[string][]string, error) {
	logger.Log(ctx, slog.LevelInfo, "checking registries images and tags")

	// List all repository tags on Docker Hub by walking the entire image dependencies across all charts
	assetsImageTagMap, err := createAssetValuesRepoTagMap(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Prime registry
	primeImgTags, err := listRegistryImageTags(ctx, assetsImageTagMap, primeRegistry)
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
	logger.Log(ctx, slog.LevelInfo, "listing registry images/tags")

	remoteImgTagMap := make(map[string][]string)

	for asset := range imageTagMap {
		if asset == "" {
			continue
		}

		logger.Log(ctx, slog.LevelDebug, "listing...", slog.String("registry", asset))
		tags, err := fetchTagsFromRegistryRepo(ctx, registry, asset)
		if err != nil {
			logger.Log(ctx, slog.LevelError, "remote fetch failure", slog.Group(asset, logger.Err(err)))
			return nil, err
		}

		logger.Log(ctx, slog.LevelDebug, "", slog.Any("tags", tags))
		if len(tags) > 0 {
			remoteImgTagMap[asset] = tags
		}
	}

	return remoteImgTagMap, nil
}

// fetchTagsFromRegistryRepo will check a remote registry repository image for its tags.
// will be mocked using monkey patching.
var fetchTagsFromRegistryRepo = func(ctx context.Context, registry, asset string) ([]string, error) {

	repo, err := name.NewRepository(registry + asset)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "remote repository failure", logger.Err(err))
		return nil, err
	}

	var options []remote.Option
	options = append(options, remote.WithContext(ctx))
	// 1st: 0s | 2nd: 60s | 3rd: 180s
	options = append(options, remote.WithRetryBackoff(remote.Backoff{
		Duration: 60.0 * time.Second,
		Factor:   2.0,
		Steps:    3,
	}))
	options = append(options, remote.WithRetryStatusCodes([]int{
		http.StatusTooManyRequests, // 429
	}...))

	if registry == DockerURL {
		if auth := dockerCredentials(ctx); auth != nil {
			options = append(options, remote.WithAuth(auth))
		}
	}

	if registry != DockerURL && registry != StagingURL && strings.Contains(registry, "registry") {
		if auth := primeCredentials(ctx); auth != nil {
			options = append(options, remote.WithAuth(auth))
		}
	}

	// the default tag amount is 1000,
	// but remote package handles pagination internally if needed
	tags, err := remote.List(repo, options...)
	if err != nil {
		var transportError *transport.Error
		if errors.As(err, &transportError) {
			for _, d := range transportError.Errors {
				if d.Code == "NAME_UNKNOWN" {
					logger.Log(ctx, slog.LevelWarn, "repository not found", slog.String("repo", repo.Name()))
					return []string{}, nil
				}
			}
		}
		logger.Log(ctx, slog.LevelError, "list failure", logger.Err(err))
		return nil, err
	}

	return tags, nil
}

func dockerCredentials(ctx context.Context) authn.Authenticator {
	once.Do(func() {
		username := os.Getenv("DOCKER_USERNAME")
		password := os.Getenv("DOCKER_PASSWORD")

		if username == "" || password == "" {
			logger.Log(ctx, slog.LevelWarn, "Docker credentials not provided, proceeding with unauthenticated requests")
			authenticator = nil
		} else {
			authenticator = &authn.Basic{
				Username: username,
				Password: password,
			}
		}
	})
	return authenticator
}

func primeCredentials(ctx context.Context) authn.Authenticator {
	once.Do(func() {
		username := os.Getenv("REGISTRY_USERNAME")
		password := os.Getenv("REGISTRY_PASSWORD")

		if username == "" || password == "" {
			logger.Log(ctx, slog.LevelWarn, "Prime credentials not provided, proceeding with unauthenticated requests")
			authenticator = nil
		} else {
			authenticator = &authn.Basic{
				Username: username,
				Password: password,
			}
		}
	})
	return authenticator
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

	logger.Log(ctx, slog.LevelDebug, "docker only imgs", slog.Int("imgs", len(dockerOnly)))
	logger.Log(ctx, slog.LevelDebug, "staging also imgs", slog.Int("imgs", len(stgAlso)))

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

// DockerScan lists the repository tags from the local assets/ folder, compares them
// against the corresponding Docker Hub repository tags, and reports any discrepancies.
// It returns an error if an image tag from the assets/ folder is not found on Docker Hub,
// or if a repository is not in the `rancher` namespace.
func DockerScan(ctx context.Context) error {
	// Get required tags for all images retrieved from the all the values.yaml files in the .tgz files
	assetsTagMap, err := createAssetValuesRepoTagMap(ctx)
	if err != nil {
		return err
	}

	// check docker images against the assets/ folder values.yaml repositories/tags
	failedImages, outOfNamespaceImages, err := checkImagesFromDocker(ctx, assetsTagMap)
	if err != nil {
		return err
	}

	if len(failedImages) > 0 || len(outOfNamespaceImages) > 0 {
		logger.Log(ctx, slog.LevelError, "found images outside the rancher namespace", slog.Any("outOfNamespaceImages", outOfNamespaceImages))
		logger.Log(ctx, slog.LevelError, "images that are not on Docker Hub", slog.Any("failedImages", failedImages))
		return errors.New("image check has failed")
	}

	logger.Log(ctx, slog.LevelInfo, "all images checked")
	return nil
}

// checkImagesFromDocker receives a map of repository tags from local assets and fetches the corresponding tags from Docker Hub.
// It identifies images with tags that are not present on Docker Hub ("failedImages")
// and images that are not in the "rancher" namespace ("outOfNamespaceImages").
func checkImagesFromDocker(ctx context.Context, assetsTagMap map[string][]string) (map[string][]string, []string, error) {
	failedImages := make(map[string][]string, 0)
	outOfNamespaceImages := make([]string, 0)

	logger.Log(ctx, slog.LevelInfo, "comparing image tags from Docker Hub and local assets")

	for asset, assetTags := range assetsTagMap {
		if !strings.HasPrefix(asset, "rancher/") {
			logger.Log(ctx, slog.LevelError, "image is outside the rancher namespace", slog.String("img", asset))
			outOfNamespaceImages = append(outOfNamespaceImages, asset)
			continue
		}

		logger.Log(ctx, slog.LevelDebug, "comparing", slog.String("img", asset))
		dockerTags, err := fetchTagsFromRegistryRepo(ctx, DockerURL, asset)
		if err != nil {
			return failedImages, outOfNamespaceImages, err
		}

		if len(dockerTags) == 0 {
			logger.Log(ctx, slog.LevelError, "no docker tags found", slog.String("img", asset))
			failedImages[asset] = append(failedImages[asset], "no docker tags found for this image!")
			continue
		}

		tagHashSet := make(map[string]struct{}, len(dockerTags))
		for _, tag := range dockerTags {
			tagHashSet[tag] = struct{}{}
		}

		for _, tag := range assetTags {
			if _, exist := tagHashSet[tag]; !exist {
				logger.Log(ctx, slog.LevelError, "image tag not found on Docker Hub", slog.Group("img/tag", asset, tag))
				failedImages[asset] = append(failedImages[asset], tag)
			}
		}
	}

	return failedImages, outOfNamespaceImages, nil
}

// Legacy Code below;
// todos:
// 1. New implementation for checking RC tags

// DockerCheckRCTags checks for any images that have RC tags
func DockerCheckRCTags(ctx context.Context, repoRoot string) map[string][]string {

	// Get the release options from the release.yaml file
	releaseOptions := getReleaseOptions(ctx, repoRoot)
	logger.Log(ctx, slog.LevelInfo, "checking for RC tags in charts", slog.Any("releaseOptions", releaseOptions))

	rcImageTagMap := make(map[string][]string, 0)

	// Get required tags for all images
	imageTagMap, err := GenerateFilteredImageTagMap(ctx, releaseOptions)
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("failed to generate image tag map: %s", err).Error())
	}

	logger.Log(ctx, slog.LevelInfo, "checking for RC tags in all collected images")

	// Grab all images that contain RC tags
	for image := range imageTagMap {
		for _, tag := range imageTagMap[image] {
			if strings.Contains(tag, "-rc") {
				rcImageTagMap[image] = append(rcImageTagMap[image], tag)
			}
		}
	}

	return rcImageTagMap
}

// getReleaseOptions returns the release options from the release.yaml file
func getReleaseOptions(ctx context.Context, repoRoot string) options.ReleaseOptions {
	// Get the filesystem on the repo root
	repoFs := filesystem.GetFilesystem(repoRoot)

	// Load the release options from the release.yaml file
	releaseOptions, err := options.LoadReleaseOptionsFromFile(ctx, repoFs, "release.yaml")
	if err != nil {
		logger.Fatal(ctx, fmt.Errorf("unable to unmarshall release.yaml: %s", err).Error())
	}

	return releaseOptions
}

// GenerateFilteredImageTagMap returns a map of container images and their tags
func GenerateFilteredImageTagMap(ctx context.Context, filter map[string][]string) (map[string][]string, error) {
	imageTagMap := make(map[string][]string)

	err := filesystem.WalkFilteredAssetsFolder(ctx, imageTagMap, filter, chartsToIgnoreTags)
	if err != nil {
		return imageTagMap, err
	}

	return imageTagMap, nil
}
