package images

import (
	"errors"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/regsync"
	"github.com/sirupsen/logrus"
)

func CheckRCTags() error {
	rCImageTagMap := make(map[string][]string, 0)

	// Get required tags for all images
	imageTagMap, err := regsync.GenerateImageTagMap()
	if err != nil {
		return err
	}

	// Grab all images that contain RC tags
	for image := range imageTagMap {
		for _, tag := range imageTagMap[image] {
			if strings.Contains(tag, "-rc") {
				rCImageTagMap[image] = append(rCImageTagMap[image], tag)
			}
		}
	}

	// If there are any images that contains RC tags, log them and return an error
	if len(rCImageTagMap) > 0 {
		logrus.Errorf("found images with RC tags: %v", rCImageTagMap)
		return errors.New("rc tags check has failed")
	}

	return nil
}
