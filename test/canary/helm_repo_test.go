package canary_test

import (
	"testing"

	helmChart "helm.sh/helm/v3/pkg/chart"
	helmRepo "helm.sh/helm/v3/pkg/repo"
)

// TestHelmRepoCompat is a canary for helm.sh/helm/v3/pkg/repo.
//
// Used in: pkg/auto/remove.go, pkg/helm/helm.go, pkg/charts/dependencies.go,
// pkg/lifecycle/parser.go, pkg/validate/pull_requests.go.
//
// Pins: NewIndexFile(), IndexFile.Entries field, ChartVersions type,
// ChartVersion struct, LoadIndexFile().
func TestHelmRepoCompat(t *testing.T) {
	t.Run("NewIndexFile returns initialised IndexFile", func(t *testing.T) {
		index := helmRepo.NewIndexFile()
		if index == nil {
			t.Fatal("NewIndexFile() returned nil")
		}
		if index.Entries == nil {
			t.Fatal("IndexFile.Entries is nil after NewIndexFile()")
		}
	})

	t.Run("IndexFile.Entries accepts ChartVersions", func(t *testing.T) {
		index := helmRepo.NewIndexFile()
		index.Entries["rancher-monitoring"] = helmRepo.ChartVersions{
			{Metadata: &helmChart.Metadata{Name: "rancher-monitoring", Version: "108.0.0+up0.9.0"}},
			{Metadata: &helmChart.Metadata{Name: "rancher-monitoring", Version: "108.0.0+up0.9.0-rc.1"}},
		}
		if len(index.Entries["rancher-monitoring"]) != 2 {
			t.Errorf("expected 2 entries, got %d", len(index.Entries["rancher-monitoring"]))
		}
	})

	t.Run("ChartVersion.Metadata fields accessible", func(t *testing.T) {
		cv := &helmRepo.ChartVersion{
			Metadata: &helmChart.Metadata{
				Name:    "rancher-webhook",
				Version: "108.0.0+up0.9.0-rc.1",
			},
		}
		if cv.Name != "rancher-webhook" {
			t.Errorf("Name: got %q, want %q", cv.Name, "rancher-webhook")
		}
		if cv.Version != "108.0.0+up0.9.0-rc.1" {
			t.Errorf("Version: got %q, want %q", cv.Version, "108.0.0+up0.9.0-rc.1")
		}
	})

	t.Run("LoadIndexFile reads real index fixture", func(t *testing.T) {
		// Mirrors pkg/helm/helm.go and pkg/validate/pull_requests.go.
		// Fixture: test/canary/testdata/test.index.yaml — first 100 lines of
		// the real rancher/charts index.yaml (elemental entries only).
		loaded, err := helmRepo.LoadIndexFile(fixturePath("test.index.yaml"))
		if err != nil {
			t.Fatalf("LoadIndexFile: %v", err)
		}

		entries, ok := loaded.Entries["elemental"]
		if !ok {
			t.Fatalf("expected 'elemental' entry in index, got keys: %v", loaded.Entries)
		}
		if len(entries) != 4 {
			t.Errorf("expected 4 elemental versions, got %d", len(entries))
		}
		// Latest entry is first — 109.0.1+up1.9.1
		if entries[0].Version != "109.0.1+up1.9.1" {
			t.Errorf("latest version: got %q, want %q", entries[0].Version, "109.0.1+up1.9.1")
		}
		if entries[0].Name != "elemental" {
			t.Errorf("name: got %q, want %q", entries[0].Name, "elemental")
		}
	})
}
