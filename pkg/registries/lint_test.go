package registries

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// traverseViolations
// ---------------------------------------------------------------------------

func Test_traverseViolations(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantKeys []string // expected violation yamlPaths
		wantNone bool     // expect zero violations
	}{
		{
			name: "valid rancher namespace pair — no violation",
			input: map[string]interface{}{
				"repository": "rancher/my-image",
				"tag":        "v1.0.0",
			},
			wantNone: true,
		},
		{
			name: "orphan repository — tag key absent",
			input: map[string]interface{}{
				"repository": "rancher/my-image",
			},
			wantKeys: []string{""},
		},
		{
			name: "orphan repository — tag is empty string",
			input: map[string]interface{}{
				"repository": "rancher/my-image",
				"tag":        "",
			},
			wantKeys: []string{""},
		},
		{
			name: "orphan repository — tag is null (nil)",
			input: map[string]interface{}{
				"repository": "rancher/my-image",
				"tag":        nil,
			},
			wantKeys: []string{""},
		},
		{
			name: "wrong namespace — non-rancher repository with valid tag",
			input: map[string]interface{}{
				"repository": "docker.io/thirdparty/image",
				"tag":        "v2.0.0",
			},
			wantKeys: []string{""},
		},
		{
			name: "null repository — should be skipped entirely",
			input: map[string]interface{}{
				"repository": nil,
				"tag":        "v1.0.0",
			},
			wantNone: true,
		},
		{
			name: "empty string repository — should be skipped",
			input: map[string]interface{}{
				"repository": "",
				"tag":        "v1.0.0",
			},
			wantNone: true,
		},
		{
			name: "nested orphan repository under named key",
			input: map[string]interface{}{
				"image": map[string]interface{}{
					"repository": "rancher/agent",
					// no tag
				},
			},
			wantKeys: []string{"image"},
		},
		{
			name: "nested wrong namespace under named key",
			input: map[string]interface{}{
				"proxy": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "quay.io/external/image",
						"tag":        "latest",
					},
				},
			},
			wantKeys: []string{"proxy.image"},
		},
		{
			name: "multiple violations at different paths",
			input: map[string]interface{}{
				"alpha": map[string]interface{}{
					"repository": "rancher/alpha",
					// no tag — orphan
				},
				"beta": map[string]interface{}{
					"repository": "ghcr.io/beta/image",
					"tag":        "v1.0",
					// wrong namespace
				},
			},
			wantKeys: []string{"alpha", "beta"},
		},
		{
			name: "valid pair stops traversal — children not checked",
			input: map[string]interface{}{
				"repository": "rancher/parent",
				"tag":        "v1.0.0",
				"nested": map[string]interface{}{
					// would be a violation if reached
					"repository": "quay.io/child",
					"tag":        "v2.0.0",
				},
			},
			wantNone: true, // traversal stops at the valid parent node
		},
		{
			name: "array of image objects",
			input: []interface{}{
				map[string]interface{}{
					"repository": "rancher/img-a",
					"tag":        "v1",
				},
				map[string]interface{}{
					"repository": "rancher/img-b",
					// no tag — orphan
				},
			},
			wantKeys: []string{"[1]"},
		},
		{
			name: "no repository anywhere — no violations",
			input: map[string]interface{}{
				"replicaCount": 1,
				"service": map[string]interface{}{
					"port": 8080,
				},
			},
			wantNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := make(map[string]LintWarning)
			traverseViolations(tt.input, "", "test.tgz", violations)

			if tt.wantNone {
				assert.Empty(t, violations)
				return
			}

			require.Len(t, violations, len(tt.wantKeys), "unexpected number of violations")
			for _, key := range tt.wantKeys {
				_, ok := violations[key]
				assert.Truef(t, ok, "expected violation at path %q not found", key)
			}
		})
	}
}

func Test_traverseViolations_reasons(t *testing.T) {
	t.Run("orphan_repository reason set correctly", func(t *testing.T) {
		input := map[string]interface{}{
			"repository": "rancher/my-image",
		}
		violations := make(map[string]LintWarning)
		traverseViolations(input, "", "test.tgz", violations)
		require.Contains(t, violations, "")
		assert.Equal(t, "orphan_repository", violations[""].Reason)
		assert.Equal(t, "rancher/my-image", violations[""].Repository)
		assert.Equal(t, "", violations[""].Tag)
		assert.Equal(t, "test.tgz", violations[""].Asset)
	})

	t.Run("wrong_namespace reason set correctly", func(t *testing.T) {
		input := map[string]interface{}{
			"repository": "quay.io/external/image",
			"tag":        "v1.2.3",
		}
		violations := make(map[string]LintWarning)
		traverseViolations(input, "", "test.tgz", violations)
		require.Contains(t, violations, "")
		assert.Equal(t, "wrong_namespace", violations[""].Reason)
		assert.Equal(t, "quay.io/external/image", violations[""].Repository)
		assert.Equal(t, "v1.2.3", violations[""].Tag)
	})
}

// ---------------------------------------------------------------------------
// parseChangedTgzFiles
// ---------------------------------------------------------------------------

// makeChange builds an object.Change that looks added/modified (non-empty To,
// empty From → Insert action) with the given path in the To entry.
func makeAddedChange(path string) *object.Change {
	return &object.Change{
		To: object.ChangeEntry{Name: path},
		// From is zero-value → Action() returns Insert
	}
}

// makeDeletedChange builds an object.Change that looks deleted (non-empty From,
// empty To → Delete action).
func makeDeletedChange(path string) *object.Change {
	return &object.Change{
		From: object.ChangeEntry{Name: path},
		// To is zero-value → Action() returns Delete
	}
}

func Test_parseChangedTgzFiles(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		changes object.Changes
		want    []string
	}{
		{
			name: "single added tgz under assets/",
			changes: object.Changes{
				makeAddedChange("assets/my-chart/my-chart-1.0.0.tgz"),
			},
			want: []string{"assets/my-chart/my-chart-1.0.0.tgz"},
		},
		{
			name: "deleted tgz is ignored",
			changes: object.Changes{
				makeDeletedChange("assets/my-chart/my-chart-1.0.0.tgz"),
			},
			want: nil,
		},
		{
			name: "non-tgz file is ignored",
			changes: object.Changes{
				makeAddedChange("assets/my-chart/values.yaml"),
				makeAddedChange("charts/my-chart/Chart.yaml"),
			},
			want: nil,
		},
		{
			name: "tgz outside assets/ is ignored",
			changes: object.Changes{
				makeAddedChange("pkg/some/file.tgz"),
			},
			want: nil,
		},
		{
			name: "mixed changes — only added tgz under assets/ returned",
			changes: object.Changes{
				makeAddedChange("assets/chart-a/chart-a-1.0.0.tgz"),
				makeDeletedChange("assets/chart-b/chart-b-0.9.0.tgz"),
				makeAddedChange("charts/chart-a/Chart.yaml"),
				makeAddedChange("assets/chart-c/chart-c-2.0.0.tgz"),
			},
			want: []string{
				"assets/chart-a/chart-a-1.0.0.tgz",
				"assets/chart-c/chart-c-2.0.0.tgz",
			},
		},
		{
			name:    "empty changeset",
			changes: object.Changes{},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChangedTgzFiles(ctx, tt.changes)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
