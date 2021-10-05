package options

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"gopkg.in/yaml.v2"
)

// PackageOptions represent the options presented to users to be able to configure the way a package is built using these scripts
// The YAML that corresponds to these options are stored within packages/<package-name>/package.yaml for each package
type PackageOptions struct {
	// Version represents the version of the package. It will override other values if it exists
	Version *string `yaml:"version,omitempty"`
	// PackageVersion represents the current version of the package. It needs to be incremented whenever there are changes
	PackageVersion *int `yaml:"packageVersion" default:"0"`
	// MainChartOptions represent options presented to the user to configure the main chart
	MainChartOptions ChartOptions `yaml:",inline"`
	// AdditionalChartOptions represent options presented to the user to configure any additional charts
	AdditionalChartOptions []AdditionalChartOptions `yaml:"additionalCharts,omitempty"`
	// DoNotRelease represents a boolean flag that indicates a package should not be tracked in make charts
	DoNotRelease bool `yaml:"doNotRelease,omitempty"`
}

// LoadPackageOptionsFromFile unmarshalls the struct found at the file to YAML and reads it into memory
func LoadPackageOptionsFromFile(fs billy.Filesystem, path string) (PackageOptions, error) {
	var packageOptions PackageOptions
	exists, err := filesystem.PathExists(fs, path)
	if err != nil {
		return packageOptions, err
	}
	if !exists {
		return packageOptions, fmt.Errorf("unable to load package options from file %s since it does not exist", filesystem.GetAbsPath(fs, path))
	}
	chartOptionsBytes, err := ioutil.ReadFile(filesystem.GetAbsPath(fs, path))
	if err != nil {
		return packageOptions, err
	}
	return packageOptions, yaml.Unmarshal(chartOptionsBytes, &packageOptions)
}

// WriteToFile marshals the struct to yaml and writes it into the path specified
func (p PackageOptions) WriteToFile(fs billy.Filesystem, path string) error {
	chartOptionsBytes, err := yaml.Marshal(p)
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
	_, err = file.Write(chartOptionsBytes)
	return err
}
