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
    1. **Chart Generation**: It immediately fires up to generate local charts.
    2. **Repository Cleanliness Check**: It revisits the cleanliness of your git repository.

5. **Script Options Set**: If you've already set the script options:
    1. **Local Mode Verification**: First, it checks to make sure you're not in local mode.
        1. **Repository Root Retrieval**: It then finds your repository's root directory.
        2. **Loading Release Options**: Next, it fetches the release options from your `release.yaml`.
        3. **Asset & Chart Verification**: It verifies if the generated assets and charts are in sync with the upstream ones.
        4. **Discrepancy Check**: If any discrepancies are found, such as charts that need to be added to `release.yaml`, charts that have been removed from the upstream, or charts that have been modified from the upstream:
            1. **Logging Discrepancies**: These inconsistencies are logged for you to examine.
            2. **Correcting release.yaml**: It goes ahead to create a correct `release.yaml` file, incorporating all the discrepancies found in the previous steps.

6. **Zip Charts**: Zipping charts to ensure that contents of assets, charts, and index.yaml are in sync.

7. **Final Cleanliness Check**: Finally, it does one last sweep to ensure that your git repository is clean. If not, it promptly alerts you of the situation, helping to avoid a potential mishap.

That's it! Follow these simple steps to use the Validation Command effectively.
