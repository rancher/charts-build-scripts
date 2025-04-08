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

// ApplyChanges applies the changes from the gcOverlayDirpath, gcExcludeDirpath, and gcPatchDirpath within gcDir to toDir within the package filesystem
func ApplyChanges(ctx context.Context, fs billy.Filesystem, toDir, gcRootDir string) error {
	logger.Log(ctx, slog.LevelInfo, "applying changes")
	// gcRootDir should always end with path.GeneratedChangesDir
	if !strings.HasSuffix(gcRootDir, path.GeneratedChangesDir) {
		return fmt.Errorf("root directory for generated changes should end with %s, received: %s", path.GeneratedChangesDir, gcRootDir)
	}
	chartsOverlayDirpath := filepath.Join(gcRootDir, path.GeneratedChangesOverlayDir)
	chartsExcludeDirpath := filepath.Join(gcRootDir, path.GeneratedChangesExcludeDir)
	chartsPatchDirpath := filepath.Join(gcRootDir, path.GeneratedChangesPatchDir)
	applyPatchFile := func(ctx context.Context, fs billy.Filesystem, patchPath string, isDir bool) error {
		if isDir {
			return nil
		}

		logger.Log(ctx, slog.LevelDebug, "applying patch", slog.String("path", patchPath))
		return diff.ApplyPatch(ctx, fs, patchPath, toDir)
	}

	applyOverlayFile := func(ctx context.Context, fs billy.Filesystem, overlayPath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := filesystem.MovePath(ctx, overlayPath, chartsOverlayDirpath, toDir)
		if err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelDebug, "Adding", slog.String("filepath", filepath))
		return filesystem.CopyFile(ctx, fs, overlayPath, filepath)
	}

	applyExcludeFile := func(ctx context.Context, fs billy.Filesystem, excludePath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := filesystem.MovePath(ctx, excludePath, chartsExcludeDirpath, toDir)
		if err != nil {
			return err
		}

		logger.Log(ctx, slog.LevelDebug, "Removing", slog.String("filepath", filepath))
		return filesystem.RemoveAll(fs, filepath)
	}
	exists, err := filesystem.PathExists(ctx, fs, chartsPatchDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(ctx, fs, chartsPatchDirpath, applyPatchFile)
		if err != nil {
			return err
		}
	}
	exists, err = filesystem.PathExists(ctx, fs, chartsExcludeDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(ctx, fs, chartsExcludeDirpath, applyExcludeFile)
		if err != nil {
			return err
		}
	}
	exists, err = filesystem.PathExists(ctx, fs, chartsOverlayDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(ctx, fs, chartsOverlayDirpath, applyOverlayFile)
		if err != nil {
			return err
		}
	}
	return nil
}
