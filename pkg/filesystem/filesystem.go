package filesystem

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

// GetFilesystem returns a filesystem rooted at the provided path
func GetFilesystem(path string) billy.Filesystem {
	return osfs.New(path)
}

// GetAbsPath returns the absolute path given the relative path within a filesystem
func GetAbsPath(fs billy.Filesystem, path string) string {
	return filepath.Join(fs.Root(), path)
}

// GetRelativePath returns the relative path given the absolute path within a filesystem
func GetRelativePath(fs billy.Filesystem, abspath string) (string, error) {
	if abspath == "" {
		return fs.Root(), nil
	}
	fsRoot := fmt.Sprintf("%s/", filepath.Clean(fs.Root()))
	relativePath := strings.TrimPrefix(abspath, fsRoot)
	if relativePath == abspath {
		return "", fmt.Errorf("Cannot get relative path; path %s does not exist within %s", abspath, fsRoot)
	}
	return relativePath, nil
}

// PathExists checks if a path exists on the filesystem or returns an error
func PathExists(fs billy.Filesystem, path string) (bool, error) {
	absPath := GetAbsPath(fs, path)
	_, err := os.Stat(absPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// UpdatePermissions updates the permissions for a given path to the mode provided
func UpdatePermissions(fs billy.Filesystem, path string, mode int64) error {
	absPath := GetAbsPath(fs, path)
	return os.Chmod(absPath, os.FileMode(mode))
}

// CreateFileAndDirs creates a file on the filesystem and all relevant directories along the way if they do not exist.
// The file that is created must be closed by the caller
func CreateFileAndDirs(fs billy.Filesystem, path string) (billy.File, error) {
	if err := fs.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}
	return fs.Create(path)
}

// RemoveAll removes all files and directories located at the path
func RemoveAll(fs billy.Filesystem, path string) error {
	return os.RemoveAll(GetAbsPath(fs, path))
}

// PruneEmptyDirsInPath removes all empty directories located within the path
func PruneEmptyDirsInPath(fs billy.Filesystem, path string) error {
	for len(path) > 0 {
		exists, err := PathExists(fs, path)
		if err != nil {
			return err
		}
		if !exists {
			path = filepath.Dir(path)
			continue
		}
		empty, err := IsEmptyDir(fs, path)
		if err != nil {
			return err
		}
		if !empty {
			return nil
		}
		if err := fs.Remove(path); err != nil {
			return err
		}
		path = filepath.Dir(path)
	}
	return nil
}

// IsEmptyDir returns whether the path provided is an empty directory or an error
func IsEmptyDir(fs billy.Filesystem, path string) (bool, error) {
	exists, err := PathExists(fs, path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, fmt.Errorf("Path %s does not exist", path)
	}
	fileInfos, err := fs.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(fileInfos) == 0, nil
}

// CopyFile copies a file from srcPath to dstPath within a filesystem. It creates any relevant directories along the way
func CopyFile(fs billy.Filesystem, srcPath string, dstPath string) error {
	var srcFile, dstFile billy.File
	// Get srcFile
	srcExists, err := PathExists(fs, srcPath)
	if err != nil {
		return err
	}
	if !srcExists {
		return fmt.Errorf("Cannot copy nonexistent file from %s to %s", srcPath, dstPath)
	}
	srcFile, err = fs.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	// Get or create dstFile
	dstExists, err := PathExists(fs, dstPath)
	if err != nil {
		return err
	}
	if !dstExists {
		dstFile, err = CreateFileAndDirs(fs, dstPath)
	} else {
		dstFile, err = fs.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	}
	if err != nil {
		return err
	}
	defer dstFile.Close()
	// Copy the file contents over
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("Encountered error while trying to copy from %s to %s: %s", srcPath, dstPath, err)
	}
	return nil
}

// GetChartArchive gets a chart tgz file from an url and drops it into the path specified on the filesystem
func GetChartArchive(fs billy.Filesystem, url string, path string) error {
	// Create file
	tgz, err := CreateFileAndDirs(fs, path)
	if err != nil {
		return fmt.Errorf("Unable to create tgz file: %s", err)
	}
	defer tgz.Close()
	// Get tgz
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Unable to get chart archive: %s", err)
	}
	defer resp.Body.Close()
	// Copy into the tgz
	if _, err = io.Copy(tgz, resp.Body); err != nil {
		return fmt.Errorf("Unable to create chart archive: %s", err)
	}
	return nil
}

// UnarchiveTgz attempts to unarchive the tgz file found at tgzPath in the filesystem
func UnarchiveTgz(fs billy.Filesystem, tgzPath, tgzSubdirectory, destPath string, overwrite bool) error {
	// Check whether the destPath already exists to avoid overwriting it
	if !overwrite {
		exists, err := PathExists(fs, destPath)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("Cannot unarchive %s into %s/ since the path already exists", tgzPath, destPath)
		}
	}
	// Check if you can open the tgzPath as a tar file
	tgz, err := fs.OpenFile(tgzPath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer tgz.Close()
	gzipReader, err := gzip.NewReader(tgz)
	if err != nil {
		return fmt.Errorf("Unable to read gzip formatted file: %s", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	// Iterate through the contents of the tgz to unarchive it
	subdirectoryFound := false
	for {
		h, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		rootPath, err := GetRootPath(h.Name)
		if err != nil {
			return err
		}
		rootPathWithSubdir := filepath.Join(rootPath, tgzSubdirectory)
		if len(tgzSubdirectory) > 0 && !strings.HasPrefix(h.Name, rootPathWithSubdir) {
			continue
		}
		subdirectoryFound = true
		path, err := MovePath(h.Name, rootPathWithSubdir, destPath)
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeDir {
			if err := fs.MkdirAll(path, os.FileMode(h.Mode)); err != nil {
				return err
			}
			continue
		}
		if h.Typeflag == tar.TypeReg {
			f, err := CreateFileAndDirs(fs, path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(f, tarReader); err != nil {
				return err
			}
			if err := UpdatePermissions(fs, path, h.Mode); err != nil {
				return err
			}
			continue
		}
		if h.Name == "pax_global_header" {
			continue
		}
		return fmt.Errorf("Encountered unknown type of file (name=%s) when unarchiving %s", h.Name, tgzPath)
	}
	if len(tgzSubdirectory) > 0 && !subdirectoryFound {
		return fmt.Errorf("Subdirectory %s was not found within the folder outputted by the tgz file", tgzSubdirectory)
	}
	return nil
}

// ArchiveDir archives a directory or a file into a tgz file and put it at destTgzPath which should end with .tgz
func ArchiveDir(fs billy.Filesystem, srcPath, destTgzPath string) error {
	if !strings.HasSuffix(destTgzPath, ".tgz") {
		return fmt.Errorf("the destTgzPath %s does not end with .tgz", destTgzPath)
	}
	tgzFile, err := fs.Create(destTgzPath)
	if err != nil {
		return err
	}
	defer tgzFile.Close()

	gz := gzip.NewWriter(tgzFile)
	defer gz.Close()

	tarWriter := tar.NewWriter(gz)
	defer tarWriter.Close()

	return WalkDir(fs, srcPath, func(fs billy.Filesystem, path string, isDir bool) error {
		info, err := fs.Stat(path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		// overwrite the name to be the full path to the file
		header.Name = path
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		// The directory structure is preserved, but there is no data to read from a Dir
		if isDir {
			return nil
		}
		file, err := fs.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		return err
	})
}

// RelativePathFunc is a function that is applied on a relative path within the given filesystem
type RelativePathFunc func(fs billy.Filesystem, path string, isDir bool) error

// RelativePathPairFunc is a function that is applied on a pair of relative paths in a filesystem
type RelativePathPairFunc func(fs billy.Filesystem, leftPath, rightPath string, isDir bool) error

// WalkDir walks through a directory given by dirpath rooted in the filesystem and performs doFunc at the path
// The path on each call will be relative to the filesystem provided.
func WalkDir(fs billy.Filesystem, dirpath string, doFunc RelativePathFunc) error {
	// Create all necessary directories
	return filepath.Walk(GetAbsPath(fs, dirpath), func(abspath string, info os.FileInfo, err error) error {
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				// Path does not exist anymore, so do not walk it
				return nil
			}
			return err
		}
		path, err := GetRelativePath(fs, abspath)
		if err != nil {
			return err
		}
		return doFunc(fs, path, info.IsDir())
	})
}

// CopyDir copies all files from srcDir to dstDir
func CopyDir(fs billy.Filesystem, srcDir string, dstDir string) error {
	return WalkDir(fs, srcDir, func(fs billy.Filesystem, srcPath string, isDir bool) error {
		dstPath, err := MovePath(srcPath, srcDir, dstDir)
		if err != nil {
			return err
		}
		if isDir {
			return fs.MkdirAll(dstPath, os.ModePerm)
		}
		data, err := ioutil.ReadFile(GetAbsPath(fs, srcPath))
		if err != nil {
			return err
		}
		return ioutil.WriteFile(GetAbsPath(fs, dstPath), data, os.ModePerm)
	})
}

// MakeSubdirectoryRoot makes a particular subdirectory of a path its main directory
func MakeSubdirectoryRoot(fs billy.Filesystem, path, subdirectory string) error {
	exists, err := PathExists(fs, filepath.Join(path, subdirectory))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Subdirectory %s does not exist in path %s in filesystem %s", subdirectory, path, fs.Root())
	}
	absTempDir, err := ioutil.TempDir(fs.Root(), "make-subdirectory-root")
	if err != nil {
		return err
	}
	defer os.RemoveAll(absTempDir)
	tempDir, err := GetRelativePath(fs, absTempDir)
	if err != nil {
		return err
	}
	if err := CopyDir(fs, filepath.Join(path, subdirectory), tempDir); err != nil {
		return err
	}
	if err := RemoveAll(fs, path); err != nil {
		return nil
	}
	if err := os.Rename(absTempDir, GetAbsPath(fs, path)); err != nil {
		return err
	}
	return nil
}

// CompareDirs compares the contents of the directory at fromDirpath against that of the directory at toDirpath within a given filesystem
// It execute leftOnlyFunc on paths that only exist on the leftDirpath and rightOnlyFunc on paths that only exist on rightDirpath
// It executes bothFunc on paths that exist on both the left and the right. Order will be preserved in the function arguments
func CompareDirs(fs billy.Filesystem, leftDirpath, rightDirpath string, leftOnlyFunc, rightOnlyFunc RelativePathFunc, bothFunc RelativePathPairFunc) error {
	applyLeftOnlyOrBoth := func(fs billy.Filesystem, leftPath string, isDir bool) error {
		rightPath, err := MovePath(leftPath, leftDirpath, rightDirpath)
		if err != nil {
			return err
		}
		exists, err := PathExists(fs, rightPath)
		if err != nil {
			return err
		}
		if !exists {
			return leftOnlyFunc(fs, leftPath, isDir)
		}
		return bothFunc(fs, leftPath, rightPath, isDir)
	}
	applyRightOnly := func(fs billy.Filesystem, rightPath string, isDir bool) error {
		leftPath, err := MovePath(rightPath, rightDirpath, leftDirpath)
		if err != nil {
			return err
		}
		exists, err := PathExists(fs, leftPath)
		if err != nil {
			return err
		}
		if !exists {
			return rightOnlyFunc(fs, rightPath, isDir)
		}
		return nil
	}
	if err := WalkDir(fs, leftDirpath, applyLeftOnlyOrBoth); err != nil {
		return err
	}
	return WalkDir(fs, rightDirpath, applyRightOnly)
}

// GetRootPath returns the first directory in a given path
func GetRootPath(path string) (string, error) {
	rootPathList := strings.SplitN(path, "/", 2)
	if len(rootPathList) == 0 {
		return "", fmt.Errorf("Unable to get root path of %s", path)
	}
	return filepath.Clean(rootPathList[0]), nil
}

// MovePath takes a path that is contained within fromDir and returns the same path contained within toDir
func MovePath(path string, fromDir string, toDir string) (string, error) {
	if !strings.HasPrefix(path, fromDir) {
		return "", fmt.Errorf("Path %s does not contain directory %s", path, fromDir)
	}
	relativePath := strings.TrimPrefix(path, fromDir)
	relativePath = strings.TrimPrefix(relativePath, "/")
	return filepath.Join(toDir, relativePath), nil
}
