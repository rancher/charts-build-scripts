package lifecycle

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
)

func Test_getAssetsMapFromIndex(t *testing.T) {
	t.Run("Fail to load Index File", func(t *testing.T) {
		_, err := getAssetsMapFromIndex("", "", false)
		if err == nil {
			t.Errorf("Expected error when failing to load Index file")
		}
	})

	t.Run("Load all charts successfully", func(t *testing.T) {
		assetsVersionsMapTest1, err := getAssetsMapFromIndex("mocks/test.yaml", "", false)

		if err != nil {
			t.Errorf("Error not expected when failing to load Index file %v", err)
		}
		if len(assetsVersionsMapTest1) != 2 {
			t.Errorf("Expected to load 2 charts")
		}
		if len(assetsVersionsMapTest1["chart-one"]) != 2 {
			t.Errorf("Expected to load 2 versions for chart1, got %v", len(assetsVersionsMapTest1["chart-one"]))
		}
		if len(assetsVersionsMapTest1["chart-two"]) != 1 {
			t.Errorf("Expected to load 1 version for chart2, got %v", len(assetsVersionsMapTest1["chart-one"]))
		}
	})

	t.Run("Fail to load target chart (chart-zero)", func(t *testing.T) {
		_, err := getAssetsMapFromIndex("mocks/test.yaml", "chart-zero", false)
		if err == nil {
			t.Errorf("Expected error when failing to load Index file")
		}
	})

	t.Run("Load target chart (chart-one) successfully", func(t *testing.T) {
		assetsVersionsMapTest, err := getAssetsMapFromIndex("mocks/test.yaml", "chart-one", false)

		if err != nil {
			t.Errorf("Error not expected when failing to load Index file %v", err)
		}
		if len(assetsVersionsMapTest) != 1 {
			t.Errorf("Expected to load 1 chart")
		}
		if len(assetsVersionsMapTest["chart-one"]) != 2 {
			t.Errorf("Expected to load 1 version for chart2, got %v", len(assetsVersionsMapTest["chart-one"]))
		}
	})

	t.Run("Load target chart (chart-two) successfully", func(t *testing.T) {
		assetsVersionsMapTest, err := getAssetsMapFromIndex("mocks/test.yaml", "chart-two", false)

		if err != nil {
			t.Errorf("Error not expected when failing to load Index file %v", err)
		}
		if len(assetsVersionsMapTest) != 1 {
			t.Errorf("Expected to load 1 chart")
		}
		if len(assetsVersionsMapTest["chart-two"]) != 1 {
			t.Errorf("Expected to load 1 version for chart2, got %v", len(assetsVersionsMapTest["chart-one"]))
		}
	})
}

func Test_populateAssetsVersionsPath(t *testing.T) {
	t.Run("Populate assets versions map successfully", func(t *testing.T) {
		// Create a test instance of Dependencies with a pre-populated assetsVersionsMap
		ld := &Dependencies{
			// rootFs: fs,
			assetsVersionsMap: map[string][]Asset{
				"chart1": {
					{version: "1.0.0"},
				},
				"chart2": {
					{version: "1.0.0"},
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
		err := ld.populateAssetsVersionsPath(false)
		if err != nil {
			t.Fatalf("populateAssetsVersionsPath returned an error: %v", err)
		}

		// Check that the assetsVersionsMap was populated correctly
		expected := map[string][]Asset{
			"chart1": {
				{version: "1.0.0", path: "assets/chart1/chart1-1.0.0.tgz"},
			},
			"chart2": {
				{version: "1.0.0", path: "assets/chart2/chart2-1.0.0.tgz"},
			},
		}
		if !reflect.DeepEqual(ld.assetsVersionsMap, expected) {
			t.Errorf("assetsVersionsMap was not populated correctly, got: %v, want: %v", ld.assetsVersionsMap, expected)
		}
	})

	t.Run("Fail to walk through the assets directory", func(t *testing.T) {
		// Create a test instance of Dependencies with a pre-populated assetsVersionsMap
		ld := &Dependencies{
			// rootFs: fs,
			assetsVersionsMap: map[string][]Asset{
				"chart1": {
					{version: "1.0.0"},
				},
			},
			walkDirWrapper: func(fs billy.Filesystem, dirPath string, doFunc filesystem.RelativePathFunc) error {
				doFunc(nil, "", false)
				return fmt.Errorf("Some random error")
			},
		}

		// Call the function we're testing
		err := ld.populateAssetsVersionsPath(false)
		if err == nil {
			t.Fatalf("populateAssetsVersionsPath returned an error: %v", err)
		}
	})
}

func Test_sortAssetsVersions(t *testing.T) {
	// Arrange
	dep := &Dependencies{
		assetsVersionsMap: map[string][]Asset{
			"key1": {
				{version: "1.0.0"},
				{version: "0.1.0"},
				{version: "0.0.1"},
			},
			"key2": {
				{version: "2.0.0"},
				{version: "1.1.0"},
				{version: "1.0.1"},
			},
		},
	}

	// Act
	dep.sortAssetsVersions()

	// Assert
	if dep.assetsVersionsMap["key1"][0].version != "0.0.1" ||
		dep.assetsVersionsMap["key1"][1].version != "0.1.0" ||
		dep.assetsVersionsMap["key1"][2].version != "1.0.0" {
		t.Errorf("assetsVersionsMap was not sorted correctly for key1")
	}

	if dep.assetsVersionsMap["key2"][0].version != "1.0.1" ||
		dep.assetsVersionsMap["key2"][1].version != "1.1.0" ||
		dep.assetsVersionsMap["key2"][2].version != "2.0.0" {
		t.Errorf("assetsVersionsMap was not sorted correctly for key2")
	}
}
