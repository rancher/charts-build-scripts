package images

import (
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/regsync"
	"github.com/sirupsen/logrus"
)

// CheckRCTags checks for any images that have RC tags
func CheckRCTags() map[string][]string {
	rcImageTagMap := make(map[string][]string, 0)

	// Get required tags for all images
	imageTagMap, err := regsync.GenerateImageTagMap()
	if err != nil {
		logrus.Fatal("failed to generate image tag map: ", err)
	}

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
