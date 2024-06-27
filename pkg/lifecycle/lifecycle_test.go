package lifecycle

import (
	"fmt"
	"testing"
)

func Test_removeAssetsVersions(t *testing.T) {

	t.Run("Remove Versions Assets fail at makeRemoveWrapper", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			// initialize other fields as necessary
			makeRemoveWrapper: func(chart, version string, debug bool) error {
				return fmt.Errorf("Some error at makeRemoveWrapper")
			},
			statusPorceLainWrapper: func(debug bool) (bool, error) { return false, nil },
			addAndCommitWrapper:    func(message string) error { return nil },
			assetsVersionsMap:      map[string][]Asset{"chart1": {{Version: "999.0.0"}}},
			VR:                     vr,
		}

		// Execute
		_, err := dep.removeAssetsVersions(false)

		// Assert
		if err == nil {
			t.Errorf("removeAssetsVersions should have returned an error: %v", err)
		}
	})

	t.Run("Remove Versions Assets fail at statusPorceLainWrapper", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			// initialize other fields as necessary
			makeRemoveWrapper: func(chart, version string, debug bool) error { return nil },
			statusPorceLainWrapper: func(debug bool) (bool, error) {
				return false, fmt.Errorf("Some error at statusPorceLainWrapper")
			},
			addAndCommitWrapper: func(message string) error { return nil },
			assetsVersionsMap:   map[string][]Asset{"chart1": {{Version: "999.0.0"}}},
			VR:                  vr,
		}

		// Execute
		_, err := dep.removeAssetsVersions(false)

		// Assert
		if err == nil {
			t.Errorf("removeAssetsVersions should have returned an error: %v", err)
		}
	})

	t.Run("Remove Versions Assets fail at addAndCommitWrapper", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			// initialize other fields as necessary
			makeRemoveWrapper:      func(chart, version string, debug bool) error { return nil },
			statusPorceLainWrapper: func(debug bool) (bool, error) { return false, nil },
			addAndCommitWrapper: func(message string) error {
				return fmt.Errorf("Some error at addAndCommitWrapper")
			},
			assetsVersionsMap: map[string][]Asset{"chart1": {{Version: "999.0.0"}}},
			VR:                vr,
		}

		// Execute
		_, err := dep.removeAssetsVersions(false)

		// Assert
		if err == nil {
			t.Errorf("removeAssetsVersions should have returned an error: %v", err)
		}
	})

	t.Run("Remove Versions Assets successfully", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			makeRemoveWrapper:      func(chart, version string, debug bool) error { return nil },
			statusPorceLainWrapper: func(debug bool) (bool, error) { return false, nil },
			addAndCommitWrapper:    func(message string) error { return nil },
			assetsVersionsMap: map[string][]Asset{
				"chart1": {
					{Version: "105.0.0"},
					{Version: "104.1.0"},
					{Version: "103.0.1"},
					{Version: "102.9.150"},
					{Version: "101.0.0"},
					{Version: "100.0.0"},
					{Version: "99.9.9"},
				},
				"chart2": {
					{Version: "110.0.0"},
					{Version: "0.1.0"},
				},
			},
			VR: vr,
		}

		// Execute
		removedAssetsVersions, err := dep.removeAssetsVersions(false)

		// Assert
		switch {
		case err != nil:
			t.Errorf("removeAssetsVersions returned an error: %v", err)

		case len(removedAssetsVersions) != 2:
			t.Errorf("Expected 2 removed assets, got %d", len(removedAssetsVersions))

		case len(removedAssetsVersions["chart1"]) != 3:
			t.Errorf("Expected 3 removed assets for chart1, got %d", len(removedAssetsVersions["chart1"]))

		case len(removedAssetsVersions["chart2"]) != 2:
			t.Errorf("Expected 2 removed assets for chart2, got %d", len(removedAssetsVersions["chart2"]))
		}

		for _, asset := range removedAssetsVersions["chart1"] {
			if asset.Version == "105.0.0" || asset.Version == "100.0.0" || asset.Version == "99.9.9" {
				continue
			}
			t.Errorf("Unexpected removed asset version on chart1: %s", asset.Version)
		}

		for _, asset := range removedAssetsVersions["chart2"] {
			if asset.Version == "110.0.0" || asset.Version == "0.1.0" {
				continue
			}
			t.Errorf("Unexpected removed asset version on chart2: %s", asset.Version)
		}
	})

	t.Run("Remove Versions Assets successfully (nothing to remove)", func(t *testing.T) {
		// Init and mock dependencies
		vr, _ := GetVersionRules("2.9", false)
		dep := &Dependencies{
			makeRemoveWrapper:      func(chart, version string, debug bool) error { return nil },
			statusPorceLainWrapper: func(debug bool) (bool, error) { return false, nil },
			addAndCommitWrapper:    func(message string) error { return nil },
			assetsVersionsMap: map[string][]Asset{
				"chart1": {
					{Version: "103.0.1"},
					{Version: "102.9.150"},
					{Version: "101.0.0"},
				},
			},
			VR: vr,
		}

		// Execute
		removedAssetsVersions, err := dep.removeAssetsVersions(false)

		// Assert
		switch {
		case err != nil:
			t.Errorf("removeAssetsVersions returned an error: %v", err)

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
			makeRemoveWrapper:      func(chart, version string, debug bool) error { return nil },
			statusPorceLainWrapper: func(debug bool) (bool, error) { return true, nil },
			addAndCommitWrapper:    func(message string) error { return nil },
			assetsVersionsMap: map[string][]Asset{
				"chart1": {},
			},
			VR: vr,
		}

		// Execute
		removedAssetsVersions, err := dep.removeAssetsVersions(false)

		// Assert
		switch {
		case err != nil:
			t.Errorf("removeAssetsVersions returned an error: %v", err)

		case len(removedAssetsVersions) != 0:
			t.Errorf("Expected 0 removed assets, got %d", len(removedAssetsVersions))

		}
	})
}
