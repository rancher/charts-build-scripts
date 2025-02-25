package auto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v41/github"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"golang.org/x/oauth2"

	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// Referred to: https://github.com/rancher/charts
const owner = "rancher"
const repo = "charts"

var (
	errReleaseYaml       = errors.New("release.yaml errors")
	errModifiedChart     = errors.New("released chart cannot be modified")
	errMinorPatchVersion = errors.New("chart version must be exactly 1 more patch/minor version than the previous chart version")
)

// validation struct will hold the pull request and its files to be validated.
type validation struct {
	pr    *github.PullRequest
	files []*github.CommitFile
	dep   *lifecycle.Dependencies
}

// loadPullRequestValidation will load the pull request validation struct with the pull request and its files.
// it will also load the dependencies struct with the filesystem, assets versions map, version rules and methods to enforce the lifecycle rules in the given branch.
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

// ValidatePullRequest will execute the check a given pull request for the following:
//   - Checkpoint 0: release.yaml file is valid
//   - TODO: Checkpoint 1: Compare contents of assets/ to charts/
//   - TODO: Checkpoint 2: Compare assets against index.yaml
func ValidatePullRequest(token, prNum string, dep *lifecycle.Dependencies) error {

	v, err := loadPullRequestValidation(token, prNum, dep)
	if err != nil {
		return err
	}

	// Checkpoint 0
	releaseOpts, err := options.LoadReleaseOptionsFromFile(dep.RootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}
	if err := v.validateReleaseYaml(releaseOpts); err != nil {
		return err
	}

	return nil
}

// validateReleaseYaml will validate the release.yaml file for:
//   - each chart version in release.yaml does not modify an already released chart.
//   - each chart version in release.yaml is exactly 1 more patch/minor version than the previous chart version if the chart is being released.
//   - each chart version in release.yaml that is being forward-ported is within the range of the current branch version.
func (v *validation) validateReleaseYaml(releaseOpts options.ReleaseOptions) error {
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

			if err := v.checkMinorPatchVersion(version, releasedVersions); err != nil {
				return err
			}
		}
	}

	return v.checkNeverModifyReleasedChart(assetFilePaths)
}

// checkMinorPatchVersion will check if the chart version is exactly 1 more patch/minor version than the previous chart version or if the chart is being released. If the chart is being forward-ported, this validation is skipped.
func (v *validation) checkMinorPatchVersion(version string, releasedVersions []lifecycle.Asset) error {

	latestReleasedVersion := releasedVersions[0].Version

	// check if the chart version is being released or forward-ported
	release, err := v.dep.VR.CheckChartVersionToRelease(version)
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

// checkNeverModifyReleasedChart will check if the files in the pull request modify a released chart. If a file is found to modify a released chart, an error is returned.
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

// CompareIndexFiles will load the current index.yaml file from the root filesystem and compare it with the index.yaml file from charts.rancher.io
func CompareIndexFiles(rootFs billy.Filesystem) error {
	// verify, search & open current index.yaml file
	localIndexYaml, err := helm.OpenIndexYaml(rootFs)
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
		fmt.Println(diff)
		return errors.New("index.yaml files are different at git repository and charts.rancher.io")
	}
	return nil
}
