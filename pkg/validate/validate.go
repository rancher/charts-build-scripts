package validate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/standardize"
	"github.com/rancher/charts-build-scripts/pkg/zip"
	"github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/repo"
)

// CompareGeneratedAssetsResponse tracks resources that are added, deleted, and modified when comparing two charts repositories
type CompareGeneratedAssetsResponse struct {
	// UntrackedInRelease represents charts that need to be added to the release.yaml
	UntrackedInRelease options.ReleaseOptions `yaml:"untrackedInRelease,omitempty"`
	// RemovedPostRelease represents charts that have been removed from the upstream
	RemovedPostRelease options.ReleaseOptions `yaml:"removedPostRelease,omitempty"`
	// ModifiedPostRelease represents charts that have been modified from the upstream
	ModifiedPostRelease options.ReleaseOptions `yaml:"modifiedPostRelease,omitempty"`
}

// PassedValidation returns whether the response seems to indicate that the chart repositories are in sync
func (r CompareGeneratedAssetsResponse) PassedValidation() bool {
	return len(r.UntrackedInRelease) == 0 && len(r.RemovedPostRelease) == 0 && len(r.ModifiedPostRelease) == 0
}

// LogDiscrepancies produces logs that can be used to pretty-print why a validation might have failed
func (r CompareGeneratedAssetsResponse) LogDiscrepancies() {
	logrus.Errorf("The following new assets have been introduced: %s", r.UntrackedInRelease)
	logrus.Errorf("The following released assets have been removed: %s", r.RemovedPostRelease)
	logrus.Errorf("The following released assets have been modified: %s", r.ModifiedPostRelease)
	logrus.Errorf("If this was intentional, to allow validation to pass, these charts must be added to the release.yaml.")
}

// DumpReleaseYaml takes the response collected by this CompareGeneratedAssetsResponse and automatically creates the appropriate release.yaml,
// assuming that the user does indeed intend to add, delete, or modify all assets that were marked in this comparison
func (r CompareGeneratedAssetsResponse) DumpReleaseYaml(repoFs billy.Filesystem) error {
	releaseYaml := options.ReleaseOptions{}
	releaseYaml.Merge(r.UntrackedInRelease)
	releaseYaml.Merge(r.RemovedPostRelease)
	releaseYaml.Merge(r.ModifiedPostRelease)
	return releaseYaml.WriteToFile(repoFs, "release.yaml")
}

// CompareGeneratedAssets checks to see if current assets and charts match upstream, aside from those indicated in the release.yaml
// It returns a boolean indicating if the comparison has passed or an error
func CompareGeneratedAssets(repoFs billy.Filesystem, u options.UpstreamOptions, branch string, releaseOptions options.ReleaseOptions) (CompareGeneratedAssetsResponse, error) {
	response := CompareGeneratedAssetsResponse{
		UntrackedInRelease:  options.ReleaseOptions{},
		ModifiedPostRelease: options.ReleaseOptions{},
		RemovedPostRelease:  options.ReleaseOptions{},
	}
	// Pull repository
	logrus.Infof("Pulling upstream repository %s at branch %s", u.URL, branch)
	releasedChartsRepoBranch, err := puller.GetGithubRepository(u, &branch)
	if err != nil {
		return response, fmt.Errorf("failed to get Github repository pointing to new upstream: %s", err)
	}
	if err := releasedChartsRepoBranch.Pull(repoFs, repoFs, path.ChartsRepositoryUpstreamBranchDir); err != nil {
		return response, fmt.Errorf("failed to pull assets from upstream: %s", err)
	}
	defer filesystem.RemoveAll(repoFs, path.ChartsRepositoryUpstreamBranchDir)
	// Standardize the upstream repository
	logrus.Infof("Standardizing upstream repository to compare it against local")
	releaseFs, err := repoFs.Chroot(path.ChartsRepositoryUpstreamBranchDir)
	if err != nil {
		return response, fmt.Errorf("failed to get filesystem for %s: %s", path.ChartsRepositoryUpstreamBranchDir, err)
	}
	if err := standardize.RestructureChartsAndAssets(releaseFs); err != nil {
		return response, fmt.Errorf("failed to standardize upstream: %s", err)
	}
	// Walk through directories and execute release logic
	localOnly := func(fs billy.Filesystem, localPath string, isDir bool) error {
		if isDir {
			// We only care about original files
			return nil
		}
		isAsset := strings.HasPrefix(localPath, path.RepositoryAssetsDir+"/")
		hasTgzExtension := filepath.Ext(localPath) == ".tgz"
		if !isAsset || !hasTgzExtension {
			// We only care about assets
			return nil
		}
		// Check if the chart is tracked in release
		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, localPath))
		if err != nil {
			return err
		}
		if releaseOptions.Contains(chart.Metadata.Name, chart.Metadata.Version) {
			// Chart is tracked in release.yaml
			return nil
		}
		// Chart exists in local and is not tracked by release.yaml
		logrus.Infof("%s/%s is untracked", chart.Metadata.Name, chart.Metadata.Version)
		response.UntrackedInRelease = response.UntrackedInRelease.Append(chart.Metadata.Name, chart.Metadata.Version)
		return nil
	}

	upstreamOnly := func(fs billy.Filesystem, upstreamPath string, isDir bool) error {
		if isDir {
			// We only care about original files
			return nil
		}
		isAsset := strings.HasPrefix(upstreamPath, filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir)+"/")
		hasTgzExtension := filepath.Ext(upstreamPath) == ".tgz"
		if !isAsset || !hasTgzExtension {
			// We only care about assets
			return nil
		}
		// Check if the chart is tracked in release
		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, upstreamPath))
		if err != nil {
			return err
		}
		if releaseOptions.Contains(chart.Metadata.Name, chart.Metadata.Version) {
			// Chart is tracked in release.yaml; this chart was removed intentionally
			return nil
		}
		// Chart was removed from local and is not tracked by release.yaml
		logrus.Infof("%s/%s was removed", chart.Metadata.Name, chart.Metadata.Version)
		response.RemovedPostRelease = response.RemovedPostRelease.Append(chart.Metadata.Name, chart.Metadata.Version)
		// Found asset that only exists in upstream and is not tracked by release.yaml
		localPath, err := filesystem.MovePath(upstreamPath, path.ChartsRepositoryUpstreamBranchDir, "")
		if err != nil {
			return err
		}
		return copyAndUnzip(repoFs, upstreamPath, localPath)
	}

	localAndUpstream := func(fs billy.Filesystem, localPath, upstreamPath string, isDir bool) error {
		if isDir {
			// We only care about modified files
			return nil
		}
		isAsset := strings.HasPrefix(localPath, path.RepositoryAssetsDir+"/")
		hasTgzExtension := filepath.Ext(localPath) == ".tgz"
		if !isAsset || !hasTgzExtension {
			// We only care about assets
			return nil
		}
		// Check if the chart is tracked in release
		chart, err := helmLoader.Load(filesystem.GetAbsPath(fs, upstreamPath))
		if err != nil {
			return err
		}
		if releaseOptions.Contains(chart.Metadata.Name, chart.Metadata.Version) {
			// Chart is tracked in release.yaml
			return nil
		}
		// Deep compare the inner contents of the tgzs
		identical, err := filesystem.CompareTgzs(fs, upstreamPath, localPath)
		if err != nil {
			return err
		}
		if identical {
			return nil
		}
		// Chart was modified in local and is not tracked by release.yaml
		logrus.Infof("%s/%s was modified", chart.Metadata.Name, chart.Metadata.Version)
		response.ModifiedPostRelease = response.RemovedPostRelease.Append(chart.Metadata.Name, chart.Metadata.Version)
		return copyAndUnzip(repoFs, upstreamPath, localPath)
	}

	logrus.Infof("Comparing standardized upstream assets against local assets")
	if err := filesystem.CompareDirs(repoFs, "", path.ChartsRepositoryUpstreamBranchDir, localOnly, upstreamOnly, localAndUpstream); err != nil {
		return response, fmt.Errorf("encountered error while trying to compare local against upstream: %s", err)
	}
	return response, nil
}

func copyAndUnzip(repoFs billy.Filesystem, upstreamPath, localPath string) error {
	specificAsset, err := filesystem.MovePath(upstreamPath, filepath.Join(path.ChartsRepositoryUpstreamBranchDir, path.RepositoryAssetsDir), "")
	if err != nil {
		return fmt.Errorf("encountered error while trying to find repository path for upstream path %s: %s", upstreamPath, err)
	}
	if err := filesystem.CopyFile(repoFs, upstreamPath, localPath); err != nil {
		return err
	}
	if err := zip.DumpAssets(repoFs.Root(), specificAsset); err != nil {
		return fmt.Errorf("encountered error while copying over contents of modified upstream asset to charts: %s", err)
	}
	return nil
}

func CheckImageRancherPrefix() error {
	index, err := repo.LoadIndexFile("index.yaml")
	if err != nil {
		return err
	}
	for _, versions := range index.Entries {
		latestVersion := versions[0]
		tgzPath := filepath.Join("", latestVersion.URLs[0])

		versionValues, err := DecodeValuesFilesInTgz(tgzPath)
		if err != nil {
			logrus.Info(err)
			continue
		}
		chartNameAndVersion := fmt.Sprintf("%s:%s", latestVersion.Name, latestVersion.Version)
		for _, values := range versionValues {
			chartImages := PickImagesFromValuesMap(values, chartNameAndVersion)
			for _, chartImage := range chartImages {
				if !imageHasRancherPrefix(chartImage) {
					return fmt.Errorf("Image [%s] does not start with rancher/\n", chartImage)
				}
			}
		}
	}
	return nil
}

// DecodeValueFilesInTgz reads tarball in tgzPath and returns a slice of values corresponding to values.yaml files found inside of it.
func DecodeValuesFilesInTgz(tgzPath string) ([]map[interface{}]interface{}, error) {
	tgz, err := os.Open(tgzPath)
	if err != nil {
		return nil, err
	}
	defer tgz.Close()
	gzr, err := gzip.NewReader(tgz)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var valuesSlice []map[interface{}]interface{}
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return valuesSlice, nil
		case err != nil:
			return nil, err
		case header.Typeflag == tar.TypeReg && isValuesFile(header.Name):
			var values map[interface{}]interface{}
			if err := decodeYAMLFile(tr, &values); err != nil {
				return nil, err
			}
			valuesSlice = append(valuesSlice, values)
		default:
			continue
		}
	}
}

func decodeYAMLFile(r io.Reader, target interface{}) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func isValuesFile(path string) bool {
	basename := filepath.Base(path)
	return basename == "values.yaml" || basename == "values.yml"
}

// PickImagesFromValuesMap walks a values map to find images, and returns the slice
func PickImagesFromValuesMap(values map[interface{}]interface{}, chartNameAndVersion string) []string {
	var images []string
	walkMap(values, func(inputMap map[interface{}]interface{}) {
		repository, ok := inputMap["repository"].(string)
		if !ok {
			return
		}
		// No string type assertion because some charts have float typed image tags
		tag, ok := inputMap["tag"]
		if !ok {
			return
		}
		imageName := fmt.Sprintf("%s:%v", repository, tag)
		images = append(images, imageName)
		return
	})
	return images
}

func imageHasRancherPrefix(image string) bool {
	return strings.HasPrefix(image, "rancher/")
}

// walkMap walks inputMap and calls the callback function on all map type nodes including the root node.
func walkMap(inputMap interface{}, callback func(map[interface{}]interface{})) {
	switch data := inputMap.(type) {
	case map[interface{}]interface{}:
		callback(data)
		for _, value := range data {
			walkMap(value, callback)
		}
	case []interface{}:
		for _, elem := range data {
			walkMap(elem, callback)
		}
	}
}
