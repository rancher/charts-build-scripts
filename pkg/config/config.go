// Package config provides centralized configuration loading and management
// for the charts-build-scripts tooling.
//
// Design Philosophy:
// This package follows a "load once, use everywhere" pattern. The Config struct
// is initialized once at the start of any command and provides access to all
// necessary configuration data throughout the application lifecycle.
//
// Trade-offs:
// - Some commands may load more data than strictly needed
// - Performance impact is negligible (YAML parsing + simple calculations)
// - Benefits: Simplified code, single source of truth, easier testing
//
// Usage:
//
//	cfg, err := config.Init(ctx, repoRoot, rootFS, branchVersion, chartName)
//	if err != nil {
//	    return err
//	}
package config

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"gopkg.in/yaml.v2"

	helmRepo "helm.sh/helm/v3/pkg/repo"
)

var (
	errGitNotClean               = errors.New("local git repository should be clean")
	errorBranchVersionNotInRules = errors.New("the given branch version is not defined in the rules")
)

// Config is the central configuration struct that holds all necessary data
// for executing chart build operations. It is initialized once and passed
// throughout the application.
type Config struct {
	Root                string                       // Absolute path to the repository root
	RootFS              billy.Filesystem             // Filesystem abstraction for the repository
	Repo                *git.Git                     // Git repository interface
	VersionRules        *VersionRules                // Version rules loaded from config/versionRules.yaml
	TrackedCharts       *TrackedCharts               // Active, legacy and deprecated charts list
	AssetsVersionsMap   map[string][]string          // index.yaml file entry/versions
	ChartsScriptOptions *options.ChartsScriptOptions // TODO Docs this
	SoftErrorMode       bool                         // allows for skipping certain non-fatal errors from breaking execution
}

// Init initializes and returns a fully-loaded Config struct.
// This is the main entry point for configuration loading.
//
// Parameters:
//   - ctx: Context for logging and cancellation
//   - root: Absolute path to the repository root directory
//   - rootFS: Billy filesystem abstraction for the repository
//   - branchVersion: Current Rancher version (e.g., "2.13")
//   - currentChart: Specific chart name (may be empty for global operations)
//
// Returns:
//   - *Config: Fully initialized configuration with all data loaded
//   - error: Any error encountered during initialization
//
// Validation performed:
//   - Git repository must be clean (no uncommitted changes)
//   - Required directories must exist (assets/, index.yaml)
//   - Version rules must be loaded and validated
func Init(ctx context.Context, root string, rootFS billy.Filesystem) (*Config, error) {
	c := &Config{
		Root:          root,
		RootFS:        rootFS,
		SoftErrorMode: false,
	}

	// Load and validate Git repository
	g, err := git.OpenGitRepo(ctx, root)
	if err != nil {
		return nil, err
	}
	c.Repo = g

	// Ensure the working tree is clean before proceeding
	// This prevents data loss and ensures a consistent state
	clean, err := c.Repo.StatusProcelain(ctx)
	if err != nil {
		return nil, err
	}
	if !clean {
		return nil, errGitNotClean
	}

	// Validate that required repository structure exists
	if err := c.checkFilePaths(ctx); err != nil {
		return nil, err
	}

	// Load version rules and calculate retention boundaries
	vr, err := versionRules(ctx, c.RootFS)
	if err != nil {
		return nil, err
	}
	c.VersionRules = vr

	trackedCharts, err := loadTrackedCharts(ctx, c.RootFS)
	if err != nil {
		return nil, err
	}
	c.TrackedCharts = trackedCharts

	assetsVersionsMap, err := loadAssetsVersionsMap(c.RootFS)
	if err != nil {
		return nil, err
	}
	c.AssetsVersionsMap = assetsVersionsMap

	scriptOptions, err := loadScriptOptions(ctx)
	if err != nil {
		return nil, err
	}
	c.ChartsScriptOptions = scriptOptions

	return c, nil
}

// checkFilePaths validates that the required repository structure exists.
// This ensures the repository is in a valid state before operations begin.
func (c *Config) checkFilePaths(ctx context.Context) error {
	// Verify assets directory exists
	exists, err := filesystem.PathExists(ctx, c.RootFS, PathAssetsDir)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(PathAssetsDir + ":directory should exist")
	}

	// Verify Helm index file exists
	exists, err = filesystem.PathExists(ctx, c.RootFS, PathIndexYaml)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(PathIndexYaml + ":file should exist")
	}

	exists, err = filesystem.PathExists(ctx, c.RootFS, PathTrackChartsYaml)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(PathTrackChartsYaml + ":file should exist")
	}

	exists, err = filesystem.PathExists(ctx, c.RootFS, PathVersionRulesYaml)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(PathVersionRulesYaml + ":file should exist")
	}

	exists, err = filesystem.PathExists(ctx, c.RootFS, PathReleaseYaml)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(PathReleaseYaml + ":file should exist")
	}

	exists, err = filesystem.PathExists(ctx, c.RootFS, PathConfigYaml)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(PathConfigYaml + ":file should exist")
	}

	return nil
}

// loadAssetsVersionsMap will map the current state of the index.yaml assets and versions
func loadAssetsVersionsMap(RootFS billy.Filesystem) (map[string][]string, error) {
	assetsMap := make(map[string][]string)

	indexFile, err := helmRepo.LoadIndexFile(filesystem.GetAbsPath(RootFS, PathIndexYaml))
	if err != nil {
		return nil, err
	}

	for chart, entryVersions := range indexFile.Entries {
		versions := make([]string, 0, len(entryVersions))
		for _, version := range entryVersions {
			versions = append(versions, version.Version)
		}
		assetsMap[chart] = versions
	}

	return assetsMap, nil
}

// loadScriptOptions will load provided options at configuration.yaml
func loadScriptOptions(ctx context.Context) (*options.ChartsScriptOptions, error) {
	configYaml, err := os.ReadFile(PathConfigYaml)
	if err != nil {
		return nil, errors.New("unable to find configuration file: " + err.Error())
	}

	chartsScriptOptions := options.ChartsScriptOptions{}
	if err := yaml.UnmarshalStrict(configYaml, &chartsScriptOptions); err != nil {
		return nil, errors.New("unable to unmarshall configuration file: " + err.Error())
	}

	if chartsScriptOptions.ValidateOptions != nil {
		logger.Log(ctx, slog.LevelInfo, "chart script options", slog.Group("opts",
			slog.Group("validate",
				slog.String("branch", chartsScriptOptions.ValidateOptions.Branch),
				slog.Group("upstream",
					slog.String("url", chartsScriptOptions.ValidateOptions.UpstreamOptions.URL),
					slog.Any("commit", chartsScriptOptions.ValidateOptions.UpstreamOptions.Commit),
					slog.Any("subdirectory", chartsScriptOptions.ValidateOptions.UpstreamOptions.Subdirectory),
				),
			),
			slog.Group("helmRepo",
				slog.String("CNAME", chartsScriptOptions.HelmRepoConfiguration.CNAME),
			),
			slog.String("template", chartsScriptOptions.Template),
			slog.Bool("omitBuildMetadata", chartsScriptOptions.OmitBuildMetadataOnExport),
		))
	}

	return &chartsScriptOptions, nil
}
