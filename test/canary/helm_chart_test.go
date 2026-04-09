package canary_test

import (
	"os"
	"path/filepath"
	"testing"

	helmChart "helm.sh/helm/v3/pkg/chart"
	helmChartUtil "helm.sh/helm/v3/pkg/chartutil"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// TestHelmChartCompat is a canary for helm.sh/helm/v3/pkg/chart.
//
// Used in: pkg/charts/dependencies.go, pkg/charts/icons.go, pkg/helm/charts.go,
// pkg/helm/export.go, pkg/helm/metadata.go, pkg/helm/standardize.go.
//
// Pins: APIVersionV2 constant, Metadata struct fields (Name, Version, APIVersion,
// Icon), Dependency struct, Chart struct.
func TestHelmChartCompat(t *testing.T) {
	t.Run("APIVersionV2 constant value", func(t *testing.T) {
		// pkg/helm/metadata.go uses helmChart.APIVersionV2 when building chart metadata.
		if helmChart.APIVersionV2 != "v2" {
			t.Errorf("APIVersionV2: got %q, want %q", helmChart.APIVersionV2, "v2")
		}
	})

	t.Run("Metadata struct fields used in production code", func(t *testing.T) {
		m := &helmChart.Metadata{
			Name:       "rancher-monitoring",
			Version:    "108.0.0+up0.9.0",
			APIVersion: helmChart.APIVersionV2,
			Icon:       "file://assets/logos/rancher-monitoring.svg",
		}
		if m.Name != "rancher-monitoring" {
			t.Errorf("Name: got %q", m.Name)
		}
		if m.Version != "108.0.0+up0.9.0" {
			t.Errorf("Version: got %q", m.Version)
		}
		if m.APIVersion != "v2" {
			t.Errorf("APIVersion: got %q", m.APIVersion)
		}
		if m.Icon != "file://assets/logos/rancher-monitoring.svg" {
			t.Errorf("Icon: got %q", m.Icon)
		}
	})

	t.Run("Dependency struct accessible", func(t *testing.T) {
		// Used in pkg/charts/dependencies.go for chart dependency resolution.
		dep := &helmChart.Dependency{
			Name:       "fleet",
			Version:    ">=0.14.0",
			Repository: "https://charts.rancher.io",
		}
		if dep.Name != "fleet" {
			t.Errorf("Dependency.Name: got %q", dep.Name)
		}
		if dep.Version != ">=0.14.0" {
			t.Errorf("Dependency.Version: got %q", dep.Version)
		}
		if dep.Repository != "https://charts.rancher.io" {
			t.Errorf("Dependency.Repository: got %q", dep.Repository)
		}
	})
}

// TestHelmLoaderCompat is a canary for helm.sh/helm/v3/pkg/chart/loader.
//
// Used in: pkg/helm/crds.go, pkg/helm/export.go, pkg/helm/metadata.go,
// pkg/helm/standardize.go, pkg/helm/zip.go, pkg/charts/icons.go,
// pkg/validate/validate.go.
//
// Pins: Load() — loads a chart from a directory path and returns *chart.Chart
// with accessible Metadata.Name, Metadata.Version, and CRDObjects().
func TestHelmLoaderCompat(t *testing.T) {
	t.Run("Load reads chart from directory and exposes Metadata", func(t *testing.T) {
		// Build a minimal chart directory — the same structure our chart pipeline produces.
		dir := t.TempDir()
		chartYAML := `apiVersion: v2
name: rancher-webhook
version: 108.0.0+up0.9.0
description: Rancher Webhook
type: application
`
		if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0644); err != nil {
			t.Fatalf("write Chart.yaml: %v", err)
		}

		chart, err := helmLoader.Load(dir)
		if err != nil {
			t.Fatalf("helmLoader.Load: %v", err)
		}
		if chart.Metadata.Name != "rancher-webhook" {
			t.Errorf("Metadata.Name: got %q, want %q", chart.Metadata.Name, "rancher-webhook")
		}
		if chart.Metadata.Version != "108.0.0+up0.9.0" {
			t.Errorf("Metadata.Version: got %q, want %q", chart.Metadata.Version, "108.0.0+up0.9.0")
		}
	})

	t.Run("Load exposes CRDObjects on chart with CRDs", func(t *testing.T) {
		// Mirrors pkg/helm/crds.go: chart.CRDObjects()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(`apiVersion: v2
name: rancher-compliance
version: 108.0.0+up1.0.0
type: application
`), 0644); err != nil {
			t.Fatalf("write Chart.yaml: %v", err)
		}

		crdsDir := filepath.Join(dir, "crds")
		if err := os.MkdirAll(crdsDir, 0755); err != nil {
			t.Fatalf("mkdir crds: %v", err)
		}
		if err := os.WriteFile(filepath.Join(crdsDir, "test.yaml"), []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: tests.example.com
`), 0644); err != nil {
			t.Fatalf("write crd: %v", err)
		}

		chart, err := helmLoader.Load(dir)
		if err != nil {
			t.Fatalf("helmLoader.Load: %v", err)
		}
		if len(chart.CRDObjects()) == 0 {
			t.Error("expected at least one CRD object, got none")
		}
	})
}

// TestHelmChartUtilCompat is a canary for helm.sh/helm/v3/pkg/chartutil.
//
// Used in: pkg/helm/charts.go, pkg/helm/metadata.go, pkg/charts/icons.go.
//
// Pins: LoadChartfile() and SaveChartfile() — the read/write pair used to
// mutate Chart.yaml on disk (e.g. updating chart name, icon path).
func TestHelmChartUtilCompat(t *testing.T) {
	t.Run("LoadChartfile reads real Chart.yaml fixture", func(t *testing.T) {
		// Fixture: test/canary/testdata/test.chart.yaml — real fleet Chart.yaml
		// (fleet 109.0.1+up0.15.1-beta.2).
		meta, err := helmChartUtil.LoadChartfile(fixturePath("test.chart.yaml"))
		if err != nil {
			t.Fatalf("LoadChartfile: %v", err)
		}
		if meta.Name != "fleet" {
			t.Errorf("Name: got %q, want %q", meta.Name, "fleet")
		}
		if meta.Version != "109.0.1+up0.15.1-beta.2" {
			t.Errorf("Version: got %q, want %q", meta.Version, "109.0.1+up0.15.1-beta.2")
		}
		if meta.APIVersion != helmChart.APIVersionV2 {
			t.Errorf("APIVersion: got %q, want %q", meta.APIVersion, helmChart.APIVersionV2)
		}
	})

	t.Run("SaveChartfile round-trips metadata loaded from real fixture", func(t *testing.T) {
		// Load real fixture, write it out, reload — verifies the write path
		// used in pkg/helm/metadata.go does not corrupt data.
		meta, err := helmChartUtil.LoadChartfile(fixturePath("test.chart.yaml"))
		if err != nil {
			t.Fatalf("LoadChartfile fixture: %v", err)
		}

		out := filepath.Join(t.TempDir(), "Chart.yaml")
		if err := helmChartUtil.SaveChartfile(out, meta); err != nil {
			t.Fatalf("SaveChartfile: %v", err)
		}

		reloaded, err := helmChartUtil.LoadChartfile(out)
		if err != nil {
			t.Fatalf("LoadChartfile after save: %v", err)
		}
		if reloaded.Name != meta.Name {
			t.Errorf("Name: got %q, want %q", reloaded.Name, meta.Name)
		}
		if reloaded.Version != meta.Version {
			t.Errorf("Version: got %q, want %q", reloaded.Version, meta.Version)
		}
	})
}
