package options

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sort"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/util"
)

// ReleaseOptions represent the values provided in the release.yaml to avoid validation failing on seeing a to-be-released chart.
// This is only used if ValidateOptions are provided in the configuration.yaml
type ReleaseOptions map[string][]string

// Contains checks if a chart and version is tracked by the ReleaseOptions
func (r ReleaseOptions) Contains(chart, version string) bool {
	versions, ok := r[chart]
	if !ok {
		return false
	}
	return slices.Contains(versions, version)
}

// Append adds a chart and version to the ReleaseOptions and returns it
//   - Duplicate-Safe
func (r ReleaseOptions) Append(chart, version string) ReleaseOptions {
	versions, ok := r[chart]
	if !ok {
		versions = []string{}
	}

	if slices.Contains(versions, version) {
		return r // value is present, do not include it
	}

	versions = append(versions, version)
	r[chart] = versions

	return r
}

// Merge merges two ReleaseOptions and returns the merged copy
//   - Duplicate-Safe
func (r ReleaseOptions) Merge(new ReleaseOptions) ReleaseOptions {
	for chart, versions := range new {
		for _, version := range versions {
			r = r.Append(chart, version)
		}
	}
	return r
}

// LoadReleaseYaml unmarshalls the struct found at the file to YAML and reads it into memory
func LoadReleaseYaml(ctx context.Context, fs billy.Filesystem) (ReleaseOptions, error) {
	logger.Log(ctx, slog.LevelDebug, "loading release.yaml")

	exists, err := filesystem.PathExists(ctx, fs, path.RepositoryReleaseYaml)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New(path.RepositoryReleaseYaml + " not found")
	}

	releaseOptions, err := filesystem.LoadYamlFile[ReleaseOptions](ctx, filesystem.GetAbsPath(fs, path.RepositoryReleaseYaml), false)
	if err != nil {
		return ReleaseOptions{}, err
	}
	if releaseOptions == nil {
		return ReleaseOptions{}, nil // file exists but empty
	}

	return *releaseOptions, nil // file has content -> safe to dereference
}

func (r *ReleaseOptions) SortReleaseYaml() error {
	if r == nil {
		return errors.New("nil pointer ReleaseOptions")
	}

	for _, versions := range *r {
		if len(versions) <= 1 {
			continue
		}
		sort.Slice(versions, func(i, j int) bool {
			return util.SortUpstreamAppVersions(versions[i], versions[j])
		})
	}

	return nil
}

// Write marshals the struct to yaml and writes it into the path specified
func (r *ReleaseOptions) Write(ctx context.Context, fs billy.Filesystem) error {
	if err := r.SortReleaseYaml(); err != nil {
		return err
	}

	f, err := filesystem.CreateAndOpenYamlFile(ctx, filesystem.GetAbsPath(fs, path.RepositoryReleaseYaml), true)
	if err != nil {
		return err
	}
	defer f.Close()

	return filesystem.UpdateYamlFile(f, *r)
}
