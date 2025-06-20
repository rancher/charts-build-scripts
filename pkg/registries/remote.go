package registries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/rest"

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

// Legacy Code below;
// todos:
// 1. Refactor
// 2. Stop checking DockerHub per tag and start listing tags with less http requests.
// 3. getReleaseOptions must be deleted and use the new LoadYamlFile approach

// TokenRequest is the request body for the Docker Hub API Login endpoint
type TokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// TokenReponse is the response body for the Docker Hub API Login endpoint
type TokenReponse struct {
	Token string `json:"token"`
}

// DockerCheckImages checks if all container images used in charts belong to the rancher namespace
func DockerCheckImages(ctx context.Context) error {
	failedImages := make(map[string][]string, 0)

	// Get required tags for all images retrieved from the all the values.yaml files in the .tgz files
	assetsTagMap, err := createAssetValuesRepoTagMap(ctx)
	if err != nil {
		return err
	}

	// Check if there's any image outside the rancher namespace
	imagesOutsideNamespace := checkPattern(ctx, assetsTagMap)

	// Get a token to access the Docker Hub API
	token, err := retrieveToken(ctx)
	if err != nil {
		logger.Log(ctx, slog.LevelWarn, "failed to retrieve token, requests will be unauthenticated", logger.Err(err))
	}

	// Loop through all images and tags to check if they exist
	for image := range assetsTagMap {
		if len(image) == 0 {
			logger.Log(ctx, slog.LevelWarn, "found blank image, skipping tag check")
			continue
		}

		// Split image into namespace and repository
		location := strings.Split(image, "/")
		if len(location) != 2 {
			logger.Log(ctx, slog.LevelError, "failed to split image into namespace and repository", slog.String("image", image))
			return fmt.Errorf("failed to generate namespace and repository for image: %s", image)
		}

		// Check if all tags exist
		for _, tag := range assetsTagMap[image] {
			err := checkTag(ctx, location[0], location[1], tag, token)
			if err != nil {
				failedImages[image] = append(failedImages[image], tag)
			}
		}
	}

	// If there are any images that have failed the check, log them and return an error
	if len(failedImages) > 0 || len(imagesOutsideNamespace) > 0 {
		logger.Log(ctx, slog.LevelError, "found images outside the rancher namespace", slog.Any("imagesOutsideNamespace", imagesOutsideNamespace))
		logger.Log(ctx, slog.LevelError, "images that are not on Docker Hub", slog.Any("failedImages", failedImages))
		return errors.New("image check has failed")
	}

	return nil
}

// checkPattern checks for pattern "rancher/*" in an array and returns items that do not match.
func checkPattern(ctx context.Context, imageTagMap map[string][]string) []string {
	nonMatchingImages := make([]string, 0)

	for image := range imageTagMap {
		if len(image) == 0 {
			logger.Log(ctx, slog.LevelWarn, "found blank image, skipping image namespace check")
			continue
		}
		if !strings.HasPrefix(image, "rancher/") {
			nonMatchingImages = append(nonMatchingImages, image)
		}
	}

	return nonMatchingImages
}

// checkTag checks if a tag exists in a namespace/repository
func checkTag(ctx context.Context, namespace, repository, tag, token string) error {
	logger.Log(ctx, slog.LevelDebug, "checking tag", slog.String("namespace", namespace), slog.String("repository", repository), slog.String("tag", tag))

	url := fmt.Sprintf("https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags/%s", namespace, repository, tag)

	// Sends HEAD request to check if namespace/repository:tag exists
	err := rest.Head(ctx, url, token)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "failed to check tag", logger.Err(err))
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "tag found", slog.String("repository", repository), slog.String("tag", tag))
	return nil
}

// retrieveToken retrieves a token to access the Docker Hub API
func retrieveToken(ctx context.Context) (string, error) {

	// Retrieve credentials from environment variables
	credentials := retrieveCredentials(ctx)
	if credentials == nil {
		logger.Log(ctx, slog.LevelWarn, "no credentials found, requests will be unauthenticated")
		return "", nil
	}

	var response TokenReponse

	// Sends POST request to retrieve token
	err := rest.Post(loginURL, credentials, &response)
	if err != nil {
		return "", err
	}

	return response.Token, nil
}

// retrieveCredentials retrieves credentials from environment variables
func retrieveCredentials(ctx context.Context) *TokenRequest {

	username := os.Getenv("DOCKER_USERNAME")
	password := os.Getenv("DOCKER_PASSWORD")

	if strings.Compare(username, "") == 0 {
		logger.Log(ctx, slog.LevelError, "DOCKER_USERNAME not set", slog.String("username", username))
		return nil
	}

	if strings.Compare(password, "") == 0 {
		logger.Log(ctx, slog.LevelError, "DOCKER_PASSWORD not set", slog.String("password", password))
		return nil
	}

	return &TokenRequest{
		Username: username,
		Password: password,
	}
}

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
