# charts-build-scripts

## How to Use `charts-build-scripts`

### Charts Repository Setup

Checkout the Git branch that corresponds to your Staging or Live branch.

Export BRANCH_ROLE as `staging`, `live`, or `custom`. Then run:

```
curl -s https://raw.githubusercontent.com/rancher/charts-build-scripts/master/init.sh > /dev/null | sh
```

### Building

`make`

### Running

`./bin/charts-build-scripts`

### Validation command

For more information on the validation command, please see [`docs/validate.md`](docs/validate.md).

### Debugging

For more information on how to debug this project, please see [`docs/debugging.md`](docs/debugging.md).


## Developing `charts-build-scripts`

### How to Run

```
go run main.go
```

### How to Run Unit Tests

```
go test ./...
```

### How to Lint

```
golangci-lint run ./...
```

### How to Release

Releases are done via a github action that uses [`goreleaser`](https://goreleaser.com/).
In order to release, simply tag the commit that you want the release
to be based off of with a tag that is in semver format. `goreleaser`
takes care of the rest.


## License

Copyright (c) 2024 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
