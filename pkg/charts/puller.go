package charts

import (
	"fmt"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/sirupsen/logrus"
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
		return fmt.Errorf("cannot add package to itself")
	}
	pkg, err := GetPackage(rootFs, u.Name)
	if err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("could not find local package %s", u.Name)
	}
	// Check if the chart's working directory has already been prepared
	packageAlreadyPrepared, err := filesystem.PathExists(pkg.fs, pkg.Chart.WorkingDir)
	if err != nil {
		return fmt.Errorf("encountered error while checking if package %s was already prepared: %s", u.Name, err)
	}
	if packageAlreadyPrepared {
		logrus.Infof("Package %s seems to be already prepared, skipping prepare", u.Name)
	} else {
		if err := pkg.Prepare(); err != nil {
			return err
		}
		defer pkg.Clean()
	}
	// Copy
	repositoryPackageWorkingDir, err := filesystem.GetRelativePath(rootFs, filesystem.GetAbsPath(pkg.fs, pkg.WorkingDir))
	if err != nil {
		return err
	}
	repositoryPath, err := filesystem.GetRelativePath(rootFs, filesystem.GetAbsPath(fs, path))
	if err != nil {
		return err
	}
	if err := filesystem.CopyDir(rootFs, repositoryPackageWorkingDir, repositoryPath); err != nil {
		return fmt.Errorf("encountered error while copying prepared package into path: %s", err)
	}
	if !pkg.Chart.Upstream.IsWithinPackage() && !packageAlreadyPrepared {
		// Remove the non-local package that was not already prepared before we encountered it in the scripts
		logrus.Debugf("Removing %s", pkg.Name)
		if err = filesystem.RemoveAll(rootFs, repositoryPackageWorkingDir); err != nil {
			return fmt.Errorf("encountered error while removing already copied package: %s", err)
		}
	}
	if u.Subdirectory != nil && len(*u.Subdirectory) > 0 {
		if err := filesystem.MakeSubdirectoryRoot(fs, path, *u.Subdirectory); err != nil {
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
