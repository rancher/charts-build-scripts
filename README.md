charts-build-scripts
========

## Build Scripts For Rancher Charts

## Before running the scripts

#### If you are creating a new charts repository 

Checkout the Git branch that corresponds to your Staging or Live branch.

Export BRANCH_ROLE as `staging`, `live`, or `custom`. Then run:

```
curl -s https://raw.githubusercontent.com/rancher/charts-build-scripts/master/init.sh > /dev/null | sh
```

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
