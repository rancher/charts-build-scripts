package charts

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/sumdb/dirhash"
)

// ApplyChanges applies the changes from the gcOverlayDirpath, gcExcludeDirpath, and gcPatchDirpath within gcDir to toDir within the package filesystem
func ApplyChanges(fs billy.Filesystem, toDir, gcRootDir string) error {
	logrus.Infof("Applying changes from %s", GeneratedChangesDirpath)
	// gcRootDir should always end with GeneratedChangesDirpath
	if !strings.HasSuffix(gcRootDir, GeneratedChangesDirpath) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", GeneratedChangesDirpath, gcRootDir)
	}
	chartsOverlayDirpath := filepath.Join(gcRootDir, GeneratedChangesOverlayDirpath)
	chartsExcludeDirpath := filepath.Join(gcRootDir, GeneratedChangesExcludeDirpath)
	chartsPatchDirpath := filepath.Join(gcRootDir, GeneratedChangesPatchDirpath)
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

// GenerateChanges generates the change between fromDir and toDir and places it in the appropriate directories within gcDir
func GenerateChanges(fs billy.Filesystem, fromDir, toDir, gcRootDir string) error {
	logrus.Infof("Generating changes to %s", GeneratedChangesDirpath)
	// gcRootDir should always end with GeneratedChangesDirpath
	if !strings.HasSuffix(gcRootDir, GeneratedChangesDirpath) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", GeneratedChangesDirpath, gcRootDir)
	}
	if err := removeAllGeneratedChanges(fs, gcRootDir); err != nil {
		return fmt.Errorf("Encountered error while trying to remove all existing generated changes before generating new changes: %s", err)
	}
	generatePatchFile := func(fs billy.Filesystem, fromPath, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		patchPath, err := utils.MovePath(fromPath, fromDir, filepath.Join(gcRootDir, GeneratedChangesPatchDirpath))
		if err != nil {
			return err
		}
		patchPathWithExt := fmt.Sprintf(patchFmt, patchPath)
		generatedPatch, err := utils.GeneratePatch(fs, patchPathWithExt, fromPath, toPath)
		if err != nil {
			return err
		}
		if generatedPatch {
			logrus.Infof("Patch: %s", patchPath)
		}
		return nil
	}
	generateOverlayFile := func(fs billy.Filesystem, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		overlayPath, err := utils.MovePath(toPath, toDir, filepath.Join(gcRootDir, GeneratedChangesOverlayDirpath))
		if err != nil {
			return err
		}
		if err := utils.CopyFile(fs, toPath, overlayPath); err != nil {
			return err
		}
		logrus.Infof("Overlay: %s", toPath)
		return nil
	}
	generateExcludeFile := func(fs billy.Filesystem, fromPath string, isDir bool) error {
		if isDir {
			return nil
		}
		excludePath, err := utils.MovePath(fromPath, fromDir, filepath.Join(gcRootDir, GeneratedChangesExcludeDirpath))
		if err != nil {
			return err
		}
		if err := utils.CopyFile(fs, fromPath, excludePath); err != nil {
			return err
		}
		logrus.Infof("Exclude: %s", fromPath)
		return nil
	}
	return utils.CompareDirs(fs, fromDir, toDir, generateExcludeFile, generateOverlayFile, generatePatchFile)
}

func removeAllGeneratedChanges(fs billy.Filesystem, gcRootDir string) error {
	// gcRootDir should always end with GeneratedChangesDirpath
	if !strings.HasSuffix(gcRootDir, GeneratedChangesDirpath) {
		return fmt.Errorf("Root directory for generated changes should end with %s, received: %s", GeneratedChangesDirpath, gcRootDir)
	}
	// Remove all overlays
	if err := utils.RemoveAll(fs, filepath.Join(gcRootDir, GeneratedChangesOverlayDirpath)); err != nil {
		return err
	}
	// Remove all excludes
	if err := utils.RemoveAll(fs, filepath.Join(gcRootDir, GeneratedChangesExcludeDirpath)); err != nil {
		return err
	}
	// Remove all patches
	if err := utils.RemoveAll(fs, filepath.Join(gcRootDir, GeneratedChangesPatchDirpath)); err != nil {
		return err
	}
	dependenciesPath := filepath.Join(gcRootDir, GeneratedChangesDependenciesDirpath)
	exists, err := utils.PathExists(fs, dependenciesPath)
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
		newRootDir := filepath.Join(dependenciesPath, dependencyName, GeneratedChangesDirpath)
		if err := removeAllGeneratedChanges(fs, newRootDir); err != nil {
			return err
		}
	}
	return nil
}

// DoesNotModifyContentsAtLevel compare hashed contents of oldDir and newDir at a particular directory level and returns an error if directories are modified at that level
// If the directories provided are not available on the repository or do not go down that many subdirectories, it returns no error
func DoesNotModifyContentsAtLevel(rootFs billy.Filesystem, oldDir, newDir string, level int) error {
	// Check if there are charts to compare to each other
	oldDirExists, err := utils.PathExists(rootFs, oldDir)
	if err != nil {
		return fmt.Errorf("Encountered error while checking if %s exists: %s", oldDir, err)
	}
	newDirExists, err := utils.PathExists(rootFs, newDir)
	if err != nil {
		return fmt.Errorf("Encountered error while checking if %s exists: %s", newDir, err)
	}
	if !oldDirExists || !newDirExists {
		// Nothing to modify or nothing to add modifications
		return nil
	}
	// Check hashes if reached level
	if level <= 1 {
		absNewDir := utils.GetAbsPath(rootFs, newDir)
		absOldDir := utils.GetAbsPath(rootFs, oldDir)
		newHash, err := dirhash.HashDir(absNewDir, "", dirhash.DefaultHash)
		if err != nil {
			return fmt.Errorf("Error while trying to generate hash of %s: %s", absNewDir, err)
		}
		oldHash, err := dirhash.HashDir(absOldDir, "", dirhash.DefaultHash)
		if err != nil {
			return fmt.Errorf("Error while trying to generate hash of %s: %s", absOldDir, err)
		}
		if newHash != oldHash {
			// Found conflict at level!
			return fmt.Errorf("Hash of new contents at %s does not match that of old contents at %s", newDir, oldDir)
		}
		return nil
	}
	// Recurse down one level in the newDir, since we only care if newDir modifies oldDir
	newSubDirs, err := rootFs.ReadDir(newDir)
	if err != nil {
		return fmt.Errorf("Error while reading files in %s: %s", newDir, err)
	}
	for _, newSubDir := range newSubDirs {
		newSubDirPath := filepath.Join(newDir, newSubDir.Name())
		oldSubDirPath := filepath.Join(oldDir, newSubDir.Name())
		if err := DoesNotModifyContentsAtLevel(rootFs, oldSubDirPath, newSubDirPath, level-1); err != nil {
			return err
		}
	}
	return nil
}
