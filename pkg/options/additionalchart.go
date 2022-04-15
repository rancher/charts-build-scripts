package options

// AdditionalChartOptions represent the options presented to users to be able to configure the way an additional chart is built using these scripts
type AdditionalChartOptions struct {
	// WorkingDir is the working directory for this chart within packages/<package-name>
	WorkingDir string `yaml:"workingDir"`
	// UpstreamOptions is any options provided on how to get this chart from upstream. It is mutually exclusive with CRDChartOptions
	UpstreamOptions *UpstreamOptions `yaml:"upstreamOptions,omitempty"`
	// CRDChartOptions is any options provided on how to generate a CRD chart. It is mutually exclusive with UpstreamOptions
	CRDChartOptions *CRDChartOptions `yaml:"crdOptions,omitempty"`
	// IgnoreDependencies drops certain dependencies from the list that is parsed from upstream
	IgnoreDependencies []string `yaml:"ignoreDependencies"`
}

// CRDChartOptions represent any options that are configurable for CRD charts
type CRDChartOptions struct {
	// The directory within packages/<package-name>/templates/ that will contain the template for your CRD chart
	TemplateDirectory string `yaml:"templateDirectory"`
	// The directory within your templateDirectory in which CRD files should be placed
	CRDDirectory string `yaml:"crdDirectory" default:"templates"`
	// Whether to add a validation file to your main chart to check that CRDs exist
	AddCRDValidationToMainChart bool `yaml:"addCRDValidationToMainChart"`
	// UseTarArchive indicates whether to bundle and compress CRD files into a tgz file
	UseTarArchive bool `yaml:"useTarArchive"`
}
