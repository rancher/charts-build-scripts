package auto

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/validate"
)

// Asset holds necessary metadata to release a chart version
type Asset struct {
	Tgz     string
	Path    string
	Version string
	Chart   string
}

// Release will release a chart from a dev branch to a release branch
func Release(ctx context.Context, version, chart string) error {
	if chart == "" {
		return errors.New("CHART environment variable must be set to run release cmd")
	}

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	devBranch := cfg.VersionRules.DevPrefix + cfg.VersionRules.BranchVersion

	status, err := validate.LoadStateFile(ctx)
	if err != nil {
		return fmt.Errorf("could not load state; please run lifecycle-status before this command: %w", err)
	}

	asset, err := loadAssetInfo(status, chart, version)
	if err != nil {
		return err
	}

	if err := PullAsset(devBranch, asset.Path, cfg.Repo); err != nil {
		return fmt.Errorf("failed to execute release: %w", err)
	}

	// Unzip assets
	currentAsset := asset.Chart + "/" + asset.Tgz
	if err := helm.DumpAssets(ctx, currentAsset); err != nil {
		return err
	}
	// make index
	if err := helm.CreateOrUpdateHelmIndex(ctx); err != nil {
		return err
	}

	if err := PullIcon(ctx, cfg.RootFS, cfg.Repo, asset.Chart, asset.Version, devBranch); err != nil {
		return err
	}

	if err := UpdateReleaseYaml(ctx, true, asset.Chart, asset.Version, config.PathReleaseYaml); err != nil {
		return err
	}

	return helm.CreateOrUpdateHelmIndex(ctx)
}

// loadAssetInfo will create the Asset struct with access to the necessary data.
func loadAssetInfo(status *validate.Status, chart, version string) (*Asset, error) {
	r := &Asset{
		Version: version,
		Chart:   chart,
	}

	var ok bool
	var assetVersions []string

	assetVersions, ok = status.ToRelease[r.Chart]
	if !ok {
		assetVersions, ok = status.ToForwardPort[r.Chart]
		if !ok {
			return nil, errors.New("no asset version to release for chart:" + r.Chart)
		}
	}

	var assetVersion string
	for _, version := range assetVersions {
		if version == r.Version {
			assetVersion = version
			break
		}
	}
	if assetVersion == "" {
		return nil, errors.New("no asset version to release for chart:" + r.Chart + " version:" + r.Version)
	}

	r.Path, r.Tgz = mountAssetVersionPath(r.Chart, assetVersion)

	// Check again if the asset was already released in the local repository
	if err := checkAssetReleased(r.Path); err != nil {
		return nil, fmt.Errorf("failed to check for chart:%s ; err: %w", r.Chart, err)
	}

	return r, nil
}

// PullAsset will execute the release porting for a chart in the repository
func PullAsset(sourceBranch string, assetPath string, g *git.Git) error {
	if err := g.FetchBranch(sourceBranch); err != nil {
		return err
	}

	if err := g.CheckFileExists(assetPath, sourceBranch); err != nil {
		return fmt.Errorf("asset version not found in dev branch: %w", err)
	}

	if err := g.CheckoutFile(sourceBranch, assetPath); err != nil {
		return err
	}

	return g.ResetHEAD()
}

func checkAssetReleased(chartVersion string) error {
	if _, err := os.Stat(chartVersion); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// mountAssetVersionPath returns the asset path and asset tgz name for a given chart and version.
// example: assets/longhorn/longhorn-100.0.0+up0.0.0.tgz
func mountAssetVersionPath(chart, version string) (string, string) {
	assetTgz := chart + "-" + version + ".tgz"
	assetPath := "assets/" + chart + "/" + assetTgz
	return assetPath, assetTgz
}

func readReleaseYaml(ctx context.Context, path string) (map[string][]string, error) {
	releaseVersions, err := filesystem.LoadYamlFile[map[string][]string](ctx, path, true)
	if err != nil {
		return nil, err
	}

	if releaseVersions == nil {
		return map[string][]string{}, nil
	}

	return *releaseVersions, nil
}

// UpdateReleaseYaml reads and parse the release.yaml file to a map, appends the new version and writes it back to the file.
func UpdateReleaseYaml(ctx context.Context, overwrite bool, chart, version, releaseYamlPath string) error {
	releaseVersions, err := readReleaseYaml(ctx, releaseYamlPath)
	if err != nil {
		return err
	}

	// Overwrite with the target version or append
	if overwrite {
		releaseVersions[chart] = []string{version}
	} else {
		releaseVersions[chart] = append(releaseVersions[chart], version)
	}

	file, err := filesystem.CreateAndOpenYamlFile(ctx, releaseYamlPath, true)
	if err != nil {
		return err
	}

	return filesystem.UpdateYamlFile(file, releaseVersions)
}

// PullIcon will pull the icon from the chart and save it to the local assets/logos directory
func PullIcon(ctx context.Context, rootFs billy.Filesystem, g *git.Git, chart, version, branch string) error {
	logger.Log(ctx, slog.LevelInfo, "starting to pull icon process")

	// Get Chart.yaml path and load it
	chartMetadata, err := helm.LoadChartYaml(rootFs, chart, version)
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelDebug, "checking if chart has downloaded icon")
	iconField := chartMetadata.Icon

	// Check file prefix if it is a URL just skip this process
	if !strings.HasPrefix(iconField, "file://") {
		logger.Log(ctx, slog.LevelInfo, "icon path is not a file:// prefix")
		return nil
	}

	relativeIconPath, _ := strings.CutPrefix(iconField, "file://")

	// Check if icon is already present
	exists, err := filesystem.PathExists(ctx, rootFs, relativeIconPath)
	if err != nil {
		return err
	}

	// Icon is already present, no need to pull it
	if exists {
		logger.Log(ctx, slog.LevelDebug, "icon already exists")
		return nil
	}

	// Check if the icon exists in the dev branch
	if err := g.CheckFileExists(relativeIconPath, branch); err != nil {
		logger.Log(ctx, slog.LevelError, "icon file not found in dev branch but should", slog.String("icon", relativeIconPath), logger.Err(err))
		return errors.New("icon file not found in dev branch but should: " + err.Error())
	}

	// checkout the icon file from the dev branch
	if err := g.CheckoutFile(branch, relativeIconPath); err != nil {
		return err
	}

	// git reset return
	return g.ResetHEAD()
}
