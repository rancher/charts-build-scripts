# Package Dependency Map

## Dependency Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                        APPLICATION LAYER                        │
│                           main.go                               │
└────────────────┬────────────────┬────────────────┬──────────────┘
                 │                │                │
        ┌────────▼────────┐  ┌────▼──────────┐  ┌─▼────────────┐
        │   AUTOMATION    │  │  VALIDATION   │  │  REGISTRIES  │
        ├────────────────┤  ├────────────────┤  ├──────────────┤
        │   pkg/auto     │  │ pkg/validate   │  │pkg/registries│
        │                │  │                │  │              │
        │ - chart_bump   │  │ - lifecycle    │  │ - OCI ops    │
        │ - forward_port │  │ - pull_request │  │ - image sync │
        │ - release      │  │ - charts repo  │  │ - remote push│
        │ - versioning   │  │ - index files  │  │              │
        └────────┬───────┘  └────────┬───────┘  └──────┬───────┘
                 │                   │                  │
        ┌────────▼───────────────────▼──────────────────▼──────┐
        │                  DOMAIN LOGIC LAYER                   │
        ├──────────────────────────────────────────────────────┤
        │  pkg/charts/          pkg/helm/                      │
        │                                                      │
        │ - Package           - helm wrapper                   │
        │ - Chart             - standardize                    │
        │ - parse             - metadata                       │
        │ - dependencies      - zip/unzip                      │
        │                     - index management               │
        │  pkg/change/                                         │
        │ - diff gen                                           │
        │ - patch apply                                        │
        └────────┬──────────────────┬───────────────────────────┘
                 │                  │
        ┌────────▼────────┐ ┌───────▼────────────────────────┐
        │ INFRASTRUCTURE  │ │       CONFIGURATION             │
        ├────────────────┤ ├────────────────────────────────┤
        │  pkg/puller    │ │  pkg/config      pkg/options   │
        │                │ │                                │
        │ - git pulling  │ │ - Config struct  - YAML structs│
        │ - cache        │ │ - context inject - PackageOpts │
        │ - templates    │ │ - VersionRules   - ReleaseOpts │
        │                │ │ - TrackedCharts  - ChartOptions│
        └────────┬───────┘ └────────┬───────────────────────┘
                 │                  │
        ┌────────▼──────────────────▼──────────────┐
        │         CORE INFRASTRUCTURE LAYER         │
        ├──────────────────────────────────────────┤
        │  pkg/filesystem/        pkg/git/          │
        │                                          │
        │ - file ops            - git wrapper      │
        │ - WalkDir             - clone/fetch      │
        │ - abstraction         - branch ops       │
        └──────────────────────┬───────────────────┘
                               │
        ┌──────────────────────▼────────────────┐
        │              LOGGING                  │
        ├───────────────────────────────────────┤
        │  pkg/logger/                          │
        │                                       │
        │ - slog wrapper                        │
        │ - structured logging                  │
        └───────────────────────────────────────┘
```

## Detailed Import Dependency Matrix

### Outbound Dependencies by Package

#### pkg/auto
```
├── pkg/charts
├── pkg/config
├── pkg/filesystem
├── pkg/git
├── pkg/helm
├── pkg/logger
├── pkg/options
├── pkg/validate
└── (external: blang/semver, go-billy, os, fmt)
```

#### pkg/validate
```
├── pkg/charts
├── pkg/config
├── pkg/filesystem
├── pkg/git  (also aliased as bashGit)
├── pkg/helm
├── pkg/logger
├── pkg/options
├── pkg/puller
└── (external: go-git/v5, google/go-cmp, github API)
```

#### pkg/charts
```
├── pkg/change
├── pkg/config
├── pkg/filesystem
├── pkg/helm
├── pkg/logger
├── pkg/options
├── pkg/puller
└── (external: blang/semver, go-billy)
```

#### pkg/helm
```
├── pkg/config
├── pkg/filesystem
├── pkg/logger
└── (external: helm.sh, go-billy)
```

#### pkg/registries
```
├── pkg/charts
├── pkg/config
├── pkg/filesystem
├── pkg/git
├── pkg/logger
├── pkg/options
└── (external: helm.sh, google/go-containerregistry)
```

#### pkg/config
```
├── pkg/filesystem
├── pkg/git
├── pkg/logger
├── pkg/options
└── (external: go-billy, yaml, errors)
```

#### pkg/puller
```
├── pkg/config
├── pkg/filesystem
├── pkg/git
├── pkg/logger
├── pkg/options
└── (external: go-billy)
```

#### pkg/change
```
├── pkg/config
├── pkg/filesystem
├── pkg/logger
└── (external: os/exec)
```

#### pkg/options
```
├── pkg/filesystem
├── pkg/logger
└── (external: yaml, os, errors)
```

#### pkg/filesystem
```
├── pkg/logger
└── (external: go-billy, filepath, os, errors)
```

#### pkg/git
```
├── pkg/filesystem
├── pkg/logger
└── (external: go-git/v5)
```

#### pkg/logger
```
└── (external: slog, runtime, context, os, time)
```

## Reverse Dependencies (What Depends on Each Package)

### pkg/config
- Depended on by: `auto`, `validate`, `charts`, `helm`, `registries`, `puller`, `change`
- Status: CRITICAL — central DI hub, flows via context

### pkg/logger
- Depended on by: everything
- Status: CRITICAL — pervasive

### pkg/filesystem
- Depended on by: `config`, `options`, `validate`, `charts`, `helm`, `puller`, `change`, `git`, `registries`, `auto`
- Status: CRITICAL — infrastructure

### pkg/helm
- Depended on by: `auto`, `validate`, `charts`, `registries`
- Status: IMPORTANT — domain logic; also owns zip/unzip and standardize

### pkg/options
- Depended on by: `config`, `validate`, `charts`, `registries`, `puller`
- Status: IMPORTANT — configuration structs

### pkg/charts
- Depended on by: `auto`, `validate`, `registries`
- Status: IMPORTANT — package/chart domain model

### pkg/git
- Depended on by: `config`, `validate`, `puller`, `registries`, `auto`
- Status: IMPORTANT — git operations

### pkg/puller
- Depended on by: `validate`, `charts`
- Status: MODERATE — remote pulling and cache

### pkg/change
- Depended on by: `charts`
- Status: MODERATE — diff/patch generation

## Removed Packages (post-refactor)

| Package | Removed in | Functionality moved to |
|---|---|---|
| `pkg/lifecycle` | refactor-auto-forward-ports | `pkg/validate` |
| `pkg/standardize` | refactor-auto-forward-ports | `pkg/helm` |
| `pkg/zip` | refactor-auto-forward-ports | `pkg/helm` |
| `pkg/update` | refactor-auto-forward-ports | `pkg/puller` |
| `pkg/path` | refactor-auto-forward-ports | `pkg/config` (as constants) |
| `pkg/util` | refactor-auto-forward-ports | `pkg/config` (soft error mode) |

## Potential Circular Dependency Risks

**Current Status**: No circular dependencies detected

**High Risk Areas** (avoided):
- `config` does not import from high-level packages (`auto`, `validate`)
- `options` does not import from domain packages (`charts`, `helm`)
- `filesystem` does not import domain packages
- `helm` does not import `charts` or `validate`

## Critical Dependency Chains

### Config Loading Chain
```
main.go (init())
  ↓
config.Init(ctx, repoRoot, fs)
  ├→ options.LoadReleaseOptionsFromFile(ctx, fs)
  ├→ git.Init(repoRoot)
  ├→ loadTrackedCharts(ctx, fs)
  ├→ loadVersionRules(ctx, fs)
  └→ config.WithConfig(ctx, cfg)  →  stored in global Ctx
```

### Chart Preparation Chain
```
Package.Prepare(ctx)
  ├→ config.FromContext(ctx)
  ├→ Chart.Prepare(ctx)
  │  ├→ puller.Pull(ctx) ────→ git.Clone()
  │  ├→ change.GenerateChanges()
  │  │  ├→ change.GeneratePatchDiff()
  │  │  └→ change.ApplyPatchDiff()
  │  └→ helm.StandardizeChartYaml()
  └→ AdditionalChart.Prepare(ctx)
```

### Release Chain
```
auto.Release(ctx, chartVersion, chart)
  ├→ config.FromContext(ctx)
  ├→ validate.LoadStateFile(ctx)
  ├→ auto.PullAsset(branch, assetPath, cfg.Repo)
  │  └→ git.FetchBranch / git.CheckoutFile
  ├→ helm.DumpAssets(ctx, asset)
  ├→ helm.CreateOrUpdateHelmIndex(ctx)
  ├→ auto.PullIcon(ctx, rootFs, repo, chart, version, branch)
  ├→ auto.UpdateReleaseYaml(ctx, ...)
  └→ helm.CreateOrUpdateHelmIndex(ctx)
```

### Forward Port Chain
```
auto.ForwardPort(ctx, branch)
  ├→ config.FromContext(ctx)
  ├→ validate.LifecycleStatus(ctx, branchVersion)
  │  └→ returns status.ToForwardPort map[string][]string
  └→ per chart: auto.PullAsset + helm ops + git push + PR creation
```

### Lifecycle Status Chain
```
validate.LifecycleStatus(ctx, branchVersion)
  ├→ config.FromContext(ctx)
  ├→ helm.GetAssetsVersionsMap(ctx)
  ├→ helm.RemoteIndexYaml(ctx)
  └→ returns *validate.Status{ToRelease, ToForwardPort}
```

## Data Flow Diagram

```
┌────────────────────────────────────────────────────────────────┐
│                        User Input (CLI)                        │
└────────────────┬─────────────────────────────────────────────┘
                 │
                 ▼
        ┌────────────────┐
        │  main.go       │
        │ init(): load   │
        │ config → Ctx   │
        └────────┬───────┘
                 │
                 ▼
        ┌────────────────────┐
        │ config.Init()      │
        │ Load YAML configs  │
        │ Setup context      │
        └────────┬───────────┘
                 │
        ┌────────▼──────────┐
        │  Command Handler  │
        │  (auto/validate/  │
        │   registries)     │
        └────────┬──────────┘
                 │
        ┌────────▼──────────────────────────┐
        │  Domain Logic                     │
        │  (charts, helm, registries)       │
        └────────┬──────────────────────────┘
                 │
        ┌────────▼──────────────────────────┐
        │  Infrastructure                   │
        │  (filesystem, git, puller)        │
        └────────┬──────────────────────────┘
                 │
        ┌────────▼──────────────────────────┐
        │  External Systems                 │
        │  (GitHub, OCI Registry, Helm)     │
        └────────────────────────────────────┘
```

## Context Flow

```
Context at Entry Point (main.go init()):
  ctx := context.Background()
  cfg, err := config.Init(ctx, repoRoot, fs)
  Ctx = config.WithConfig(ctx, cfg)  // stored as global

Context Propagation:
  All functions take context.Context as first parameter

  func DoWork(ctx context.Context, ...) error {
      cfg, err := config.FromContext(ctx)
      if err != nil { return err }

      logger.Log(ctx, level, "msg", attrs...)

      // Pass ctx to nested calls
      return nestedFunc(ctx, ...)
  }

Benefits:
  ✓ Cancellation propagation
  ✓ Timeout support
  ✓ Value threading (Config)
  ✓ Structured logging context
  ✓ Single initialization point (init())
```
