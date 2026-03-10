package integration_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/registries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixturePath returns an absolute path to a file in testdata/ relative to this
// file, so tests work regardless of where `go test` is invoked from.
func fixturePath(name string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata", name)
}

// TestLintImageTags_tgzOverride exercises the full pipeline:
// DecodeValueYamlInTgz → traverseViolations → LintWarning collection.
//
// The fixture lint_images_test.tgz contains mock-chart/values.yaml with:
//   - validImage:   rancher/valid-image + tag → no violation
//   - orphanImage:  rancher/orphan-image, no tag → orphan_repository
//   - wrongNsImage: quay.io/external/image + tag → wrong_namespace
func TestLintImageTags_tgzOverride(t *testing.T) {
	ctx := context.Background()
	tgz := fixturePath("lint_images_test.tgz")

	warnings, err := registries.LintImageTags(ctx, "", tgz)
	require.NoError(t, err)
	require.Len(t, warnings, 2, "expected exactly 2 violations")

	byPath := make(map[string]registries.LintWarning, len(warnings))
	for _, w := range warnings {
		byPath[w.YAMLPath] = w
	}

	t.Run("orphan_repository warning", func(t *testing.T) {
		w, ok := byPath["orphanImage"]
		require.True(t, ok, "expected violation at path 'orphanImage'")
		assert.Equal(t, "orphan_repository", w.Reason)
		assert.Equal(t, "rancher/orphan-image", w.Repository)
		assert.Equal(t, "", w.Tag)
		assert.Equal(t, tgz, w.Asset)
	})

	t.Run("wrong_namespace warning", func(t *testing.T) {
		w, ok := byPath["wrongNsImage"]
		require.True(t, ok, "expected violation at path 'wrongNsImage'")
		assert.Equal(t, "wrong_namespace", w.Reason)
		assert.Equal(t, "quay.io/external/image", w.Repository)
		assert.Equal(t, "v2.0.0", w.Tag)
		assert.Equal(t, tgz, w.Asset)
	})

	t.Run("valid rancher image produces no warning", func(t *testing.T) {
		_, ok := byPath["validImage"]
		assert.False(t, ok, "validImage should not appear in warnings")
	})
}

// TestLintImageTags_noBaseBranch asserts that omitting both --base-branch and
// --tgz returns an error rather than silently passing.
func TestLintImageTags_noBaseBranch(t *testing.T) {
	ctx := context.Background()
	_, err := registries.LintImageTags(ctx, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BASE_BRANCH")
}
