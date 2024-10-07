package auto

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

type loadAssetFunc func(chart, asset string) ([]byte, error)
type checkAssetFunc func(regClient *registry.Client, ociDNS, chart, version string) (bool, error)
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
func UpdateOCI(rootFs billy.Filesystem, ociDNS, ociUser, ociPass string, debug bool) error {

	release, err := options.LoadReleaseOptionsFromFile(rootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	oci, err := setupOCI(ociDNS, ociUser, ociPass, debug)
	if err != nil {
		return err
	}

	pushedAssets, err := oci.update(&release)
	if err != nil {
		return err
	}

	fmt.Printf("Pushed assets: %v", pushedAssets)

	return nil
}

func setupOCI(ociDNS, ociUser, ociPass string, debug bool) (*oci, error) {
	var err error
	o := &oci{
		DNS:      ociDNS,
		user:     ociUser,
		password: ociPass,
	}

	o.helmClient, err = setupHelm(o.DNS, o.user, o.password, debug)
	if err != nil {
		return nil, err
	}

	o.loadAsset = loadAsset
	o.checkAsset = checkAsset
	o.push = push

	return o, nil
}

func setupHelm(ociDNS, ociUser, ociPass string, debug bool) (*registry.Client, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v...)
	}); err != nil {
		return nil, err
	}

	var regClient *registry.Client

	if debug {
		fmt.Println("debug mode you need to provide a self-signed certificate")
		caFile := "/etc/docker/certs.d/" + ociDNS + "/ca.crt"

		regClient, err := registry.NewRegistryClientWithTLS(os.Stdout, "", "", caFile, false, "", true)
		if err != nil {
			return nil, err
		}

		if err := regClient.Login(
			ociDNS,
			registry.LoginOptInsecure(false),
			registry.LoginOptTLSClientConfig("", "", caFile),
			registry.LoginOptBasicAuth(ociUser, ociPass),
		); err != nil {
			return nil, err
		}

		return regClient, nil
	}

	regClient, err := registry.NewClient(registry.ClientOptDebug(false))
	if err != nil {
		return nil, err
	}

	if err := regClient.Login(ociDNS,
		registry.LoginOptInsecure(false),
		registry.LoginOptBasicAuth(ociUser, ociPass)); err != nil {
		return nil, err
	}

	return regClient, nil
}

func (o *oci) update(release *options.ReleaseOptions) ([]string, error) {
	var pushedAssets []string

	for chart, versions := range *release {
		for _, version := range versions {
			asset := chart + "-" + version + ".tgz"
			assetData, err := o.loadAsset(chart, asset)
			if err != nil {
				fmt.Printf("Failed to load asset: %s; error: %s \n", asset, err.Error())
				return pushedAssets, err
			}

			// Check if the asset version already exists in the OCI registry
			// Never overwrite a previously released chart!
			exists, err := o.checkAsset(o.helmClient, o.DNS, chart, version)
			if err != nil {
				fmt.Printf("Failed to check registry for %s; error: %s \n", asset, err.Error())
				return pushedAssets, err
			}
			if exists {
				fmt.Printf("Asset %s already exists in the OCI registry \n", asset)
				return pushedAssets, fmt.Errorf("asset %s already exists in the OCI registry", asset)
			}

			fmt.Printf("Pushing asset to OCI Registry: %s \n", asset)
			if err := o.push(o.helmClient, assetData, buildPushURL(o.DNS, chart, version)); err != nil {
				err = fmt.Errorf("failed to push %s; error: %w ", asset, err)
				fmt.Printf(err.Error())
				return pushedAssets, err
			}
			pushedAssets = append(pushedAssets, asset)
		}
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
func checkAsset(helmClient *registry.Client, ociDNS, chart, version string) (bool, error) {
	// Once issue is resolved: https://github.com/helm/helm/issues/13368
	// Replace by: helmClient.Tags(ociDNS + "/" + chart + ":" + version)
	existingVersions, err := helmClient.Tags(ociDNS + "/" + chart)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected status code 404: name unknown: repository name not known to registry") {
			return false, nil
		}
		return false, err
	}

	for _, existingVersion := range existingVersions {
		if existingVersion == version {
			return true, nil
		}
	}

	return false, nil
}
