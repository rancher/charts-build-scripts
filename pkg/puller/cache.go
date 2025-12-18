package puller

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
)

// RootCache is the cache at the root of the repository
var RootCache cacher = &noopCache{}

// InitRootCache initializes a cache at the repository's root to be used, if it does not currently exist
func InitRootCache(ctx context.Context, cacheMode bool, path string) error {
	if !cacheMode {
		return nil
	}

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "setting up cache", slog.String("path", path))

	// Instantiate cache
	if err := cfg.RootFS.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create cache directory at %s/%s: %s", cfg.RootFS.Root(), path, err)
	}
	cacheFs, err := cfg.RootFS.Chroot(path)
	if err != nil {
		return fmt.Errorf("failed to get cacheFs based on cache in %s/%s: %s", cfg.RootFS.Root(), path, err)
	}
	RootCache = &cache{
		rootFs:  cfg.RootFS,
		cacheFs: cacheFs,
	}
	return nil
}

// CleanRootCache removes any existing entries in the cache
func CleanRootCache(ctx context.Context, path string) error {
	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	if err := filesystem.RemoveAll(cfg.RootFS, path); err != nil {
		return err
	}
	return filesystem.PruneEmptyDirsInPath(ctx, cfg.RootFS, path)
}

type cacher interface {
	// Add adds a provided path in the fs to the cache under the given key
	// If it was successfully added, it returns true
	Add(ctx context.Context, key string, fs billy.Filesystem, path string) (bool, error)

	// Get gets the value of key from the cache and places it at the path in fs
	// If it does not exist, it return false
	Get(ctx context.Context, key string, fs billy.Filesystem, path string) (bool, error)
}

// noopCache doesn't do anything
type noopCache struct{}

// Add does not do anything
func (c *noopCache) Add(ctx context.Context, key string, fs billy.Filesystem, path string) (bool, error) {
	return false, nil
}

// Get does not do anything
func (c *noopCache) Get(ctx context.Context, key string, fs billy.Filesystem, path string) (bool, error) {
	return false, nil
}

// cache stores contents at the root of wherever cacheFs points to
type cache struct {
	rootFs  billy.Filesystem
	cacheFs billy.Filesystem
}

// Add copies the contents of the path in fs to the path specified by key in the cacheFs
func (c *cache) Add(ctx context.Context, key string, fs billy.Filesystem, path string) (bool, error) {
	if len(key) == 0 {
		// cannot cache without key
		return false, nil
	}
	// Get paths from perspective of rootFs
	rootPath, err := c.getRootPath(ctx, fs, path)
	if err != nil {
		return false, err
	}
	rootCacheKeyPath, err := c.getRootPath(ctx, c.cacheFs, key)
	if err != nil {
		return false, err
	}
	// Remove any existing keys at that path
	if err := c.removeKey(ctx, key); err != nil {
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
	err = filesystem.CopyDir(ctx, c.rootFs, rootPath, rootCacheKeyPath, config.IsSoftError(ctx))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *cache) Get(ctx context.Context, key string, fs billy.Filesystem, path string) (bool, error) {
	if len(key) == 0 {
		// cannot cache without key
		return false, nil
	}
	// Check if cache entry exists
	exists, err := filesystem.PathExists(ctx, c.cacheFs, key)
	if err != nil {
		return false, fmt.Errorf("encountered error while trying to get key %s from cache: %s", key, err)
	}
	if !exists {
		// cache entry does not exist
		return false, nil
	}
	// Get paths from perspective of rootFs
	rootPath, err := c.getRootPath(ctx, fs, path)
	if err != nil {
		return false, err
	}
	rootCacheKeyPath, err := c.getRootPath(ctx, c.cacheFs, key)
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
	err = filesystem.CopyDir(ctx, c.rootFs, rootCacheKeyPath, rootPath, config.IsSoftError(ctx))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *cache) getRootPath(ctx context.Context, fs billy.Filesystem, path string) (string, error) {
	absPath := filesystem.GetAbsPath(fs, path)
	rootPath, err := filesystem.MovePath(ctx, absPath, c.rootFs.Root(), "")
	if err != nil {
		return "", fmt.Errorf("unable to find %s in %s: %s", absPath, c.rootFs.Root(), err)
	}
	return rootPath, err
}

func (c *cache) removeKey(ctx context.Context, key string) error {
	if err := filesystem.RemoveAll(c.cacheFs, key); err != nil {
		return fmt.Errorf("unable to remove key %s from cache: %s", key, err)
	}
	return nil
}
