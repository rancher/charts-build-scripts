package registries

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

// Config represents the regsync.yaml file
type Config struct {
	Version  int         `yaml:"version"`
	Creds    interface{} `yaml:"creds"`
	Defaults interface{} `yaml:"defaults"`
	Sync     []SyncEntry `yaml:"sync"`
}

// SyncEntry represents a single entry in the regsync.yaml file
type SyncEntry struct {
	Source string `yaml:"source"` // image name
	Target string `yaml:"target"` // If needed
	Type   string `yaml:"type"`   // If needed
	Tags   Tags   `yaml:"tags"`   // existing tags
}

// Tags represents the tags in the regsync.yaml file related to a single entry at the prime registry
type Tags struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"` // If needed
}

// mapAllowTagsFromRegsyncYaml loads the regsync.yaml file and maps the 'source' fields as
// keys and the tags inside the 'allow' field of each repository as a slice.
// These are all docker images across all the values.yaml files in the charts repository,
// that are expected to be already pushed.
func mapAllowTagsFromRegsyncYaml(ctx context.Context) (map[string][]string, error) {
	regsyncYaml, err := filesystem.LoadYamlFile[Config](ctx, path.RegsyncYamlFile, false)
	if err != nil {
		return nil, err
	}

	allowTagMap := make(map[string][]string)
	for _, entry := range regsyncYaml.Sync {
		source := entry.Source
		allow := entry.Tags.Allow
		allowTagMap[source] = allow
	}

	return allowTagMap, nil
}

// createRegSyncConfigFile create the regsync configuration file from the image list map provided.
func createRegSyncConfigFile(imageTagMap map[string][]string) error {
	filename := path.RegsyncYamlFile

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintln(file, `---
version: 1
creds:
- registry: '{{ env "REGISTRY_ENDPOINT" }}'
  user: '{{ env "REGISTRY_USERNAME" }}'
  pass: '{{ env "REGISTRY_PASSWORD" }}'
  reqConcurrent: 10
  reqPerSec: 50
defaults:
  mediaTypes:
  - application/vnd.docker.distribution.manifest.v2+json
  - application/vnd.docker.distribution.manifest.list.v2+json
  - application/vnd.oci.image.manifest.v1+json
  - application/vnd.oci.image.index.v1+json
sync:`)

	// We collect all repos and then sort them so there is consistency
	// in the update of the regsync file always. This has to be done
	// since golang range iterates over a map in a randomised manner.
	repositories := make([]string, 0)
	for repo := range imageTagMap {
		repositories = append(repositories, repo)
	}
	sort.Strings(repositories)

	for _, repo := range repositories {
		if repo == "" {
			continue // skip empty repository
		}
		fmt.Fprintf(file, "%s%s\n", "- source: docker.io/", repo)
		fmt.Fprintf(file, `  target: '{{ env "REGISTRY_ENDPOINT" }}/%s'`, repo)
		fmt.Fprintln(file)
		fmt.Fprintln(file, "  type: repository")
		fmt.Fprintln(file, "  tags:")

		// We collect all tags and then sort them so there is consistency
		// in the update of the regsync file always.
		tags := make([]string, 0)
		tags = append(tags, imageTagMap[repo]...)
		if len(tags) == 0 {
			fmt.Fprintln(file, "    deny:")
			fmt.Fprintln(file, `      - "*"`)
			continue
		}
		sort.Strings(tags)
		fmt.Fprintln(file, "    allow:")
		for _, tag := range tags {
			if tag == "" {
				continue // skip empty tag
			}
			fmt.Fprintf(file, "    - %s\n", tag)
		}
	}

	return nil
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
