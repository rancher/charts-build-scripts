package change

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// ApplyChanges applies the changes from the gcOverlayDirpath, gcExcludeDirpath, and gcPatchDirpath within gcDir to toDir within the package filesystem
func ApplyChanges(ctx context.Context, fs billy.Filesystem, toDir, gcRootDir string) error {
	logger.Log(ctx, slog.LevelInfo, "applying changes")
	// gcRootDir should always end with config.PathChangesDir
	if !strings.HasSuffix(gcRootDir, config.PathChangesDir) {
		return fmt.Errorf("root directory for generated changes should end with %s, received: %s", config.PathChangesDir, gcRootDir)
	}
	chartsOverlayDirpath := filepath.Join(gcRootDir, config.PathOverlayDir)
	chartsExcludeDirpath := filepath.Join(gcRootDir, config.PathExcludeDir)
	chartsPatchDirpath := filepath.Join(gcRootDir, config.PathPatchDir)
	applyPatchFile := func(ctx context.Context, fs billy.Filesystem, patchPath string, isDir bool) error {
		if isDir {
			return nil
		}

		logger.Log(ctx, slog.LevelDebug, "applying patch", slog.String("path", patchPath))
		return ApplyPatchDiff(ctx, fs, patchPath, toDir)
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
		err = filesystem.WalkDir(ctx, fs, chartsPatchDirpath, config.IsSoftError(ctx), applyPatchFile)
		if err != nil {
			return err
		}
	}
	exists, err = filesystem.PathExists(ctx, fs, chartsExcludeDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(ctx, fs, chartsExcludeDirpath, config.IsSoftError(ctx), applyExcludeFile)
		if err != nil {
			return err
		}
	}
	exists, err = filesystem.PathExists(ctx, fs, chartsOverlayDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(ctx, fs, chartsOverlayDirpath, config.IsSoftError(ctx), applyOverlayFile)
		if err != nil {
			return err
		}
	}
	return nil
}
