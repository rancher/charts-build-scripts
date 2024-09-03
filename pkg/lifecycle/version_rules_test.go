package lifecycle

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
)

func Test_branchVersionMinorSum(t *testing.T) {
	t.Run("branchVersion is 2.9 and sum is -2", func(t *testing.T) {
		result := branchVersionMinorSum("2.9", -2)
		assert.Equal(t, "2.7", result, "Expected 2.7")
	})

	t.Run("branchVersion is 2.8 and sum is 2", func(t *testing.T) {
		result := branchVersionMinorSum("2.8", 2)
		assert.Equal(t, "2.10", result, "Expected 2.10")
	})
}

func Test_rules(t *testing.T) {
	fs := memfs.New()

	type input struct {
		fs            billy.Filesystem
		branchVersion string
		mockLoad      jsonLoader
	}
	type expected struct {
		vr  *VersionRules
		err error
	}
	type test struct {
		name string
		i    input
		ex   expected
	}

	tests := []test{
		{
			name: "#1 - branchVersion is empty",
			i: input{
				fs:            fs,
				branchVersion: "",
			},
			ex: expected{
				vr:  nil,
				err: errorNoBranchVersion,
			},
		},
		{
			name: "#2 - branchVersion is not defined in rules",
			i: input{
				fs:            fs,
				branchVersion: "99.99",
				mockLoad: func(fs billy.Filesystem) (*VersionRules, error) {
					return &VersionRules{
						Rules: map[string]Version{
							"2.9": {Min: "101", Max: "105"},
						},
					}, nil
				},
			},
			ex: expected{
				vr:  nil,
				err: errorBranchVersionNotInRules,
			},
		},
		{
			name: "#3 - branchVersion defined in rules [simplest case]",
			i: input{
				fs:            fs,
				branchVersion: "2.9",
				mockLoad: func(fs billy.Filesystem) (*VersionRules, error) {
					return &VersionRules{
						Rules: map[string]Version{
							"2.9": {Min: "104.0.0", Max: "105.0.0"},
						},
						DevBranchPrefix:  "dev-v",
						ProdBranchPrefix: "release-v",
					}, nil
				},
			},
			ex: expected{
				vr: &VersionRules{
					Rules: map[string]Version{
						"2.9": {Min: "104.0.0", Max: "105.0.0"},
					},
					BranchVersion:    "2.9",
					DevBranchPrefix:  "dev-v",
					DevBranch:        "dev-v2.9",
					ProdBranchPrefix: "release-v",
					ProdBranch:       "release-v2.9",
					MinVersion:       0,
					MaxVersion:       105,
				},
				err: nil,
			},
		},
		{
			name: "#4 - branchVersion defined in rules [full case]",
			i: input{
				fs:            fs,
				branchVersion: "2.9",
				mockLoad: func(fs billy.Filesystem) (*VersionRules, error) {
					return &VersionRules{
						Rules: map[string]Version{
							"2.9": {Min: "104.0.0", Max: "105.0.0"},
							"2.8": {Min: "103.0.0", Max: "104.0.0"},
							"2.7": {Min: "101.0.0", Max: "103.0.0"},
						},
						DevBranchPrefix:  "dev-v",
						ProdBranchPrefix: "release-v",
					}, nil
				},
			},
			ex: expected{
				vr: &VersionRules{
					Rules: map[string]Version{
						"2.9": {Min: "104.0.0", Max: "105.0.0"},
						"2.8": {Min: "103.0.0", Max: "104.0.0"},
						"2.7": {Min: "101.0.0", Max: "103.0.0"},
					},
					BranchVersion:    "2.9",
					DevBranchPrefix:  "dev-v",
					DevBranch:        "dev-v2.9",
					ProdBranchPrefix: "release-v",
					ProdBranch:       "release-v2.9",
					MinVersion:       101,
					MaxVersion:       105,
				},
				err: nil,
			},
		},
		{
			name: "#5 - branchVersion defined in rules [edge case]",
			i: input{
				fs:            fs,
				branchVersion: "2.10",
				mockLoad: func(fs billy.Filesystem) (*VersionRules, error) {
					return &VersionRules{
						Rules: map[string]Version{
							"2.10": {Min: "105.0.0", Max: "106.0.0"},
							"2.9":  {Min: "104.0.0", Max: "105.0.0"},
							"2.8":  {Min: "103.0.0", Max: "104.0.0"},
							"2.7":  {Min: "101.0.0", Max: "103.0.0"},
						},
						DevBranchPrefix:  "dev-v",
						ProdBranchPrefix: "release-v",
					}, nil
				},
			},
			ex: expected{
				vr: &VersionRules{
					Rules: map[string]Version{
						"2.10": {Min: "105.0.0", Max: "106.0.0"},
						"2.9":  {Min: "104.0.0", Max: "105.0.0"},
						"2.8":  {Min: "103.0.0", Max: "104.0.0"},
						"2.7":  {Min: "101.0.0", Max: "103.0.0"},
					},
					BranchVersion:    "2.10",
					DevBranchPrefix:  "dev-v",
					DevBranch:        "dev-v2.10",
					ProdBranchPrefix: "release-v",
					ProdBranch:       "release-v2.10",
					MinVersion:       103,
					MaxVersion:       106,
				},
				err: nil,
			},
		},
		{
			name: "#6 - branchVersion defined in rules [edge case]",
			i: input{
				fs:            fs,
				branchVersion: "2.7",
				mockLoad: func(fs billy.Filesystem) (*VersionRules, error) {
					return &VersionRules{
						Rules: map[string]Version{
							"2.10": {Min: "105.0.0", Max: "106.0.0"},
							"2.9":  {Min: "104.0.0", Max: "105.0.0"},
							"2.8":  {Min: "103.0.0", Max: "104.0.0"},
							"2.7":  {Min: "101.0.0", Max: "103.0.0"},
							"2.6":  {Min: "100.0.0", Max: "101.0.0"},
							"2.5":  {Min: "", Max: "100.0.0"},
						},
						DevBranchPrefix:  "dev-v",
						ProdBranchPrefix: "release-v",
					}, nil
				},
			},
			ex: expected{
				vr: &VersionRules{
					Rules: map[string]Version{
						"2.10": {Min: "105.0.0", Max: "106.0.0"},
						"2.9":  {Min: "104.0.0", Max: "105.0.0"},
						"2.8":  {Min: "103.0.0", Max: "104.0.0"},
						"2.7":  {Min: "101.0.0", Max: "103.0.0"},
						"2.6":  {Min: "100.0.0", Max: "101.0.0"},
						"2.5":  {Min: "", Max: "100.0.0"},
					},
					BranchVersion:    "2.7",
					DevBranchPrefix:  "dev-v",
					DevBranch:        "dev-v2.7",
					ProdBranchPrefix: "release-v",
					ProdBranch:       "release-v2.7",
					MinVersion:       0,
					MaxVersion:       103,
				},
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dependencies{RootFs: tt.i.fs}
			vr, err := d.rules(tt.i.branchVersion, tt.i.mockLoad)
			if tt.ex.err == nil {
				assert.Nil(t, err, "Expected nil error")
			} else {
				assert.Equal(t, tt.ex.err, err, "Expected error")
			}

			assert.Equal(t, tt.ex.vr, vr, "Expected VersionRules")
		})
	}
}

// func TestGetVersionRules(t *testing.T) {
// 	t.Run("branchVersion is empty", func(t *testing.T) {
// 		_, err := GetVersionRules("", false)
// 		if err == nil {
// 			t.Errorf("Expected error when branchVersion is empty")
// 		}
// 	})

// 	t.Run("branchVersion is not convertible to float32", func(t *testing.T) {
// 		_, err := GetVersionRules("not a float", false)
// 		if err == nil {
// 			t.Errorf("Expected error when branchVersion is not convertible to float32")
// 		}
// 	})

// 	t.Run("branchVersion is not defined in the rules", func(t *testing.T) {
// 		_, err := GetVersionRules("3.0", false)
// 		if err == nil {
// 			t.Errorf("Expected error when branchVersion is not defined in the rules")
// 		}
// 	})

// 	t.Run("branchVersion is defined in the rules for branch: 2.9", func(t *testing.T) {
// 		vr, err := GetVersionRules("2.9", false)
// 		if err != nil {
// 			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
// 		}
// 		switch {
// 		case vr.BranchVersion != 2.9:
// 			t.Errorf("Expected branchVersion to be 2.9, got %v", vr.BranchVersion)
// 		case vr.MinVersion != 101:
// 			t.Errorf("Expected minVersion to be 101, got %v", vr.MinVersion)
// 		case vr.MaxVersion != 105:
// 			t.Errorf("Expected maxVersion to be 105, got %v", vr.MaxVersion)
// 		}
// 	})

// 	t.Run("branchVersion is defined in the rules for branch: 2.8", func(t *testing.T) {
// 		vr, err := GetVersionRules("2.8", false)
// 		if err != nil {
// 			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
// 		}
// 		switch {
// 		case vr.BranchVersion != 2.8:
// 			t.Errorf("Expected branchVersion to be 2.8, got %v", vr.BranchVersion)
// 		case vr.MinVersion != 100:
// 			t.Errorf("Expected minVersion to be 100, got %v", vr.MinVersion)
// 		case vr.MaxVersion != 104:
// 			t.Errorf("Expected maxVersion to be 104, got %v", vr.MaxVersion)
// 		}
// 	})

// 	t.Run("branchVersion is defined in the rules for branch: 2.7", func(t *testing.T) {
// 		vr, err := GetVersionRules("2.7", false)
// 		if err != nil {
// 			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
// 		}
// 		switch {
// 		case vr.BranchVersion != 2.7:
// 			t.Errorf("Expected branchVersion to be 2.7, got %v", vr.BranchVersion)
// 		case vr.MinVersion != 0:
// 			t.Errorf("Expected minVersion to be 0, got %v", vr.MinVersion)
// 		case vr.MaxVersion != 103:
// 			t.Errorf("Expected maxVersion to be 103, got %v", vr.MaxVersion)
// 		}
// 	})

// 	t.Run("branchVersion is defined in the rules for branch: 2.6", func(t *testing.T) {
// 		vr, err := GetVersionRules("2.6", false)
// 		if err != nil {
// 			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
// 		}
// 		switch {
// 		case vr.BranchVersion != 2.6:
// 			t.Errorf("Expected branchVersion to be 2.6, got %v", vr.BranchVersion)
// 		case vr.MinVersion != 0:
// 			t.Errorf("Expected minVersion to be 0, got %v", vr.MinVersion)
// 		case vr.MaxVersion != 101:
// 			t.Errorf("Expected maxVersion to be 101, got %v", vr.MaxVersion)
// 		}
// 	})

// 	t.Run("branchVersion is defined in the rules for branch: 2.5", func(t *testing.T) {
// 		vr, err := GetVersionRules("2.5", false)
// 		if err != nil {
// 			t.Errorf("Unexpected error when branchVersion is defined in the rules: %v", err)
// 		}
// 		switch {
// 		case vr.BranchVersion != 2.5:
// 			t.Errorf("Expected branchVersion to be 2.5, got %v", vr.BranchVersion)
// 		case vr.MinVersion != 0:
// 			t.Errorf("Expected minVersion to be 0, got %v", vr.MinVersion)
// 		case vr.MaxVersion != 100:
// 			t.Errorf("Expected maxVersion to be 100, got %v", vr.MaxVersion)
// 		}
// 	})
// }
