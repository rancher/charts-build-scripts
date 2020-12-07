package chart

import (
	"fmt"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/config"
)

// PackageOptions represents the options presented to users to be able to configure the way a chart is built using these scripts
// The YAML that corresponds to these options are stored within packages/<package-name>/package.yaml for each package
type PackageOptions struct {
	UpstreamOptions `yaml:",inline"`
	// PackageVersion represents the current version of the package. It needs to be incremented whenever there are changes
	PackageVersion int `yaml:"packageVersion" default:"0"`
	// CRDChartOptions represents any options that are configurable for CRD charts
	CRDChartOptions CRDChartOptions `yaml:"crdChart"`
}

// UpstreamOptions represents the options presented to users to define where the upstream Helm chart is located
type UpstreamOptions struct {
	// URL represents a source for your upstream (e.g. a Github repository URL or a download link for an archive)
	URL *string `yaml:"url,omitempty"`
	// Subdirectory represents a specific directory within the upstream pointed to by the URL to treat as the root
	Subdirectory *string `yaml:"subdirectory,omitempty"`
	// Commit represents a specific commit hash to treat as the head, if the URL points to a Github repository
	Commit *string `yaml:"commit,omitempty"`
}

// GetUpstream returns the appropriate Upstream given the options provided
func (u UpstreamOptions) GetUpstream() (Upstream, error) {
	if u.URL == nil {
		return nil, fmt.Errorf("URL is not defined")
	}
	if strings.HasPrefix(*u.URL, "packages/") {
		upstream := UpstreamLocal{
			Name: strings.Split(*u.URL, "/")[1],
		}
		return upstream, nil
	}
	if strings.HasSuffix(*u.URL, ".git") {
		rc, err := config.GetRepositoryConfiguration(*u.URL)
		if err != nil {
			return nil, err
		}
		upstream := UpstreamRepository{RepositoryConfiguration: rc}
		if u.Subdirectory != nil {
			upstream.Subdirectory = u.Subdirectory
		}
		if u.Commit != nil {
			upstream.Commit = u.Commit
		}
		return upstream, nil
	}
	if strings.HasSuffix(*u.URL, ".tgz") || strings.Contains(*u.URL, ".tar.gz") {
		upstream := UpstreamChartArchive{
			URL: *u.URL,
		}
		if u.Subdirectory != nil {
			upstream.Subdirectory = u.Subdirectory
		}
		return upstream, nil
	}
	return nil, fmt.Errorf("URL is invalid (must contain .git or .tgz)")
}

// CRDChartOptions represent any options that are configurable for CRD charts
type CRDChartOptions struct {
	// Whether to generate a CRD chart automatically
	Enabled bool `yaml:"enabled"`
}

func (c CRDChartOptions) String() string {
	return fmt.Sprintf("{enabled: %t}", c.Enabled)
}
