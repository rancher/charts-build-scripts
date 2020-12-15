package options

// ChartsScriptOptions represents the options provided to the charts scripts for this branch
type ChartsScriptOptions struct {
	// SyncOptions represents any options that are configurable when syncing with another branch
	SyncOptions SyncOptions `yaml:"sync"`
	// ValidateOptions represent any options that are configurable when validating a chart
	ValidateOptions ValidateOptions `yaml:"validate"`
	// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
	HelmRepoConfiguration `yaml:"helmRepo"`
	// Template can be 'source', 'staging', or 'live'
	Template string `yaml:"template"`
}

// SyncOptions represent any options that are configurable when exporting a chart
type SyncOptions []CompareGeneratedAssetsOptions

// ValidateOptions represent any options that are configurable when validating a chart
type ValidateOptions []CompareGeneratedAssetsOptions

// CompareGeneratedAssetsOptions represent any options that are configurable when comparing the generated assets of the current branch against another branch
type CompareGeneratedAssetsOptions struct {
	// UpstreamOptions points to the configuration that contains the branch you want to compare against
	UpstreamOptions UpstreamOptions `yaml:",inline"`
	// Branch represents the branch of the GithubConfiguration that you want to compare against
	Branch string `yaml:"branch"`
	// DropReleaseCandidates indicates that we should drop the release candidate versions from the current branch when comparing with
	DropReleaseCandidates bool `yaml:"dropReleaseCandidates"`
}

// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
type HelmRepoConfiguration struct {
	CNAME string `yaml:"cname"`
}
