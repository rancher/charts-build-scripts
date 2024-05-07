package options

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
	// The directory in which to place your crds withint the chart generated from TemplateDirectory. Mutually exclusive with UseTarArchive
	CRDDirectory string `yaml:"crdDirectory" default:"templates"`
	// UseTarArchive indicates whether to bundle and compress CRD files into a tgz file. Mutually exclusive with CRDDirectory
	UseTarArchive bool `yaml:"useTarArchive"`
	// Whether to add a validation file to your main chart to check that CRDs exist
	AddCRDValidationToMainChart bool `yaml:"addCRDValidationToMainChart"`
}
