package filesystem

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/util"

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
	if abspath == fs.Root() {
		return "", nil
	}
	fsRoot := fmt.Sprintf("%s/", filepath.Clean(fs.Root()))
	relativePath := strings.TrimPrefix(abspath, fsRoot)
	if relativePath == abspath {
		return "", fmt.Errorf("cannot get relative path; path %s does not exist within %s", abspath, fsRoot)
	}
	return relativePath, nil
}

// PathExists checks if a path exists on the filesystem or returns an error
func PathExists(fs billy.Filesystem, path string) (bool, error) {
	absPath := GetAbsPath(fs, path)
	logger.Log(slog.LevelDebug, "checking if path exists", slog.String("absPath", absPath))

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
		return false, fmt.Errorf("path %s does not exist", path)
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
		return fmt.Errorf("cannot copy nonexistent file from %s to %s", srcPath, dstPath)
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
		return fmt.Errorf("encountered error while trying to copy from %s to %s: %s", srcPath, dstPath, err)
	}
	return nil
}

// GetChartArchive gets a chart tgz file from a url and drops it into the path specified on the filesystem
func GetChartArchive(fs billy.Filesystem, url string, path string) error {
	// Create file
	tgz, err := CreateFileAndDirs(fs, path)
	if err != nil {
		return fmt.Errorf("unable to create tgz file: %s", err)
	}
	defer tgz.Close()
	// Get tgz
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("unable to get chart archive: %s", err)
	}
	defer resp.Body.Close()
	// Copy into the tgz
	if _, err = io.Copy(tgz, resp.Body); err != nil {
		return fmt.Errorf("unable to create chart archive: %s", err)
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
			return fmt.Errorf("cannot unarchive %s into %s/ since the path already exists", tgzPath, destPath)
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
		return fmt.Errorf("unable to read gzip formatted file: %s", err)
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
		return fmt.Errorf("encountered unknown type of file (name=%s) when unarchiving %s", h.Name, tgzPath)
	}
	if len(tgzSubdirectory) > 0 && !subdirectoryFound {
		return fmt.Errorf("subdirectory %s was not found within the folder outputted by the tgz file", tgzSubdirectory)
	}
	return nil
}

// CompareTgzs checks to see if the file contents of the archive found at leftTgzPath matches that of the archive found at rightTgzPath
// It does this by comparing the sha256sum of the file contents, ignoring any other information (e.g. file modes, timestamps, etc.)
func CompareTgzs(fs billy.Filesystem, leftTgzPath string, rightTgzPath string) (bool, error) {
	// Read left
	leftFile, err := fs.OpenFile(leftTgzPath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return false, err
	}
	defer leftFile.Close()
	// Read right
	rightFile, err := fs.OpenFile(rightTgzPath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return false, err
	}
	defer rightFile.Close()
	return compareTgzs(leftFile, rightFile)
}

func compareTgzs(leftFile, rightFile io.Reader) (bool, error) {
	leftGzipReader, err := gzip.NewReader(leftFile)
	if err != nil {
		return false, fmt.Errorf("unable to read gzip formatted file: %s", err)
	}
	defer leftGzipReader.Close()
	rightGzipReader, err := gzip.NewReader(rightFile)
	if err != nil {
		return false, fmt.Errorf("unable to read gzip formatted file: %s", err)
	}
	defer rightGzipReader.Close()
	return compareTars(leftGzipReader, rightGzipReader)
}

func compareTars(leftFile, rightFile io.Reader) (bool, error) {
	leftTarReader := tar.NewReader(leftFile)
	rightTarReader := tar.NewReader(rightFile)
	// Get the hashes of all the files
	hashMap := make(map[string][]string)
	// Get the contents of all the tars and tgzs
	tarMap := make(map[string][]io.Reader)
	tgzMap := make(map[string][]io.Reader)
	var isRightEOF, isLeftEOF bool
	var left, right *tar.Header
	var leftErr, rightErr error
	for {
		if !isLeftEOF {
			left, leftErr = leftTarReader.Next()
		}
		if !isRightEOF {
			right, rightErr = rightTarReader.Next()
		}
		if leftErr == io.EOF && rightErr == io.EOF {
			// both tgz files have finished reading
			break
		}
		switch {
		case leftErr == io.EOF:
			isLeftEOF = true
		case rightErr == io.EOF:
			isRightEOF = true
		case leftErr != nil:
			// ran into unknown error
			return false, fmt.Errorf("ran into error while trying to read files in left tgz: %v", leftErr)
		case rightErr != nil:
			// ran into unknown error
			return false, fmt.Errorf("ran into error while trying to read files in right tgz: %v", rightErr)
		}
		if !isLeftEOF && left.Typeflag == tar.TypeReg {
			switch filepath.Ext(left.Name) {
			case ".tgz":
				// collect contents of file into reader
				var b bytes.Buffer
				if _, err := b.ReadFrom(leftTarReader); err != nil {
					return false, fmt.Errorf("could not read contents of %s: %v", left.Name, err)
				}
				r, ok := tgzMap[left.Name]
				if !ok {
					r = make([]io.Reader, 2)
				}
				r[0] = &b
				tgzMap[left.Name] = r
			case ".tar":
				// collect contents of file into reader
				var b bytes.Buffer
				if _, err := b.ReadFrom(leftTarReader); err != nil {
					return false, fmt.Errorf("could not read contents of %s: %v", left.Name, err)
				}
				r, ok := tarMap[left.Name]
				if !ok {
					r = make([]io.Reader, 2)
				}
				r[0] = &b
				tarMap[left.Name] = r
			default:
				// compute left hash
				leftHash := sha1.New()
				if _, err := io.Copy(leftHash, leftTarReader); err != nil {
					return false, fmt.Errorf("could not compute hash of %s: %v", left.Name, err)
				}
				// update left hash
				h, ok := hashMap[left.Name]
				if !ok {
					h = make([]string, 2)
				}
				h[0] = string(leftHash.Sum(nil))
				hashMap[left.Name] = h
			}
		}
		if !isRightEOF && right.Typeflag == tar.TypeReg {
			switch filepath.Ext(right.Name) {
			case ".tgz":
				// collect contents of file into reader
				var b bytes.Buffer
				if _, err := b.ReadFrom(rightTarReader); err != nil {
					return false, fmt.Errorf("could not read contents of %s: %v", right.Name, err)
				}
				r, ok := tgzMap[right.Name]
				if !ok {
					r = make([]io.Reader, 2)
				}
				r[1] = &b
				tgzMap[right.Name] = r
			case ".tar":
				// collect contents of file into reader
				var b bytes.Buffer
				if _, err := b.ReadFrom(rightTarReader); err != nil {
					return false, fmt.Errorf("could not read contents of %s: %v", right.Name, err)
				}
				r, ok := tarMap[right.Name]
				if !ok {
					r = make([]io.Reader, 2)
				}
				r[1] = &b
				tarMap[right.Name] = r
			default:
				// compute right hash
				rightHash := sha1.New()
				if _, err := io.Copy(rightHash, rightTarReader); err != nil {
					return false, fmt.Errorf("could not compute hash of %s: %v", right.Name, err)
				}
				// Update right hash
				h, ok := hashMap[right.Name]
				if !ok {
					h = make([]string, 2)
				}
				h[1] = string(rightHash.Sum(nil))
				hashMap[right.Name] = h
			}
		}
	}

	identical := true

	// Sort the files to make it easier to view in debug
	files := make([]string, 0, len(hashMap))
	for filename := range hashMap {
		files = append(files, filename)
	}
	sort.Strings(files)
	tarFiles := make([]string, 0, len(tarMap))
	for filename := range tarMap {
		tarFiles = append(tarFiles, filename)
	}
	sort.Strings(tarFiles)
	tgzFiles := make([]string, 0, len(tgzMap))
	for filename := range tgzMap {
		tgzFiles = append(tgzFiles, filename)
	}
	sort.Strings(tgzFiles)

	// Go through hashes to see if there are any mismatches
	for _, filename := range files {
		hashes, ok := hashMap[filename]
		if !ok {
			return false, fmt.Errorf("could not find %s in hashMap", filename)
		}
		// Check if both archives contain the files
		switch {
		case len(hashes[0]) == 0:
			logger.Log(slog.LevelDebug, "file does not exist in left tar", slog.String("filename", filename))
			identical = false
		case len(hashes[1]) == 0:
			logger.Log(slog.LevelDebug, "file does not exist in right tar", slog.String("filename", filename))
			identical = false
		case hashes[0] != hashes[1]:
			// Hashes do not match
			logger.Log(slog.LevelWarn, "hashes do not match", slog.String("filename", filename), slog.String("leftHash", hashes[0]), slog.String("rightHash", hashes[1]))
			identical = false
		}
	}
	// Go through tar files to see if there are any mismatches
	for _, filename := range tarFiles {
		tars, ok := tarMap[filename]
		if !ok {
			return false, fmt.Errorf("could not find %s in tarMap", filename)
		}
		// Check if both archives contain the files
		switch {
		case tars[0] == nil:
			logger.Log(slog.LevelDebug, "file does not exist in left tar", slog.String("filename", filename))
			identical = false
		case tars[1] == nil:
			logger.Log(slog.LevelDebug, "file does not exist in right tar", slog.String("filename", filename))
			identical = false
		default:
			// Deep compare tars
			logger.Log(slog.LevelDebug, "deep compare contents of tar file", slog.String("filename", filename))

			matches, err := compareTars(tars[0], tars[1])
			if err != nil {
				return false, fmt.Errorf("could not compare contents of %s: %s", filename, err)
			}

			if !matches {
				logger.Log(slog.LevelWarn, "contents do not match for tar file", slog.String("filename", filename))
				identical = false
			}
		}
	}
	// Go through tgz files to see if there are any mismatches
	for _, filename := range tgzFiles {
		tgzs, ok := tgzMap[filename]
		if !ok {
			return false, fmt.Errorf("could not find %s in tgzMap", filename)
		}
		// Check if both archives contain the files
		switch {
		case tgzs[0] == nil:
			logger.Log(slog.LevelDebug, "file does not exist in left tgz", slog.String("filename", filename))
			identical = false
		case tgzs[1] == nil:
			logger.Log(slog.LevelDebug, "file does not exist in right tgz", slog.String("filename", filename))
			identical = false
		default:
			// Deep compare tars
			logger.Log(slog.LevelDebug, "deep compare contents of tgz file", slog.String("filename", filename))

			matches, err := compareTgzs(tgzs[0], tgzs[1])
			if err != nil {
				return false, fmt.Errorf("could not compare contents of %s: %s", filename, err)
			}

			if !matches {
				logger.Log(slog.LevelWarn, "contents do not match for tgz file", slog.String("filename", filename))
				identical = false
			}
		}
	}

	return identical, nil
}

// ArchiveDir archives a directory or a file into a tgz file and put it at destTgzPath which should end with .tgz
func ArchiveDir(fs billy.Filesystem, srcPath, destTgzPath string) error {
	logger.Log(slog.LevelDebug, "archive directory inside .tgz", slog.String("srcPath", srcPath), slog.String("destTgzPath", destTgzPath))

	if !strings.HasSuffix(destTgzPath, ".tgz") {
		return fmt.Errorf("cannot archive %s to %s since the archive path does not end with '.tgz'", srcPath, destTgzPath)
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
		// The directory structure is preserved, but there is no data to read from a directory
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

// WalkDir walks through a directory given by dirPath rooted in the filesystem and performs doFunc at the path
// The path on each call will be relative to the filesystem provided.
func WalkDir(fs billy.Filesystem, dirPath string, doFunc RelativePathFunc) error {
	// Create all necessary directories
	return filepath.Walk(GetAbsPath(fs, dirPath), func(abspath string, info os.FileInfo, err error) error {
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				// Path does not exist anymore, so do not walk it
				return nil
			}
			if !util.IsSoftErrorOn() {
				return err
			}
			// Log the error if soft errors are enabled
			logger.Log(slog.LevelError, "", logger.Err(err))
		}
		path, err := GetRelativePath(fs, abspath)
		if err != nil {
			return err
		}
		walkFuncRes := doFunc(fs, path, info.IsDir())
		if !util.IsSoftErrorOn() {
			return walkFuncRes
		} else if walkFuncRes != nil {
			logger.Log(slog.LevelError, "error walkFunc", logger.Err(walkFuncRes))
		}
		return nil
	})
}

// CopyDir copies all files from srcDir to dstDir
func CopyDir(fs billy.Filesystem, srcDir string, dstDir string) error {
	logger.Log(slog.LevelDebug, "copying files", slog.String("srcDir", srcDir), slog.String("dstDir", dstDir))

	return WalkDir(fs, srcDir, func(fs billy.Filesystem, srcPath string, isDir bool) error {
		dstPath, err := MovePath(srcPath, srcDir, dstDir)
		if err != nil {
			return err
		}

		if isDir {
			return fs.MkdirAll(dstPath, os.ModePerm)
		}

		data, err := os.ReadFile(GetAbsPath(fs, srcPath))
		if err != nil {
			return err
		}

		return os.WriteFile(GetAbsPath(fs, dstPath), data, os.ModePerm)
	})
}

// MakeSubdirectoryRoot makes a particular subdirectory of a path its main directory
func MakeSubdirectoryRoot(fs billy.Filesystem, path, subdirectory string) error {
	exists, err := PathExists(fs, filepath.Join(path, subdirectory))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("subdirectory %s does not exist in path %s in filesystem %s", subdirectory, path, fs.Root())
	}
	absTempDir, err := os.MkdirTemp(fs.Root(), "make-subdirectory-root")
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
	logger.Log(slog.LevelDebug, "compare directories", slog.String("leftDirpath", leftDirpath), slog.String("rightDirpath", rightDirpath))

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
	logger.Log(slog.LevelDebug, "get root path", slog.String("path", path))

	rootPathList := strings.SplitN(path, "/", 2)
	if len(rootPathList) == 0 {
		return "", fmt.Errorf("unable to get root path of %s", path)
	}
	return filepath.Clean(rootPathList[0]), nil
}

// MovePath takes a path that is contained within fromDir and returns the same path contained within toDir
func MovePath(path string, fromDir string, toDir string) (string, error) {
	logger.Log(slog.LevelDebug, "moving path", slog.String("path", path))

	if !strings.HasPrefix(path, fromDir) {
		return "", fmt.Errorf("path %s does not contain directory %s", path, fromDir)
	}

	relativePath := strings.TrimPrefix(path, fromDir)
	relativePath = strings.TrimPrefix(relativePath, "/")
	return filepath.Join(toDir, relativePath), nil
}
