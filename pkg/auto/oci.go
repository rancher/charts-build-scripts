package auto

import (
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/path"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

func OCI(rootFs billy.Filesystem) error {
	releaseOptions, err := options.LoadReleaseOptionsFromFile(rootFs, path.RepositoryReleaseYaml)
	if err != nil {
		return err
	}

	if err := pushOCI(&releaseOptions); err != nil {
		return err
	}

	return nil
}

func helmRegistryClient() (*registry.Client, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v...)
	}); err != nil {
		return nil, err
	}

	ociURL := "ec2-18-119-135-219.us-east-2.compute.amazonaws.com"

	// Path to the certificate file
	certFilePath := "/etc/docker/certs.d/" + ociURL + "/ca.crt"
	// Set the SSL_CERT_FILE environment variable to use the custom certificate
	os.Setenv("SSL_CERT_FILE", certFilePath)

	// registry.NewClientWithTLS(ociURL, certFilePath, "", "")

	// Create a new registry client
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(true),
		registry.ClientOptEnableCache(false),
		registry.ClientOptWriter(os.Stdout),
	)
	if err != nil {
		return nil, err
	}

	// Login to the registry
	if err := registryClient.Login(ociURL, registry.LoginOptInsecure(false), registry.LoginOptBasicAuth("nick", "123456")); err != nil {
		return nil, err
	}

	return registryClient, nil
}

func pushOCI(releaseOptions *options.ReleaseOptions) error {
	registryClient, err := helmRegistryClient()
	if err != nil {
		return err
	}

	for chart, versions := range *releaseOptions {
		for _, version := range versions {
			chartPath := path.RepositoryAssetsDir + "/" + chart + "/" + chart + "-" + version + ".tgz"
			// Read the chart file into a byte slice
			chartData, err := os.ReadFile(chartPath)
			if err != nil {
				return err
			}

			repositoryURL := "localhost/" + chart + ":" + version

			// Push the chart to the OCI registry
			result, err := registryClient.Push(chartData, repositoryURL)
			if err != nil {
				return err
			}
			fmt.Println(result)
		}
	}

	// fmt.Printf("Successfully pushed chart %s to %s\n", chartPath, registryURL)
	return nil
}
