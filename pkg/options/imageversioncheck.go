package options

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// ImageVersionCheckOptions holds the list of images to check for version updates.
type ImageVersionCheckOptions struct {
	Images []ImageVersionEntry `yaml:"images"`
}

// ImageVersionEntry describes a single image to validate.
type ImageVersionEntry struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
}

// LoadImageVersionCheck reads and strictly parses the image version check config file.
func LoadImageVersionCheck(path string) (*ImageVersionCheckOptions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image version check config %q: %w", path, err)
	}
	var opts ImageVersionCheckOptions
	if err := yaml.UnmarshalStrict(data, &opts); err != nil {
		return nil, fmt.Errorf("parsing image version check config %q: %w", path, err)
	}
	return &opts, nil
}
