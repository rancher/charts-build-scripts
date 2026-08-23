package config

import (
	"context"
	"errors"

	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"go.yaml.in/yaml/v2"
)

// ImageVersionCheckOptions holds the list of images to check for version updates.
type ImageVersionCheckOptions map[string]ImageConfig

// ImageConfig describes a single image to validate.
type ImageConfig struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag,omitempty"` // optional
}

func LoadImageVersionList(ctx context.Context) (*ImageVersionCheckOptions, error) {
	// Open git repo
	repo, err := git.OpenGitRepo(ctx, ".")
	if err != nil {
		return nil, errors.New("load image-version-check open git repo: " + err.Error())
	}

	// Fetch latest automation-core branch
	if err := repo.FetchBranch(path.AutoCoreBranch); err != nil {
		return nil, errors.New("load image-version-check fetch branch: " + err.Error())
	}

	// Fetch image-version-check.yaml from automation-core branch
	data, err := repo.ShowFileFromRemoteBranch(ctx, path.AutoCoreBranch, path.ImageVersionCheckFile)
	if err != nil {
		return nil, errors.New("load image-version-check show: " + err.Error())
	}

	// Parse YAML directly into struct
	var opts ImageVersionCheckOptions
	if err := yaml.UnmarshalStrict(data, &opts); err != nil {
		return nil, errors.New("load image-version-check unmarshal: " + err.Error())
	}

	return &opts, nil
}
