package change

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/diff"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
)

const (
	patchFmt = "%s.patch"
)

// GenerateChanges generates the change between fromDir and toDir and places it in the appropriate directories within gcDir
func GenerateChanges(fs billy.Filesystem, fromDir, toDir, gcRootDir string) error {
	logrus.Infof("Generating changes to %s", path.GeneratedChangesDir)
	// gcRootDir should always end with path.GeneratedChangesDir
	if !strings.HasSuffix(gcRootDir, path.GeneratedChangesDir) {
		return fmt.Errorf("root directory for generated changes should end with %s, received: %s", path.GeneratedChangesDir, gcRootDir)
	}
	if err := removeAllGeneratedChanges(fs, gcRootDir); err != nil {
		return fmt.Errorf("encountered error while trying to remove all existing generated changes before generating new changes: %s", err)
	}
	generateOverlayFile := func(fs billy.Filesystem, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		overlayPath, err := filesystem.MovePath(toPath, toDir, filepath.Join(gcRootDir, path.GeneratedChangesOverlayDir))
		if err != nil {
			return err
		}
		if err := filesystem.CopyFile(fs, toPath, overlayPath); err != nil {
			return err
		}
		logrus.Infof("Overlay: %s", toPath)
		return nil
	}
	generateExcludeFile := func(fs billy.Filesystem, fromPath string, isDir bool) error {
		if isDir {
			return nil
		}
		excludePath, err := filesystem.MovePath(fromPath, fromDir, filepath.Join(gcRootDir, path.GeneratedChangesExcludeDir))
		if err != nil {
			return err
		}
		if err := filesystem.CopyFile(fs, fromPath, excludePath); err != nil {
			return err
		}
		logrus.Infof("Exclude: %s", fromPath)
		return nil
	}
	generatePatchFile := func(fs billy.Filesystem, fromPath, toPath string, isDir bool) error {
		if isDir {
			return nil
		}
		patchPath, err := filesystem.MovePath(fromPath, fromDir, filepath.Join(gcRootDir, path.GeneratedChangesPatchDir))
		if err != nil {
			return err
		}
		patchPathWithExt := fmt.Sprintf(patchFmt, patchPath)
		generatedPatch, err := diff.GeneratePatch(fs, patchPathWithExt, fromPath, toPath)
		if err != nil {
			return err
		}
		if generatedPatch {
			logrus.Infof("Patch: %s", patchPath)
		}
		return nil
	}
	return filesystem.CompareDirs(fs, fromDir, toDir, generateExcludeFile, generateOverlayFile, generatePatchFile)
}

func removeAllGeneratedChanges(fs billy.Filesystem, gcRootDir string) error {
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
	exists, err := filesystem.PathExists(fs, dependenciesPath)
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
		if err := removeAllGeneratedChanges(fs, newRootDir); err != nil {
			return err
		}
	}
	return nil
}
