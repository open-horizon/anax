# anax

## Introduction

This project contains the Horizon client system source code. To consume the Horizon system, please browse to http://bluehorizon.network.

Related Projects:

* `bluehorizon-snap` (http://github.com/open-horizon/bluehorizon-snap): A Ubuntu Snappy bundling of the complete Horizon client components
* `ubuntu-classic-image` (http://github.com/open-horizon/ubuntu-classic-image): Produces complete system images

## Documentation

* [APIs](api.md)

## Development

### Preconditions

* To execute the lint and other code checkers (`make lint`), you must install: `go vet`, `golint`, and `jshint`

### Operations

#### Build executable

    make

#### Execute code checks

    make lint

#### Execute unit tests

    make test

#### Execute integration tests

    make test-integration

#### Debug Logging

* Add `"ANAX_LOG_LEVEL=5"` to the `Environment=` configuration in the systemd unit file `/etc/systemd/system/snap.bluehorizon.anax.service`. Note that the value `5` is the classification of most debug log messages, `6` is used for even more granular log messages, something like a 'trace' level.
* Reload the systemd unit file with `systemctl daemon-reload`.
* Restart the anax process with `systemctl restart snap.bluehorizon.anax.service`.
