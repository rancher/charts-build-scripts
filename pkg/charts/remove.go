package charts

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/helm"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"

	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// Delete, iterate and remove assets/<chart>; charts/<chart>; remove entry from index.yaml
func Delete(ctx context.Context, rootFs billy.Filesystem, assets []string, version string) error {
	for _, asset := range assets {
		if err := DeleteVersion(ctx, rootFs, asset, version); err != nil {
			return err
		}
	}

	return nil
}

// DeleteVersion will check and remove assets/<chart>; charts/<chart>
func DeleteVersion(ctx context.Context, rootFs billy.Filesystem, chart, version string) error {
	logger.Log(ctx, slog.LevelInfo, "remove", slog.String("chart", chart), slog.String("version", version))
	if chart == "" {
		return errors.New("chart not provided")
	}
	if version == "" {
		return errors.New("version not provided")
	}

	chartPath := filepath.Join(path.RepositoryChartsDir, chart, version)
	assetPath := filepath.Join(path.RepositoryAssetsDir, chart, chart+"-"+version+".tgz")
	logger.Log(ctx, slog.LevelDebug, "", slog.String("chartPath", chartPath), slog.String("assetPath", assetPath))

	// check if the charts dir exists
	exist, err := filesystem.PathExists(ctx, rootFs, chartPath)
	if err != nil {
		return errors.New("while trying to check chartPath to delete: " + err.Error())
	}
	if !exist {
		return errors.New("chartPath doesn't exist")
	}

	// check if the assets dir exists
	exist, err = filesystem.PathExists(ctx, rootFs, assetPath)
	if err != nil {
		return errors.New("while trying to check assetPath to delete: " + err.Error())
	}
	if !exist {
		return errors.New("assetPath doesn't exist")
	}

	// remove charts version dir
	if err := removeDirFile(ctx, rootFs, chartPath, filepath.Join(path.RepositoryChartsDir, chart)); err != nil {
		return err
	}
	logger.Log(ctx, slog.LevelDebug, "removed", slog.String("chartPath", chartPath))

	// remove asset version .tgz
	if err := removeDirFile(ctx, rootFs, assetPath, filepath.Join(path.RepositoryAssetsDir, chart)); err != nil {
		return err
	}
	logger.Log(ctx, slog.LevelDebug, "removed", slog.String("assetPath", assetPath))

	// remove asset version entry from index.yaml
	if err := deleteIndexEntry(ctx, rootFs, chart, version); err != nil {
		return err
	}
	logger.Log(ctx, slog.LevelDebug, "removed index.yaml chart-version entry")

	// update release.yaml
	releaseYaml, err := options.LoadReleaseYaml(ctx, rootFs)
	if err != nil {
		return err
	}
	releaseYaml = releaseYaml.Append(chart, version) // Safe does not duplicate

	return releaseYaml.Write(ctx, rootFs)
}

func removeDirFile(ctx context.Context, rootFs billy.Filesystem, target, parent string) error {
	// remove target file or directory
	logger.Log(ctx, slog.LevelDebug, "removing", slog.String("target", target))
	if err := filesystem.RemoveAll(rootFs, target); err != nil {
		return errors.New("failed to remove: " + target + " error: " + err.Error())
	}

	// if parent dir now empty, also remove it
	empty, err := isDirEmpty(rootFs, parent)
	if err != nil {
		return errors.New("trying to delete; checking dir empty: " + err.Error())
	}
	if empty {
		logger.Log(ctx, slog.LevelWarn, "pruning parent empty directory", slog.String("dir", parent))
		if err := rootFs.Remove(parent); err != nil {
			return errors.New("removeDirFile failed: " + err.Error())
		}
	}

	return nil
}

func isDirEmpty(fs billy.Filesystem, dirPath string) (bool, error) {
	entries, err := fs.ReadDir(dirPath)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// deleteIndexEntry seeks a chart at index.yaml and remove it's entry version
func deleteIndexEntry(ctx context.Context, rootFs billy.Filesystem, chart, version string) error {
	index, err := helm.OpenIndexYaml(ctx, rootFs)
	if err != nil {
		return errors.New("deleting index entry; failed to open index.yaml: " + err.Error())
	}

	// check if the chart exists in the index
	if _, exist := index.Entries[chart]; !exist {
		return errors.New("the chart entry does not exist in the index.yaml")
	}

	// create a new map without the entry version
	newEntry := make(map[string]helmRepo.ChartVersions, len(index.Entries[chart])-1)
	found := false
	for _, chartVersion := range index.Entries[chart] {
		if chartVersion.Version == version {
			found = true
			continue
		}
		newEntry[chart] = append(newEntry[chart], chartVersion)
	}
	// entry not found, warn the user
	if !found {
		return errors.New("the version entry could not be found in the index.yaml")
	}

	// replace the old map with the new one without the entry version
	index.Entries[chart] = newEntry[chart]
	if err := index.WriteFile(filesystem.GetAbsPath(rootFs, path.RepositoryHelmIndexFile), os.ModePerm); err != nil {
		return errors.New("deleting index entry; failed to write index.yaml: " + err.Error())
	}

	return nil
}
