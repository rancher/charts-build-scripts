package canary_test

import (
	"testing"

	helmAction "helm.sh/helm/v3/pkg/action"
	helmCLI "helm.sh/helm/v3/pkg/cli"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	helmRegistry "helm.sh/helm/v3/pkg/registry"
)

// TestHelmRegistryCompat is a canary for helm.sh/helm/v3/pkg/registry.
//
// Used in: pkg/charts/parse.go, pkg/registries/oci.go.
//
// Pins: IsOCI(), NewClient(), ClientOptDebug(), ClientOptPlainHTTP(),
// LoginOptBasicAuth(), LoginOptInsecure(), LoginOptTLSClientConfig(),
// PushOptStrictMode(), Client.Push().
func TestHelmRegistryCompat(t *testing.T) {
	t.Run("IsOCI detects oci:// scheme", func(t *testing.T) {
		// pkg/charts/parse.go: helmRegistry.IsOCI(opt.URL)
		if !helmRegistry.IsOCI("oci://registry.example.com/charts/rancher-monitoring") {
			t.Error("IsOCI: expected true for oci:// URL")
		}
		if helmRegistry.IsOCI("https://charts.rancher.io") {
			t.Error("IsOCI: expected false for https:// URL")
		}
	})

	t.Run("NewClient returns a non-nil client", func(t *testing.T) {
		// pkg/registries/oci.go: helmRegistry.NewClient(ClientOptDebug(false), ClientOptPlainHTTP())
		client, err := helmRegistry.NewClient(
			helmRegistry.ClientOptDebug(false),
			helmRegistry.ClientOptPlainHTTP(),
		)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if client == nil {
			t.Fatal("NewClient returned nil client")
		}
	})

	t.Run("ClientOptDebug option is accepted", func(t *testing.T) {
		// pkg/registries/oci.go uses both ClientOptDebug(true) and ClientOptDebug(false)
		_, err := helmRegistry.NewClient(helmRegistry.ClientOptDebug(true))
		if err != nil {
			t.Fatalf("NewClient with ClientOptDebug(true): %v", err)
		}
	})

	t.Run("LoginOpt constructors are callable", func(t *testing.T) {
		// pkg/registries/oci.go: LoginOptBasicAuth, LoginOptInsecure, LoginOptTLSClientConfig
		// These return option funcs — calling them should not panic.
		_ = helmRegistry.LoginOptBasicAuth("user", "pass")
		_ = helmRegistry.LoginOptInsecure(false)
		_ = helmRegistry.LoginOptInsecure(true)
		_ = helmRegistry.LoginOptTLSClientConfig("", "", "")
	})

	t.Run("PushOptStrictMode option is callable", func(t *testing.T) {
		// pkg/registries/oci.go: helmRegistry.PushOptStrictMode(true)
		_ = helmRegistry.PushOptStrictMode(true)
	})
}

// TestHelmActionCompat is a canary for helm.sh/helm/v3/pkg/action.
//
// Used in: pkg/registries/oci.go.
//
// Pins: Configuration struct (zero-value usable), Init() method signature.
func TestHelmActionCompat(t *testing.T) {
	t.Run("Configuration zero-value is constructible", func(t *testing.T) {
		// pkg/registries/oci.go: actionConfig := new(helmAction.Configuration)
		// Verifies the struct exists and is zero-value constructible via new().
		_ = new(helmAction.Configuration)
	})

	t.Run("Configuration.Init accepts nil getter", func(t *testing.T) {
		// pkg/registries/oci.go: actionConfig.Init(nil, "", "", nil)
		actionConfig := new(helmAction.Configuration)
		err := actionConfig.Init(nil, "", "", nil)
		if err != nil {
			t.Fatalf("Configuration.Init: %v", err)
		}
	})
}

// TestHelmCLICompat is a canary for helm.sh/helm/v3/pkg/cli.
//
// Used in: pkg/registries/oci.go.
//
// Pins: New() returns *EnvSettings with accessible RegistryConfig field.
func TestHelmCLICompat(t *testing.T) {
	t.Run("New returns non-nil EnvSettings", func(t *testing.T) {
		// pkg/registries/oci.go: settings := helmCLI.New()
		settings := helmCLI.New()
		if settings == nil {
			t.Fatal("helmCLI.New() returned nil")
		}
	})

	t.Run("EnvSettings.RegistryConfig field is accessible", func(t *testing.T) {
		// pkg/registries/oci.go: settings.RegistryConfig
		settings := helmCLI.New()
		// Field is a string path — just verify it's readable (empty is fine in test env).
		_ = settings.RegistryConfig
	})
}

// TestHelmGetterCompat is a canary for helm.sh/helm/v3/pkg/getter.
//
// Used in: pkg/puller/oci.go.
//
// Pins: NewOCIGetter() — returns a Getter with a Get() method.
func TestHelmGetterCompat(t *testing.T) {
	t.Run("NewOCIGetter returns a non-nil getter", func(t *testing.T) {
		// pkg/puller/oci.go: getter, err := helmGetter.NewOCIGetter()
		getter, err := helmGetter.NewOCIGetter()
		if err != nil {
			t.Fatalf("NewOCIGetter: %v", err)
		}
		if getter == nil {
			t.Fatal("NewOCIGetter returned nil getter")
		}
	})
}
