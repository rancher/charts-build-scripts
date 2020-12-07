package chart

import (
	"fmt"
)

// BranchOptions represents the options used by branches to be able to configure the way a chart is built using these scripts
type BranchOptions struct {
	// ExportOptions represents any options that are configurable when exporting a chart
	ExportOptions ExportOptions `yaml:"export"`
	// CleanOptions represents any options that are configurable when cleaning a chart
	CleanOptions CleanOptions `yaml:"clean"`
}

// ExportOptions represent any options that are configurable when exporting a chart
type ExportOptions struct {
	// Whether to prevent a package from overwriting an existing chart
	// Should be disabled for Source branch and enabled for Staging / Live branches
	PreventOverwrite bool `yaml:"allowOverwrite"`
}

func (e ExportOptions) String() string {
	return fmt.Sprintf("{overwriteExistingCharts: %t}", !e.PreventOverwrite)
}

// CleanOptions represent any options that are configurable when cleaning charts
type CleanOptions struct {
	// Whether to avoid cleaning the generated assets on a clean
	// Should be disabled for Source branch and enabled for Staging / Live branches
	PreventCleanAssets bool `yaml:"cleanAssets"`
}

func (c CleanOptions) String() string {
	return fmt.Sprintf("{cleanAssets: %t}", !c.PreventCleanAssets)
}
