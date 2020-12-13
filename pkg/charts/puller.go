package charts

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
)

// LocalPackage represents source that is a package
type LocalPackage struct {
	// Name represents the name of the package
	Name         string
	Subdirectory *string
}

// Pull grabs the package
func (u LocalPackage) Pull(rootFs, fs billy.Filesystem, path string) error {
	if strings.HasPrefix(path, u.Name) {
		return fmt.Errorf("Cannot add package to itself")
	}
	pkg, err := GetPackage(rootFs, u.Name)
	if err != nil {
		return err
	}
	if err := pkg.Prepare(); err != nil {
		return err
	}
	defer pkg.Clean()
	if pkg.Chart.Upstream.IsWithinPackage() {
		// Copy
		repositoryPackageWorkingDir, err := utils.GetRelativePath(rootFs, utils.GetAbsPath(pkg.fs, pkg.WorkingDir))
		if err != nil {
			return err
		}
		repositoryPath, err := utils.GetRelativePath(rootFs, utils.GetAbsPath(fs, path))
		if err != nil {
			return err
		}
		if err := utils.CopyDir(rootFs, repositoryPackageWorkingDir, repositoryPath); err != nil {
			return fmt.Errorf("Encountered error while moving prepared package into path: %s", err)
		}
	} else {
		// Move
		if err := os.Rename(utils.GetAbsPath(pkg.fs, pkg.WorkingDir), utils.GetAbsPath(fs, path)); err != nil {
			return fmt.Errorf("Encountered error while renaming prepared package into path: %s", err)
		}
	}
	if u.Subdirectory != nil && len(*u.Subdirectory) > 0 {
		if err := utils.MakeSubdirectoryRoot(fs, path, *u.Subdirectory); err != nil {
			return err
		}
	}
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u LocalPackage) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: fmt.Sprintf("packages/%s", u.Name),
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u LocalPackage) IsWithinPackage() bool {
	return false
}

func (u LocalPackage) String() string {
	return fmt.Sprintf("packages/%s", u.Name)
}

// Local represents a local chart that exists within the package itself
type Local struct{}

// Pull grabs the Helm chart by preparing the package itself
func (u Local) Pull(rootFs, fs billy.Filesystem, path string) error {
	return nil
}

// GetOptions returns the path used to construct this upstream
func (u Local) GetOptions() options.UpstreamOptions {
	return options.UpstreamOptions{
		URL: "local",
	}
}

// IsWithinPackage returns whether this upstream already exists within the package
func (u Local) IsWithinPackage() bool {
	return true
}

func (u Local) String() string {
	return "local"
}
