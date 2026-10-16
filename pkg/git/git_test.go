package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	base := []string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@test", "-c", "commit.gpgsign=false"}
	if out, err := exec.Command("git", append(base, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// commitBlocklist writes content into config/blocklist.yaml and commits it, amending the last commit if amend is true
func commitBlocklist(t *testing.T, dir, content string, amend bool) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "blocklist.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	if amend {
		runGit(t, dir, "commit", "--amend", "-m", "blocklist")
		return
	}
	runGit(t, dir, "commit", "-m", "blocklist")
}

// newAutomationCoreFixture creates an upstream repository with main and automation-core branches,
// and a single-branch clone of main, which is configured as a rancher/charts upstream remote.
func newAutomationCoreFixture(t *testing.T) (upstream string, clone *Git) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}

	upstream = t.TempDir()
	runGit(t, upstream, "init", "-b", "main")
	runGit(t, upstream, "commit", "--allow-empty", "-m", "init")
	runGit(t, upstream, "checkout", "-b", "automation-core")
	commitBlocklist(t, upstream, "a: [1]\n", false)
	runGit(t, upstream, "checkout", "main")

	cloneDir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "--single-branch", "-b", "main", "file://"+upstream, cloneDir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}

	clone = &Git{
		Dir:     cloneDir,
		Remotes: map[string]string{"https://github.com/rancher/charts": "origin"},
	}
	return upstream, clone
}

func TestFetchRemoteTrackingBranch(t *testing.T) {
	const branch, file = "automation-core", "config/blocklist.yaml"
	ctx := context.Background()

	t.Run("single-branch clone", func(t *testing.T) {
		_, g := newAutomationCoreFixture(t)

		if err := g.FetchRemoteTrackingBranch(branch); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := g.ShowFileFromRemoteBranch(ctx, branch, file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != "a: [1]\n" {
			t.Errorf("expected %q, got %q", "a: [1]\n", got)
		}
	})

	t.Run("force-pushed upstream branch", func(t *testing.T) {
		upstream, g := newAutomationCoreFixture(t)

		if err := g.FetchRemoteTrackingBranch(branch); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// rewrite the upstream history, a non fast-forward update
		runGit(t, upstream, "checkout", branch)
		commitBlocklist(t, upstream, "b: [2]\n", true)
		runGit(t, upstream, "checkout", "main")

		if err := g.FetchRemoteTrackingBranch(branch); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := g.ShowFileFromRemoteBranch(ctx, branch, file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != "b: [2]\n" {
			t.Errorf("expected %q, got %q", "b: [2]\n", got)
		}
	})

	t.Run("local branch is checked out", func(t *testing.T) {
		_, g := newAutomationCoreFixture(t)

		// a local automation-core, checked out and diverged from upstream
		runGit(t, g.Dir, "checkout", "-b", branch)
		commitBlocklist(t, g.Dir, "local: [0]\n", false)

		if err := g.FetchRemoteTrackingBranch(branch); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := g.ShowFileFromRemoteBranch(ctx, branch, file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != "a: [1]\n" {
			t.Errorf("expected %q, got %q", "a: [1]\n", got)
		}
	})
}
