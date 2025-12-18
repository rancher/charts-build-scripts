package puller

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/config"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

var (
	// ChartsBuildScriptsRepositoryBranch is the branch that has the latest documentation. Defaults to master
	ChartsBuildScriptsRepositoryBranch string
	// ChartsBuildScriptsRepositoryURL is the URL pointing to the charts builds scripts
	ChartsBuildScriptsRepositoryURL string

	// ChartsBuildScriptRepositoryTemplatesDirectory is the folder within our repository that contains templates
	ChartsBuildScriptRepositoryTemplatesDirectory = "templates"
	// ChartsBuildScriptRepositoryTemplateDirectory is the folder within templates that contains the Go templates to add to the repository
	ChartsBuildScriptRepositoryTemplateDirectory = "template"
)

// ApplyUpstreamTemplate updates a charts-build-scripts repository based on the Go templates tracked in this repository
func ApplyUpstreamTemplate(ctx context.Context) error {
	cfg, err := config.FromContext(ctx)
	if err != nil {
		return err
	}

	for _, dir := range []string{"template", "generated"} {
		if err := cfg.RootFS.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("unable to create empty directory at %s: %v", filesystem.GetAbsPath(cfg.RootFS, dir), err)
		}
		defer filesystem.RemoveAll(cfg.RootFS, dir)
	}

	if err := pullUpstream(ctx, cfg.RootFS, cfg.RootFS, "template"); err != nil {
		return err
	}

	err = applyTemplate(ctx, cfg.RootFS, cfg.ChartsScriptOptions, "template", "generated")
	if err != nil {
		return fmt.Errorf("could not apply template: %v", err)
	}

	if err := filesystem.CopyDir(ctx, cfg.RootFS, "generated", ".", config.IsSoftError(ctx)); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "successfully updated repository based on upstream template")
	return nil
}

func pullUpstream(ctx context.Context, rootFs billy.Filesystem, pkgFs billy.Filesystem, templateDir string) error {
	upstreamTemplateDir := filepath.Join(ChartsBuildScriptRepositoryTemplatesDirectory, ChartsBuildScriptRepositoryTemplateDirectory)

	// Get upstream contents at templates/template
	templateRepository, err := GetGithubRepository(
		options.UpstreamOptions{
			URL:          ChartsBuildScriptsRepositoryURL,
			Subdirectory: &upstreamTemplateDir,
		},
		&ChartsBuildScriptsRepositoryBranch)
	if err != nil {
		return fmt.Errorf("unable to get the charts build script repository: %s", err)
	}

	// Pull them into the templateDir specified
	err = templateRepository.Pull(ctx, rootFs, pkgFs, templateDir)
	if err != nil {
		return fmt.Errorf("unable to pull the charts build script repository: %s", err)
	}
	return nil
}

func applyTemplate(ctx context.Context, pkgFs billy.Filesystem, chartsScriptOptions *options.ChartsScriptOptions, templateDir string, generatedDir string) error {
	tempFs := filesystem.GetFilesystem(filesystem.GetAbsPath(pkgFs, generatedDir))
	applySingleTemplate := func(ctx context.Context, fs billy.Filesystem, path string, isDir bool) error {
		if isDir {
			return nil
		}
		repoPath, err := filesystem.MovePath(ctx, path, templateDir, "")
		if err != nil {
			return fmt.Errorf("unable to get path of %s within %s: %s", repoPath, templateDir, err)
		}
		f, err := filesystem.CreateFileAndDirs(tempFs, repoPath)
		if err != nil {
			return fmt.Errorf("unable to create files and directories in %s: %s", repoPath, err)
		}
		defer f.Close()
		t, err := template.New(filepath.Base(path)).ParseFiles(filesystem.GetAbsPath(pkgFs, path))
		if err != nil {
			return fmt.Errorf("error while defining Go template for %s: %s", path, err)
		}
		if err := t.Execute(f, chartsScriptOptions); err != nil {
			return fmt.Errorf("error while executing Go template for %s: %s", path, err)
		}
		return nil
	}
	return filesystem.WalkDir(ctx, pkgFs, templateDir, config.IsSoftError(ctx), applySingleTemplate)
}
