# Automated tests

## Overview

This script:

* Gets specified packages distributives
* Verifies installation and functionality of the `agent-install.sh` script
* Can be used for cluster heatchcheck
* Checks if the `agent-install.sh` script performs switching from one configuration to another (it doesn't require any (re)installation but makes changes in the agent configuration so it's set up against new environment)

## Requirements

Currently supported OS and architectures:

* Ubuntu 16.04, 18.04
  * amd64

## Quick start

1. Get the configuration file `agent-install.cfg` and certificate `agent-install.crt` files for your environment, and copy them in `config` subdirectory.
2. Get the configuration file `agent-install.cfg` and certificate `agent-install.crt` files for second environment (the script will test switching between you primary environment and one you specifed as "switch" or "secondary" environment), and copy them in `config/switch` subdirectory.
3. Run the script `./test.sh <current_packages_versions> <previous_packages_version>` (e.g. `./test.sh 2.24.18 2.24.16`).

## Description

This script first checks if all two packages versions are provided. If required information is not specified, the script exits.

If versions have been specified, the script downloads the [shUnit2](https://github.com/kward/shunit2/) framework. The script then downloads packages for the versions specified.

After that, the script runs the test cases (in order they are located in the script).

## Configuration

The script uses the same configuration file as the `agent-install.sh` does.

The config file should contain:
```bash
HZN_EXCHANGE_URL="https://<MGMT_HUB>:8443/ec-exchange/v1/"
HZN_FSS_CSSURL="https://<MGMT_HUB>:8443/ec-css/"
HZN_ORG_ID="<your_MGMT_HUB_cluster>"
HZN_EXCHANGE_USER_AUTH="iamapikey:<your_MGMT_HUB_API_key>"
HZN_EXCHANGE_PATTERN="IBM/pattern-ibm.helloworld"
```

It requires the user to provide configuration (the `agent-install.cfg` and `agent-install.crt`) files for two environments, however. So the script can check how the `agent-install.sh` can do "switching" to another environment.

The complete configuration should look like:

```bash
config
|-- agent-install.cfg
|-- agent-install.crt
|-- node_policy.json
`-- switch
    |-- agent-install.cfg
    `-- agent-install.crt
```