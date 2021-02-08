{{ if (eq .Template "source") -}}
## Source Branch

This branch contains packages that contain Packages that will be synced to another branch.

See the README.md under `packages/` for more information.

The following directory structure is expected:
```text
package/
  <package>/
```
{{- end }}

{{- if (eq .Template "staging") -}}
## Staging Branch

This branch contains generated assets that have not been officially released yet.

The following directory structure is expected:
```text
assets/
  <package>/
    <chart>-<packageVersion>.tgz
  ...
charts/
  <package>
    <chart>
      <packageVersion>
        # Unarchived Helm chart
  ...
```
{{- end }}

{{- if (eq .Template "live") -}}
## Live Branch

This branch contains generated assets that have been officially released on {{ .HelmRepoConfiguration.CNAME }}.

The following directory structure is expected:
```text
assets/
  <package>/
    <chart>-<packageVersion>.tgz
  ...
charts/
  <package>
    <chart>
      <packageVersion>
        # Unarchived Helm chart
```
{{- end }}

### Configuration

This repository branch contains a `configuration.yaml` file that is used to specify how it interacts with other repository branches.

{{- if .SyncOptions }}

#### Sync

This branch syncs with the generated assets from the following branches:
{{- range .SyncOptions }}
- {{ .Branch }} at {{ .UpstreamOptions.URL }}{{ if .DropReleaseCandidates }} (only latest assets){{ end }}
{{- end }}

To release a new version of a chart, please open the relevant PRs to one of these branches. 

Merging should trigger a sync workflow on pushing to these branches.

{{- end }}
{{- if .ValidateOptions }}

#### Validate

This branch validates against the generated assets of the following branches to make sure it isn't overriding already released charts.
{{- range .ValidateOptions }}
- {{ .Branch }} at {{ .UpstreamOptions.URL }}{{ if .DropReleaseCandidates }} (only latest assets){{ end }}
{{- end }}

Before submitting any PRs, a Github Workflow will check to see if your package doesn't break any already released packages in these repository branches.

{{- end }}

{{- if (eq .Template "source") }}

### Making Changes

As a developer making changes to a particular package, you will usually follow the following steps:

#### If this is the first time you are adding a package:

```shell
PACKAGE=<packageName>
mkdir -p packages/${PACKAGE}
touch packages/${PACKAGE}/package.yaml
```

See `packages/README.md` to configure the `packages/${PACKAGE}/package.yaml` file based on the Package that you are planning to add.

To make changes, see the steps listed below.

#### If the package already exists

If you are working with a single Package, set `export PACKAGE=<packageName>` to inform the scripts that you only want to make changes to a particular package.

This will prevent the scripts from running commands on every package in this repository.

You'll also want to update the `packageVersion` and `releaseCandidateVersion` located in `packages/${PACKAGE}/package.yaml`.

See the section below for how to update this field.

Once you have made those changes, the Workflow will be:
```shell
make prepare # Instantiates the chart in the workingDir specified in the package.yaml
# Make your changes here to the workingDir directly here
make patch # Saves changes to generated-changes/
make clean # Cleans up your workingDir, leaving behind only the generated-changes/
```

Once your directory is clean, you are ready to submit a PR.

#### Versioning Packages

If this `major.minor.patch` (e.g. `0.0.1`) version of the Chart has never been released, reset the `packageVersion` to `01` and the `releaseCandidateVersion` to `00`.

If this `major.minor.patch` (e.g. `0.0.1`) version of the Chart has been released before:
- If this is the first time you are making a change to this chart for a specific Rancher release (i.e. the current `packageVersion` has already been released in the Live Branch), increment the `packageVersion` by 1 and reset the `releaseCandidateVersion` to `00`.
- Otherwise, only increment the `releaseCandidateVersion` by 1.

{{ end -}}

{{- if (eq .Template "staging") }}

### Cutting a Release

In the Staging branch, cutting a release involves moving the contents of the `assets/` directory into `released/assets/`, deleting the contents of the `charts/` directory, and updating the `index.yaml` to point to the new location for those assets.

This process is entirely automated via the `make release` command.

Once you successfully run the `make release` command, ensure the following is true:
- The `assets/` and `charts/` directories each only have a single file contained within them: `README.md`
- The `released/assets/` directory has a .tgz file for each releaseCandidateVersion of a Chart that was created during this release.
- The `index.yaml` and `released/assets/index.yaml` both are identical and the `index.yaml`'s diff shows only two types of changes: a timestamp update or a modification of an existing URL from `assets/*` to `released/assets/*`.

No other changes are expected.

Note: these steps should be taken only after following the steps to cut a release to your Live Branch.

{{ end -}}

{{- if (eq .Template "live") }}

### Cutting a Release

In the Live branch, cutting a release requires you to run the `make sync` command.

This command will automatically get the latest charts / resources merged into the the branches you sync with (as indicated in this branch's `configuration.yaml`) and will fail if any of those branches try to modify already released assets.

If the `make sync` command fails, you might have to manually make changes to the contents of the Staging Branch to resolve any issues.

Once you successfully run the `make sync` command, the logs outputted will itemize the releaseCandidateVersions picked out from the Staging branch and make exactly two changes:

1. It will update the `Chart.yaml`'s version for each chart to drop the `-rcXX` from it

2. It will update the `Chart.yaml`'s annotations for each chart to drop the `-rcXX` from it only for some special annotations (note: currently, the only special annotation we track is `catalog.cattle.io/auto-install`).

Once you successfully run the `make release` command, ensure the following is true:
- The `assets/` and `charts/` directories each only have a single file contained within them: `README.md`
- The `released/assets/` directory has a .tgz file for each releaseCandidateVersion of a Chart that was created during this release.
- The `index.yaml` and `released/assets/index.yaml` both are identical and the `index.yaml`'s diff shows only two types of changes: a timestamp update or a modification of an existing URL from `assets/*` to `released/assets/*`.

No other changes are expected.

{{ end -}}

### Makefile

#### Basic Commands

`make pull-scripts`: Pulls in the version of the `charts-build-scripts` indicated in scripts.

{{- if (eq .Template "source") }}

`make prepare`: Pulls in your charts from upstream and creates a basic `generated-changes/` directory with your dependencies from upstream

`make patch`: Updates your `generated-changes/` to reflect the difference between upstream and the current working directory of your branch (note: this command should only be run after `make prepare`).

`make clean`: Cleans up all the working directories of charts to get your repository ready for a PR

#### Advanced Commands

`make charts`: Runs `make prepare` and then exports your charts to `assets/` and `charts/` and generates or updates your `index.yaml`.

{{ else }}

`make sync`: Syncs the assets in your current repository with the merged contents of all of the repository branches indicated in your configuration.yaml

{{ end -}}

`make validate`: Validates your current repository branch against all the repository branches indicated in your configuration.yaml

`make docs`: Pulls in the latest docs, scripts, etc. from the charts-build-scripts repository

{{- if (eq .Template "staging") }}

`make release`: moves the contents of the `assets/` directory into `released/assets/`, deletes the contents of the `charts/` directory, and updates the `index.yaml` to point to the new location for those assets.
{{ end -}}