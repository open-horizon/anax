# anax

## Introduction

This project contains the Horizon client system source code. To learn more about the Horizon system, including how to try the Blue Horizon instance of it, please browse to http://bluehorizon.network. Note that the **HEAD** of this repository's `master` branch includes alpha-grade code under current development. Stable versions of this application are bundled in Ubuntu Snaps (cf. https://www.ubuntu.com/desktop/snappy), consult the `bluehorizon-snap` project listed below to learn more.

Related Projects:

* `anax-ui` (http://github.com/open-horizon/anax-ui): The source for the Anax web UI
* `horizon-pkg` (http://github.com/open-horizon/horizon-pkg): A system for packaging Horizon system `deb`s for multiple distributions and architectures. It also produces Ubuntu snaps
 * `raspbian-image` (http://github.com/open-horizon/raspbian-image): The Raspbian image builder for Raspberry Pi 2 and 3 models dedicated to Horizon

## Documentation

* [Anax APIs](doc/api.md)
* [Managed Workloads](doc/managed_workloads.md)

## Development

### Preconditions

* To execute the lint and other code checkers (`make lint`), you must install: `go vet`, `golint`, and `jshint`

### Operations

#### Build executable

    make

#### Execute code checks

    make lint

#### Format code

    make format

#### Execute both unit and integration tests

    make check

#### Execute unit tests

    make test

#### Execute integration tests

    make test-integration

#### Debug Logging

* Add `"ANAX_LOG_LEVEL=5"` to the `Environment=` configuration in the systemd unit file `/etc/systemd/system/snap.bluehorizon.anax.service`. Note that the value `5` is the classification of most debug log messages, `6` is used for even more granular log messages, something like a 'trace' level.
* Reload the systemd unit file with `systemctl daemon-reload`.
* Restart the anax process with `systemctl restart snap.bluehorizon.anax.service`.

#### Development Environment

Note that the Makefile constructs its own `GOPATH` and builds from it; this is a convenience that can sometimes cause problems for development tooling that expects a project to be in a subdirector of `$GOPATH/src`. To use the Makefile to build the project inside your user's `GOPATH`, set the `TMPGOPATH` variable to your `GOPATH` and execute make like this: `make deps TMPGOPATH=$GOPATH`
