package update

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
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
func ApplyUpstreamTemplate(rootFs billy.Filesystem, chartsScriptOptions options.ChartsScriptOptions) error {
	for _, dir := range []string{"template", "generated"} {
		if err := rootFs.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("unable to create empty directory at %s: %v", filesystem.GetAbsPath(rootFs, dir), err)
		}
		defer filesystem.RemoveAll(rootFs, dir)
	}

	err := pullUpstream(rootFs, rootFs, "template")
	if err != nil {
		return err
	}

	err = applyTemplate(rootFs, chartsScriptOptions, "template", "generated")
	if err != nil {
		return fmt.Errorf("could not apply template: %v", err)
	}

	return filesystem.CopyDir(rootFs, "generated", ".")
}

func pullUpstream(rootFs billy.Filesystem, pkgFs billy.Filesystem, templateDir string) error {
	upstreamTemplateDir := filepath.Join(ChartsBuildScriptRepositoryTemplatesDirectory, ChartsBuildScriptRepositoryTemplateDirectory)

	// Get upstream contents at templates/template
	templateRepository, err := puller.GetGithubRepository(
		options.UpstreamOptions{
			URL:          ChartsBuildScriptsRepositoryURL,
			Subdirectory: &upstreamTemplateDir,
		},
		&ChartsBuildScriptsRepositoryBranch)
	if err != nil {
		return fmt.Errorf("unable to get the charts build script repository: %s", err)
	}

	// Pull them into the templateDir specified
	err = templateRepository.Pull(rootFs, pkgFs, templateDir)
	if err != nil {
		return fmt.Errorf("unable to pull the charts build script repository: %s", err)
	}
	return nil
}

func applyTemplate(pkgFs billy.Filesystem, chartsScriptOptions options.ChartsScriptOptions, templateDir string, generatedDir string) error {
	tempFs := filesystem.GetFilesystem(filesystem.GetAbsPath(pkgFs, generatedDir))
	applySingleTemplate := func(fs billy.Filesystem, path string, isDir bool) error {
		if isDir {
			return nil
		}
		repoPath, err := filesystem.MovePath(path, templateDir, "")
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
	return filesystem.WalkDir(pkgFs, templateDir, applySingleTemplate)
}
