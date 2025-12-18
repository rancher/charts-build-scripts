package config

// Release and Registries related paths (used by release team)
const (
	// PathCache represents the default place to put a cache on pulled values
	PathCache = ".charts-build-scripts/.cache"
	// PathReleasedAssetsDir is a directory that will be used to store the latest copy of the branch containing your released assets
	PathReleasedAssetsDir = "released-assets"
	// PathIndexYaml is the file on your Staging/Production branch that contains your Helm repository index
	PathIndexYaml = "index.yaml"
	// PathLogosDir is a directory on your Staging/Production branch that contains the files with the logos of each chart
	PathLogosDir = "assets/logos"
	// PathReleaseYaml is the file on your Staging/Production branch that contains the release information
	PathReleaseYaml = "release.yaml"
	// PathDockerSyncYaml file contains docker image/tags that will be synced from Docker
	PathDockerSyncYaml = "config/release/dockerToPrime.yaml"
	// PathStagingSyncYaml file contains docker image/tags that will be synced from Staging registry
	PathStagingSyncYaml = "config/release/stagingToPrime.yaml"
	// PathVersionRulesYaml is the file that contains the version rules for the current branch on charts-build-scripts
	PathVersionRulesYaml = "config/versionRules.yaml"
	// PathStateYaml holds the current status of the released and developed assets versions
	PathStateYaml = "config/state.yaml"
	// PathBumpJSON holds the version that was bumped
	PathBumpJSON = "config/bump_version.json"
	// PathConfigYaml contains the configuration for the charts-build-scripts
	PathConfigYaml = "config/configuration.yaml"
	// PathTrackChartsYaml contains the tracked charts configuration and status
	PathTrackChartsYaml = "config/trackCharts.yaml"
)

// Charts operations related paths (used by chart owners)
const (
	// PathPackageYaml is the name of a file that contains information about how to prepare your package
	PathPackageYaml = "package.yaml"
	// PathDependencyYaml is a file that contains information about how to prepare your dependency (marshalled into a ChartOptions struct)
	PathDependencyYaml = "dependency.yaml"
	// PathPackagesDir is a directory on your Staging branch that contains the files necessary to generate your package
	PathPackagesDir = "packages"
	// PathAssetsDir is a directory on your Staging/Live branch that contains chart archives for each version of your package
	PathAssetsDir = "assets"
	// PathChartsDir is a directory on your Staging/Live branch that contains unarchived charts for each version of your package
	PathChartsDir = "charts"
	// PathTemplatesDir is a directory containing templates used as additional chart options
	PathTemplatesDir = "templates"
	// PathChangesDir is a directory that contains the generated-changes dir to compare chart changes during a version bump
	PathChangesDir = "generated-changes"
	// PathAdditionalDir is a directory that contains the additional-charts dir with changes for dependencies/crds i.e., any kind of additional charts
	PathAdditionalDir = "additional-charts"
	// PathDependenciesDir is a directory that contains dependencies within generated-changes dir
	PathDependenciesDir = "dependencies"
	// PathExcludeDir is a directory that contains excludes within generated-changes dir
	PathExcludeDir = "exclude"
	// PathOverlayDir is a directory that contains overlays within generated-changes dir
	PathOverlayDir = "overlay"
	// PathPatchDir is a directory that contains patches within generated-changes dir
	PathPatchDir = "patch"
	// PathCrdsDir represents the directory that we expect to contain CRDs within the chart
	PathCrdsDir = "crds"
	// PathFilesDir represents the directory that contains extra non-YAML files
	PathFilesDir = "files"
	// PathCrdTgz represents the filename of the crd's tgz file
	PathCrdTgz = "crd-manifest.tgz"
	// PathValidateCrdYaml is the path to the file pushed to upstream that validates the existence of CRDs in the chart
	PathValidateCrdYaml = "templates/validate-install-crd.yaml"
)
