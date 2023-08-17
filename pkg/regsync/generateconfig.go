package regsync

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
)

// chartsToIgnoreTags and systemChartsToIgnoreTags defines the charts and system charts in which a specified
// image tag should be ignored.
var chartsToIgnoreTags = map[string]string{
	"rancher-vsphere-csi": "latest",
	"rancher-vsphere-cpi": "latest",
}

// GenerateConfigFile creates a regsync config file out of the current branch.
func GenerateConfigFile() error {
	imageTagMap := make(map[string][]string)

	err := walkAssetsFolder(imageTagMap)
	if err != nil {
		return err
	}

	// Create the regsync config file
	err = createRegSyncConfigFile(imageTagMap)
	if err != nil {
		return err
	}

	return nil
}

// walkAssetsFolder walks over the assets folder, untars files, stores the values.yaml content
// into a map and then iterates over the map to collect the image repo and tag values
// into another map.
func walkAssetsFolder(imageTagMap map[string][]string) error {
	// Walk through the assets folder of the repo
	filepath.Walk("./assets/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error occurred while walking over the assets directory file %s:%s", path, err)
		}

		// Check if the file name ends with tgz ? since we only care about them
		// to untar them and check for values.yaml files.
		if strings.HasSuffix(info.Name(), ".tgz") {
			valuesYamlMaps, err := decodeValuesFilesInTgz(path)
			if err != nil {
				return fmt.Errorf("error occurred while getting values yaml into map in %s:%s", path, err)
			}

			// There can be multiple values yaml files for single chart. So, making a for loop.
			for _, valuesYaml := range valuesYamlMaps {
				// Collecting all images with the following notation in the values yaml files
				// reposoitory :
				// tag :
				walkMap(valuesYaml, func(inputMap map[interface{}]interface{}) {
					repository, ok := inputMap["repository"].(string)
					if !ok {
						return
					}
					// No string type assertion because some charts have float typed image tags
					tag, ok := inputMap["tag"]
					if !ok {
						return
					}

					// If the chart & tag are in the ignore charttags map, we ignore them
					for ignoreChartName, ignoreTag := range chartsToIgnoreTags {
						// find the chart name using the path variable
						if strings.Contains(path, fmt.Sprintf("/%s/", ignoreChartName)) {
							if tag == ignoreTag {
								return
							}
						}
					}

					// If the tag is already found, we don't append it again
					found := slices.Contains(imageTagMap[repository], fmt.Sprintf("%v", tag))
					if !found {
						imageTagMap[repository] = append(imageTagMap[repository], fmt.Sprintf("%v", tag))
					}
				})
			}
		}
		return nil
	})

	return nil
}

// createRegSyncConfigFile create the regsync configuration file from the image list map provided.
func createRegSyncConfigFile(imageTagMap map[string][]string) error {
	filename := "regsync.yaml"

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
		fmt.Fprintf(file, "%s%s\n", "- source: docker.io/", repo)
		fmt.Fprintf(file, `  target: '{{ env "REGISTRY_ENDPOINT" }}/%s'`, repo)
		fmt.Fprintln(file)
		fmt.Fprintln(file, "  type: repository")
		fmt.Fprintln(file, "  tags:")
		fmt.Fprintln(file, "    allow:")

		// We collect all tags and then sort them so there is consistency
		// in the update of the regsync file always.
		tags := make([]string, 0)
		tags = append(tags, imageTagMap[repo]...)
		sort.Strings(tags)

		for _, tag := range tags {
			fmt.Fprintf(file, "    - %s\n", tag)
		}
	}

	return nil
}

// decodeValueFilesInTgz reads tarball in tgzPath and returns a slice of values corresponding to values.yaml files found inside of it.
func decodeValuesFilesInTgz(tgzPath string) ([]map[interface{}]interface{}, error) {
	tgz, err := os.Open(tgzPath)
	if err != nil {
		return nil, err
	}
	defer tgz.Close()
	gzr, err := gzip.NewReader(tgz)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var valuesSlice []map[interface{}]interface{}
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return valuesSlice, nil
		case err != nil:
			return nil, err
		case header.Typeflag == tar.TypeReg && isValuesFile(header.Name):
			var values map[interface{}]interface{}
			if err := decodeYAMLFile(tr, &values); err != nil {
				return nil, err
			}
			valuesSlice = append(valuesSlice, values)
		default:
			continue
		}
	}
}

// decodeYAMLFile unmarshals the values into the target interface
func decodeYAMLFile(r io.Reader, target interface{}) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

// isValuesFile checks if the current file is helm values.yaml or not
func isValuesFile(path string) bool {
	basename := filepath.Base(path)
	return basename == "values.yaml" || basename == "values.yml"
}

// walkMap walks inputMap and calls the callback function on all map type nodes including the root node.
func walkMap(inputMap interface{}, callback func(map[interface{}]interface{})) {
	switch data := inputMap.(type) {
	case map[interface{}]interface{}:
		callback(data)
		for _, value := range data {
			walkMap(value, callback)
		}
	case []interface{}:
		for _, elem := range data {
			walkMap(elem, callback)
		}
	}
}
