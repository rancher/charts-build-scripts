package local

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

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
	relativePath := strings.TrimPrefix(path, fromDir)
	if relativePath == path {
		return "", fmt.Errorf("Path %s does not contain directory %s", path, fromDir)
	}
	return filepath.Join(toDir, relativePath), nil
}

// GetLocalBranchRefName returns the reference name of a given local branch
func GetLocalBranchRefName(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}

// GetRemoteBranchRefName returns the reference name of a given remote branch
func GetRemoteBranchRefName(branch, remote string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/remote/%s/%s", remote, branch))
}
