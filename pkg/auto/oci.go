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

// loadAssetFunc reads a packaged chart archive (.tgz) from the local assets directory.
type loadAssetFunc func(chart, asset string) ([]byte, error)

// checkAssetFunc reports whether a specific chart version already exists in the OCI registry.
type checkAssetFunc func(ctx context.Context, regClient *registry.Client, ociDNS, customPath, chart, version string) (bool, error)

// pushFunc uploads a packaged chart archive to the OCI registry at the given URL.
type pushFunc func(helmClient *registry.Client, data []byte, url string) error

// oci holds all state required for a single push session against an OCI registry.
// The three function fields (loadAsset, checkAsset, push) are injectable for testing.
type oci struct {
	DNS        string // registry hostname, stripped of http/https scheme
	CustomPath string // optional registry path override; defaults to rancher/charts
	user       string
	password   string
	helmClient *registry.Client
	loadAsset  loadAssetFunc
	checkAsset checkAssetFunc
	push       pushFunc
}

// PushChartToOCI reads release.yaml and pushes all listed chart versions to an OCI registry.
// It validates credentials, sets up an authenticated Helm registry client, then runs a
// two-phase check-and-push: pre-flight validation first, then push of only the new assets.
// Existing versions in the registry are skipped rather than overwritten.
func PushChartToOCI(ctx context.Context, rootFs billy.Filesystem, ociDNS, customPath, ociUser, ociPass string, debug bool) error {

	if customPath != "" {
		logger.Log(ctx, slog.LevelDebug, "custom override path", slog.String("path", customPath))
	}

	emptyUser := ociUser == ""
	emptyPass := ociPass == ""
	emptyDNS := ociDNS == ""
	if ociDNS == "" || ociUser == "" || ociPass == "" {
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("OCI User Empty", emptyUser))
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("OCI Password Empty", emptyPass))
		logger.Log(ctx, slog.LevelError, "missing credential", slog.Bool("OCI DNS Empty", emptyDNS))
		return errors.New("no credentials provided for pushing helm chart to OCI registry")
	}

	release, err := options.LoadReleaseOptionsFromFile(ctx, rootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	oci, err := setupOCI(ctx, ociDNS, customPath, ociUser, ociPass, debug)
	if err != nil {
		return err
	}

	pushedAssets, err := oci.checkAndPush(ctx, &release)
	if err != nil {
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "pushed", slog.Any("assets", pushedAssets))
	return nil
}

// setupOCI constructs an oci instance with an authenticated Helm registry client
// and wires the real loadAsset, checkAsset, and push implementations.
func setupOCI(ctx context.Context, ociDNS, customPath, ociUser, ociPass string, debug bool) (*oci, error) {
	// Strip http:// or https:// scheme if present
	ociDNS = strings.TrimPrefix(ociDNS, "https://")
	ociDNS = strings.TrimPrefix(ociDNS, "http://")

	var err error
	o := &oci{
		DNS:        ociDNS,
		CustomPath: customPath,
		user:       ociUser,
		password:   ociPass,
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

// setupHelm creates and authenticates a Helm registry client against the given OCI registry.
// Three modes are supported based on the debug flag and whether the target is localhost:
//   - debug + remote host: TLS client using CA from /etc/docker/certs.d/<dns>/ca.crt
//   - debug + localhost:   plain HTTP client (insecure login, no TLS)
//   - production (default): standard HTTPS client with basic auth
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

	isLocalHost := strings.HasPrefix(ociDNS, "localhost:")

	switch {
	// Debug Mode but pointing to a server with custom-certificates
	case debug && !isLocalHost:
		logger.Log(ctx, slog.LevelDebug, "debug mode", slog.Bool("localhost", isLocalHost))
		caFile := "/etc/docker/certs.d/" + ociDNS + "/ca.crt"
		regClient, err = registry.NewRegistryClientWithTLS(os.Stdout, "", "", caFile, false, "", true)
		if err != nil {
			logger.Log(ctx, slog.LevelError, "failed to create registry client with TLS")
			return nil, err
		}
		if err = regClient.Login(
			ociDNS,
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
		if err = regClient.Login(ociDNS,
			registry.LoginOptInsecure(true), // true for localhost, false for production
			registry.LoginOptBasicAuth(ociUser, ociPass)); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to login to registry", slog.Group(ociDNS, ociUser, ociPass))
			return nil, err
		}

	// Production code with Secure Mode and authentication
	default:
		logger.Log(ctx, slog.LevelInfo, "production mode")
		regClient, err = registry.NewClient(
			registry.ClientOptDebug(false),
		)
		if err != nil {
			logger.Log(ctx, slog.LevelError, "failed to create registry client")
			return nil, err
		}
		if err = regClient.Login(ociDNS,
			registry.LoginOptInsecure(false),
			registry.LoginOptBasicAuth(ociUser, ociPass)); err != nil {
			logger.Log(ctx, slog.LevelError, "failed to login")
			return nil, err
		}
		logger.Log(ctx, slog.LevelDebug, "creds", slog.String("u", ociUser), slog.String("p", ociPass))
	}

	return regClient, nil
}

// checkAndPush runs a two-phase push of all chart versions listed in release.yaml.
//
// Phase 1 — Pre-flight: for each chart/version, loads the .tgz from assets/ and checks
// whether it already exists in the registry. Existing versions are skipped. Only new
// assets are queued for Phase 2.
//
// Phase 2 — Push: uploads each queued asset. Errors are accumulated rather than
// short-circuiting, so a single failure does not abort the remaining pushes.
// If any pushes fail, the returned error lists the failed assets.
func (o *oci) checkAndPush(ctx context.Context, release *options.ReleaseOptions) ([]string, error) {
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
			exists, err := o.checkAsset(ctx, o.helmClient, o.DNS, o.CustomPath, chart, version)
			if err != nil {
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

		if err := o.push(o.helmClient, info.data, buildPushURL(o.DNS, o.CustomPath, info.chart, info.version)); err != nil {
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

// push uploads a packaged chart archive to the OCI registry at the given URL.
// StrictMode ensures the artifact is validated as a proper OCI Helm chart on arrival.
func push(helmClient *registry.Client, data []byte, url string) error {
	if _, err := helmClient.Push(data, url, registry.PushOptStrictMode(true)); err != nil {
		return err
	}
	return nil
}

// loadAsset reads a packaged chart archive from assets/<chart>/<asset>.
func loadAsset(chart, asset string) ([]byte, error) {
	return os.ReadFile(path.RepositoryAssetsDir + "/" + chart + "/" + asset)
}

// buildPushURL constructs the OCI push target URL for a chart version.
// Format: <dns>/<customPath>/<chart>:<version>  (custom path)
//
//	or <dns>/rancher/charts/<chart>:<version>  (default)
func buildPushURL(ociDNS, customPath, chart, version string) string {
	if customPath != "" {
		return ociDNS + "/" + customPath + "/" + chart + ":" + version
	}
	return ociDNS + "/rancher/charts/" + chart + ":" + version
}

// buildTagsURL constructs the OCI tags listing URL for a chart repository (no version suffix).
// Used by checkAsset to enumerate existing tags via helmClient.Tags().
func buildTagsURL(ociDNS, customPath, chart, version string) string {
	if customPath != "" {
		return ociDNS + "/" + customPath + "/" + chart
	}
	return ociDNS + "/rancher/charts/" + chart
}

// checkAsset reports whether a specific chart version already exists in the OCI registry.
// It lists all tags for the chart repository and checks for an exact version match.
// A 404 response is treated as "not found" (not an error) — the chart simply hasn't been pushed yet.
//
// NOTE: helmClient.Tags() is used as a workaround because direct tag existence checks
// are not yet supported. Track: https://github.com/helm/helm/issues/13368
func checkAsset(ctx context.Context, helmClient *registry.Client, ociDNS, customPath, chart, version string) (bool, error) {
	tagsURL := buildTagsURL(ociDNS, customPath, chart, version)
	existingVersions, err := helmClient.Tags(tagsURL)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			logger.Log(ctx, slog.LevelDebug, "asset does not exist at registry", slog.String("chart", chart))
			return false, nil
		}
		logger.Log(ctx, slog.LevelError, "failed to check registry for asset", slog.String("asset", chart))
		return false, err
	}

	for _, existingVersion := range existingVersions {
		if existingVersion == version {
			return true, nil
		}
	}

	return false, nil
}
