## Makefile

### Basic Commands

`make pull-scripts`: Pulls in the version of the `charts-build-scripts` indicated in scripts.

`make prepare`: Pulls in your charts from upstream and creates a basic `generated-changes/` directory with your dependencies from upstream. *If you are working with a local chart with no dependencies, this command does nothing.*

`make patch`: Updates your `generated-changes/` to reflect the difference between upstream and the current working directory of your branch (note: this command should only be run after `make prepare`). *If you are working with a local chart with no dependencies, this command does nothing.*

`make clean`: Cleans up all the working directories of charts to get your repository ready for a PR. *If you are working with a local chart with no dependencies, this command does nothing.*

`make charts`: Runs `make prepare` and then exports your charts to `assets/` and `charts/` and generates or updates your `index.yaml`.

### Advanced Commands

`make remove`: Removes the asset and chart associated with a provided chart version. Also runs `make index` to remove the entry from the `index.yaml`.

`make list`: Prints the list of all packages tracked in the current repository and recognized by the scripts.

`make index`: Reconstructs the `index.yaml` based on the existing charts. Used by `make charts` and `make validate` under the hood.

`make unzip`: Reconstructs all charts in the `charts` directory based on the current contents in `assets`. Can be scoped to specific charts via specifying `CHART={chart}` or `CHART={chart}/{version}`. Runs `make index` after reconstruction.

`make zip`: Reconstructs all archives in the `assets` directory based on the current contents in `charts` and updates the `charts/` contents based on the packaged archive. Can be scoped to specific assets via specifying `ASSET={chart}` or `ASSET={chart}/{filename}.tgz`. Runs `make index` after reconstruction.

`make standardize`: Takes an arbitrary Helm repository (defined as any repository with a set of Helm charts under `charts/`) and standardizes it to the expected repository structure of these scripts.

`make validate`: Checks whether all generated assets used to serve a Helm repository (`charts/`, `assets/`, and `index.yaml`) are up-to-date. If `validate.url` and `validate.branch` are provided in the configuration.yaml, it will also ensure that any additional changes introduced only modify chart or package versions specified in the `release.yaml`; otherwise it will output the expected `release.yaml` based on assets it detected changes in.

`make template`: Updates the current directory by applying the configuration.yaml on [upstream Go templates](https://github.com/rancher/charts-build-scripts/tree/master/templates/template) to pull in the most up-to-date docs, scripts, etc. from [rancher/charts-build-scripts](https://github.com/rancher/charts-build-scripts).