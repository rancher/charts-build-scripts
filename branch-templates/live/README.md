# Live Branch

This branch contains Helm charts that are released or ready to be released. Most of the commits to this branch should be cherry-picked from {{ .BranchConfiguration.Staging }}, which picks up the generated changes from {{ .BranchConfiguration.Source }}.

To release a new version of a chart, please open the relevant PRs to {{ .BranchConfiguration.Source }}. Once the changes are merged, they will automatically update the {{ .BranchConfiguration.Staging }} branch, which can then be pulled into this branch after the changes are tested right before a release.