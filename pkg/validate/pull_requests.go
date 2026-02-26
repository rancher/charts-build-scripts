package validate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v41/github"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"golang.org/x/oauth2"

	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// owner and repo identify the GitHub repository used for pull request lookups.
// Referred to: https://github.com/rancher/charts
const (
	owner = "rancher"
	repo  = "charts"
)

var (
	// errReleaseYaml is returned when the release.yaml file contains validation errors.
	errReleaseYaml = errors.New("release.yaml errors")
	// errModifiedChart is returned when a previously released chart asset is modified in a PR.
	errModifiedChart = errors.New("released chart cannot be modified")
	// errMinorPatchVersion is returned when a chart version bumps by more than one patch or minor step.
	errMinorPatchVersion = errors.New("chart version must be exactly 1 more patch/minor version than the previous chart version")
)

// validation holds the pull request metadata and changed files for a single validation session.
type validation struct {
	pr    *github.PullRequest
	files []*github.CommitFile
	dep   *lifecycle.Dependencies
}

// loadPullRequestValidation authenticates with GitHub using the given token, fetches the pull
// request and its changed files, and returns a validation ready for checkpoint evaluation.
func loadPullRequestValidation(token, prNum string, dep *lifecycle.Dependencies) (*validation, error) {
	ctx := context.Background()

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	gitClient := github.NewClient(tokenClient)

	pNum, err := strconv.Atoi(prNum)
	if err != nil {
		return nil, err
	}

	pr, resp, err := gitClient.PullRequests.Get(ctx, owner, repo, pNum)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get pull request, status code: %s", resp.Status)
	}

	prFiles, _, err := gitClient.PullRequests.ListFiles(ctx, owner, repo, pNum, nil)
	if err != nil {
		return nil, err
	}

	return &validation{pr, prFiles, dep}, nil
}

// PullRequests validates a pull request against the release.yaml checkpoints:
//   - Checkpoint 0: release.yaml is internally consistent and no released chart is modified
//   - TODO Checkpoint 1: compare contents of assets/ to charts/
//   - TODO Checkpoint 2: compare assets against index.yaml
func PullRequests(ctx context.Context,
	token, prNum, branch string,
	dep *lifecycle.Dependencies) error {
	if token == "" {
		return errors.New("GH_TOKEN environment variable must be set to run validate-release-charts")
	}
	if prNum == "" {
		return errors.New("PR_NUMBER environment variable must be set to run validate-release-charts")
	}
	if branch == "" {
		return errors.New("BRANCH environment variable must be set to run validate-release-charts")
	}
	if !strings.HasPrefix(branch, "release-v") {
		return errors.New("branch must be in the format release-v2.x")
	}

	v, err := loadPullRequestValidation(token, prNum, dep)
	if err != nil {
		return err
	}

	// Checkpoint 0
	releaseOpts, err := options.LoadReleaseOptionsFromFile(ctx, dep.RootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}
	if err := v.validateReleaseYaml(ctx, releaseOpts); err != nil {
		return err
	}

	return nil
}

// validateReleaseYaml validates the release.yaml file against three rules:
//   - no chart version in release.yaml modifies an already released chart asset
//   - each chart version is exactly 1 patch or minor bump above the previous released version
//   - forward-ported chart versions are within the range of the current branch version
func (v *validation) validateReleaseYaml(ctx context.Context, releaseOpts options.ReleaseOptions) error {
	assetFilePaths := make(map[string]string, len(releaseOpts))

	for chart, versions := range releaseOpts {
		for _, version := range versions {
			// save this for checking modified charts later
			assetFilePath := "assets/" + chart + "/" + chart + "-" + version + ".tgz"
			assetFilePaths[assetFilePath] = version

			// check patch/minor version update if not net-new chart
			releasedVersions := v.dep.AssetsVersionsMap[chart]
			if len(releasedVersions) <= 1 {
				continue // net-new chart
			}

			if err := v.checkMinorPatchVersion(ctx, version, releasedVersions); err != nil {
				return err
			}
		}
	}

	return v.checkNeverModifyReleasedChart(assetFilePaths)
}

// checkMinorPatchVersion verifies that version is exactly one patch or minor bump above the
// latest released version. Forward-ported charts (outside the current branch range) are skipped.
func (v *validation) checkMinorPatchVersion(ctx context.Context, version string, releasedVersions []lifecycle.Asset) error {

	latestReleasedVersion := releasedVersions[0].Version

	// check if the chart version is being released or forward-ported
	release, err := v.dep.VR.CheckChartVersionToRelease(ctx, version)
	if err != nil {
		return err
	}
	// skip this validation for forward-ported charts
	if !release {
		return nil
	}

	// Parse the versions
	latestVer, err := semver.NewVersion(latestReleasedVersion)
	if err != nil {
		return err
	}

	newVer, err := semver.NewVersion(version)
	if err != nil {
		return err
	}

	if newVer.Minor() < latestVer.Minor() {
		// get the latest version that will be the 1 minor version below the new version
		for _, releasedVersion := range releasedVersions {
			releasedSemver, err := semver.NewVersion(releasedVersion.Version)
			if err != nil {
				continue
			}
			if newVer.Minor() > releasedSemver.Minor() {
				continue
			}
			if releasedSemver.Patch() == newVer.Patch() &&
				releasedSemver.Minor() == newVer.Minor() &&
				releasedSemver.Major() == newVer.Major() {
				continue
			}
			if newVer.Minor() == releasedSemver.Minor() {
				if newVer.Patch() == releasedSemver.Patch()+1 {
					latestVer = releasedSemver
					break
				}
			}

		}
	}

	// calculate the version bumps
	minorDiff := newVer.Minor() - latestVer.Minor()
	patchDiff := newVer.Patch() - latestVer.Patch()

	// the version bump must be exactly 1 more patch or minor version than the previous chart version
	if minorDiff > 1 || patchDiff > 1 || minorDiff > 0 && patchDiff > 0 {
		return fmt.Errorf("%w: version: %s", errMinorPatchVersion, version)
	}

	return nil
}

// checkNeverModifyReleasedChart returns an error for every asset in assetFilePaths that appears
// in the PR with a status other than "added" or "removed" (i.e. modified).
func (v *validation) checkNeverModifyReleasedChart(assetFilePaths map[string]string) error {
	assetFilePathErrors := make(map[string]error)

	for _, file := range v.files {
		if _, found := assetFilePaths[*file.Filename]; !found {
			continue
		}
		if file.Status != nil && (*file.Status == "added" || *file.Status == "removed") {
			continue
		}
		// any status different from "added" or "removed" means the file was modified
		assetFilePathErrors[*file.Filename] = errModifiedChart
	}

	// give the biggest amount of information possible to the user that will need to fix the pull request.
	if len(assetFilePathErrors) > 0 {
		return fmt.Errorf("%w: %v", errReleaseYaml, assetFilePathErrors)
	}

	return nil
}

// CompareIndexFiles loads the local index.yaml and compares it against the live index.yaml
// from charts.rancher.io, returning an error if the two differ.
func CompareIndexFiles(ctx context.Context, rootFs billy.Filesystem, branch string) error {
	if branch == "" {
		return errors.New("BRANCH environment variable must be set to run compare-index-files")
	}
	// verify, search & open current index.yaml file
	localIndexYaml, err := helm.OpenIndexYaml(ctx, rootFs)
	if err != nil {
		return err
	}

	// download index.yaml from charts.rancher.io
	resp, err := http.Get("https://charts.rancher.io/index.yaml")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// save it to a temporary file
	tempIndex, err := os.CreateTemp("", "temp-index.yaml")
	if err != nil {
		return err
	}
	defer tempIndex.Close()

	_, err = io.Copy(tempIndex, bytes.NewReader(body))
	if err != nil {
		return err
	}

	tempIndexYaml, err := helmRepo.LoadIndexFile(tempIndex.Name())
	if err != nil {
		return err
	}
	defer os.Remove(tempIndex.Name())

	// compare both index.yaml files
	if diff := cmp.Diff(localIndexYaml, tempIndexYaml); diff != "" {
		logger.Log(ctx, slog.LevelDebug, "index.yaml files are different", slog.String("diff", diff))
		return errors.New("index.yaml files are different at git repository and charts.rancher.io")
	}
	return nil
}

// Icons checks that every chart listed in release.yaml has a local icon present
// under a file:// path. Charts matching IsIconException are skipped.
func Icons(ctx context.Context, rootFs billy.Filesystem) error {
	releaseOpts, err := options.LoadReleaseOptionsFromFile(ctx, rootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	releaseOpts.SortBySemver(ctx)

	logger.Log(ctx, slog.LevelInfo, "checking if icons are present in the local filesystem")
	for chart, versions := range releaseOpts {
		if IsIconException(chart) {
			logger.Log(ctx, slog.LevelDebug, "skipping icon check for:", slog.String("chart", chart))
			continue
		}

		version := versions[len(versions)-1]
		logger.Log(ctx, slog.LevelDebug, "checking chart", slog.String("chart", chart))
		if err := loadAndCheckIconPrefix(ctx, rootFs, chart, version); err != nil {
			return err
		}
	}

	return nil
}

// IsIconException reports whether the given chart name is exempt from icon validation.
func IsIconException(chart string) bool {
	if strings.Contains(chart, "-crd") ||
		strings.Contains(chart, "fleet") ||
		strings.Contains(chart, "harvester") ||
		chart == "rancher-webhook" ||
		chart == "rancher-aks-operator" ||
		chart == "rancher-eks-operator" ||
		chart == "rancher-gke-operator" ||
		chart == "rancher-provisioning-capi" ||
		chart == "rancher-pushprox" ||
		chart == "rancher-wins-upgrader" ||
		chart == "remotedialer-proxy" ||
		chart == "system-upgrade-controller" ||
		chart == "ui-plugin-operator" ||
		chart == "rancher-csp-adapter" ||
		chart == "rancher-ali-operator" {
		return true
	}
	return false
}

// loadAndCheckIconPrefix loads Chart.yaml for the given chart version and verifies
// that the icon field uses a local file:// path that actually exists on the filesystem.
func loadAndCheckIconPrefix(ctx context.Context, rootFs billy.Filesystem, chart string, chartVersion string) error {
	metaData, err := helm.LoadChartYaml(rootFs, chart, chartVersion)
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelDebug, "checking if chart has downloaded icon")
	iconField := metaData.Icon

	// Check file prefix if it is a URL just skip this process
	if !strings.HasPrefix(iconField, "file://") {
		logger.Log(ctx, slog.LevelError, "icon path is not a file:// prefix")
		return errors.New("icon path is not a file:// prefix, after make prepare, you need to run make icon for chart:" + chart)
	}

	exist, err := filesystem.PathExists(ctx, rootFs, strings.TrimPrefix(iconField, "file://"))
	if err != nil {
		return err
	}

	if !exist {
		return errors.New("icon path is a file:// prefix, but the icon does not exist, after 'make prepare', you need to run 'make icon' for chart:" + chart)
	}

	return nil
}
