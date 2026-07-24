package registries

import (
	"context"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/config"
)

func TestFilterBlocklistedAssets(t *testing.T) {
	ctx := context.Background()

	tgzPaths := []string{
		"assets/rancher-monitoring/rancher-monitoring-109.0.1+up80.9.1.tgz",
		"assets/rancher-monitoring/rancher-monitoring-109.0.2+up80.9.1-rancher.8.tgz",
		"assets/rancher-monitoring/rancher-monitoring-109.0.3+up80.9.2.tgz",
	}

	blocklist := &config.Blocklist{
		Charts: map[string][]string{
			"rancher-monitoring": {"109.0.2+up80.9.1-rancher.8"},
		},
	}

	filtered := filterBlocklistedAssets(ctx, tgzPaths, blocklist)

	expectedCount := 2
	if len(filtered) != expectedCount {
		t.Fatalf("expected %d filtered assets, got %d", expectedCount, len(filtered))
	}

	expected := []string{
		"assets/rancher-monitoring/rancher-monitoring-109.0.1+up80.9.1.tgz",
		"assets/rancher-monitoring/rancher-monitoring-109.0.3+up80.9.2.tgz",
	}

	for i, path := range expected {
		if filtered[i] != path {
			t.Errorf("expected filtered[%d] = %s, got %s", i, path, filtered[i])
		}
	}
}
