package change

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"golang.org/x/mod/sumdb/dirhash"
)

// DoesNotModifyContentsAtLevel compare hashed contents of oldDir and newDir at a particular directory level and returns an error if directories are modified at that level
// If the directories provided are not available on the repository or do not go down that many subdirectories, it returns no error
func DoesNotModifyContentsAtLevel(rootFs billy.Filesystem, oldDir, newDir string, level int) error {
	// Check if there are charts to compare to each other
	oldDirExists, err := filesystem.PathExists(rootFs, oldDir)
	if err != nil {
		return fmt.Errorf("Encountered error while checking if %s exists: %s", oldDir, err)
	}
	newDirExists, err := filesystem.PathExists(rootFs, newDir)
	if err != nil {
		return fmt.Errorf("Encountered error while checking if %s exists: %s", newDir, err)
	}
	if !oldDirExists || !newDirExists {
		// Nothing to modify or nothing to add modifications
		return nil
	}
	// Check hashes if reached level
	if level <= 1 {
		absNewDir := filesystem.GetAbsPath(rootFs, newDir)
		absOldDir := filesystem.GetAbsPath(rootFs, oldDir)
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
