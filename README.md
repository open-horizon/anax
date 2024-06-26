# DO NOT MERGE
# anax

## Introduction

This project contains the Horizon client system source code. Stable versions of the Horizon agent are packaged for many Debian-based distributions. They are available for download at https://github.com/open-horizon/anax/releases. To build the packages yourself, consult https://github.com/open-horizon/horizon-deb-packager.
To run the agent, you will need access to systems where the Exchange (https://github.com/open-horizon/exchange-api), the CSS (https://github.com/open-horizon/anax/tree/master/css/image/cloud-sync-service-amd64), and an Agbot (which is just anax deployed as an agbot instead of an agent) are deployed.

Related Projects:

* `horizon-deb-packager` (https://github.com/open-horizon/horizon-deb-packager): A system for packaging Horizon system `deb`s for multiple distributions and architectures. It also produces Ubuntu snaps
* `sync-service` (https://github.com/open-horizon/edge-sync-service): A subsystem for managing machine learning models at the edge.


## Documentation

* [Anax APIs](docs/api.md)
* [Managed Workloads](docs/managed_workloads.md)

## Development

### Preconditions

* Go version >=1.19 is a required dependency, download it [here](https://golang.org/dl/)
* To execute the lint and other code checkers (`make lint` or `make check`), you must install: `go vet`, `golint`, and `jshint`

### Pull Request Guidelines
* The PR should have only 1 commit in it when you submit for review. You can commit as often as you want in your Git workspace, but you will need to squash them all to 1 before you submit for review.
* The commit message should be in the form: "Issue xxxx - some short description"
* Don't forget to commit with the -s flag to digitally sign your PR as your original work.
Items 1 and 2 are necessary because the build process automatically produces a change log that goes into the final packages, and it uses commit message to indicate each change, this the need for 1 commit per PR and a standard message format.
Please feel free to reach out for help to the Agent workgroup in the [LF Edge IM system](https://chat.lfx.linuxfoundation.org/).

### Operations

Note that the Makefile silences a lot of its output by default. If you want to see more output from build steps, execute a build like this:

    make mostlyclean check verbose=y

#### Build executable

Regular usage for the same anax runtime platform:

    make

> Note: If you want to run anax build on another platform (cross-platform build), set up the target architecture and kind of operating system platform (Linux/Darwin) as arch and opsys variables respectively:
>
> ```sh
> # For example, to build ppc64le anax binary for Linux OS on Mac OS host use the commands below
> # List of possible values:
> #   `arch`: armhf, arm64, amd64, ppc64el
> #   `opsys`: Linux, Darwin
> export arch=ppc64el
> export opsys=Linux
> # then call make to run anax build
> make
> ```

#### Execute code checks

    make lint

#### Format code

    make format

#### Fetch dependencies

    make deps

Note that this target is automatically executed when executing targets `check` and `all`. It is not automatically executed when executing `test`, `test-integration`, and generating specific executables.

#### Execute both unit and integration tests

    make check

#### Execute unit tests

    make test

#### Execute integration tests

    make test-integration

#### Debug Logging

* Add `"ANAX_LOG_LEVEL=5"` to the `Environment=` configuration in the systemd unit file `/etc/systemd/system/horizon.service`. Note that the value `5` is the classification of most debug log messages, `6` is used for even more granular log messages, something like a 'trace' level.
* Reload the systemd unit file with `systemctl daemon-reload`.
* Restart the anax process with `systemctl restart horizon.service`.

#### Generate swagger documentation

    swagger generate spec -o ./swagger.json -m

*Note - Place agbot secure api swagger file in `docs/agbot_secure_api`*

    cd agreementbot
    swagger generate spec -o ../docs/agbot_secure_api.json --exclude=edge-sync-service -m

*Note - Place mms/secrets API swagger file in `docs/mms_swagger`*

    cd ../resource
    swagger generate spec -o ../docs/mms_swagger.json --include=resource --include=edge-sync-service -m

### Internationalization

    make i18n-catalog
    make

Only `hzn` command supports internationalization. To test, set LANG or HZN_LANG enviromental variable. For example:

    HZN_LANG=fr hzn version

#### Development Environment

Note that this Makefile can construct its own `GOPATH` and build from it; this is a convenience that can sometimes cause problems for development tooling that expects a project to be in a subdirector of `$GOPATH/src`. To get full tool support clone this project as `$GOPATH/src/github.com/open-horizon/anax`.

Information for setting up the e2e development environment can be found in the [test](https://github.com/open-horizon/anax/tree/master/test) folder.

## Deprecated Commands and APIs
* `hzn policy patch` command is deprecated. Please use `hzn policy update` to update the node policy.
* `hzn exchange node updatepolicy` command is deprecated. Please use `hzn exchange node addpolicy` to update the node policy in the Exchange.
