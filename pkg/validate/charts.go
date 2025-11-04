package validate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/auto"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/zip"
	"github.com/urfave/cli"
)

// ChartsRepository TODO
func ChartsRepository(ctx context.Context, c *cli.Context, repoRoot string, rootFs billy.Filesystem, csOptions *options.ChartsScriptOptions, skip, remoteMode, localMode bool, chart string) error {

	if err := isGitClean(ctx, repoRoot, false); err != nil {
		return err
	}

	// Only skip icon validations for forward-ports
	if !skip {
		if err := auto.ValidateIcons(ctx, rootFs); err != nil {
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
	if err := zip.ArchiveCharts(ctx, repoRoot, chart); err != nil {
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
