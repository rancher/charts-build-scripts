package git

import (
	"testing"
)

func TestGetUpstreamRemote(t *testing.T) {
	tests := []struct {
		name        string
		remotes     map[string]string
		expectName  string
		expectError bool
	}{
		{
			name: "HTTPS with .git suffix",
			remotes: map[string]string{
				"https://github.com/rancher/charts.git": "upstream",
			},
			expectName:  "upstream",
			expectError: false,
		},
		{
			name: "HTTPS without .git suffix",
			remotes: map[string]string{
				"https://github.com/rancher/charts": "upstream",
			},
			expectName:  "upstream",
			expectError: false,
		},
		{
			name: "SSH with .git suffix",
			remotes: map[string]string{
				"git@github.com:rancher/charts.git": "upstream",
			},
			expectName:  "upstream",
			expectError: false,
		},
		{
			name: "SSH without .git suffix",
			remotes: map[string]string{
				"git@github.com:rancher/charts": "upstream",
			},
			expectName:  "upstream",
			expectError: false,
		},
		{
			name: "Multiple remotes with upstream",
			remotes: map[string]string{
				"https://github.com/fork/charts.git":    "origin",
				"https://github.com/rancher/charts.git": "upstream",
			},
			expectName:  "upstream",
			expectError: false,
		},
		{
			name: "Remote named 'origin' but correct URL",
			remotes: map[string]string{
				"git@github.com:rancher/charts.git": "origin",
			},
			expectName:  "origin",
			expectError: false,
		},
		{
			name: "Wrong repository",
			remotes: map[string]string{
				"https://github.com/rancher/other-repo.git": "upstream",
			},
			expectName:  "",
			expectError: true,
		},
		{
			name:        "No remotes",
			remotes:     map[string]string{},
			expectName:  "",
			expectError: true,
		},
		{
			name: "Fork of rancher/charts",
			remotes: map[string]string{
				"https://github.com/myuser/rancher/charts.git": "origin",
			},
			expectName:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Git{
				Remotes: tt.remotes,
			}

			remoteName, err := g.getUpstreamRemote()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if remoteName != tt.expectName {
					t.Errorf("expected remote name %q, got %q", tt.expectName, remoteName)
				}
			}
		})
	}
}
