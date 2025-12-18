package validate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

// ChartsRepository TODO
func ChartsRepository(ctx context.Context, skip, remoteMode, localMode bool, chart string) error {
	logger.Log(ctx, slog.LevelInfo, "", slog.Group("inputs",
		"LocalMode", localMode,
		"RemoteMode", remoteMode,
		"Skip", skip,
		"CurrentPackage", chart))

	if localMode && remoteMode {
		return errors.New("cannot specify both local and remote validation")
	}

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	if err := isGitClean(ctx, false); err != nil {
		return err
	}

	// Only skip icon validations for forward-ports
	if !skip {
		if err := ValidateIcons(ctx); err != nil {
			return err
		}
	}

	// Only validate remotely
	if remoteMode {
		logger.Log(ctx, slog.LevelInfo, "remote validation only")
	} else {
		logger.Log(ctx, slog.LevelInfo, "generating charts")
		if err := generateChartsConcurrently(ctx, chart); err != nil {
			return err
		}

		if err := isGitClean(ctx, true); err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelInfo, "successfully validated that current charts and assets are up-to-date")
	}

	if cfg.ChartsScriptOptions.ValidateOptions != nil {
		if localMode {
			logger.Log(ctx, slog.LevelInfo, "local validation only")
		} else {
			releaseOptions, err := options.LoadReleaseOptionsFromFile(ctx, cfg.RootFS, "release.yaml")
			if err != nil {
				return err
			}
			u := cfg.ChartsScriptOptions.ValidateOptions.UpstreamOptions
			branch := cfg.ChartsScriptOptions.ValidateOptions.Branch

			logger.Log(ctx, slog.LevelInfo, "upstream validation against repository", slog.String("url", u.URL), slog.String("branch", branch))
			compareGeneratedAssetsResponse, err := CompareGeneratedAssets(ctx, cfg.Root, cfg.RootFS, u, branch, releaseOptions)
			if err != nil {
				return err
			}
			if !compareGeneratedAssetsResponse.PassedValidation() {
				// Output charts that have been modified
				compareGeneratedAssetsResponse.LogDiscrepancies(ctx)

				logger.Log(ctx, slog.LevelInfo, "dumping release.yaml to track changes that have been introduced")
				if err := compareGeneratedAssetsResponse.DumpReleaseYaml(ctx, cfg.RootFS); err != nil {
					logger.Log(ctx, slog.LevelError, "unable to dump newly generated release.yaml", logger.Err(err))
				}

				logger.Log(ctx, slog.LevelInfo, "updating index.yaml")
				if err := helm.CreateOrUpdateHelmIndex(ctx); err != nil {
					return err
				}

				return errors.New("validation against upstream repository: " + u.URL + " at branch: " + branch + " failed")
			}
		}
	}

	logger.Log(ctx, slog.LevelInfo, "zipping charts to ensure that contents of assets, charts, and index.yaml are in sync")

	// zipCharts
	if err := helm.ArchiveCharts(ctx, chart); err != nil {
		return err
	}

	// createOrUpdateIndex
	if err := helm.CreateOrUpdateHelmIndex(ctx); err != nil {
		return err
	}

	if err := isGitClean(ctx, false); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "make validate success")
	return nil
}

func isGitClean(ctx context.Context, checkExceptions bool) error {
	logger.Log(ctx, slog.LevelInfo, "checking if Git is clean")

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	_, _, status, err := git.GetGitInfo(ctx, cfg.Root)
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

func generateChartsConcurrently(ctx context.Context, chart string) error {
	packages, err := charts.GetPackages(ctx, chart)
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
			if err := pkg.GenerateCharts(ctx); err != nil {
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
