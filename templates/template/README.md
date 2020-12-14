{{- if eq (len .SyncOptions) 0 }}
{{- if ge (len .ValidateOptions) 0 }}
## Source Branch

This branch contains packages that contain Packages that will be synced to another branch. See the README.md under `packages/` for more information.

The following directory structure is expected:
```text
package/
  <package>/
```

{{- end }}
{{- end }}

{{- if ge (len .SyncOptions) 0 }}
{{- if ge (len .ValidateOptions) 0 }}
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
{{- end }}
{{- if ge (len .SyncOptions) 0 }}
{{- if eq (len .ValidateOptions) 0 }}
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
{{- end }}
### Configuration

This repository branch contains a `configuration.yaml` file that is used to specify how it interacts with other repository branches.

{{- if ge (len .SyncOptions) 0 }}
#### Sync

This branch syncs with the generated assets from the following branches:
{{- range .SyncOptions }}
- {{ .Branch }} at {{ .UpstreamOptions.URL }}{{ if .DropReleaseCandidates }} (only latest assets){{ end }}
{{- end }}

To release a new version of a chart, please open the relevant PRs to one of these branches. Merging should trigger a sync workflow on pushing to these branches.

{{- end }}
{{- if ge (len .ValidateOptions) 0 }}
#### Validate

This branch validates against the generated assets of the following branches to make sure it isn't overriding already released charts.
{{- range .ValidateOptions }}
- {{ .Branch }} at {{ .UpstreamOptions.URL }}{{ if .DropReleaseCandidates }} (only latest assets){{ end }}
{{- end }}

Before submitting any PRs, a Github Workflow will check to see if your package doesn't break any already released packages in these repository branches.

{{- end }}

### Makefile

#### Package-Level (requires packages/ to exist)

`make prepare`: Pulls in your charts from upstream and creates a basic `generated-changes/` directory with your dependencies from upstream

`make patch`: Updates your `generated-changes/` to reflect the difference between upstream and the current working directory of your branch. Requires prepare

`make charts`: Runs prepare and then exports your charts to `assets/` and `charts/` and generates or updates your `index.yaml`. Can be used for testing; a Rancher Helm Repository that points to a branch that has these directories with the `index.yaml` should be able to find and deploy working copies of your chart.

`make clean`: Cleans up all the working directories of charts to get your repository ready for a PR

#### Repository-Branch-Level (requires either packages/ or assets/ + charts/)

`make pull-scripts`: Pulls in the version of the `charts-build-scripts` indicated in scripts

`make validate`: Validates your current repository branch against all the repository branches indicated in your configuration.yaml

`make sync`: Syncs the assets in your current repository with the merged contents of all of the repository branches indicated in your configuration.yaml

`make update`: Pulls in the latest docs, scripts, etc. from the charts-build-scripts repository