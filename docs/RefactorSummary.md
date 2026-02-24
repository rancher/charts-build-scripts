# Refactoring Summary: Major Reorganization

**Current Branch**: `refactor-auto-forward-ports`

## What Changed: Before vs. After

### BEFORE Refactoring

```
pkg/
├── auto/
│   ├── auto.go              ← Contains ForwardPort, base shared structs
│   ├── auto_test.go
│   ├── chart_bump.go
│   ├── oci.go               ← OCI registry operations
│   ├── validate.go          ← Validation operations
│   ├── forward_port.go
│   ├── release.go
│   └── versioning.go
│
├── lifecycle/               ← Mixed responsibility package
│   ├── lifecycle.go
│   ├── logs.go
│   ├── parser.go            ← Parse version rules
│   ├── state.go
│   ├── status.go
│   ├── version_rules.go     ← Version management logic
│   └── version_rules_test.go
│
├── diff/                    ← Small package
│   └── diff.go
│
├── standardize/             ← Small package
│   └── standardize.go
│
├── icons/                   ← Small package
│   └── icons.go
│
├── validate/
│   ├── validate.go
│   └── pull_requests.go
│
└── ... (other packages)
```

**Issues**:
- `auto.go` was kitchen sink of shared types
- `lifecycle/` had multiple unrelated responsibilities
- Small packages (`diff`, `standardize`, `icons`) were scattered
- Configuration and validation mixed across packages
- OCI operations spread between `auto` and `registries`

### AFTER Refactoring

```
pkg/
├── auto/                     ← PURE automation logic
│   ├── chart_bump.go
│   ├── chart_bump_test.go
│   ├── forward_port.go
│   ├── forward_port_test.go
│   ├── release.go
│   ├── release_test.go
│   ├── remove.go
│   ├── versioning.go
│   └── versioning_test.go
│
├── config/                   ← NEW: Centralized configuration
│   ├── config.go            ← Main Config struct
│   ├── context.go           ← NEW: Context-based injection
│   ├── charts.go            ← NEW: TrackedCharts from lifecycle
│   ├── versions.go          ← NEW: VersionRules from lifecycle
│   └── path.go              ← NEW: Path constants from pkg/path
│
├── validate/                 ← CONSOLIDATED: All validation
│   ├── validate.go
│   ├── charts.go
│   ├── pull_requests.go
│   ├── pull_requests_test.go
│   ├── icons.go             ← MOVED: From pkg/icons
│   ├── lifecycle.go         ← MOVED: From pkg/lifecycle
│   └── versions.go
│
├── helm/                     ← CONSOLIDATED: All Helm operations
│   ├── helm.go
│   ├── helm_test.go
│   ├── standardize.go       ← MOVED: From pkg/standardize
│   ├── zip.go
│   ├── crds.go
│   ├── export.go
│   └── metadata.go
│
├── registries/              ← CONSOLIDATED: All registry operations
│   ├── oci.go               ← MOVED: From pkg/auto
│   ├── oci_test.go
│   ├── remote.go
│   ├── remote_test.go
│   ├── slsactl.go           ← NEW: SLSA attestation (replaces cosign.go)
│   ├── assets.go
│   ├── registries.go
│   └── mock_data.go
│
├── change/                  ← CONSOLIDATED: All diff/patch
│   ├── diff.go              ← MOVED: From pkg/diff
│   ├── apply.go
│   └── generate.go
│
└── ... (cleanup packages)
```

**Improvements**:
- Clear responsibility boundaries
- No kitchen-sink packages
- Central config hub
- Consistent import patterns
- Small packages consolidated
- Lifecycle functionality distributed logically

## Deleted Files (15 files)

### Package Deletions

1. **pkg/lifecycle/** (entire package - 7 files)
   - `lifecycle.go`
   - `logs.go`
   - `parser.go`
   - `state.go`
   - `status.go`
   - `version_rules.go`
   - `version_rules_test.go`
   - `mocks/test.yaml`

2. **pkg/diff/** (entire package)
   - `diff.go` → Moved to `pkg/change/diff.go`

3. **pkg/standardize/** (entire package)
   - `standardize.go` → Moved to `pkg/helm/standardize.go`

4. **pkg/icons/** (entire package)
   - `icons.go` → Moved to `pkg/validate/icons.go`

5. **pkg/path/** (entire package)
   - `path.go` → Constants moved to `pkg/config/path.go`

6. **pkg/util/** (entire package)
   - `util.go` → Soft error mode merged into `pkg/config`

7. **pkg/update/** (entire package)
   - `pull.go` → Moved to `pkg/puller/update.go`

### File Deletions

5. **pkg/auto/auto.go** (kitchen sink)
   - Content distributed across existing `auto/` files

6. **pkg/auto/auto_test.go**
   - Tests distributed to respective test files

7. **pkg/auto/oci.go**
   - Moved to `pkg/registries/oci.go`

8. **pkg/auto/validate.go**
   - Moved to `pkg/validate/validate.go` (merged)

## New Files (16 files)

### Configuration Package (4 files)

1. **pkg/config/context.go** (NEW)
   - `WithConfig(ctx, cfg)` - Attach config to context
   - `FromContext(ctx)` - Retrieve config from context
   - Enables dependency injection pattern

2. **pkg/config/charts.go** (NEW)
   - `ChartEntry` struct - Chart metadata
   - `TrackedCharts` struct - All tracked charts
   - `loadTrackedCharts()` - Load from trackCharts.yaml
   - Chart status helpers

3. **pkg/config/versions.go** (NEW from lifecycle)
   - `VersionRules` struct - Version management
   - Version retention policy calculation

4. **pkg/config/path.go** (NEW from path)
   - All path constants previously in `pkg/path/path.go`
   - Cache, config, and asset path definitions

### Validation Package (3 files)

4. **pkg/validate/icons.go** (NEW from icons)
   - `ValidateIcons(ctx)` - Check icons exist
   - `IsIconException(cfg, chart)` - Handle special cases
   - `loadAndCheckIconPrefix()` - Verify icon paths

5. **pkg/validate/lifecycle.go** (NEW from lifecycle)
   - `Status` struct - Release/forward-port status
   - `LifecycleStatus(ctx, branchVersion)` - Identify needed actions
   - `ListMissingAssetsVersions()` - Compare versions

6. **pkg/validate/versions.go** (NEW)
   - `ValidateVersionStandards()` - Ensure version format compliance

### Change Package (1 file)

7. **pkg/change/diff.go** (NEW from diff)
   - `GeneratePatchDiff()` - Create unified diffs
   - `ApplyPatchDiff()` - Apply patch files

### Helm Package (1 file)

8. **pkg/helm/standardize.go** (NEW from standardize)
   - `RestructureChartsAndAssets()` - Reorganize chart structure
   - `standardizeAssetsFromCharts()` - Normalize assets

### Registries Package (2 files)

9. **pkg/registries/oci.go** (NEW from auto)
   - `PushToOci(ctx, ...)` - Push charts to OCI registry
   - OCI registry wrapper structs

10. **pkg/registries/oci_test.go** (NEW)
    - OCI operations unit tests

11. **pkg/registries/slsactl.go** (NEW)
    - SLSA attestation via rancherlabs/slsactl; replaces former cosign-based signing

### Test Files (2 files)

12. **pkg/auto/forward_port_test.go** (NEW)
    - Forward port operation tests

### Puller Package (2 files)

13. **pkg/puller/cache.go** (NEW)
    - Cache logic extracted from existing code

14. **pkg/puller/update.go** (NEW from update)
    - Template update logic moved from `pkg/update/pull.go`

## Changed Files (27 files)

### Major Changes

#### main.go
- Global variables reorganized
- Config initialization updated to use `config.Init()`
- All commands updated to use `config.FromContext()`
- Import updated to use new package locations

#### pkg/config/config.go
- Complete rewrite with new architecture
- Now loads all configuration
- Manages VersionRules and TrackedCharts
- Uses context-based dependency injection

#### pkg/auto/chart_bump.go
- Removed dependency on `auto.go`
- Updated to use `config.FromContext()`
- Imports reorganized

#### pkg/auto/forward_port.go
- Removed base types from `auto.go`
- Updated logging patterns
- Context-based config access

#### pkg/auto/release.go
- Simplified imports
- Uses new `config.FromContext()` pattern

#### pkg/auto/versioning.go
- Updated version comparisons
- References to `lifecycle` removed
- Uses `config.VersionRules`

#### pkg/validate/validate.go
- Now main validation orchestrator
- Merged functionality from old `auto/validate.go`
- Uses new config structure
- Calls reorganized validate functions

#### pkg/validate/charts.go
- Updated to use new config
- References updated to new package locations

#### pkg/validate/pull_requests.go
- Import adjustments for moved files
- Uses new validation utilities

#### pkg/charts/package.go
- Config access pattern updated
- Imports adjusted for moved files
- Uses new chart constants from path

#### pkg/charts/parse.go
- Updated for new config structure
- Imports reorganized

#### pkg/helm/helm.go
- Updated for new config access pattern
- Uses `pkg/options` directly
- Simplified imports

#### pkg/helm/helm_test.go
- Tests updated for new structure
- Mock setup revised

#### pkg/filesystem/assets.go
- Minor import adjustments

#### pkg/filesystem/yaml.go
- No functional changes
- Import cleanup

#### pkg/registries/cosign.go
- Updated context usage
- Import path adjustments

#### pkg/registries/registries.go
- Updated for new config location

#### pkg/registries/remote.go
- Uses new config pattern
- OCI operations better organized

## Import Pattern Changes

### BEFORE Pattern
```go
import (
    "github.com/rancher/charts-build-scripts/pkg/lifecycle"
    "github.com/rancher/charts-build-scripts/pkg/auto"
)

// Scattered config access
versionRules := lifecycle.NewVersionRules(...)
forwardPort := &auto.ForwardPort{...}
```

### AFTER Pattern
```go
import (
    "github.com/rancher/charts-build-scripts/pkg/config"
    "github.com/rancher/charts-build-scripts/pkg/auto"
    "github.com/rancher/charts-build-scripts/pkg/validate"
)

// Centralized config access
cfg, err := config.FromContext(ctx)
if err != nil {
    return err
}
// Use cfg.VersionRules, cfg.TrackedCharts, etc.
```

## Architecture Improvements

### 1. Single Source of Truth
- Config loaded once at startup
- Reused everywhere via context
- No duplicate configuration loading
- Consistent state across operations

### 2. Clear Package Responsibilities

| Package | Responsibility |
|---------|----------------|
| config | Load and provide configuration; owns path constants and soft error mode |
| logger | Handle all logging |
| options | Define YAML structures (PackageOptions, ReleaseOptions, ChartOptions) |
| filesystem | Filesystem abstraction |
| git | Git operations |
| charts | Chart types and operations |
| helm | Helm CLI wrapper; owns zip/unzip and standardize |
| validate | All validation logic; owns lifecycle status |
| auto | Automation (releases, forward-port, chart-bump) |
| registries | OCI registry operations; owns SLSA attestation |
| change | Patch/diff operations |
| puller | Fetch upstream charts; owns template updates and cache |

### 3. Dependency Injection
- Context-based configuration passing
- Easier to test (mock config in context)
- No global state (except logger)
- Cleaner function signatures

### 4. No Circular Dependencies
- Clear hierarchy maintained
- Low-level packages never import high-level
- Config is leaf of infrastructure layer
- Safe for concurrent operations

## Breaking Changes

### For Users of This Library

1. **Import paths changed**:
   ```go
   // OLD
   import "github.com/rancher/charts-build-scripts/pkg/lifecycle"
   
   // NEW
   import "github.com/rancher/charts-build-scripts/pkg/config"
   ```

2. **Config access pattern changed**:
   ```go
   // OLD
   vr := lifecycle.NewVersionRules(...)
   
   // NEW
   cfg, _ := config.FromContext(ctx)
   vr := cfg.VersionRules
   ```

3. **OCI operations moved**:
   ```go
   // OLD
   import "github.com/rancher/charts-build-scripts/pkg/auto"
   auto.PushToOci(...)
   
   // NEW
   import "github.com/rancher/charts-build-scripts/pkg/registries"
   registries.PushToOci(...)
   ```

### For Developers

1. Context must contain Config (via `config.WithConfig()`)
2. All functions must accept `context.Context` as first parameter
3. Package initialization order matters (config before operations)

## Benefits Summary

Organized from highest impact to lowest:

1. **Maintainability**: Clear responsibilities, easier to understand code flow
2. **Testability**: Context-based DI makes mocking easier
3. **Scalability**: Adding new packages doesn't require touching existing ones
4. **Consistency**: All packages follow same patterns
5. **Debugging**: Clear separation helps isolate issues
6. **Performance**: Minimal overhead, all operations lean on context
7. **Future-proofing**: Architecture supports future features (prime-charts)

## Migration Checklist for External Code

If you have code using charts-build-scripts:

- [ ] Update imports from `lifecycle` to `config` / `validate`
- [ ] Update imports from `icons` to `validate`
- [ ] Update imports from `standardize` to `helm`
- [ ] Update imports from `diff` to `change`
- [ ] Update imports from `path` to `config`
- [ ] Update imports from `util` to `config`
- [ ] Update imports from `update` to `puller`
- [ ] Update OCI imports from `auto` to `registries`
- [ ] Replace direct struct usage with `config.FromContext(ctx)`
- [ ] Ensure all functions accept `context.Context` as first param
- [ ] Initialize config with `config.WithConfig(ctx, cfg)`
- [ ] Test thoroughly - architecture changes are significant

