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
	// Auto represent the trigger for auto chart bumps
	Auto bool `yaml:"auto,omitempty"`
}

// ChartOptions represent the options presented to users to be able to configure the way a main chart is built using these scripts
type ChartOptions struct {
	// WorkingDir is the working directory for this chart within packages/<package-name>
	WorkingDir string `yaml:"workingDir" default:"charts"`
	// UpstreamOptions is any options provided on how to get this chart from upstream
	UpstreamOptions UpstreamOptions `yaml:",inline"`
	// IgnoreDependencies drops certain dependencies from the list that is parsed from upstream
	IgnoreDependencies []string `yaml:"ignoreDependencies"`
	// ReplacePaths marks paths as those that should be replaced instead of patches. Consequently, these paths will exist in both generated-changes/excludes and generated-changes/overlay
	ReplacePaths []string `yaml:"replacePaths"`
}

// UpstreamOptions represents the options presented to users to define where the upstream Helm chart is located
type UpstreamOptions struct {
	// URL represents a source for your upstream (e.g. a Github repository URL or a download link for an archive)
	URL string `yaml:"url,omitempty"`
	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory,omitempty"`
	// Commit represents a specific commit hash to treat as the head, if the URL points to a Github repository
	Commit *string `yaml:"commit,omitempty"`
	// ChartRepoBranch represents a specific branch to pull from a upstream remote repository
	ChartRepoBranch *string `yaml:"chartRepoBranch,omitempty"`
}

// AdditionalChartOptions represent the options presented to users to be able to configure the way an additional chart is built using these scripts
type AdditionalChartOptions struct {
	// WorkingDir is the working directory for this chart within packages/<package-name>
	WorkingDir string `yaml:"workingDir"`
	// UpstreamOptions is any options provided on how to get this chart from upstream.
	UpstreamOptions *UpstreamOptions `yaml:"upstreamOptions,omitempty"`
	// CRDChartOptions is any options provided on how to generate a CRD chart.
	CRDChartOptions *CRDChartOptions `yaml:"crdOptions,omitempty"`
	// IgnoreDependencies drops certain dependencies from the list that is parsed from upstream
	IgnoreDependencies []string `yaml:"ignoreDependencies"`
	// ReplacePaths marks paths as those that should be replaced instead of patches. Consequently, these paths will exist in both generated-changes/excludes and generated-changes/overlay
	ReplacePaths []string `yaml:"replacePaths"`
}

// CRDChartOptions represent any options that are configurable for CRD charts
type CRDChartOptions struct {
	// The directory within packages/<package-name>/templates/ that will contain the template for your CRD chart
	TemplateDirectory string `yaml:"templateDirectory"`
	// The directory in which to place your crds within the chart generated from TemplateDirectory. Mutually exclusive with UseTarArchive
	CRDDirectory string `yaml:"crdDirectory" default:"templates"`
	// UseTarArchive indicates whether to bundle and compress CRD files into a tgz file. Mutually exclusive with CRDDirectory
	UseTarArchive bool `yaml:"useTarArchive"`
	// Whether to add a validation file to your main chart to check that CRDs exist
	AddCRDValidationToMainChart bool `yaml:"addCRDValidationToMainChart"`
}

// ChartsScriptOptions represents the options provided to the charts scripts for this branch
type ChartsScriptOptions struct {
	// ValidateOptions represent any options that are configurable when validating a chart
	ValidateOptions *ValidateOptions `yaml:"validate"`
	// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
	HelmRepoConfiguration `yaml:"helmRepo"`
	// Template can be 'staging' or 'live'
	Template string `yaml:"template"`
	// OmitBuildMetadataOnExport instructs the scripts to not add in a +up build metadata flag for forked charts
	// If false, any forked chart whose version differs from the original source version will have the version VERSION+upORIGINAL_VERSION
	OmitBuildMetadataOnExport bool `yaml:"omitBuildMetadataOnExport"`
}

// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
type HelmRepoConfiguration struct {
	CNAME string `yaml:"cname"`
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

// LoadChartOptionsFromFile unmarshalls the struct found at the file to YAML and reads it into memory
func LoadChartOptionsFromFile(fs billy.Filesystem, path string) (ChartOptions, error) {
	var chartOptions ChartOptions
	exists, err := filesystem.PathExists(fs, path)
	if err != nil {
		return chartOptions, err
	}
	if !exists {
		return chartOptions, fmt.Errorf("unable to load chart options from file %s since it does not exist", filesystem.GetAbsPath(fs, path))
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
		file, err = fs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(chartOptionsBytes)
	return err
}
