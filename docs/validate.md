## Using the Validation Command

The validation command runs a series of checks on a clean Git repository to ensure that the current state of a repository is valid.

### Usage 

The command accepts either a local (l or local) or a remote (r or remote) flag. These are used to determine whether to validate local or remote charts.

1. **Mode Verification**: The command first establishes whether you're operating in remote or local mode.

2. **Script Options Parsing**: It proceeds to decode your script options file, making sense of all the parameters specified. The script options file is `configuration.yaml`. It contains the following validation settings:
    1. The upstream options (URL, subdirectory, and commit of a git repository)
    2. The git branch

    The script options are used later to pull the specified branch of the git repository to compare with the generated assets.

3. **Repository Cleanliness Check**: Then, it ensures that your git repository is in a clean state, free from uncommitted changes, and ready to process.

4. **Local Flag Operations**: If you have set the local flag:
    1. **Chart Generation**: It immediately fires up to generate local charts. It loops through the packages, pulls the main chart into the charts folder, and packs the main chart and its dependencies into the assets folder.
    2. **Repository Cleanliness Check**: It revisits the cleanliness of your git repository.

5. **Script Options Set**: If you've already set the script options:
    1. **Local Mode Verification**: First, it checks to make sure you're not in local mode.
        1. **Repository Root Retrieval**: It then finds your repository's root directory.
        2. **Loading Release Options**: Next, it fetches the release options from your `release.yaml`.
        3. **Asset & Chart Verification**: It verifies if the generated assets and charts are in sync with the upstream ones.
        4. **Discrepancy Check**: It checks for three types of discrepancies:
            1. Chart exists in local and is not tracked by `release.yaml`. If that's the case, the chart is added to the `UntrackedInRelease` list.
            2. Chart was removed from local and is not tracked by `release.yaml`. If that's the case, the chart is added to the `RemovedPostRelease` list.
            3. Chart was modified in local and is not tracked by `release.yaml`. If that's the case, the chart is added to the `ModifiedPostRelease` list.

        5. **Logging Discrepancies**: These discrepancies found in the last step are logged for you to examine. It prints the `UntrackedInRelease`, `RemovedPostRelease` and `ModifiedPostRelease` lists.
        6. **Correcting release.yaml**: It goes ahead to create a correct `release.yaml` file, incorporating all the discrepancies found in the previous steps.

6. **Zip Charts**: Zipping charts to ensure that contents of assets, charts, and index.yaml are in sync. It zips charts from charts/ into assets/. If the asset was re-ordered, it will also update charts/. If specificChart is provided, it will filter the set of charts that will be targeted for zipping. It will also not update an asset if its internal contents have not changed.

    **Note**: since we use helm package to zip charts, it's possible that the tgz file that is created reorders the contents of Chart.yaml and requirements.yaml to be alphabetical. Therefore, when zipping a chart we always need to unzip the finalized chart(s) back to the charts/ directory.

7. **Final Cleanliness Check**: Finally, it does one last sweep to ensure that your git repository is clean. If not, it promptly alerts you of the situation, helping to avoid a potential mishap.

That's it! Follow these simple steps to use the Validation Command effectively.
