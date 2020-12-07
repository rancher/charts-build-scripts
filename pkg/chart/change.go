package chart

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/local"
	"github.com/sirupsen/logrus"
)

// applyChanges applies the changes from PackagePatchDirpath, PackageOverlayDirpath, and PackageExcludeDirpath to a given directory in the filesystem
func applyChanges(fs billy.Filesystem, toDir string) error {
	applyPatchFile := func(fs billy.Filesystem, patchPath string, isDir bool) error {
		if isDir {
			return nil
		}
		logrus.Infof("Applying: %s", patchPath)
		return local.ApplyPatch(fs, patchPath, toDir)
	}

	applyOverlayFile := func(fs billy.Filesystem, overlayPath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := local.MovePath(overlayPath, PackageOverlayDirpath, toDir)
		if err != nil {
			return err
		}
		logrus.Infof("Adding: %s", filepath)
		return local.CopyFile(fs, overlayPath, filepath)
	}

	applyExcludeFile := func(fs billy.Filesystem, excludePath string, isDir bool) error {
		if isDir {
			return nil
		}
		filepath, err := local.MovePath(excludePath, PackageExcludeDirpath, toDir)
		if err != nil {
			return err
		}
		logrus.Infof("Removing: %s", filepath)
		return local.RemoveAll(fs, filepath)
	}
	exists, err := local.PathExists(fs, PackagePatchDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = local.WalkDir(fs, PackagePatchDirpath, applyPatchFile)
		if err != nil {
			return err
		}
	}
	exists, err = local.PathExists(fs, PackageOverlayDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = local.WalkDir(fs, PackageOverlayDirpath, applyOverlayFile)
		if err != nil {
			return err
		}
	}
	exists, err = local.PathExists(fs, PackageExcludeDirpath)
	if err != nil {
		return err
	}
	if exists {
		err = local.WalkDir(fs, PackageExcludeDirpath, applyExcludeFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateChanges(fs billy.Filesystem, fromDir, toDir string) error {
	generatePatchFile := func(fs billy.Filesystem, fromPath, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		patchPath, err := local.MovePath(fromPath, fromDir, PackagePatchDirpath)
		if err != nil {
			return err
		}
		patchPathWithExt := fmt.Sprintf(patchFmt, patchPath)
		patchPathWithExt, err = relocatePathToDependency(patchPathWithExt)
		if err != nil {
			return err
		}
		generatedPatch, err := local.GeneratePatch(fs, patchPathWithExt, fromPath, toPath)
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
		overlayPath, err := local.MovePath(toPath, toDir, PackageOverlayDirpath)
		if err != nil {
			return err
		}
		overlayPath, err = relocatePathToDependency(overlayPath)
		if err != nil {
			return err
		}
		if err := local.CopyFile(fs, toPath, overlayPath); err != nil {
			return err
		}
		logrus.Infof("Overlay: %s", toPath)
		return nil
	}
	generateExcludeFile := func(fs billy.Filesystem, fromPath string, isDir bool) error {
		if isDir {
			return nil
		}
		excludePath, err := local.MovePath(fromPath, fromDir, PackageExcludeDirpath)
		if err != nil {
			return err
		}
		excludePath, err = relocatePathToDependency(excludePath)
		if err != nil {
			return err
		}
		if err := local.CopyFile(fs, fromPath, excludePath); err != nil {
			return err
		}
		logrus.Infof("Exclude: %s", fromPath)
		return nil
	}
	return local.CompareDirs(fs, fromDir, toDir, generateExcludeFile, generateOverlayFile, generatePatchFile)
}

// relocatePathToDependency returns the right path to place the patch file at
func relocatePathToDependency(path string) (string, error) {
	var packageDirpath string
	if strings.HasPrefix(path, PackageOverlayDirpath) {
		packageDirpath = PackageOverlayDirpath
	}
	if strings.HasPrefix(path, PackagePatchDirpath) {
		packageDirpath = PackagePatchDirpath
	}
	if strings.HasPrefix(path, PackageExcludeDirpath) {
		packageDirpath = PackageExcludeDirpath
	}
	if len(packageDirpath) == 0 {
		return "", fmt.Errorf("Path provided does not point to any directory to relocate from: %s", path)
	}
	pathWithoutPrefix, err := local.MovePath(path, packageDirpath, "")
	if err != nil {
		return "", err
	}
	splitPath := strings.Split(pathWithoutPrefix, "/")
	var lastReplacementIndex int
	for i := 0; i < len(splitPath); i++ {
		// Replace every charts/ with generated-changes/dependencies/ to get the right path
		if splitPath[i] == "charts" {
			splitPath[i] = PackageDependenciesDirpath
			lastReplacementIndex = i + 1
		}
	}
	fixedPath := filepath.Join(
		filepath.Join(splitPath[:lastReplacementIndex+1]...),
		packageDirpath,
		filepath.Join(splitPath[lastReplacementIndex+1:]...),
	)
	return fixedPath, nil
}
