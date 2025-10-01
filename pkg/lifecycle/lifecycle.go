package lifecycle

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

var (
	errChartRepository = errors.New("chart repository is in an inconsistent state")
	errGitNotClean     = errors.New("local git repository should be clean")
)

// Asset represents an asset with its version and path in the repository
type Asset struct {
	Version string
	path    string
}

// Dependencies holds the necessary filesystem,
// assets versions map, version rules and methods to apply the lifecycle rules in the target branch
type Dependencies struct {
	RootFs            billy.Filesystem
	AssetsVersionsMap map[string][]Asset
	VR                *VersionRules
	Git               *git.Git
	walkDirWrapper    WalkDirFunc // Used to mock the filesystem in unit-tests
}

// WalkDirFunc is a function type that will be used to walk through the filesystem
type WalkDirFunc func(ctx context.Context, fs billy.Filesystem, dirPath string, doFunc filesystem.RelativePathFunc) error

// InitDependencies will check the filesystem, branch version,
// git status, initialize the Dependencies struct and populate it.
// If anything fails the operation will be aborted.
func InitDependencies(ctx context.Context, rootFs billy.Filesystem, repoRoot, branchVersion, currentChart string, newChart bool) (*Dependencies, error) {
	if newChart && currentChart == "" {
		return nil, errors.New("can't create a new empty chart")
	}

	var err error

	workDir := repoRoot

	git, err := git.OpenGitRepo(ctx, workDir)
	if err != nil {
		return nil, err
	}

	// Create the Dependencies struct which will be used for the entire process
	dep := &Dependencies{
		RootFs:         rootFs,
		walkDirWrapper: filesystem.WalkDir, // Assign the WalkDir function to the wrapper
		Git:            git,
	}

	// Git tree must be clean before proceeding with removing charts
	clean, err := git.StatusProcelain(ctx)
	if err != nil {
		return nil, err
	}
	if !clean {
		return nil, errGitNotClean
	}

	// Initialize, load, and check version rules for the current branch
	dep.VR, err = dep.rules(ctx, branchVersion, loadFromJSON)
	if err != nil {
		return nil, err
	}
	logger.Log(ctx, slog.LevelDebug, "version rules loaded", slog.Any("dep.VR.Rules[branchVersion]", dep.VR.Rules[branchVersion]))

	if err := checkFilePaths(ctx, dep.RootFs); err != nil {
		return nil, err
	}

	if newChart {
		dep.AssetsVersionsMap = make(map[string][]Asset)
		dep.AssetsVersionsMap[currentChart] = []Asset{}
		return dep, nil
	}

	// Get the absolute path of the Helm index file and assets versions map to apply rules
	helmIndexPath := filesystem.GetAbsPath(dep.RootFs, path.RepositoryHelmIndexFile)
	dep.AssetsVersionsMap, err = getAssetsMapFromIndex(helmIndexPath, currentChart)
	if err != nil {
		return nil, err
	}
	if len(dep.AssetsVersionsMap) == 0 {
		return nil, errChartRepository
	}

	return dep, nil
}

func checkFilePaths(ctx context.Context, rootFs billy.Filesystem) error {
	// Check if the assets folder and Helm index file exists in the repository
	exists, err := filesystem.PathExists(ctx, rootFs, path.RepositoryAssetsDir)
	if err != nil {
		return err
	}
	if !exists {
		return errChartRepository
	}
	exists, err = filesystem.PathExists(ctx, rootFs, path.RepositoryHelmIndexFile)
	if err != nil {
		return err
	}
	if !exists {
		return errChartRepository
	}
	return nil
}
