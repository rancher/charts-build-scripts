package update

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/utils"
)

// Options are a list of options on what files to copy
type Options map[string][]string

// CopyTemplate applies the configuration to a template located in templateDir and adds it to the repo
func (u Options) CopyTemplate(rootFs billy.Filesystem, chartsScriptOptions options.ChartsScriptOptions, templateDir string) error {
	templateFiles, ok := u[chartsScriptOptions.Template]
	if !ok {
		return fmt.Errorf("No templates defined for type %s. Only source, staging, or live are supported", chartsScriptOptions.Template)
	}
	templateFileMap := make(map[string]bool, len(templateFiles))
	for _, p := range templateFiles {
		templateFileMap[p] = true
	}
	absTempDir, err := ioutil.TempDir(filepath.Join(rootFs.Root(), templateDir), "generated")
	if err != nil {
		return fmt.Errorf("Encountered error while trying to create temporary directory: %s", err)
	}
	defer os.RemoveAll(absTempDir)
	tempDir, err := utils.GetRelativePath(rootFs, absTempDir)
	if err != nil {
		return fmt.Errorf("Encountered error while getting relative path for %s in %s: %s", absTempDir, rootFs.Root(), err)
	}
	tempFs := utils.GetFilesystem(absTempDir)
	err = utils.WalkDir(rootFs, templateDir, func(fs billy.Filesystem, path string, isDir bool) error {
		repoPath, err := utils.MovePath(path, templateDir, "")
		if err != nil {
			return fmt.Errorf("Unable to get path of %s within %s: %s", repoPath, templateDir, err)
		}
		if exists, ok := templateFileMap[repoPath]; !ok || !exists {
			return nil
		}
		f, err := utils.CreateFileAndDirs(tempFs, repoPath)
		if err != nil {
			return err
		}
		defer f.Close()
		t := template.Must(template.New(filepath.Base(path)).ParseFiles(utils.GetAbsPath(rootFs, path)))
		if err := t.Execute(f, chartsScriptOptions); err != nil {
			return fmt.Errorf("Error while executing Go template for %s: %s", path, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return utils.CopyDir(rootFs, tempDir, ".")
}
