package options

import (
	"io/ioutil"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"gopkg.in/yaml.v2"
)

// ValidateOptions specify an upstream GitHub repository you would like to validate against
type ValidateOptions struct {
	// UpstreamOptions points to the configuration that contains the branch you want to compare against
	UpstreamOptions UpstreamOptions `yaml:",inline"`
	// Branch represents the branch of the GithubConfiguration that you want to compare against
	Branch string `yaml:"branch"`
}

// ReleaseOptions represent the values provided in the release.yaml to avoid validation failing on seeing a to-be-released chart.
// This is only used if ValidateOptions are provided in the configuration.yaml
type ReleaseOptions map[string][]string

// Contains checks if a chartName and chartVersion is tracked by the ReleaseOptions
func (r ReleaseOptions) Contains(chartName string, chartVersion string) bool {
	versions, ok := r[chartName]
	if !ok {
		return false
	}
	for _, v := range versions {
		if v == chartVersion {
			return true
		}
	}
	return false
}

// Append adds a chartName and chartVersion to the ReleaseOptions and returns it
func (r ReleaseOptions) Append(chartName string, chartVersion string) ReleaseOptions {
	versions, ok := r[chartName]
	if !ok {
		versions = []string{}
	}
	versions = append(versions, chartVersion)
	r[chartName] = versions
	return r
}

// Merge merges two ReleaseOptions and returns the merged copy
func (r ReleaseOptions) Merge(new ReleaseOptions) ReleaseOptions {
	for chartName, versions := range new {
		for _, version := range versions {
			r = r.Append(chartName, version)
		}
	}
	return r
}

// LoadReleaseOptionsFromFile unmarshalls the struct found at the file to YAML and reads it into memory
func LoadReleaseOptionsFromFile(fs billy.Filesystem, path string) (ReleaseOptions, error) {
	var releaseOptions ReleaseOptions
	exists, err := filesystem.PathExists(fs, path)
	if err != nil {
		return releaseOptions, err
	}
	if !exists {
		// If release.yaml does not exist, return an empty ReleaseOptions
		return releaseOptions, nil
	}
	releaseOptionsBytes, err := ioutil.ReadFile(filesystem.GetAbsPath(fs, path))
	if err != nil {
		return releaseOptions, err
	}
	return releaseOptions, yaml.Unmarshal(releaseOptionsBytes, &releaseOptions)
}

// WriteToFile marshals the struct to yaml and writes it into the path specified
func (r ReleaseOptions) WriteToFile(fs billy.Filesystem, path string) error {
	releaseOptionsBytes, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	exists, err := filesystem.PathExists(fs, path)
	if err != nil {
		return err
	}
	var file billy.File
	if !exists {
		file, err = filesystem.CreateFileAndDirs(fs, path)
	} else {
		file, err = fs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(releaseOptionsBytes)
	return err
}
