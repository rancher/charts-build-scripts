#!/bin/bash
set -e

CBS_RAW_LINK=https://raw.githubusercontent.com/rancher/charts-build-scripts/master

mkdir -p scripts
curl -s ${CBS_RAW_LINK}/templates/template/scripts/version --output scripts/version > /dev/null
chmod +x scripts/version
curl -s ${CBS_RAW_LINK}/templates/template/scripts/pull-scripts --output scripts/pull-scripts > /dev/null
chmod +x scripts/pull-scripts

curl -s ${CBS_RAW_LINK}/templates/configuration.example.yaml --output configuration.yaml > /dev/null
./scripts/pull-scripts
./bin/charts-build-scripts template
echo "Pulled in basic template into configuration.yaml and constructed charts directory"
echo "Next Steps:"
echo "1. Modify the configuration.yaml with your expected setup and re-run make template to automatically update the repository."
