package validate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/urfave/cli"
)

// ChartsRepository validates the charts repository by running a series of checks
// to ensure charts, assets, index.yaml and release.yaml are in a valid work state
//
// Sequence:
//   - Git working tree must be clean
//   - release.yaml must be valid
//   - icons must be present for particular charts
//   - upstream/remote or local repository comparison
//   - charts/ vs assets/ must match
//   - helm index.yaml regeneration
func ChartsRepository(ctx context.Context, c *cli.Context, repoRoot string, rootFs billy.Filesystem, csOptions *options.ChartsScriptOptions, skip, remoteMode, localMode bool, chart string) error {

	if err := isGitClean(ctx, repoRoot, false); err != nil {
		return err
	}

	if err := validateReleaseYaml(ctx, rootFs); err != nil {
		return err
	}

	if err := validateIndexYaml(ctx, rootFs); err != nil {
		return err
	}

	// Only skip icon validations for forward-ports
	if !skip {
		if err := Icons(ctx, rootFs); err != nil {
			return err
		}
	}

	// Only validate remotely
	if remoteMode {
		logger.Log(ctx, slog.LevelInfo, "remote validation only")
	} else {
		logger.Log(ctx, slog.LevelInfo, "generating charts")
		if err := generateChartsConcurrently(ctx, c, repoRoot, chart, csOptions, rootFs); err != nil {
			return err
		}

		if err := isGitClean(ctx, repoRoot, true); err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelInfo, "successfully validated that current charts and assets are up-to-date")
	}

	if csOptions.ValidateOptions != nil {
		if localMode {
			logger.Log(ctx, slog.LevelInfo, "local validation only")
		} else {
			releaseOptions, err := options.LoadReleaseOptionsFromFile(ctx, rootFs, "release.yaml")
			if err != nil {
				return err
			}
			u := csOptions.ValidateOptions.UpstreamOptions
			branch := csOptions.ValidateOptions.Branch

			logger.Log(ctx, slog.LevelInfo, "upstream validation against repository", slog.String("url", u.URL), slog.String("branch", branch))
			compareGeneratedAssetsResponse, err := CompareGeneratedAssets(ctx, repoRoot, rootFs, u, branch, releaseOptions)
			if err != nil {
				return err
			}
			if !compareGeneratedAssetsResponse.PassedValidation() {
				// Output charts that have been modified
				compareGeneratedAssetsResponse.LogDiscrepancies(ctx)

				logger.Log(ctx, slog.LevelInfo, "dumping release.yaml to track changes that have been introduced")
				if err := compareGeneratedAssetsResponse.DumpReleaseYaml(ctx, rootFs); err != nil {
					logger.Log(ctx, slog.LevelError, "unable to dump newly generated release.yaml", logger.Err(err))
				}

				logger.Log(ctx, slog.LevelInfo, "updating index.yaml")
				if err := helm.CreateOrUpdateHelmIndex(ctx, rootFs); err != nil {
					return err
				}

				return errors.New("validation against upstream repository: " + u.URL + " at branch: " + branch + " failed")
			}
		}
	}

	logger.Log(ctx, slog.LevelInfo, "zipping charts to ensure that contents of assets, charts, and index.yaml are in sync")

	// zipCharts
	if err := helm.ArchiveCharts(ctx, repoRoot, chart); err != nil {
		return err
	}

	// createOrUpdateIndex
	if err := helm.CreateOrUpdateHelmIndex(ctx, rootFs); err != nil {
		return err
	}

	if err := isGitClean(ctx, repoRoot, false); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "make validate success")
	return nil
}

func isGitClean(ctx context.Context, repoRoot string, checkExceptions bool) error {
	logger.Log(ctx, slog.LevelInfo, "checking if Git is clean")
	_, _, status, err := git.GetGitInfo(ctx, repoRoot)
	if err != nil {
		return err
	}
	if !checkExceptions {
		if !status.IsClean() {
			return errors.New("repository must be clean to run validation")
		}

		return nil
	}

	return StatusExceptions(ctx, status)
}

func validateReleaseYaml(ctx context.Context, rootFs billy.Filesystem) error {
	logger.Log(ctx, slog.LevelInfo, "validating release.yaml")

	// validate release.yaml format by loading it with safeDecode without ignoring format
	releaseYaml, err := filesystem.LoadYamlFile[map[string][]string](ctx, filesystem.GetAbsPath(rootFs, path.RepositoryReleaseYaml), false)
	if err != nil {
		return err
	}

	// release.yaml is nil when it is empty
	if releaseYaml == nil {
		return nil
	}

	// validate duplicate versions inside each chart in the release.yaml
	for chart, versions := range *releaseYaml {
		seen := make(map[string]bool, len(versions))
		for _, version := range versions {
			if seen[version] {
				return errors.New("duplicate version in release.yaml; " + chart + ":" + version)
			}
			seen[version] = true
		}
	}

	logger.Log(ctx, slog.LevelInfo, "release.yaml is valid")
	return nil
}

// validateIndexYaml will check if:
//
//   - every genrated assets (.tgz) has its correspondent index.yaml entry with the proper version
//   - every index entry has its correspondent (.tgz) file
func validateIndexYaml(ctx context.Context, rootFs billy.Filesystem) error {
	logger.Log(ctx, slog.LevelInfo, "validating index.yaml matches assets")

	indexFile, err := helm.OpenIndexYaml(ctx, rootFs)
	if err != nil {
		return err
	}

	// Check 1:
	logger.Log(ctx, slog.LevelInfo, "every asset must have an index entry")
	assetsPath := path.RepositoryAssetsDir
	if err := filesystem.WalkDir(ctx, rootFs, assetsPath, func(ctx context.Context, fs billy.Filesystem, filePath string, isDir bool) error {
		if isDir || filepath.Ext(filePath) != ".tgz" {
			return nil
		}

		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, filePath))
		if err != nil {
			return errors.New("validating index.yaml check(1); failed to load chart: " + err.Error())
		}

		if !indexFile.Has(chart.Metadata.Name, chart.Metadata.Version) {
			return errors.New("validating index.yaml check(1); missing entry: " +
				chart.Metadata.Name + "-" + chart.Metadata.Version)
		}

		return nil
	}); err != nil {
		return err
	}
	logger.Log(ctx, slog.LevelInfo, "all assets have index entries")

	// Check 2:
	logger.Log(ctx, slog.LevelInfo, "every index entry must have an asset file")
	for chartName, versions := range indexFile.Entries {
		for _, version := range versions {
			assetTgz := chartName + "-" + version.Version + ".tgz"
			assetPath := path.RepositoryAssetsDir + "/" + chartName + "/" + assetTgz

			exists, err := filesystem.PathExists(ctx, rootFs, assetPath)
			if err != nil {
				return errors.New("validating index.yaml check(2); failed to check path: " + err.Error())
			}
			if !exists {
				return errors.New("validating index.yaml check(2); asset missing for index entry: " + assetTgz)
			}
		}
	}
	logger.Log(ctx, slog.LevelInfo, "all index entries have assets")

	return nil
}

func generateChartsConcurrently(ctx context.Context, c *cli.Context, repoRoot, chart string, csOptions *options.ChartsScriptOptions, rootFs billy.Filesystem) error {
	packages, err := charts.GetPackages(ctx, repoRoot, chart)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		logger.Log(ctx, slog.LevelError, "no packages found")
		return errors.New("should have packages for validation")
	}

	const maxWorkers = 5
	var mu sync.Mutex
	var errs []error

	// Create a buffered channel to limit concurrent workers
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	// Filter packages that need processing
	var packagesToProcess []*charts.Package
	for _, p := range packages {
		if !p.Auto {
			packagesToProcess = append(packagesToProcess, p)
		}
	}

	logger.Log(ctx, slog.LevelInfo, "processing charts with worker pool",
		slog.Int("total_packages", len(packagesToProcess)),
		slog.Int("max_workers", maxWorkers))

	for _, p := range packagesToProcess {
		// Respect context cancellation
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore (blocks if 10 workers are running)

		go func(pkg *charts.Package) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			logger.Log(ctx, slog.LevelInfo, "generating chart", slog.String("package", pkg.Name))
			if err := pkg.GenerateCharts(ctx, csOptions.OmitBuildMetadataOnExport); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("package %s: %w", pkg.Name, err))
				mu.Unlock()
			}
		}(p)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Return all errors
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
