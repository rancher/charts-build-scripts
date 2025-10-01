package change

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/diff"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

const (
	patchFmt = "%s.patch"
)

// GenerateChanges generates the change between fromDir and toDir and places it in the appropriate directories within gcDir
func GenerateChanges(ctx context.Context, fs billy.Filesystem, fromDir, toDir, gcRootDir string, replacePaths []string) error {
	logger.Log(ctx, slog.LevelInfo, "generating changes", slog.String("GeneratedChangesDir", path.GeneratedChangesDir))

	// gcRootDir should always end with path.GeneratedChangesDir
	if !strings.HasSuffix(gcRootDir, path.GeneratedChangesDir) {
		return fmt.Errorf("root directory for generated changes should end with %s, received: %s", path.GeneratedChangesDir, gcRootDir)
	}
	if err := removeAllGeneratedChanges(ctx, fs, gcRootDir); err != nil {
		return fmt.Errorf("encountered error while trying to remove all existing generated changes before generating new changes: %s", err)
	}
	replacePathsMap := make(map[string]bool, len(replacePaths))
	for _, path := range replacePaths {
		replacePathsMap[path] = true
	}
	generateOverlayFile := func(ctx context.Context, fs billy.Filesystem, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		overlayPath, err := filesystem.MovePath(ctx, toPath, toDir, filepath.Join(gcRootDir, path.GeneratedChangesOverlayDir))
		if err != nil {
			return err
		}
		if err := filesystem.CopyFile(ctx, fs, toPath, overlayPath); err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelInfo, "overlay", slog.String("toPath", toPath))
		return nil
	}
	generateExcludeFile := func(ctx context.Context, fs billy.Filesystem, fromPath string, isDir bool) error {
		if isDir {
			return nil
		}
		excludePath, err := filesystem.MovePath(ctx, fromPath, fromDir, filepath.Join(gcRootDir, path.GeneratedChangesExcludeDir))
		if err != nil {
			return err
		}
		if err := filesystem.CopyFile(ctx, fs, fromPath, excludePath); err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelInfo, "exclude", slog.String("fromPath", fromPath))
		return nil
	}
	generatePatchFile := func(ctx context.Context, fs billy.Filesystem, fromPath, toPath string, isDir bool) error {
		if isDir {
			return nil
		}

		p, err := filesystem.MovePath(ctx, fromPath, fromDir, "")
		if err != nil {
			return err
		}

		if _, ok := replacePathsMap[p]; ok {
			if err := generateExcludeFile(ctx, fs, fromPath, isDir); err != nil {
				return err
			}
			if err := generateOverlayFile(ctx, fs, toPath, isDir); err != nil {
				return err
			}
			return nil
		}

		patchPath := filepath.Join(gcRootDir, path.GeneratedChangesPatchDir, p)
		patchPathWithExt := fmt.Sprintf(patchFmt, patchPath)
		generatedPatch, err := diff.GeneratePatch(ctx, fs, patchPathWithExt, fromPath, toPath)
		if err != nil {
			return err
		}
		if generatedPatch {
			logger.Log(ctx, slog.LevelInfo, "patch", slog.String("patchPath", patchPath))
		}

		return nil
	}
	return filesystem.CompareDirs(ctx, fs, fromDir, toDir, generateExcludeFile, generateOverlayFile, generatePatchFile)
}

func removeAllGeneratedChanges(ctx context.Context, fs billy.Filesystem, gcRootDir string) error {
	// gcRootDir should always end with path.GeneratedChangesDir
	if !strings.HasSuffix(gcRootDir, path.GeneratedChangesDir) {
		return fmt.Errorf("root directory for generated changes should end with %s, received: %s", path.GeneratedChangesDir, gcRootDir)
	}
	// Remove all overlays
	if err := filesystem.RemoveAll(fs, filepath.Join(gcRootDir, path.GeneratedChangesOverlayDir)); err != nil {
		return err
	}
	// Remove all excludes
	if err := filesystem.RemoveAll(fs, filepath.Join(gcRootDir, path.GeneratedChangesExcludeDir)); err != nil {
		return err
	}
	// Remove all patches
	if err := filesystem.RemoveAll(fs, filepath.Join(gcRootDir, path.GeneratedChangesPatchDir)); err != nil {
		return err
	}
	dependenciesPath := filepath.Join(gcRootDir, path.GeneratedChangesDependenciesDir)
	exists, err := filesystem.PathExists(ctx, fs, dependenciesPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	// Remove all generated changes from dependencies
	fileInfos, err := fs.ReadDir(dependenciesPath)
	if err != nil {
		return err
	}
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		dependencyName := fileInfo.Name()
		newRootDir := filepath.Join(dependenciesPath, dependencyName, path.GeneratedChangesDir)
		if err := removeAllGeneratedChanges(ctx, fs, newRootDir); err != nil {
			return err
		}
	}
	return nil
}
