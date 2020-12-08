package charts

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ApplyChanges applies the changes from the gcOverlayDirpath, gcExcludeDirpath, and gcPatchDirpath within gcDir to toDir within the package filesystem
func ApplyChanges(pkgFs billy.Filesystem, toDir, gcRootDir string) error {
	logrus.Infof("Applying changes from %s", GeneratedChangesDirpath)
	// gcRootDir should always end with GeneratedChangesDirpath
	if !strings.HasSuffix(gcRootDir, GeneratedChangesDirpath) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", GeneratedChangesDirpath, gcRootDir)
	}
	chartsOverlayDirpath := filepath.Join(gcRootDir, GeneratedChangesOverlayDirpath)
	chartsExcludeDirpath := filepath.Join(gcRootDir, GeneratedChangesExcludeDirpath)
	chartsPatchDirpath := filepath.Join(gcRootDir, GeneratedChangesPatchDirpath)
	applyPatchFile := func(pkgFs billy.Filesystem, patchPath string, isDir bool) error {
		if isDir {
			return nil
		}
		logrus.Infof("Applying: %s", patchPath)
		return utils.ApplyPatch(pkgFs, patchPath, toDir)
	}

	applyOverlayFile := func(pkgFs billy.Filesystem, overlayPath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := utils.MovePath(overlayPath, chartsOverlayDirpath, toDir)
		if err != nil {
			return err
		}
		logrus.Infof("Adding: %s", filepath)
		return utils.CopyFile(pkgFs, overlayPath, filepath)
	}

	applyExcludeFile := func(pkgFs billy.Filesystem, excludePath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := utils.MovePath(excludePath, chartsExcludeDirpath, toDir)
		if err != nil {
			return err
		}
		logrus.Infof("Removing: %s", filepath)
		return utils.RemoveAll(pkgFs, filepath)
	}
	exists, err := utils.PathExists(pkgFs, chartsPatchDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = utils.WalkDir(pkgFs, chartsPatchDirpath, applyPatchFile)
		if err != nil {
			return err
		}
	}
	exists, err = utils.PathExists(pkgFs, chartsOverlayDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = utils.WalkDir(pkgFs, chartsOverlayDirpath, applyOverlayFile)
		if err != nil {
			return err
		}
	}
	exists, err = utils.PathExists(pkgFs, chartsExcludeDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = utils.WalkDir(pkgFs, chartsExcludeDirpath, applyExcludeFile)
		if err != nil {
			return err
		}
	}
	return nil
}

// GenerateChanges generates the change between fromDir and toDir and places it in the appropriate directories within gcDir
func GenerateChanges(pkgFs billy.Filesystem, fromDir, toDir, gcRootDir string) error {
	logrus.Infof("Generating changes to %s", GeneratedChangesDirpath)
	// gcRootDir should always end with GeneratedChangesDirpath
	if !strings.HasSuffix(gcRootDir, GeneratedChangesDirpath) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", GeneratedChangesDirpath, gcRootDir)
	}
	if err := removeAllGeneratedChanges(pkgFs, gcRootDir); err != nil {
		return fmt.Errorf("Encountered error while trying to remove all existing generated changes before generating new changes: %s", err)
	}
	generatePatchFile := func(pkgFs billy.Filesystem, fromPath, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		patchPath, err := utils.MovePath(fromPath, fromDir, filepath.Join(gcRootDir, GeneratedChangesPatchDirpath))
		if err != nil {
			return err
		}
		patchPathWithExt := fmt.Sprintf(patchFmt, patchPath)
		generatedPatch, err := utils.GeneratePatch(pkgFs, patchPathWithExt, fromPath, toPath)
		if err != nil {
			return err
		}
		if generatedPatch {
			logrus.Infof("Patch: %s", patchPath)
		}
		return nil
	}
	generateOverlayFile := func(pkgFs billy.Filesystem, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		overlayPath, err := utils.MovePath(toPath, toDir, filepath.Join(gcRootDir, GeneratedChangesOverlayDirpath))
		if err != nil {
			return err
		}
		if err := utils.CopyFile(pkgFs, toPath, overlayPath); err != nil {
			return err
		}
		logrus.Infof("Overlay: %s", toPath)
		return nil
	}
	generateExcludeFile := func(pkgFs billy.Filesystem, fromPath string, isDir bool) error {
		if isDir {
			return nil
		}
		excludePath, err := utils.MovePath(fromPath, fromDir, filepath.Join(gcRootDir, GeneratedChangesExcludeDirpath))
		if err != nil {
			return err
		}
		if err := utils.CopyFile(pkgFs, fromPath, excludePath); err != nil {
			return err
		}
		logrus.Infof("Exclude: %s", fromPath)
		return nil
	}
	return utils.CompareDirs(pkgFs, fromDir, toDir, generateExcludeFile, generateOverlayFile, generatePatchFile)
}

func removeAllGeneratedChanges(pkgFs billy.Filesystem, gcRootDir string) error {
	// gcRootDir should always end with GeneratedChangesDirpath
	if !strings.HasSuffix(gcRootDir, GeneratedChangesDirpath) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", GeneratedChangesDirpath, gcRootDir)
	}
	// Remove all overlays
	if err := utils.RemoveAll(pkgFs, filepath.Join(gcRootDir, GeneratedChangesOverlayDirpath)); err != nil {
		return err
	}
	// Remove all excludes
	if err := utils.RemoveAll(pkgFs, filepath.Join(gcRootDir, GeneratedChangesExcludeDirpath)); err != nil {
		return err
	}
	// Remove all patches
	if err := utils.RemoveAll(pkgFs, filepath.Join(gcRootDir, GeneratedChangesPatchDirpath)); err != nil {
		return err
	}
	dependenciesPath := filepath.Join(gcRootDir, GeneratedChangesDependenciesDirpath)
	exists, err := utils.PathExists(pkgFs, dependenciesPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	// Remove all generated changes from dependencies
	fileInfos, err := pkgFs.ReadDir(dependenciesPath)
	if err != nil {
		return err
	}
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		dependencyName := fileInfo.Name()
		newRootDir := filepath.Join(dependenciesPath, dependencyName, GeneratedChangesDirpath)
		if err := removeAllGeneratedChanges(pkgFs, newRootDir); err != nil {
			return err
		}
	}
	return nil
}
