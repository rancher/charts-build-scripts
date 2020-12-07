package config

// HelmRepoConfiguration represents the configuration of the Helm Repository that exposes your charts
type HelmRepoConfiguration struct {
	URL string `yaml:"url"`
}
