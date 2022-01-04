package charts

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// ValidateInstallCRDContentsFmt is the format for the contents of ChartValidateInstallCRDFile
const ValidateInstallCRDContentsFmt = `#{{- if gt (len (lookup "rbac.authorization.k8s.io/v1" "ClusterRole" "" "")) 0 -}}
# {{- $found := dict -}}
%s
# {{- range .Capabilities.APIVersions -}}
# {{- if hasKey $found (toString .) -}}
# 	{{- set $found (toString .) true -}}
# {{- end -}}
# {{- end -}}
# {{- range $_, $exists := $found -}}
# {{- if (eq $exists false) -}}
# 	{{- required "Required CRDs are missing. Please install the corresponding CRD chart before installing this chart." "" -}}
# {{- end -}}
# {{- end -}}
#{{- end -}}`

// ValidateInstallCRDPerGVKFmt is the format that the GroupVersionKind of each CRD placed in ValidateInstallCRDContentsFmt needs to have
const ValidateInstallCRDPerGVKFmt = `# {{- set $found "%s" false -}}`

// GenerateCRDChartFromTemplate copies templateDir over to dstPath
func GenerateCRDChartFromTemplate(fs billy.Filesystem, dstHelmChartPath, templateDir, crdsDir string) error {
	exists, err := filesystem.PathExists(fs, templateDir)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("could not find directory for templates: %s", templateDir)
	}
	if err := fs.MkdirAll(filepath.Join(dstHelmChartPath, crdsDir), os.ModePerm); err != nil {
		return err
	}
	if err := filesystem.CopyDir(fs, templateDir, dstHelmChartPath); err != nil {
		return err
	}
	return nil
}

// AddCRDValidationToChart adds the validate-install-crd.yaml to helmChartPathWithoutCRDs based on CRDs located in crdsDir within helmChartPathWithCRDs
func AddCRDValidationToChart(fs billy.Filesystem, helmChartPathWithoutCRDs, helmChartPathWithCRDs, crdsDir string) error {
	// Get the CRDs
	logrus.Infof("Adding %s to main chart", path.ChartValidateInstallCRDFile)
	crdsDirpath := filepath.Join(helmChartPathWithCRDs, crdsDir)
	var crdGVKs []string
	type k8sCRDResource struct {
		APIVersion *string `yaml:"apiVersion,omitempty"`
		Spec       *struct {
			Group *string `yaml:"group,omitempty"`
			Names *struct {
				Kind *string `yaml:"kind,omitempty"`
			} `yaml:"names,omitempty"`
			Version  *string `yaml:"version,omitempty"`
			Versions *[]struct {
				Name *string `yaml:"name,omitempty"`
			} `yaml:"versions,omitempty"`
		} `yaml:"spec,omitempty"`
	}
	err := filesystem.WalkDir(fs, crdsDirpath, func(fs billy.Filesystem, path string, isDir bool) error {
		if isDir {
			return nil
		}
		absPath := filesystem.GetAbsPath(fs, path)
		yamlFile, err := ioutil.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("unable to read file %s: %s", absPath, err)
		}
		yamlDecoder := yaml.NewDecoder(bytes.NewReader(yamlFile))
		var resource k8sCRDResource
		for {
			err := yamlDecoder.Decode(&resource)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if resource.APIVersion == nil || resource.Spec == nil {
				continue
			}
			if !strings.HasPrefix(*resource.APIVersion, "apiextensions.k8s.io") {
				continue
			}
			spec := *resource.Spec
			if spec.Group == nil {
				continue
			}
			if spec.Names == nil || spec.Names.Kind == nil {
				continue
			}
			if spec.Version == nil && (spec.Versions == nil || len(*spec.Versions) == 0 || (*spec.Versions)[0].Name == nil) {
				continue
			}
			group := *spec.Group
			var version string
			if resource.Spec.Version != nil {
				version = *spec.Version
			} else {
				version = *(*spec.Versions)[0].Name
			}
			kind := *spec.Names.Kind
			crdGVKs = append(crdGVKs, fmt.Sprintf("%s/%s/%s", group, version, kind))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("encountered error while trying to read CRDs from %s: %s", crdsDirpath, err)
	}
	if len(crdGVKs) == 0 {
		return fmt.Errorf("unable to pull any GroupVersionKinds for CRDs from %s to construct %s", crdsDirpath, path.ChartValidateInstallCRDFile)
	}
	// Format them
	formattedCRDs := make([]string, len(crdGVKs))
	for i, grv := range crdGVKs {
		formattedCRDs[i] = fmt.Sprintf(ValidateInstallCRDPerGVKFmt, grv)
	}
	validateInstallCRDsContents := fmt.Sprintf(ValidateInstallCRDContentsFmt, strings.Join(formattedCRDs, "\n"))
	validateInstallCRDsDestpath := filepath.Join(helmChartPathWithoutCRDs, path.ChartValidateInstallCRDFile)
	// Write to file
	err = ioutil.WriteFile(filesystem.GetAbsPath(fs, validateInstallCRDsDestpath), []byte(validateInstallCRDsContents), os.ModePerm)
	if err != nil {
		return fmt.Errorf("encountered error while writing into %s: %s", validateInstallCRDsDestpath, err)
	}
	return nil
}

// RemoveCRDValidationFromChart removes the ChartValidateInstallCRDFile from a given chart
func RemoveCRDValidationFromChart(fs billy.Filesystem, helmChartPath string) error {
	if err := fs.Remove(filepath.Join(helmChartPath, path.ChartValidateInstallCRDFile)); err != nil {
		return err
	}
	return nil
}
