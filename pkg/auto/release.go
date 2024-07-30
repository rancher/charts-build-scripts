package auto

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"gopkg.in/yaml.v2"
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

// ReleaseVersions represents the versions within the release.yaml file
type ReleaseVersions map[string][]string

// InitRelease will create the Release struct with access to the necessary dependencies.
func InitRelease(d *lifecycle.Dependencies, s *lifecycle.Status, v, c, f string) (*Release, error) {
	r := &Release{
		git:           d.Git,
		VR:            d.VR,
		ChartVersion:  v,
		Chart:         c,
		ForkRemoteURL: f,
	}

	var assetVersions []lifecycle.Asset = make([]lifecycle.Asset, 0)
	var ok bool

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
	if exist := checkAssetReleased(r.AssetPath); exist {
		return nil, errors.New("asset already released for chart:" + r.Chart + " version:" + r.ChartVersion)
	}

	// Check if we have a release.yaml file in the expected path
	if exist, err := filesystem.PathExists(d.RootFs, path.RepositoryReleaseYaml); err != nil || !exist {
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

	if err := r.git.ResetHEAD(); err != nil {
		return err
	}

	return nil
}

func checkAssetReleased(chartVersion string) bool {
	if _, err := os.Stat(chartVersion); err != nil {
		return false
	}

	return true
}

func mountAssetVersionPath(chart, version string) (assetPath string, assetTgz string) {
	// example: assets/longhorn/longhorn-100.0.0+up0.0.0.tgz
	assetTgz = chart + "-" + version + ".tgz"
	assetPath = "assets/" + chart + "/" + assetTgz
	return
}

// UpdateReleaseYaml reads and parse the release.yaml file to a struct, appends the new version and writes it back to the file.
func (r *Release) UpdateReleaseYaml() error {
	var releaseVersions ReleaseVersions

	rvBytes, err := os.ReadFile(r.ReleaseYamlPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(rvBytes, &releaseVersions); err != nil {
		return err
	}

	// Append new version and remove duplicates if any
	releaseVersions[r.Chart] = append(releaseVersions[r.Chart], r.ChartVersion)
	releaseVersions[r.Chart] = removeDuplicates(releaseVersions[r.Chart])

	// Marshal the updated releaseVersions back to YAML
	updatedYAML, err := yaml.Marshal(releaseVersions)
	if err != nil {
		return err
	}

	// Post-process YAML to adjust indentation for list items
	updatedYAMLIndented := enforceYamlStandard(updatedYAML)

	// Write the updated YAML back to the file
	if err := os.WriteFile(r.ReleaseYamlPath, updatedYAMLIndented, 0644); err != nil {
		return err
	}

	return nil
}

// removeDuplicates takes a slice of strings and returns a new slice with duplicates removed.
func removeDuplicates(slice []string) []string {
	seen := make(map[string]struct{}) // map to keep track of seen strings
	var result []string               // slice to hold the results

	for _, val := range slice {
		if _, ok := seen[val]; !ok {
			seen[val] = struct{}{}       // mark string as seen
			result = append(result, val) // append to result if not seen before
		}
	}

	return result
}

// enforceYamlStandard adds indentation to list items in a YAML string and a new line at the end of the file.
func enforceYamlStandard(yamlBytes []byte) []byte {
	var indentedBuffer bytes.Buffer
	lines := bytes.Split(yamlBytes, []byte("\n"))
	for i, line := range lines {
		// Check if the line is not the last empty line after splitting
		if i != len(lines)-1 {
			if bytes.HasPrefix(line, []byte("- ")) {
				indentedBuffer.Write([]byte("  ")) // Add two spaces of indentation
			}
			indentedBuffer.Write(line)
			indentedBuffer.WriteByte('\n')
		}
	}
	// Ensure only one newline at the end
	return append(indentedBuffer.Bytes(), '\n')
}
