package update

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"gopkg.in/yaml.v2"
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
	// ChartsBuildScriptRepositoryTemplateUpdateOptions is the file within templates that has the update options
	ChartsBuildScriptRepositoryTemplateUpdateOptions = "update.yaml"
)

// ApplyUpstreamTemplate updates a charts-build-scripts repository based on the Go templates tracked in this repository
func ApplyUpstreamTemplate(rootFs billy.Filesystem, chartsScriptOptions options.ChartsScriptOptions) error {
	templateRepository, err := puller.GetGithubRepository(options.UpstreamOptions{
		URL:          ChartsBuildScriptsRepositoryURL,
		Subdirectory: &ChartsBuildScriptRepositoryTemplatesDirectory,
	}, &ChartsBuildScriptsRepositoryBranch)
	if err != nil {
		return fmt.Errorf("Unable to get the charts build script repository: %s", err)
	}
	absTempDir, err := ioutil.TempDir(rootFs.Root(), "templates")
	if err != nil {
		return fmt.Errorf("Encountered error while trying to create temporary directory: %s", err)
	}
	defer os.RemoveAll(absTempDir)
	tempDir, err := filesystem.GetRelativePath(rootFs, absTempDir)
	if err != nil {
		return fmt.Errorf("Encounterede error while trying to get the relative path to %s: %s", absTempDir, err)
	}
	if err := templateRepository.Pull(rootFs, rootFs, tempDir); err != nil {
		return fmt.Errorf("Unable to pull the charts build script repository: %s", err)
	}
	absUpdateOptionsFilepath := filepath.Join(absTempDir, ChartsBuildScriptRepositoryTemplateUpdateOptions)
	updateOptionsFile, err := ioutil.ReadFile(absUpdateOptionsFilepath)
	if err != nil {
		return fmt.Errorf("Unable to find update.yaml: %s", err)
	}
	var updateOptions Options
	if err := yaml.UnmarshalStrict(updateOptionsFile, &updateOptions); err != nil {
		return fmt.Errorf("Encountered error while trying to unmarshall update.yaml: %s", err)
	}
	return updateOptions.CopyTemplate(rootFs, chartsScriptOptions, filepath.Join(tempDir, ChartsBuildScriptRepositoryTemplateDirectory))
}
