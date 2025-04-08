package images

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/regsync"
	"github.com/rancher/charts-build-scripts/pkg/rest"
)

const (
	loginURL = "https://hub.docker.com/v2/users/login/"
)

// TokenRequest is the request body for the Docker Hub API Login endpoint
type TokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// TokenReponse is the response body for the Docker Hub API Login endpoint
type TokenReponse struct {
	Token string `json:"token"`
}

// CheckImages checks if all container images used in charts belong to the rancher namespace
func CheckImages() error {
	failedImages := make(map[string][]string, 0)

	// Get required tags for all images
	imageTagMap, err := regsync.GenerateImageTagMap()
	if err != nil {
		return err
	}

	// Check if there's any image outside the rancher namespace
	imagesOutsideNamespace := checkPattern(imageTagMap)

	// Get a token to access the Docker Hub API
	token, err := retrieveToken()
	if err != nil {
		logger.Log(slog.LevelWarn, "failed to retrieve token, requests will be unauthenticated", logger.Err(err))
	}

	// Loop through all images and tags to check if they exist
	for image := range imageTagMap {
		if len(image) == 0 {
			logger.Log(slog.LevelWarn, "found blank image, skipping tag check")
			continue
		}

		// Split image into namespace and repository
		location := strings.Split(image, "/")
		if len(location) != 2 {
			logger.Log(slog.LevelError, "failed to split image into namespace and repository", slog.String("image", image))
			return fmt.Errorf("failed to generate namespace and repository for image: %s", image)
		}

		// Check if all tags exist
		for _, tag := range imageTagMap[image] {
			err := checkTag(location[0], location[1], tag, token)
			if err != nil {
				failedImages[image] = append(failedImages[image], tag)
			}
		}
	}

	// If there are any images that have failed the check, log them and return an error
	if len(failedImages) > 0 || len(imagesOutsideNamespace) > 0 {
		logger.Log(slog.LevelError, "found images outside the rancher namespace", slog.Any("imagesOutsideNamespace", imagesOutsideNamespace))
		logger.Log(slog.LevelError, "images that are not on Docker Hub", slog.Any("failedImages", failedImages))
		return errors.New("image check has failed")
	}

	return nil
}

// checkPattern checks for pattern "rancher/*" in an array and returns items that do not match.
func checkPattern(imageTagMap map[string][]string) []string {
	nonMatchingImages := make([]string, 0)

	for image := range imageTagMap {
		if len(image) == 0 {
			logger.Log(slog.LevelWarn, "found blank image, skipping image namespace check")
			continue
		}

		if !strings.HasPrefix(image, "rancher/") {
			nonMatchingImages = append(nonMatchingImages, image)
		}
	}

	return nonMatchingImages
}

// checkTag checks if a tag exists in a namespace/repository
func checkTag(namespace, repository, tag, token string) error {
	logger.Log(slog.LevelDebug, "checking tag", slog.String("namespace", namespace), slog.String("repository", repository), slog.String("tag", tag))

	url := fmt.Sprintf("https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags/%s", namespace, repository, tag)

	// Sends HEAD request to check if namespace/repository:tag exists
	err := rest.Head(url, token)
	if err != nil {
		logger.Log(slog.LevelError, "failed to check tag", logger.Err(err))
		return err
	}

	logger.Log(slog.LevelInfo, "tag found", slog.String("repository", repository), slog.String("tag", tag))
	return nil
}

// retrieveToken retrieves a token to access the Docker Hub API
func retrieveToken() (string, error) {

	// Retrieve credentials from environment variables
	credentials := retrieveCredentials()
	if credentials == nil {
		logger.Log(slog.LevelWarn, "no credentials found, requests will be unauthenticated")
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
func retrieveCredentials() *TokenRequest {

	username := os.Getenv("DOCKER_USERNAME")
	password := os.Getenv("DOCKER_PASSWORD")

	if strings.Compare(username, "") == 0 {
		logger.Log(slog.LevelError, "DOCKER_USERNAME not set", slog.String("username", username))
		return nil
	}

	if strings.Compare(password, "") == 0 {
		logger.Log(slog.LevelError, "DOCKER_PASSWORD not set", slog.String("password", password))
		return nil
	}

	return &TokenRequest{
		Username: username,
		Password: password,
	}
}
