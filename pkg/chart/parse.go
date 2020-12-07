package chart

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/local"
	"gopkg.in/yaml.v2"
)

// GetPackages returns all packages in a given repoPath
func GetPackages(repoPath string, specificChart string, branchOptions BranchOptions) ([]*Package, error) {
	// Check if the repoPath is valid
	if _, err := os.Stat(repoPath); err != nil {
		return nil, err
	}
	repoFs := local.GetFilesystem(repoPath)
	if len(specificChart) != 0 {
		packageFs, err := repoFs.Chroot(filepath.Join(RepositoryPackagesDirpath, specificChart))
		if err != nil {
			return nil, err
		}
		p := &Package{
			Name:          specificChart,
			ExportOptions: branchOptions.ExportOptions,
			CleanOptions:  branchOptions.CleanOptions,

			fs:     packageFs,
			repoFs: repoFs,
		}
		if err := p.ReadPackageOptions(); err != nil {
			return nil, err
		}
		return []*Package{p}, nil
	}
	fileInfos, err := repoFs.ReadDir(RepositoryPackagesDirpath)
	if err != nil {
		return nil, err
	}
	var packages []*Package
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		packageFs, err := repoFs.Chroot(filepath.Join(RepositoryPackagesDirpath, fileInfo.Name()))
		if err != nil {
			return nil, err
		}
		p := &Package{
			Name:          fileInfo.Name(),
			ExportOptions: branchOptions.ExportOptions,
			CleanOptions:  branchOptions.CleanOptions,

			fs:     packageFs,
			repoFs: repoFs,
		}
		if err := p.ReadPackageOptions(); err != nil {
			return nil, err
		}
		packages = append(packages, p)
	}
	return packages, nil
}

// GetPackageOptions returns the PackageOptions relating to the current package
func (p *Package) GetPackageOptions() PackageOptions {
	return PackageOptions{
		PackageVersion:  p.PackageVersion,
		UpstreamOptions: p.Upstream.GetOptions(),
		CRDChartOptions: p.CRDChartOptions,
	}
}

// ReadPackageOptions updates an existing package with the options currently in package.yaml
func (p *Package) ReadPackageOptions() error {
	var options PackageOptions
	exists, err := local.PathExists(p.fs, PackageOptionsFilepath)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Could not load options from %s since the path does not exist", local.GetAbsPath(p.fs, PackageOptionsFilepath))
	}
	optionsBytes, err := ioutil.ReadFile(local.GetAbsPath(p.fs, PackageOptionsFilepath))
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(optionsBytes, &options); err != nil {
		return err
	}
	p.PackageVersion = options.PackageVersion
	p.CRDChartOptions = options.CRDChartOptions
	p.Upstream, err = options.GetUpstream()
	if err != nil {
		return fmt.Errorf("Encountered an error while trying to parse the upstream configuration provided in %s: %s", local.GetAbsPath(p.fs, PackageOptionsFilepath), err)
	}
	return nil
}

// WritePackageOptions takes the current package and writes the configuration into the package.yaml
func (p *Package) WritePackageOptions() error {
	// Get the bytes to be written into the packageYaml
	options := p.GetPackageOptions()
	optionsBytes, err := yaml.Marshal(&options)
	if err != nil {
		return fmt.Errorf("Encountered error while trying to marshall package configuration into %s: %s", PackageOptionsFilepath, err)
	}
	// Get the file to write into
	exists, err := local.PathExists(p.fs, PackageOptionsFilepath)
	if err != nil {
		return err
	}
	var packageOptionsFile billy.File
	if exists {
		packageOptionsFile, err = p.fs.OpenFile(PackageOptionsFilepath, os.O_RDWR, os.ModePerm)
	} else {
		packageOptionsFile, err = local.CreateFileAndDirs(p.fs, PackageOptionsFilepath)
	}
	if err != nil {
		return err
	}
	defer packageOptionsFile.Close()
	// Write into the file
	_, err = packageOptionsFile.Write(optionsBytes)
	return err
}
