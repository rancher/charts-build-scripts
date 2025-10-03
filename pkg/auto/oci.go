package auto

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

type loadAssetFunc func(chart, asset string) ([]byte, error)
type checkAssetFunc func(ctx context.Context, regClient *registry.Client, ociDNS, chart, version string) (bool, error)
type pushFunc func(helmClient *registry.Client, data []byte, url string) error

type oci struct {
	DNS        string
	user       string
	password   string
	helmClient *registry.Client
	loadAsset  loadAssetFunc
	checkAsset checkAssetFunc
	push       pushFunc
}

// UpdateOCI pushes Helm charts to an OCI registry
func UpdateOCI(ctx context.Context, rootFs billy.Filesystem, ociDNS, ociUser, ociPass string, debug bool) error {
	release, err := options.LoadReleaseOptionsFromFile(ctx, rootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	oci, err := setupOCI(ctx, ociDNS, ociUser, ociPass, debug)
	if err != nil {
		return err
	}

	pushedAssets, err := oci.update(ctx, &release)
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "pushed", slog.Any("assets", pushedAssets))
	return nil
}

func setupOCI(ctx context.Context, ociDNS, ociUser, ociPass string, debug bool) (*oci, error) {
	var err error
	o := &oci{
		DNS:      ociDNS,
		user:     ociUser,
		password: ociPass,
	}

	o.helmClient, err = setupHelm(ctx, o.DNS, o.user, o.password, debug)
	if err != nil {
		return nil, err
	}

	o.loadAsset = loadAsset
	o.checkAsset = checkAsset
	o.push = push

	return o, nil
}

func setupHelm(ctx context.Context, ociDNS, ociUser, ociPass string, debug bool) (*registry.Client, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v...)
	}); err != nil {
		return nil, err
	}

	var regClient *registry.Client
	var err error

	registryHost := extractRegistryHost(ociDNS)
	isLocalHost := strings.HasPrefix(registryHost, "localhost:")

	switch {
	// Debug Mode but pointing to a server with custom-certificates
	case debug && !isLocalHost:
		logger.Log(ctx, slog.LevelDebug, "debug mode", slog.Bool("localhost", isLocalHost))
		caFile := "/etc/docker/certs.d/" + registryHost + "/ca.crt"
		regClient, err = registry.NewRegistryClientWithTLS(os.Stdout, "", "", caFile, false, "", true)
		if err != nil {
			logger.Log(ctx, slog.LevelError, "failed to create registry client with TLS")
			return nil, err
		}
		if err = regClient.Login(
			registryHost,
			registry.LoginOptInsecure(false),
			registry.LoginOptTLSClientConfig("", "", caFile),
			registry.LoginOptBasicAuth(ociUser, ociPass),
		); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to login to registry with TLS", slog.Group(ociDNS, ociUser, ociPass))
			return nil, err
		}

	// Debug Mode at localhost without TLS
	case debug && isLocalHost:
		logger.Log(ctx, slog.LevelDebug, "debug mode", slog.Bool("localhost", isLocalHost))
		regClient, err = registry.NewClient(
			registry.ClientOptDebug(true),
			registry.ClientOptPlainHTTP(),
		)
		if err != nil {
			logger.Log(ctx, slog.LevelError, "failed to create registry client")
			return nil, err
		}
		if err = regClient.Login(registryHost,
			registry.LoginOptInsecure(true), // true for localhost, false for production
			registry.LoginOptBasicAuth(ociUser, ociPass)); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to login to registry", slog.Group(ociDNS, ociUser, ociPass))
			return nil, err
		}

	// Production code with Secure Mode and authentication
	default:
		regClient, err = registry.NewClient(registry.ClientOptDebug(false))
		if err != nil {
			return nil, err
		}
		if err = regClient.Login(registryHost,
			registry.LoginOptInsecure(false),
			registry.LoginOptBasicAuth(ociUser, ociPass)); err != nil {
			return nil, err
		}
	}

	return regClient, nil
}

// extractRegistryHost will extract the DNS for login
func extractRegistryHost(ociDNS string) string {
	if idx := strings.Index(ociDNS, "/"); idx != -1 {
		return ociDNS[:idx]
	}
	return ociDNS
}

// update will attempt to update a helm chart to an OCI registry.
// 2 phases:
//   - 1: Pre-Flight validations (check the current chart + check if it already exists)
//   - 2: Push
func (o *oci) update(ctx context.Context, release *options.ReleaseOptions) ([]string, error) {
	var pushedAssets []string

	// List of assets to process
	type assetInfo struct {
		chart   string
		version string
		asset   string
		data    []byte
	}
	var assetsToProcess []assetInfo

	// Phase 1: Pre-Flight Validations
	logger.Log(ctx, slog.LevelDebug, "Phase 1: Pre-Flight Validations")
	for chart, versions := range *release {
		for _, version := range versions {
			asset := chart + "-" + version + ".tgz"
			assetData, err := o.loadAsset(chart, asset)
			if err != nil {
				logger.Log(ctx, slog.LevelError, "failed to load asset", slog.String("asset", asset))
				return pushedAssets, err
			}

			// Check if the asset version already exists in the OCI registry
			// Never overwrite a previously released chart!
			exists, err := o.checkAsset(ctx, o.helmClient, o.DNS, chart, version)
			if err != nil {
				logger.Log(ctx, slog.LevelError, "failed to check registry for asset", slog.String("asset", asset))
				return pushedAssets, err
			}
			if exists {
				// Skip existing charts instead of failing
				logger.Log(ctx, slog.LevelWarn, "chart already exists in registry, will skip",
					slog.String("asset", asset))
				continue
			}

			logger.Log(ctx, slog.LevelDebug, "asset valid and doesn't exist in the registry already", slog.String("asset", asset))
			assetsToProcess = append(assetsToProcess, assetInfo{
				chart:   chart,
				version: version,
				asset:   asset,
				data:    assetData,
			})
		}
	}

	// check if there is anything to push
	if len(assetsToProcess) == 0 {
		logger.Log(ctx, slog.LevelInfo, "no new charts to push - all charts already exist in registry")
		return pushedAssets, nil
	}

	// Phase 2
	var pushErrors []error
	logger.Log(ctx, slog.LevelInfo, "Phase 2: Push")
	for _, info := range assetsToProcess {
		logger.Log(ctx, slog.LevelDebug, "pushing", slog.String("asset", info.asset))

		if err := o.push(o.helmClient, info.data, buildPushURL(o.DNS, info.chart, info.version)); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to push asset", slog.String("asset", info.asset))
			pushErrors = append(pushErrors, errors.New("asset: "+info.asset+" error: "+err.Error()))
			continue
		}
		pushedAssets = append(pushedAssets, info.asset)
		logger.Log(ctx, slog.LevelInfo, "pushed", slog.String("asset", info.asset))
	}

	if len(pushErrors) > 0 {
		logger.Log(ctx, slog.LevelError, "push phase completed with errors",
			slog.Int("successful", len(pushedAssets)),
			slog.Int("failed", len(pushErrors)))
		for _, err := range pushErrors {
			logger.Err(err)
		}
		return pushedAssets, errors.New("some assets failed, please fix and retry only these assets")
	}

	return pushedAssets, nil
}

func push(helmClient *registry.Client, data []byte, url string) error {
	if _, err := helmClient.Push(data, url, registry.PushOptStrictMode(true)); err != nil {
		return err
	}
	return nil
}

func loadAsset(chart, asset string) ([]byte, error) {
	return os.ReadFile(path.RepositoryAssetsDir + "/" + chart + "/" + asset)
}

// oci://<oci-dns>/<chart(repository)>:<version>
func buildPushURL(ociDNS, chart, version string) string {
	return ociDNS + "/" + chart + ":" + version
}

// checkAsset checks if a specific asset version exists in the OCI registry
func checkAsset(ctx context.Context, helmClient *registry.Client, ociDNS, chart, version string) (bool, error) {
	// Once issue is resolved: https://github.com/helm/helm/issues/13368
	// Replace by: helmClient.Tags(ociDNS + "/" + chart + ":" + version)
	existingVersions, err := helmClient.Tags(ociDNS + "/" + chart)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected status code 404: name unknown: repository name not known to registry") {
			logger.Log(ctx, slog.LevelDebug, "asset does not exist at registry", slog.String("chart", chart))
			return false, nil
		}
		logger.Err(err)
		return false, err
	}

	for _, existingVersion := range existingVersions {
		if existingVersion == version {
			return true, nil
		}
	}

	return false, nil
}
