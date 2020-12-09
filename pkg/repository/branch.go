package repository

import "fmt"

// BranchesConfiguration represents any special roles that certain branches hold in a charts repository
type BranchesConfiguration struct {
	// Source is where developers should push changes to
	Source BranchConfiguration `yaml:"source"`
	// Staging is where merged changes should be tested before a release
	Staging BranchConfiguration `yaml:"staging"`
	// Live contains assets that have already been released
	Live BranchConfiguration `yaml:"live"`
}

func (b BranchesConfiguration) String() string {
	return fmt.Sprintf("source=%s,staging=%s,live=%s", b.Source, b.Staging, b.Live)
}

// BranchConfiguration represents the configuration of a specific branch
type BranchConfiguration struct {
	Name    string        `yaml:"name"`
	Options BranchOptions `yaml:"options"`
}

func (b BranchConfiguration) String() string {
	return fmt.Sprintf("%s[%s]", b.Name, b.Options)
}

// BranchOptions represents the options used by branches to be able to configure the way a chart is built using these scripts
type BranchOptions struct {
	// SyncOptions represents any options that are configurable when syncing with another branch
	SyncOptions SyncOptions `yaml:"sync"`
	// ValidateOptions represent any options that are configurable when validating a chart
	ValidateOptions ValidateOptions `yaml:"validate"`
}

func (b BranchOptions) String() string {
	return fmt.Sprintf("syncOptions: %v", b.SyncOptions)
}

// SyncOptions represent any options that are configurable when exporting a chart
type SyncOptions []CompareGeneratedAssetsOptions

// ValidateOptions represent any options that are configurable when validating a chart
type ValidateOptions []CompareGeneratedAssetsOptions

// CompareGeneratedAssetsOptions represent any options that are configurable when comparing the generated assets of the current branch against another branch
type CompareGeneratedAssetsOptions struct {
	// WithBranch is the branch whose assets will be used for the sync
	WithBranch string `yaml:"withBranch"`
	// DropReleaseCandidates indicates that we should drop the release candidate versions from the current branch when comparing with WithBranch
	DropReleaseCandidates bool `yaml:"dropReleaseCandidates"`
}
