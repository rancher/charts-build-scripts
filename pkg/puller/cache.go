package puller

import (
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/sirupsen/logrus"
)

var RootCache cacher = &noopCache{}

// InitRootCache initializes a cache at the repository's root to be used, if it does not currently exist
func InitRootCache(cacheMode bool, path string) error {
	if !cacheMode {
		return nil
	}
	logrus.Infof("Setting up cache at %s", path)
	// Get repository filesystem
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to get current working directory: %s", err)
	}
	rootFs := filesystem.GetFilesystem(repoRoot)

	// Instantiate cache
	if err := rootFs.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create cache directory at %s/%s: %s", rootFs.Root(), path, err)
	}
	cacheFs, err := rootFs.Chroot(path)
	if err != nil {
		return fmt.Errorf("failed to get cacheFs based on cache in %s/%s: %s", rootFs.Root(), path, err)
	}
	RootCache = &cache{
		rootFs:  rootFs,
		cacheFs: cacheFs,
	}
	return nil
}

// CleanRootCache removes any existing entries in the cache
func CleanRootCache(path string) error {
	// Get repository filesystem
	repoRoot, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Unable to get current working directory: %s", err)
	}
	rootFs := filesystem.GetFilesystem(repoRoot)
	if err := filesystem.RemoveAll(rootFs, path); err != nil {
		return err
	}
	return filesystem.PruneEmptyDirsInPath(rootFs, path)
}

type cacher interface {
	// Add adds a provided path in the fs to the cache under the given key
	// If it was successfully added, it returns true
	Add(key string, fs billy.Filesystem, path string) (bool, error)

	// Get gets the value of key from the cache and places it at the path in fs
	// If it does not exist, it return false
	Get(key string, fs billy.Filesystem, path string) (bool, error)
}

// noopCache doesn't do anything
type noopCache struct{}

// Add does not do anything
func (c *noopCache) Add(key string, fs billy.Filesystem, path string) (bool, error) {
	return false, nil
}

// Get does not do anything
func (c *noopCache) Get(key string, fs billy.Filesystem, path string) (bool, error) {
	return false, nil
}

// cache stores contents at the root of wherever cacheFs points to
type cache struct {
	rootFs  billy.Filesystem
	cacheFs billy.Filesystem
}

// Add copies the contents of the path in fs to the path specified by key in the cacheFs
func (c *cache) Add(key string, fs billy.Filesystem, path string) (bool, error) {
	if len(key) == 0 {
		// cannot cache without key
		return false, nil
	}
	// Get paths from perspective of rootFs
	rootPath, err := c.getRootPath(fs, path)
	if err != nil {
		return false, err
	}
	rootCacheKeyPath, err := c.getRootPath(c.cacheFs, key)
	if err != nil {
		return false, err
	}
	// Remove any existing keys at that path
	if err := c.removeKey(key); err != nil {
		return false, err
	}
	// Figure out if you are adding a directory or a file
	fileInfo, err := fs.Lstat(path)
	if err != nil {
		return false, fmt.Errorf("unable to get information about directory in path %s: %s", filesystem.GetAbsPath(fs, path), err)
	}
	// Only cache directories
	if !fileInfo.IsDir() {
		return false, fmt.Errorf("cannot cache directory located at %s: caching files is not supported", filesystem.GetAbsPath(fs, path))
	}
	// Perform copying
	err = filesystem.CopyDir(c.rootFs, rootPath, rootCacheKeyPath)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *cache) Get(key string, fs billy.Filesystem, path string) (bool, error) {
	if len(key) == 0 {
		// cannot cache without key
		return false, nil
	}
	// Check if cache entry exists
	exists, err := filesystem.PathExists(c.cacheFs, key)
	if err != nil {
		return false, fmt.Errorf("encountered error while trying to get key %s from cache: %s", key, err)
	}
	if !exists {
		// cache entry does not exist
		return false, nil
	}
	// Get paths from perspective of rootFs
	rootPath, err := c.getRootPath(fs, path)
	if err != nil {
		return false, err
	}
	rootCacheKeyPath, err := c.getRootPath(c.cacheFs, key)
	if err != nil {
		return false, err
	}
	// Ensure the key is a folder
	rootCacheKeyFileInfo, err := c.cacheFs.Lstat(key)
	if err != nil {
		return false, fmt.Errorf("unable to get information about directory in path %s: %s", filesystem.GetAbsPath(c.cacheFs, key), err)
	}
	// Key is not a directory
	if !rootCacheKeyFileInfo.IsDir() {
		return false, fmt.Errorf("cannot retrieve non-directory located at %s: only retrieving directories from cache is supported", filesystem.GetAbsPath(c.cacheFs, key))
	}
	// Perform copying
	err = filesystem.CopyDir(c.rootFs, rootCacheKeyPath, rootPath)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *cache) getRootPath(fs billy.Filesystem, path string) (string, error) {
	absPath := filesystem.GetAbsPath(fs, path)
	rootPath, err := filesystem.MovePath(absPath, c.rootFs.Root(), "")
	if err != nil {
		return "", fmt.Errorf("unable to find %s in %s: %s", absPath, c.rootFs.Root(), err)
	}
	return rootPath, err
}

func (c *cache) removeKey(key string) error {
	if err := filesystem.RemoveAll(c.cacheFs, key); err != nil {
		return fmt.Errorf("unable to remove key %s from cache: %s", key, err)
	}
	return nil
}
