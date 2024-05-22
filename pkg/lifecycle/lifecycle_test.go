package lifecycle

import (
	"fmt"
	"testing"
)

func Test_removeVersionsAssets(t *testing.T) {

	t.Run("Remove Versions Assets fail at makeRemoveWrapper", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			// initialize other fields as necessary
			makeRemoveWrapper: func(chart, version string, debug bool) error {
				return fmt.Errorf("Some error at makeRemoveWrapper")
			},
			checkIfGitIsCleanWrapper: func(debug bool) (bool, error) { return false, nil },
			gitAddAndCommitWrapper:   func(message string) error { return nil },
			assetsVersionsMap:        map[string][]Asset{"chart1": {{version: "999.0.0"}}},
			vr:                       vr,
		}

		// Execute
		_, err := dep.removeVersionsAssets(false)

		// Assert
		if err == nil {
			t.Errorf("removeVersionsAssets returned an error: %v", err)
		}
	})

	t.Run("Remove Versions Assets fail at checkIfGitIsCleanWrapper", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			// initialize other fields as necessary
			makeRemoveWrapper: func(chart, version string, debug bool) error { return nil },
			checkIfGitIsCleanWrapper: func(debug bool) (bool, error) {
				return false, fmt.Errorf("Some error at checkIfGitIsCleanWrapper")
			},
			gitAddAndCommitWrapper: func(message string) error { return nil },
			assetsVersionsMap:      map[string][]Asset{"chart1": {{version: "999.0.0"}}},
			vr:                     vr,
		}

		// Execute
		_, err := dep.removeVersionsAssets(false)

		// Assert
		if err == nil {
			t.Errorf("removeVersionsAssets returned an error: %v", err)
		}
	})

	t.Run("Remove Versions Assets fail at gitAddAndCommitWrapper", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			// initialize other fields as necessary
			makeRemoveWrapper:        func(chart, version string, debug bool) error { return nil },
			checkIfGitIsCleanWrapper: func(debug bool) (bool, error) { return false, nil },
			gitAddAndCommitWrapper: func(message string) error {
				return fmt.Errorf("Some error at gitAddAndCommitWrapper")
			},
			assetsVersionsMap: map[string][]Asset{"chart1": {{version: "999.0.0"}}},
			vr:                vr,
		}

		// Execute
		_, err := dep.removeVersionsAssets(false)

		// Assert
		if err == nil {
			t.Errorf("removeVersionsAssets returned an error: %v", err)
		}
	})

	t.Run("Remove Versions Assets successfully", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			makeRemoveWrapper:        func(chart, version string, debug bool) error { return nil },
			checkIfGitIsCleanWrapper: func(debug bool) (bool, error) { return false, nil },
			gitAddAndCommitWrapper:   func(message string) error { return nil },
			assetsVersionsMap: map[string][]Asset{
				"chart1": {
					{version: "105.0.0"},
					{version: "104.1.0"},
					{version: "103.0.1"},
					{version: "102.9.150"},
					{version: "101.0.0"},
					{version: "100.0.0"},
					{version: "99.9.9"},
				},
				"chart2": {
					{version: "110.0.0"},
					{version: "0.1.0"},
				},
			},
			vr: vr,
		}

		// Execute
		removedAssetsVersions, err := dep.removeVersionsAssets(false)

		// Assert
		switch {
		case err != nil:
			t.Errorf("removeVersionsAssets returned an error: %v", err)

		case len(removedAssetsVersions) != 2:
			t.Errorf("Expected 2 removed assets, got %d", len(removedAssetsVersions))

		case len(removedAssetsVersions["chart1"]) != 3:
			t.Errorf("Expected 3 removed assets for chart1, got %d", len(removedAssetsVersions["chart1"]))

		case len(removedAssetsVersions["chart2"]) != 2:
			t.Errorf("Expected 2 removed assets for chart2, got %d", len(removedAssetsVersions["chart2"]))
		}

		for _, asset := range removedAssetsVersions["chart1"] {
			if asset.version == "105.0.0" || asset.version == "100.0.0" || asset.version == "99.9.9" {
				continue
			}
			t.Errorf("Unexpected removed asset version on chart1: %s", asset.version)
		}

		for _, asset := range removedAssetsVersions["chart2"] {
			if asset.version == "110.0.0" || asset.version == "0.1.0" {
				continue
			}
			t.Errorf("Unexpected removed asset version on chart2: %s", asset.version)
		}
	})

	t.Run("Remove Versions Assets successfully (nothing to remove)", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			makeRemoveWrapper:        func(chart, version string, debug bool) error { return nil },
			checkIfGitIsCleanWrapper: func(debug bool) (bool, error) { return false, nil },
			gitAddAndCommitWrapper:   func(message string) error { return nil },
			assetsVersionsMap: map[string][]Asset{
				"chart1": {
					{version: "103.0.1"},
					{version: "102.9.150"},
					{version: "101.0.0"},
				},
			},
			vr: vr,
		}

		// Execute
		removedAssetsVersions, err := dep.removeVersionsAssets(false)

		// Assert
		switch {
		case err != nil:
			t.Errorf("removeVersionsAssets returned an error: %v", err)

		case len(removedAssetsVersions) != 1:
			t.Errorf("Expected 1 removed assets in slice, got %d", len(removedAssetsVersions))

		case len(removedAssetsVersions["chart1"]) != 0:
			t.Errorf("Expected 0 removed assets for chart1, got %d", len(removedAssetsVersions["chart1"]))

		}
	})

	t.Run("Remove Versions Assets successfully (empty assetVersionsMap to remove)", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			makeRemoveWrapper:        func(chart, version string, debug bool) error { return nil },
			checkIfGitIsCleanWrapper: func(debug bool) (bool, error) { return true, nil },
			gitAddAndCommitWrapper:   func(message string) error { return nil },
			assetsVersionsMap: map[string][]Asset{
				"chart1": {},
			},
			vr: vr,
		}

		// Execute
		removedAssetsVersions, err := dep.removeVersionsAssets(false)

		// Assert
		switch {
		case err != nil:
			t.Errorf("removeVersionsAssets returned an error: %v", err)

		case len(removedAssetsVersions) != 0:
			t.Errorf("Expected 0 removed assets, got %d", len(removedAssetsVersions))

		}
	})
}
