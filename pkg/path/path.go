package path

const (
	// ChartsRepositoryUpstreamBranchDir is a directory that will be used to store the latest copy of the branch containing your released assets
	ChartsRepositoryUpstreamBranchDir = "released-assets"

	// RepositoryHelmIndexFile is the file on your Staging/Live branch that contains your Helm repository index
	RepositoryHelmIndexFile = "index.yaml"

	// RepositoryPackagesDir is a directory on your Staging branch that contains the files necessary to generate your package
	RepositoryPackagesDir = "packages"

	// RepositoryAssetsDir is a directory on your Staging/Live branch that contains chart archives for each version of your package
	RepositoryAssetsDir = "assets"

	// RepositoryChartsDir is a directory on your Staging/Live branch that contains unarchived charts for each version of your package
	RepositoryChartsDir = "charts"

	// PackageOptionsFile is the name of a file that contains information about how to prepare your package
	// The expected structure of this file is one that can be marshalled into a PackageOptions struct
	PackageOptionsFile = "package.yaml"

	// PackageTemplatesDir is a directory containing templates used as additional chart options
	PackageTemplatesDir = "templates"

	// GeneratedChangesDir is a directory that contains GeneratedChanges
	GeneratedChangesDir = "generated-changes"

	// GeneratedChangesAdditionalChartDir is a directory that contains additionalCharts
	GeneratedChangesAdditionalChartDir = "additional-charts"

	// GeneratedChangesDependenciesDir is a directory that contains dependencies within GeneratedChangesDir
	GeneratedChangesDependenciesDir = "dependencies"

	// GeneratedChangesExcludeDir is a directory that contains excludes within GeneratedChangesDir
	GeneratedChangesExcludeDir = "exclude"

	// GeneratedChangesOverlayDir is a directory that contains overlays within GeneratedChangesDir
	GeneratedChangesOverlayDir = "overlay"

	// GeneratedChangesPatchDir is a directory that contains patches within GeneratedChangesDir
	GeneratedChangesPatchDir = "patch"

	// DependencyOptionsFile is a file that contains information about how to prepare your dependency
	// The expected structure of this file is one that can be marshalled into a ChartOptions struct
	DependencyOptionsFile = "dependency.yaml"

	// ChartCRDDir represents the directory that we expect to contain CRDs within the chart
	ChartCRDDir = "crds"

	// ChartExtraFileDir represents the directory that contains non-YAML files
	ChartExtraFileDir = "files"

	// ChartCRDTgzFilename represents the filename of the crd's tgz file
	ChartCRDTgzFilename = "crd-manifest.tgz"

	// ChartValidateInstallCRDFile is the path to the file pushed to upstream that validates the existence of CRDs in the chart
	ChartValidateInstallCRDFile = "templates/validate-install-crd.yaml"

	// DefaultCachePath represents the default place to put a cache on pulled values
	DefaultCachePath = ".charts-build-scripts/.cache"

	// RepositoryLogosDir is a directory on your Staging/Live branch that contains the files with the logos of each chart
	RepositoryLogosDir = "assets/logos"

	// RepositoryReleaseYaml is the file on your Staging/Live branch that contains the release information
	RepositoryReleaseYaml = "release.yaml"

	// SignedImagesFile is the file that contains the signed images that were bypassed in the last release
	SignedImagesFile = "signed-images.txt"

	// VersionRulesFile is the file that contains the version rules for the current branch on charts-build-scripts
	VersionRulesFile = "config/version_rules.json"

	// RepositoryStateFile is a file to hold the current status of the released and developed assets versions
	RepositoryStateFile = "config/state.json"

	// BumpVersionFile is a file to hold the version that was bumped
	BumpVersionFile = "config/bump_version.json"

	// ConfigurationYamlFile is the file that contains the configuration for the charts-build-scripts
	ConfigurationYamlFile = "config/configuration.yaml"

	// RegsyncYamlFile file is the file that contains the regsync configuration
	RegsyncYamlFile = "config/regsync.yaml"

	// DockerToPrimeSync file contains docker image/tags that will be synced from Docker
	DockerToPrimeSync = "config/dockerToPrime.yaml"
	// StagingToPrimeSync file contains docker image/tags that will be synced from Staging registry
	StagingToPrimeSync = "config/stagingToPrime.yaml"
)
