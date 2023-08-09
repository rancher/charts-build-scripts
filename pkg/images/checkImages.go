package images

import (
	"fmt"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/regsync"
	"github.com/rancher/charts-build-scripts/pkg/rest"
	"github.com/sirupsen/logrus"
)

// CheckImages checks if all container images used in charts belong to the rancher namespace
func CheckImages() error {
	failedImages := make(map[string][]string, 0)

	// Get required tags for all images
	imageTagMap, err := regsync.GenerateImageTagMap()
	if err != nil {
		return err
	}

	// Check if there's any image outside the rncher namespace
	nonMatchingImages := checkPattern(imageTagMap)
	if len(nonMatchingImages) > 0 {
		return fmt.Errorf("found images outside the rancher namespace: %v", nonMatchingImages)
	}

	// Loop through all images and tags to check if they exist
	for image := range imageTagMap {

		// Split image into namespace and repository
		location := strings.Split(image, "/")
		if len(location) != 2 {
			return fmt.Errorf("failed to generate namespace and repository for image: %s", image)
		}

		// Check if all tags exist
		for _, tag := range imageTagMap[image] {
			err := checkTag(location[0], location[1], tag)
			if err != nil {
				failedImages[image] = append(failedImages[image], tag)
			}
		}
	}

	logrus.Errorf("Images that have failed the check: %v", failedImages)

	return nil
}

// checkPattern checks for pattern "rancher/*" in an array and returns items that do not match.
func checkPattern(imageTagMap map[string][]string) []string {
	nonMatchingImages := make([]string, 0)

	for image := range imageTagMap {
		if !strings.HasPrefix(image, "rancher/") {
			nonMatchingImages = append(nonMatchingImages, image)
		}
	}

	return nonMatchingImages
}

// checkTag checks if a tag exists in a namespace/repository
func checkTag(namespace, repository, tag string) error {
	logrus.Infof("Checking tag %s/%s:%s", namespace, repository, tag)

	url := fmt.Sprintf("https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags/%s", namespace, repository, tag)

	// Sends HEAD request to check if namespace/repository:tag exists
	err := rest.Head(url)
	if err != nil {
		logrus.Errorf("failed to check tag %s/%s:%s", namespace, repository, tag)
		return err
	}

	logrus.Infof("tag %s/%s:%s found", namespace, repository, tag)
	return nil
}
