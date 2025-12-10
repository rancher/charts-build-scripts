package registries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"

	imagecopy "github.com/rancherlabs/slsactl/pkg/imagecopy"
)

// Sync will load the sync yaml files and iterate through each image/tags copying and pushing without overwriting anything.
// There can be 2 sources:
//   - Docker Hub
//   - Staging Registry
//
// There is only one destination:
//   - Prime Registry
func Sync(ctx context.Context, primeURL, customPath string) error {
	if err := checkCredentials(primeURL); err != nil {
		return err
	}

	// rc non-standard release process
	if customPath != "" {
		customImgTags, err := loadSyncYamlFile(ctx, "config/customToPrime.yaml")
		if err != nil {
			return err
		}
		return batchSync(ctx, StagingURL, primeURL, customPath, customImgTags)
	}

	stagingImageTags, err := loadSyncYamlFile(ctx, path.StagingToPrimeSync)
	if err != nil {
		return err
	}

	dockerImageTags, err := loadSyncYamlFile(ctx, path.DockerToPrimeSync)
	if err != nil {
		return err
	}

	// Staging to Prime Registry
	if err := batchSync(ctx, StagingURL, primeURL, "", stagingImageTags); err != nil {
		return err
	}

	// Docker to Prime Registry
	if err := batchSync(ctx, DockerURL, primeURL, "", dockerImageTags); err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "sync process complete")
	return nil
}

func checkCredentials(primeURL string) error {
	registry, err := name.NewRegistry(primeURL)
	if err != nil {
		return err
	}

	auth, err := authn.DefaultKeychain.Resolve(registry)
	if err != nil {
		return fmt.Errorf("failed to resolve prime registry credentials: %w", err)
	}

	if auth == authn.Anonymous {
		return errors.New("no credentials found for prime registry")
	}
	return nil
}

// loadSyncYamlFile will load a given sync registry yaml file located at config/ dir
func loadSyncYamlFile(ctx context.Context, path string) (map[string][]string, error) {
	yamlData, err := filesystem.LoadYamlFile[map[string][]string](ctx, path, true)
	if err != nil {
		return nil, err
	}

	if yamlData == nil {
		return map[string][]string{}, nil
	}

	return sanitizeTags(*yamlData), nil
}

func batchSync(ctx context.Context, sourceURL, primeURL, customPath string, imgTags map[string][]string) error {
	logger.Log(ctx, slog.LevelInfo,
		"syncing...", slog.String("source", sourceURL), slog.Any("imgTags", imgTags))

	if len(imgTags) == 0 {
		logger.Log(ctx, slog.LevelInfo, "nothing to sync")
		return nil
	}

	for repoImg, tags := range imgTags {
		for _, tag := range tags {
			var dstRef string
			if customPath != "" {
				dstRef = primeURL + "/" + customPath + "/" + strings.TrimPrefix(repoImg, "rancher/") + ":" + tag
			} else {
				dstRef = primeURL + "/" + repoImg + ":" + tag
			}

			srcRef := sourceURL + repoImg + ":" + tag

			if err := imagecopy.ImageAndSignature(srcRef, dstRef); err != nil {
				return err
			}
		}
	}

	return nil
}
