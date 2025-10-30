package validate

import (
	"context"
	"errors"
	"log/slog"

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
func ChartsRepository(ctx context.Context, c *cli.Context, repoRoot string, rootFs billy.Filesystem, csOptions *options.ChartsScriptOptions, skip, remoteMode, localMode bool) error {

	if err := checkGitClean(ctx, repoRoot, false); err != nil {
		return err
	}

	// Only skip icon validations for forward-ports
	if !skip {
		if err := auto.ValidateIcons(ctx, rootFs); err != nil {
			return err
		}
	}

	// REMOTE MODE
	if remoteMode {
		logger.Log(ctx, slog.LevelInfo, "remove validation only")
	} else {
		logger.Log(ctx, slog.LevelInfo, "generating charts")
		if err := generateCharts(ctx, c, repoRoot, csOptions); err != nil {
			return err
		}

		if err := checkGitClean(ctx, repoRoot, true); err != nil {
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
	// zipCharts(c)
	// TODO ReImplement CurrentChart
	if err := zip.ArchiveCharts(ctx, repoRoot, ""); err != nil {
		return err
	}
	// createOrUpdateIndex(c)
	if err := helm.CreateOrUpdateHelmIndex(ctx, rootFs); err != nil {
		return err
	}

	if err := checkGitClean(ctx, repoRoot, false); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "make validate success")

	return nil
}

func checkGitClean(ctx context.Context, repoRoot string, checkExceptions bool) error {
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

func getPackages(ctx context.Context, repoRoot string) ([]*charts.Package, error) {
	// TODO ReImplement CurrentPackage
	packages, err := charts.GetPackages(ctx, repoRoot, "")
	if err != nil {
		return nil, err
	}
	return packages, nil
}

func generateCharts(ctx context.Context, c *cli.Context, repoRoot string, csOptions *options.ChartsScriptOptions) error {
	packages, err := getPackages(ctx, repoRoot)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		logger.Log(ctx, slog.LevelInfo, "no packages found")
		return nil
	}

	for _, p := range packages {
		if !p.Auto {
			if err := p.GenerateCharts(ctx, csOptions.OmitBuildMetadataOnExport); err != nil {
				return err
			}
		}
	}

	return nil
}
