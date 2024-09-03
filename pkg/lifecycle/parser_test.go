package lifecycle

import (
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/stretchr/testify/assert"
)

func Test_getAssetsMapFromIndex(t *testing.T) {
	t.Run("Fail to load Index File", func(t *testing.T) {
		_, err := getAssetsMapFromIndex("", "")
		assert.Error(t, err, "Expected error when failing to load Index file")
	})

	t.Run("Load all charts successfully", func(t *testing.T) {
		assetsVersionsMapTest1, err := getAssetsMapFromIndex("mocks/test.yaml", "")
		assert.Nil(t, err, "Error not expected when failing to load Index file")
		assert.Equal(t, len(assetsVersionsMapTest1), 2, "Expected to load 2 charts")
		assert.Equal(t, len(assetsVersionsMapTest1["chart-one"]), 2, "Expected to load 2 versions for chart1")
		assert.Equal(t, len(assetsVersionsMapTest1["chart-two"]), 1, "Expected to load 1 versions for chart1")

	})

	t.Run("Fail to load target chart (chart-zero)", func(t *testing.T) {
		_, err := getAssetsMapFromIndex("mocks/test.yaml", "chart-zero")
		assert.Error(t, err, "Expected error when failing to load Index file")
	})

	t.Run("Load target chart (chart-one) successfully", func(t *testing.T) {
		assetsVersionsMapTest, err := getAssetsMapFromIndex("mocks/test.yaml", "chart-one")
		assert.Nil(t, err, "Error not expected when failing to load Index file")
		assert.Equal(t, len(assetsVersionsMapTest), 1, "Expected to load 1 chart")
		assert.Equal(t, len(assetsVersionsMapTest["chart-one"]), 2, "Expected to load 2 versions for chart2")
	})

	t.Run("Load target chart (chart-two) successfully", func(t *testing.T) {
		assetsVersionsMapTest, err := getAssetsMapFromIndex("mocks/test.yaml", "chart-two")
		assert.Nil(t, err, "Error not expected when failing to load Index file")
		assert.Equal(t, len(assetsVersionsMapTest), 1, "Expected to load 1 chart")
		assert.Equal(t, len(assetsVersionsMapTest["chart-two"]), 1, "Expected to load 1 version for chart2")
	})
}

func Test_populateAssetsVersionsPath(t *testing.T) {
	t.Run("Populate assets versions map successfully", func(t *testing.T) {
		// Create a test instance of Dependencies with a pre-populated assetsVersionsMap
		ld := &Dependencies{
			// rootFs: fs,
			AssetsVersionsMap: map[string][]Asset{
				"chart1": {
					{Version: "1.0.0"},
				},
				"chart2": {
					{Version: "1.0.0"},
				},
			},
			walkDirWrapper: func(fs billy.Filesystem, dirPath string, doFunc filesystem.RelativePathFunc) error {
				// Simulate the behavior of filesystem.WalkDir as needed for your test.
				if dirPath == "assets/chart1" {
					doFunc(nil, "assets/chart1/chart1-1.0.0.tgz", false)
				}
				if dirPath == "assets/chart2" {
					doFunc(nil, "assets/chart2/chart2-1.0.0.tgz", false)
				}
				return nil
			},
		}

		// Call the function we're testing
		err := ld.populateAssetsVersionsPath()
		assert.Nil(t, err, "populateAssetsVersionsPath should not have returned an error")

		// Check that the assetsVersionsMap was populated correctly
		expected := map[string][]Asset{
			"chart1": {
				{Version: "1.0.0", path: "assets/chart1/chart1-1.0.0.tgz"},
			},
			"chart2": {
				{Version: "1.0.0", path: "assets/chart2/chart2-1.0.0.tgz"},
			},
		}

		assert.EqualValues(t, ld.AssetsVersionsMap, expected, "assetsVersionsMap was not populated correctly")
	})

	t.Run("Fail to walk through the assets directory", func(t *testing.T) {
		// Create a test instance of Dependencies with a pre-populated assetsVersionsMap
		dependency := &Dependencies{
			AssetsVersionsMap: map[string][]Asset{
				"chart1": {
					{Version: "1.0.0"},
				},
			},
			walkDirWrapper: func(fs billy.Filesystem, dirPath string, doFunc filesystem.RelativePathFunc) error {
				doFunc(nil, "", false)
				return fmt.Errorf("Some random error")
			},
		}

		err := dependency.populateAssetsVersionsPath()
		assert.Error(t, err, "populateAssetsVersionsPath should have returned an error")
	})
}

func Test_sortAssetsVersions(t *testing.T) {
	// Arrange
	dependency := &Dependencies{
		AssetsVersionsMap: map[string][]Asset{
			"chart1": {
				{Version: "1.0.0"},
				{Version: "0.1.0"},
				{Version: "0.0.1"},
			},
			"chart2": {
				{Version: "2.0.0"},
				{Version: "1.1.0"},
				{Version: "1.0.1"},
			},
		},
	}

	// Act
	dependency.sortAssetsVersions()

	// Assertions
	assert.Equal(t, len(dependency.AssetsVersionsMap), 2, "Expected 2 charts in the assetsVersionsMap")
	assert.Equal(t, dependency.AssetsVersionsMap["chart1"][0].Version, "0.0.1", "Expected 0.0.1")
	assert.Equal(t, dependency.AssetsVersionsMap["chart1"][1].Version, "0.1.0", "Expected 0.1.0")
	assert.Equal(t, dependency.AssetsVersionsMap["chart1"][2].Version, "1.0.0", "Expected 1.0.0")

	assert.Equal(t, dependency.AssetsVersionsMap["chart2"][0].Version, "1.0.1", "Expected 1.0.1")
	assert.Equal(t, dependency.AssetsVersionsMap["chart2"][1].Version, "1.1.0", "Expected 1.1.0")
	assert.Equal(t, dependency.AssetsVersionsMap["chart2"][2].Version, "2.0.0", "Expected 2.0.0")
}
