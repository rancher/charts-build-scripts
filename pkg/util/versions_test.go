package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLatestSameMajor(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		available       []string
		wantLatest      string
		wantNeedsUpdate bool
	}{
		{
			name:            "newer minor available",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "v2.45.1", "v2.46.0"},
			wantLatest:      "v2.46.0",
			wantNeedsUpdate: true,
		},
		{
			name:            "newer patch available",
			current:         "v2.45.0",
			available:       []string{"v2.44.0", "v2.45.0", "v2.45.1"},
			wantLatest:      "v2.45.1",
			wantNeedsUpdate: true,
		},
		{
			name:            "newer major and patch available",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "v2.45.1", "v3.0.0"},
			wantLatest:      "v2.45.1",
			wantNeedsUpdate: true,
		},
		{
			name:            "already on latest",
			current:         "v2.46.0",
			available:       []string{"v2.45.0", "v2.46.0"},
			wantLatest:      "v2.46.0",
			wantNeedsUpdate: false,
		},
		{
			name:            "only higher major available, no update",
			current:         "v1.5.0",
			available:       []string{"v1.5.0", "v2.0.0"},
			wantLatest:      "v1.5.0",
			wantNeedsUpdate: false,
		},
		{
			name:            "pre-release tags skipped",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "v2.46.0-rc.1", "v2.46.0-alpha.1"},
			wantLatest:      "v2.45.0",
			wantNeedsUpdate: false,
		},
		{
			name:            "non-semver tags skipped",
			current:         "v2.45.0",
			available:       []string{"v2.45.0", "latest", "edge", "v2.46.0"},
			wantLatest:      "v2.46.0",
			wantNeedsUpdate: true,
		},
		{
			name:            "bare tags without v prefix",
			current:         "2.45.0",
			available:       []string{"2.45.0", "2.45.1"},
			wantLatest:      "2.45.1",
			wantNeedsUpdate: true,
		},
		{
			name:            "empty current returns no update",
			current:         "",
			available:       []string{"v1.0.0"},
			wantLatest:      "",
			wantNeedsUpdate: false,
		},
		{
			name:            "garbage current returns no update",
			current:         "not-a-version",
			available:       []string{"v1.0.0"},
			wantLatest:      "",
			wantNeedsUpdate: false,
		},
		{
			name:            "empty available list",
			current:         "v1.0.0",
			available:       []string{},
			wantLatest:      "",
			wantNeedsUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLatest, gotNeedsUpdate := LatestSameMajor(tt.current, tt.available)
			assert.Equal(t, tt.wantNeedsUpdate, gotNeedsUpdate, "needsUpdate mismatch")
			if tt.wantLatest != "" {
				assert.Equal(t, tt.wantLatest, gotLatest, "latest tag mismatch")
			}
		})
	}
}
