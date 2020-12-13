package change

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ApplyChanges applies the changes from the gcOverlayDirpath, gcExcludeDirpath, and gcPatchDirpath within gcDir to toDir within the package filesystem
func ApplyChanges(fs billy.Filesystem, toDir, gcRootDir string) error {
	logrus.Infof("Applying changes from %s", path.GeneratedChangesDir)
	// gcRootDir should always end with path.GeneratedChangesDir
	if !strings.HasSuffix(gcRootDir, path.GeneratedChangesDir) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", path.GeneratedChangesDir, gcRootDir)
	}
	chartsOverlayDirpath := filepath.Join(gcRootDir, path.GeneratedChangesOverlayDir)
	chartsExcludeDirpath := filepath.Join(gcRootDir, path.GeneratedChangesExcludeDir)
	chartsPatchDirpath := filepath.Join(gcRootDir, path.GeneratedChangesPatchDir)
	applyPatchFile := func(fs billy.Filesystem, patchPath string, isDir bool) error {
		if isDir {
			return nil
		}
		logrus.Infof("Applying: %s", patchPath)
		return utils.ApplyPatch(fs, patchPath, toDir)
	}

	applyOverlayFile := func(fs billy.Filesystem, overlayPath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := utils.MovePath(overlayPath, chartsOverlayDirpath, toDir)
		if err != nil {
			return err
		}
		logrus.Infof("Adding: %s", filepath)
		return utils.CopyFile(fs, overlayPath, filepath)
	}

	applyExcludeFile := func(fs billy.Filesystem, excludePath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := utils.MovePath(excludePath, chartsExcludeDirpath, toDir)
		if err != nil {
			return err
		}
		logrus.Infof("Removing: %s", filepath)
		return utils.RemoveAll(fs, filepath)
	}
	exists, err := utils.PathExists(fs, chartsPatchDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = utils.WalkDir(fs, chartsPatchDirpath, applyPatchFile)
		if err != nil {
			return err
		}
	}
	exists, err = utils.PathExists(fs, chartsOverlayDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = utils.WalkDir(fs, chartsOverlayDirpath, applyOverlayFile)
		if err != nil {
			return err
		}
	}
	exists, err = utils.PathExists(fs, chartsExcludeDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = utils.WalkDir(fs, chartsExcludeDirpath, applyExcludeFile)
		if err != nil {
			return err
		}
	}
	return nil
}
