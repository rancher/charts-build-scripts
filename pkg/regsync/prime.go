package regsync

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type skopeo struct {
	Repository string   `json:"Repository"`
	Tags       []string `json:"Tags"`
}

// checkPrimeImageTags checks the prime image tags on the registry.
func checkPrimeImageTags(imageTags map[string][]string) (map[string][]string, error) {
	var primeImageTags = make(map[string][]string)

	fmt.Println("checking prime image tags")
	for image := range imageTags {
		if image == "" {
			continue
		}
		fmt.Printf("image: %s\n", image)
		tags, err := skopeoListTags(image)
		if err != nil {
			return nil, err
		}
		fmt.Printf("Tags: %s\n", strings.Join(tags, ", "))
		primeImageTags[image] = tags
	}

	return primeImageTags, nil
}

// skopeoListTags uses skopeo package to list tags of a given image on a specific registry.
// at the time this was implemented there was no go package to list tags of an image on a registry.
// skopeo is written in go, for speed we are not rewriting skopeo fucntionalities in go.
func skopeoListTags(image string) ([]string, error) {
	suseRegistry := "docker://registry.suse.com/"

	cmd := exec.Command("skopeo", "list-tags", suseRegistry+image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error executing command: %v", err)
	}

	var tags skopeo
	if err := json.Unmarshal(output, &tags); err != nil {
		return nil, fmt.Errorf("error parsing JSON output: %v", err)
	}

	return tags.Tags, nil
}

// removePrimeImageTags will only allow the tags that are not present in the prime image tags.
func removePrimeImageTags(imageTagMap, newPrimeImgTags map[string][]string) map[string][]string {
	var syncImgTags = make(map[string][]string)

	for image, tags := range imageTagMap {
		if image == "" {
			continue
		}
		syncImgTags[image] = []string{}
		for _, tag := range tags {
			if tag == "" {
				continue
			}
			primeTags := newPrimeImgTags[image]
			if exist := primeTagFinder(tag, primeTags); !exist {
				syncImgTags[image] = append(syncImgTags[image], tag)
				fmt.Println(syncImgTags)
			}
		}
	}

	return syncImgTags
}

// primeTagFinder checks if the tag is present in the prime tags.
func primeTagFinder(tag string, primeTags []string) bool {
	for _, primeTag := range primeTags {
		if tag == primeTag {
			return true
		}
	}
	return false
}
