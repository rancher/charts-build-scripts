package lifecycle

import (
	"testing"
)

func TestGetVersionRules(t *testing.T) {
	t.Run("branchVersion is empty", func(t *testing.T) {
		_, err := GetVersionRules("", false)
		if err == nil {
			t.Errorf("Expected error when branchVersion is empty")
		}
	})

	t.Run("branchVersion is not convertible to float32", func(t *testing.T) {
		_, err := GetVersionRules("not a float", false)
		if err == nil {
			t.Errorf("Expected error when branchVersion is not convertible to float32")
		}
	})

	t.Run("branchVersion is not defined in the rules", func(t *testing.T) {
		_, err := GetVersionRules("3.0", false)
		if err == nil {
			t.Errorf("Expected error when branchVersion is not defined in the rules")
		}
	})

	t.Run("branchVersion is defined in the rules for branch: 2.9", func(t *testing.T) {
		vr, err := GetVersionRules("2.9", false)
		if err != nil {
			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
		}
		switch {
		case vr.BranchVersion != 2.9:
			t.Errorf("Expected branchVersion to be 2.9, got %v", vr.BranchVersion)
		case vr.MinVersion != 101:
			t.Errorf("Expected minVersion to be 101, got %v", vr.MinVersion)
		case vr.MaxVersion != 105:
			t.Errorf("Expected maxVersion to be 105, got %v", vr.MaxVersion)
		}
	})

	t.Run("branchVersion is defined in the rules for branch: 2.8", func(t *testing.T) {
		vr, err := GetVersionRules("2.8", false)
		if err != nil {
			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
		}
		switch {
		case vr.BranchVersion != 2.8:
			t.Errorf("Expected branchVersion to be 2.8, got %v", vr.BranchVersion)
		case vr.MinVersion != 100:
			t.Errorf("Expected minVersion to be 100, got %v", vr.MinVersion)
		case vr.MaxVersion != 104:
			t.Errorf("Expected maxVersion to be 104, got %v", vr.MaxVersion)
		}
	})

	t.Run("branchVersion is defined in the rules for branch: 2.7", func(t *testing.T) {
		vr, err := GetVersionRules("2.7", false)
		if err != nil {
			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
		}
		switch {
		case vr.BranchVersion != 2.7:
			t.Errorf("Expected branchVersion to be 2.7, got %v", vr.BranchVersion)
		case vr.MinVersion != 0:
			t.Errorf("Expected minVersion to be 0, got %v", vr.MinVersion)
		case vr.MaxVersion != 103:
			t.Errorf("Expected maxVersion to be 103, got %v", vr.MaxVersion)
		}
	})

	t.Run("branchVersion is defined in the rules for branch: 2.6", func(t *testing.T) {
		vr, err := GetVersionRules("2.6", false)
		if err != nil {
			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
		}
		switch {
		case vr.BranchVersion != 2.6:
			t.Errorf("Expected branchVersion to be 2.6, got %v", vr.BranchVersion)
		case vr.MinVersion != 0:
			t.Errorf("Expected minVersion to be 0, got %v", vr.MinVersion)
		case vr.MaxVersion != 101:
			t.Errorf("Expected maxVersion to be 101, got %v", vr.MaxVersion)
		}
	})

	t.Run("branchVersion is defined in the rules for branch: 2.5", func(t *testing.T) {
		vr, err := GetVersionRules("2.5", false)
		if err != nil {
			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
		}
		switch {
		case vr.BranchVersion != 2.5:
			t.Errorf("Expected branchVersion to be 2.5, got %v", vr.BranchVersion)
		case vr.MinVersion != 0:
			t.Errorf("Expected minVersion to be 0, got %v", vr.MinVersion)
		case vr.MaxVersion != 100:
			t.Errorf("Expected maxVersion to be 100, got %v", vr.MaxVersion)
		}
	})
}
