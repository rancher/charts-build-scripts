package change

import (
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
func ApplyChanges(fs billy.Filesystem, toDir, gcRootDir string) error {
	logger.Log(slog.LevelInfo, "applying changes")
	// gcRootDir should always end with path.GeneratedChangesDir
	if !strings.HasSuffix(gcRootDir, path.GeneratedChangesDir) {
		return fmt.Errorf("root directory for generated changes should end with %s, received: %s", path.GeneratedChangesDir, gcRootDir)
	}
	chartsOverlayDirpath := filepath.Join(gcRootDir, path.GeneratedChangesOverlayDir)
	chartsExcludeDirpath := filepath.Join(gcRootDir, path.GeneratedChangesExcludeDir)
	chartsPatchDirpath := filepath.Join(gcRootDir, path.GeneratedChangesPatchDir)
	applyPatchFile := func(fs billy.Filesystem, patchPath string, isDir bool) error {
		if isDir {
			return nil
		}

		logger.Log(slog.LevelDebug, "applying patch", slog.String("path", patchPath))
		return diff.ApplyPatch(fs, patchPath, toDir)
	}

	applyOverlayFile := func(fs billy.Filesystem, overlayPath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := filesystem.MovePath(overlayPath, chartsOverlayDirpath, toDir)
		if err != nil {
			return err
		}

		logger.Log(slog.LevelDebug, "Adding", slog.String("filepath", filepath))
		return filesystem.CopyFile(fs, overlayPath, filepath)
	}

	applyExcludeFile := func(fs billy.Filesystem, excludePath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := filesystem.MovePath(excludePath, chartsExcludeDirpath, toDir)
		if err != nil {
			return err
		}

		logger.Log(slog.LevelDebug, "Removing", slog.String("filepath", filepath))
		return filesystem.RemoveAll(fs, filepath)
	}
	exists, err := filesystem.PathExists(fs, chartsPatchDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(fs, chartsPatchDirpath, applyPatchFile)
		if err != nil {
			return err
		}
	}
	exists, err = filesystem.PathExists(fs, chartsExcludeDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(fs, chartsExcludeDirpath, applyExcludeFile)
		if err != nil {
			return err
		}
	}
	exists, err = filesystem.PathExists(fs, chartsOverlayDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = filesystem.WalkDir(fs, chartsOverlayDirpath, applyOverlayFile)
		if err != nil {
			return err
		}
	}
	return nil
}
