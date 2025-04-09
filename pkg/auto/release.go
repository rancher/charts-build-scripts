package auto

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"gopkg.in/yaml.v3"
	helmChartutil "helm.sh/helm/v3/pkg/chartutil"
)

// Release holds necessary metadata to release a chart version
type Release struct {
	git             *git.Git
	VR              *lifecycle.VersionRules
	AssetTgz        string
	AssetPath       string
	ChartVersion    string
	Chart           string
	ReleaseYamlPath string
	ForkRemoteURL   string
}

// InitRelease will create the Release struct with access to the necessary dependencies.
func InitRelease(ctx context.Context, d *lifecycle.Dependencies, s *lifecycle.Status, v, c, f string) (*Release, error) {
	r := &Release{
		git:           d.Git,
		VR:            d.VR,
		ChartVersion:  v,
		Chart:         c,
		ForkRemoteURL: f,
	}

	var ok bool
	var assetVersions []lifecycle.Asset

	assetVersions, ok = s.AssetsToBeReleased[r.Chart]
	if !ok {
		assetVersions, ok = s.AssetsToBeForwardPorted[r.Chart]
		if !ok {
			return nil, errors.New("no asset version to release for chart:" + r.Chart)
		}
	}

	var assetVersion string
	for _, version := range assetVersions {
		if version.Version == r.ChartVersion {
			assetVersion = version.Version
			break
		}
	}
	if assetVersion == "" {
		return nil, errors.New("no asset version to release for chart:" + r.Chart + " version:" + r.ChartVersion)
	}

	r.AssetPath, r.AssetTgz = mountAssetVersionPath(r.Chart, assetVersion)

	// Check again if the asset was already released in the local repository
	if err := checkAssetReleased(r.AssetPath); err != nil {
		return nil, fmt.Errorf("failed to check for chart:%s ; err: %w", r.Chart, err)
	}

	// Check if we have a release.yaml file in the expected path
	if exist, err := filesystem.PathExists(ctx, d.RootFs, path.RepositoryReleaseYaml); err != nil || !exist {
		return nil, errors.New("release.yaml not found")
	}

	r.ReleaseYamlPath = filesystem.GetAbsPath(d.RootFs, path.RepositoryReleaseYaml)

	return r, nil
}

// PullAsset will execute the release porting for a chart in the repository
func (r *Release) PullAsset() error {
	if err := r.git.FetchBranch(r.VR.DevBranch); err != nil {
		return err
	}

	if err := r.git.CheckFileExists(r.AssetPath, r.VR.DevBranch); err != nil {
		return fmt.Errorf("asset version not found in dev branch: %w", err)
	}

	if err := r.git.CheckoutFile(r.VR.DevBranch, r.AssetPath); err != nil {
		return err
	}

	return r.git.ResetHEAD()
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

func (r *Release) readReleaseYaml() (map[string][]string, error) {
	var releaseVersions = make(map[string][]string, 0)

	file, err := os.Open(r.ReleaseYamlPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&releaseVersions); err != nil {
		if err == io.EOF {
			// Handle EOF error gracefully
			return releaseVersions, nil
		}
		return nil, err
	}

	return releaseVersions, nil
}

// UpdateReleaseYaml reads and parse the release.yaml file to a struct, appends the new version and writes it back to the file.
func (r *Release) UpdateReleaseYaml() error {
	releaseVersions, err := r.readReleaseYaml()
	if err != nil {
		return err
	}

	// Overwrite with the target version Bump only
	releaseVersions[r.Chart] = []string{r.ChartVersion}

	// Since we opened and read the file before we can truncate it.
	outputFile, err := os.Create(r.ReleaseYamlPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	encoder := yaml.NewEncoder(outputFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(releaseVersions); err != nil {
		return err
	}

	return nil
}

// PullIcon will pull the icon from the chart and save it to the local assets/logos directory
func (r *Release) PullIcon(ctx context.Context, rootFs billy.Filesystem) error {
	logger.Log(ctx, slog.LevelInfo, "starting to pull icon process")

	// Get Chart.yaml path and load it
	chartYamlPath := path.RepositoryChartsDir + "/" + r.Chart + "/" + r.ChartVersion + "/Chart.yaml"
	absChartPath := filesystem.GetAbsPath(rootFs, chartYamlPath)

	// Load Chart.yaml file
	chartMetadata, err := helmChartutil.LoadChartfile(absChartPath)
	if err != nil {
		return fmt.Errorf("could not load %s: %s", chartYamlPath, err)
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
	if err := r.git.CheckFileExists(relativeIconPath, r.VR.DevBranch); err != nil {
		logger.Log(ctx, slog.LevelError, "icon file not found in dev branch but should", slog.String("icon", relativeIconPath), logger.Err(err))
		return fmt.Errorf("icon file not found in dev branch but should: %w", err)
	}

	// checkout the icon file from the dev branch
	if err := r.git.CheckoutFile(r.VR.DevBranch, relativeIconPath); err != nil {
		return err
	}

	// git reset return
	return r.git.ResetHEAD()
}
