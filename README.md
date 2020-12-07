charts-build-scripts
========

## Build Scripts For Rancher Charts

## Before running the scripts

#### If you are creating a new charts repository with `charts-build-scripts init`

Set the environment variable `GITHUB_AUTH_TOKEN` to your [personal Github Access Token](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token).

This allows the scripts to automatically make requests to the Github API for you. The Personal Access Token you provide should have a `repo` scope.

## Building

`make`


## Running

`./bin/charts-build-scripts`

## License
Copyright (c) 2019 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
