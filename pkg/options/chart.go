package options

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"gopkg.in/yaml.v2"
)

// ChartOptions represent the options presented to users to be able to configure the way a main chart is built using these scripts
type ChartOptions struct {
	// WorkingDir is the working directory for this chart within packages/<package-name>
	WorkingDir string `yaml:"workingDir" default:"charts"`
	// UpstreamOptions is any options provided on how to get this chart from upstream
	UpstreamOptions UpstreamOptions `yaml:",inline"`
}

// UpstreamOptions represents the options presented to users to define where the upstream Helm chart is located
type UpstreamOptions struct {
	// URL represents a source for your upstream (e.g. a Github repository URL or a download link for an archive)
	URL string `yaml:"url,omitempty"`
	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory,omitempty"`
	// Commit represents a specific commit hash to treat as the head, if the URL points to a Github repository
	Commit *string `yaml:"commit,omitempty"`
}

// LoadChartOptionsFromFile unmarshalls the struct found at the file to YAML and reads it into memory
func LoadChartOptionsFromFile(fs billy.Filesystem, path string) (ChartOptions, error) {
	var chartOptions ChartOptions
	exists, err := filesystem.PathExists(fs, path)
	if err != nil {
		return chartOptions, err
	}
	if !exists {
		return chartOptions, fmt.Errorf("Unable to load chart options from file %s since it does not exist", filesystem.GetAbsPath(fs, path))
	}
	chartOptionsBytes, err := ioutil.ReadFile(filesystem.GetAbsPath(fs, path))
	if err != nil {
		return chartOptions, err
	}
	return chartOptions, yaml.Unmarshal(chartOptionsBytes, &chartOptions)
}

// WriteToFile marshals the struct to yaml and writes it into the path specified
func (c ChartOptions) WriteToFile(fs billy.Filesystem, path string) error {
	chartOptionsBytes, err := yaml.Marshal(c)
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
		file, err = fs.OpenFile(path, os.O_RDWR, os.ModePerm)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(chartOptionsBytes)
	return err
}
