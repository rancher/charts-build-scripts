package images

import (
	"fmt"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/regsync"
)

// CheckImages checks if all container images used in charts belong to the rancher namespace
func CheckImages() error {
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
