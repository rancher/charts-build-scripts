package charts

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"helm.sh/helm/v3/pkg/registry"
)

// GetPackages returns all packages found within the repository. If there is a specific package provided, it will return just that Package in the list
func GetPackages(ctx context.Context, specificPackage string) ([]*Package, error) {
	cfg, err := config.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse option or get list of all packages in the repo
	packageList, err := ListPackages(ctx, specificPackage)
	if err != nil {
		return nil, fmt.Errorf("encountered error while listing packages: %v", err)
	}

	// Instantiate each package that was requested and return the list
	var packages []*Package
	for _, packagePath := range packageList {
		pkg, err := GetPackage(ctx, cfg.RootFS, packagePath)
		if err != nil {
			return nil, err
		}
		if pkg == nil {
			return nil, fmt.Errorf("packages does not exist in path %s", packagePath)
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

// ListPackages returns a list of packages found within the repository. If there is a specific package provided, it will return just that Package in the list
func ListPackages(ctx context.Context, specificPackage string) ([]string, error) {
	var packageList []string

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return packageList, err
	}

	exists, err := filesystem.PathExists(ctx, cfg.RootFS, config.PathPackagesDir)
	if err != nil || !exists {
		return packageList, err
	}

	listPackages := func(ctx context.Context, fs billy.Filesystem, dirPath string, isDir bool) error {
		if !isDir {
			return nil
		}
		if len(specificPackage) > 0 {
			packagePrefix := filepath.Join(config.PathPackagesDir, specificPackage)
			if dirPath != packagePrefix && !strings.HasPrefix(dirPath, packagePrefix+"/") {
				// Ignore packages not selected by specificPackage
				return nil
			}
		}
		exists, err := filesystem.PathExists(ctx, cfg.RootFS, filepath.Join(dirPath, config.PathPackageYaml))
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		packageName, err := filesystem.MovePath(ctx, dirPath, config.PathPackagesDir, "")
		if err != nil {
			return err
		}
		packageList = append(packageList, packageName)
		logger.Log(ctx, slog.LevelDebug, "package list", slog.Any("packageList", packageList))

		return nil
	}

	return packageList, filesystem.WalkDir(ctx, cfg.RootFS, config.PathPackagesDir, config.IsSoftError(ctx), listPackages)
}

// GetPackage returns a Package based on the options provided
func GetPackage(ctx context.Context, rootFs billy.Filesystem, name string) (*Package, error) {
	// Get pkgFs
	packageRoot := filepath.Join(config.PathPackagesDir, name)
	exists, err := filesystem.PathExists(ctx, rootFs, packageRoot)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	pkgFs, err := rootFs.Chroot(packageRoot)
	if err != nil {
		return nil, err
	}
	// Get package options from package.yaml
	packageOpt, err := options.LoadPackageOptionsFromFile(ctx, pkgFs, config.PathPackageYaml)
	if err != nil {
		return nil, err
	}

	// version and packageVersion can not exist at the same time although both are optional
	if packageOpt.Version != nil && packageOpt.PackageVersion != nil {
		return nil, fmt.Errorf("cannot have both version and packageVersion at the same time")
	}
	var version *semver.Version
	if packageOpt.Version != nil {
		temp, err := semver.Make(*packageOpt.Version)
		if err != nil {
			return nil, fmt.Errorf("cannot parse version %s as an valid semver: %s", *packageOpt.Version, err)
		}
		version = &temp
	}

	// Get charts
	chart, err := GetChartFromOptions(ctx, packageOpt.MainChartOptions)
	if err != nil {
		return nil, err
	}
	var additionalCharts []*AdditionalChart
	for _, additionalChartOptions := range packageOpt.AdditionalChartOptions {
		additionalChart, err := GetAdditionalChartFromOptions(ctx, additionalChartOptions)
		if err != nil {
			return nil, err
		}
		additionalCharts = append(additionalCharts, &additionalChart)
	}
	p := Package{
		Chart: chart,

		Name:             name,
		Version:          version,
		PackageVersion:   packageOpt.PackageVersion,
		AdditionalCharts: additionalCharts,
		DoNotRelease:     packageOpt.DoNotRelease,
		Auto:             packageOpt.Auto,

		fs:     pkgFs,
		rootFs: rootFs,
	}
	return &p, nil
}

// GetChartFromOptions returns a Chart based on the options provided
func GetChartFromOptions(ctx context.Context, opt options.ChartOptions) (Chart, error) {
	upstream, err := GetUpstream(ctx, opt.UpstreamOptions)
	if err != nil {
		return Chart{}, err
	}
	workingDir := opt.WorkingDir
	if len(workingDir) == 0 {
		workingDir = "charts"
	}
	return Chart{
		WorkingDir:         workingDir,
		Upstream:           upstream,
		IgnoreDependencies: opt.IgnoreDependencies,
		ReplacePaths:       opt.ReplacePaths,
	}, nil
}

// GetAdditionalChartFromOptions returns an AdditionalChart based on the options provided
func GetAdditionalChartFromOptions(ctx context.Context, opt options.AdditionalChartOptions) (AdditionalChart, error) {
	var a AdditionalChart
	if opt.UpstreamOptions == nil && opt.CRDChartOptions == nil {
		return a, fmt.Errorf("cannot parse additional chart options: you must either provide a URL (UpstreamOptions) or provide CRDChartOptions")
	}
	if len(opt.WorkingDir) == 0 {
		return a, fmt.Errorf("cannot have additional chart without working directory")
	}
	if opt.WorkingDir == "charts" {
		return a, fmt.Errorf("working directory for an additional chart cannot be charts")
	}
	a = AdditionalChart{
		WorkingDir:         opt.WorkingDir,
		IgnoreDependencies: opt.IgnoreDependencies,
		ReplacePaths:       opt.ReplacePaths,
	}
	if opt.UpstreamOptions != nil {
		upstream, err := GetUpstream(ctx, *opt.UpstreamOptions)
		if err != nil {
			return a, err
		}
		a.Upstream = &upstream
	}
	if opt.CRDChartOptions != nil {
		crdDirectory := opt.CRDChartOptions.CRDDirectory
		useTarArchive := opt.CRDChartOptions.UseTarArchive
		if crdDirectory == "" && !useTarArchive {
			return a, fmt.Errorf("CRD options must provide a directory to place CRDs within or use tar archive")
		}
		if crdDirectory != "" && useTarArchive {
			return a, fmt.Errorf("CRD options cannot provide both a directory to place CRDs within and use tar archive")
		}
		templateDirectory := opt.CRDChartOptions.TemplateDirectory
		if len(templateDirectory) == 0 {
			return a, fmt.Errorf("CRD options must provide a template directory")
		}
		if crdDirectory == "" && useTarArchive {
			crdDirectory = config.PathCrdsDir
		}
		a.CRDChartOptions = &options.CRDChartOptions{
			TemplateDirectory:           templateDirectory,
			CRDDirectory:                crdDirectory,
			UseTarArchive:               useTarArchive,
			AddCRDValidationToMainChart: opt.CRDChartOptions.AddCRDValidationToMainChart,
		}
	}
	return a, nil
}

// GetUpstream returns the appropriate Upstream given the options provided
func GetUpstream(ctx context.Context, opt options.UpstreamOptions) (puller.Puller, error) {
	if opt.URL == "" {
		return nil, fmt.Errorf("URL is not defined")
	}
	if opt.URL == "local" {
		upstream := Local{}
		return upstream, nil
	}
	if strings.HasPrefix(opt.URL, "packages/") {
		packageName, err := filesystem.MovePath(ctx, opt.URL, "packages", "")
		if err != nil {
			return nil, err
		}
		upstream := LocalPackage{
			Name: packageName,
		}
		if opt.Subdirectory != nil {
			upstream.Subdirectory = opt.Subdirectory
		}
		return upstream, nil
	}
	if registry.IsOCI(opt.URL) {
		upstream := puller.Registry{
			URL: opt.URL,
		}
		return upstream, nil
	}
	if strings.HasSuffix(opt.URL, ".git") {
		upstream, err := puller.GetGithubRepository(opt, opt.ChartRepoBranch)
		if err != nil {
			return nil, err
		}
		return upstream, nil
	}
	if strings.HasSuffix(opt.URL, ".tgz") || strings.Contains(opt.URL, ".tar.gz") {
		upstream := puller.Archive{
			URL: opt.URL,
		}
		if opt.Subdirectory != nil {
			upstream.Subdirectory = opt.Subdirectory
		}
		return upstream, nil
	}
	return nil, fmt.Errorf("URL is invalid (must contain .git or .tgz)")
}
