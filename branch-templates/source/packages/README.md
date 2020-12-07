# Packages

Each package stored in this directory corresponds to the latest version of a forked Helm chart.

Each directory within `packages/` contains the following example directory structure:
```text
packages/${CHART_NAME}/
  package.yaml              # metadata manifest containing upstream chart location, package version
  ${CHART_NAME}.patch       # patch file containing the diff between modified chart and upstream
  overlay/*                 # overlay files that needs to added on top of upstream, for example, questions.yaml
```