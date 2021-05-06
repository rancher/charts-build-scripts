#!/bin/bash
set -e

if [ "${BRANCH_ROLE}" != "staging" ] && [ "${BRANCH_ROLE}" != "live" ] && [ "${BRANCH_ROLE}" != "custom" ]; then
    echo "usage: BRANCH_ROLE=<staging|live|custom> ./init.sh"
    exit 1
fi

CBS_RAW_LINK=https://raw.githubusercontent.com/rancher/charts-build-scripts/master

mkdir -p scripts
curl -s ${CBS_RAW_LINK}/templates/template/scripts/version --output scripts/version > /dev/null
chmod +x scripts/version
curl -s ${CBS_RAW_LINK}/templates/template/scripts/pull-scripts --output scripts/pull-scripts > /dev/null
chmod +x scripts/pull-scripts

if [ "${BRANCH_ROLE}" = "staging" ] || [ "${BRANCH_ROLE}" = "live" ]; then
    curl -s ${CBS_RAW_LINK}/templates/${BRANCH_ROLE}.yaml --output configuration.yaml > /dev/null
    ./scripts/pull-scripts
    ./bin/charts-build-scripts template
    echo "Pulled in basic template for ${BRANCH_ROLE} into configuration.yaml and constructed charts directory"
    echo "Next Steps:"
    echo "1. Modify the configuration.yaml with your expected setup and re-run make template to automatically update the repository."
    if [ "${BRANCH_ROLE}" = "staging" ]; then
        echo "2. Modify .github/workflows/pull-request.md and .github/workflows/push.md to set up automatic pushes to another branch."
    fi
else
    echo "Creating an empty configuration.yaml file."
    echo -n "" > configuration.yaml
    echo "You will need to run make template manually after filling in the configuration.yaml"
    echo "To add a template for Github Workflow based pull-requests, run the following script and update .github/workflows/pull-request.yaml manually"
    echo "curl ${CBS_RAW_LINK}/templates/template/.github/workflows/pull-request.yaml --output .github/workflows/pull-request.yaml"
fi
