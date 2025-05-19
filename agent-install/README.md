---
copyright: Contributors to the Open Horizon project
years: 2019 - 2025
lastupdated: "2025-05-19"
title: "Agent Installation script"
description: Instructions and flags used by the agent-install script

parent: Agent (anax)
nav_order: 1
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Edge node agent-install script

## Overview

This script:

- Verifies prerequisites and configuration information
- Installs the agent packages appropriate for the edge node
- Optionally registers the edge node with a given pattern or node policy
- Optionally waits for a specified service to begin executing on the edge node

## Requirements

Operating systems and architectures explicitly supported by the installation script.  Please note that an environment must be both supported by the installation script, and [an installation package must be available](https://github.com/open-horizon/anax/releases), in order to successfully install the agent.  Environments that meet both criteria are noted in **bold** below.

- Device
  - Ubuntu: xenial (16.x), bionic (18.x), focal (20.x), jammy (22.x), noble (24.x)
    - **amd64, arm64, s390x**
  - Raspbian/RaspberryPi OS: stretch (9), buster (10), bullseye (11), bookworm (12)
    - **armhf, arm64**
  - Debian: stretch (9), buster (10), bullseye (11), bookworm (12)
    - **amd64, armhf, arm64, s390x**
  - RHEL: 7.6, 7.9, 8.1 - 8.5 (via Docker), 8.6 - 8.10 and 9.0 - 9.5 (via Podman 4.x)
    - **amd64, ppc64le**, aarch64, riscv64, **s390x**
  - CentOS: 8.1 - 8.5 (via Docker)
    - **amd64, ppc64le**, aarch64, riscv64
  - Fedora: 32, 35 - 43
    - **amd64, ppc64le**, aarch64, riscv64
  - macOS
    - **amd64, M1, M2, M4**
- Cluster - currently supported versions
  - OpenShift Container Platform (OCP)
    - **amd64, ppc64le, s390x**
  - Microk8s
    - **amd64, ppc64le**
  - k3s
    - **amd64, arm64, ppc64le**

For more details, see [the `agent-install.sh` source code comments](https://raw.githubusercontent.com/open-horizon/anax/refs/heads/master/agent-install/agent-install.sh).

## Description

The following description describes how `agent-install.sh` works for native installs on Linux devices as well as container installs on Linux and MacOS. This description does not cover the differences with cluster agent installs.

This script needs to be run with `sudo -s` to work properly.

The script will first extract files from a given tar file specified with `-z`. This script checks all its arguments and variables. If a required environment variable is not defined, the script will use the value in the configuration file, if present. If required information is not specified, the script exits. If non-required information is missing, the script will continue with a warning.

The script then detects the active operation system, version, and architecture, and begins the agent installation process. If there is a node id mapping file, the script will check for its node id using its hostname then ip address if its hostname is not found. If a node id mapping file is found this is assumed to be a batch install and there will be no user prompts.

For container installs on Linux, the script checks for Docker or Podman and jq. On Ubuntu, Debian and Raspbian installs, the script will install these if missing. On RHEL, CentOS and Fedora installs, the user must install these before running this script. On MacOS installs, which are always installed in a Docker container, the script checks for Docker, jq and socat, exiting if any are not installed.

Before starting the installation, `agent-install.sh` checks if the node is already registered. In that case, it queries if a user wants to overwrite the current node configuration. If the response is `yes`, the node is unregistered, and the packages and configuration are updated. If it is a batch install, or the user responds `no` to the prompt, the script will install without overwriting the existing node configuration.

Next, the script will download the certificate, which is named `agent-install.crt` by default, from the CSS on the management hub if it is not already present in the install directory. The script will then update the horizon defaults in `/etc/default/horizon` and set the port for anax to run on if the agent is being installed natively. For container installs, the anax port is always exposed on the container port 8081.

The last step before installation is downloading the necessary packages if they are not already in the installation directory. The **horizon** and **horizon-cli** packages are downloaded by default, except for container installs on Linux, MacOS installs, or if the user specifies the `-C` flag. In those cases, only the **horizon-cli** package is installed.

The script will download the package(s) from the location specified by `-i`. If it was set to `'css:'` (with or without a path appended) it will download the package(s) from the CSS on the management hub. If `https://github.com/open-horizon/anax/releases/<release>` was specified, it will get the specified package version(s) from the anax releases repo. If it was set to an APT repo, the script will add the key for the APT repository and install the package(s) using the APT repo. (Note: APT installs are only compatible with Ubuntu, Debian and Raspbian Linux installs.)

On Ubuntu, Debian and Raspbian Linux, if the user spcified an APT repo, the package(s) will have already been installed using the APT repo. On all other Linux installs, the **horizon** and/or **horizon-cli** packages will be installed using the downloaded packages obtained from either the anax releases or the CSS.

If the package(s) being installed are an older version than those currently installed, the script will prompt the user if they want to continue. If it is a batch install, the older version will not be installed unless `-f` was specified. If the package version is equal to the currently installed version, the script will show a warning and exit.

On Linux container installs and MacOS installs, the `HORIZON_URL` variable is set to <http://localhost:8081> so that the `hzn` CLI will be able to reach the agent running in the container. The container is then started and the script waits for the agent to start up. On all other Linux installs, the script checks if new packages were installed or if the horizon defaults in `/etc/default/horizon` were changed. If either is true, the agent is restarted before registration.

After installation and configuration is complete, a node is created in the Exchange, and is registered with the pattern or policy, if specified. If -w \<service name\> is specified, the script will wait until the specified service begins executing on the node.

## Usage

Example:

```bash
sudo -s ./agent-install.sh -i . -u $HZN_EXCHANGE_USER_AUTH -p IBM/pattern-ibm.helloworld -w ibm.helloworld -o $HZN_ORG_ID -z $AGENT_TAR_FILE
```

### Configuration File

By default it will look for a configuration file called `agent-install.cfg` in the same directory. The config file should contain, at minimum:

```bash
HZN_EXCHANGE_URL=https://<management-hub-url>:3090/edge-exchange/v1/
HZN_FSS_CSSURL=https://<management-hub-url>:9443/edge-css/
HZN_AGBOT_URL=https://<management-hub-url>:3111/edge-agbot/
HZN_SDO_SVC_URL=https://<management-hub-url>:9008/edge-sdo-ocs/api
```

The config file can also set the following variables:
`HZN_DEVICE_ID`, `HZN_NODE_ID`, `HZN_MGMT_HUB_CERT_PATH`, and `HZN_AGENT_PORT`

For cluster installs, the config file can also set:
`AGENT_NAMESPACE`

### Certificate File

By default it will use the certificate file called `agent-install.crt` in the same directory, if it exists. If the packages are being downloaded from the CSS, the certificate is extracted from the downloaded files. This location can be overwritten in the config file.

### Environment Variables

Environment variables with the same names as the variables in the config file above can be used to override the config file values. Variables set by flags take the highest precedence.

### Command Line Flags

Command line flags override the corresponding environment variables or config file variables:

`-c <path>` - path to the agent-install.crt file from your management hub. Default: `./agent-install.crt`

`-k <path>` - path to the agent-install.cfg file that contains the horizon defaults. Default: `./agent-install.cfg`

`-i <path>` - path to the packages. Specify `css:` to get packages from the management hub MMS. Specify `https://github.com/open-horizon/anax/releases` to get latest packages from open horizon anax github repository. Default: current directory

`-z <name>` - specifies the name of your agent installation tar file. Default is ./agent-install-files.tar.gz

`-j <path>` - path to file containing public key for an APT repository specified with `-i`

`-t <branch>` - Branch to use in the APT repo specified with `-i`

`-O <exchange org>` - The exchange organization id

`-u <exchange credentials>` -  specifies your exchange user credentials in the form `iamapikey:<api-key>` or `username:password`

`-d <node id>` - the node id to register with. For individual not batch install only.

`-p <pattern>` - pattern name to register this node with. Default: registers node with policy

`-n <path>` - path to a node policy file

`-w <service name>` - causes the script to wait the number of seconds specified by `-T` (default: 60) until agreements are made and the service is deployed. If using a pattern, this value can be '*'

`-T <timeout>` - The number of seconds for the script to wait on services specified by `-w`

`-o <org name>` - specifies the org of the service that you specified with the -w flag. If it is not set, the default is your edge device org.

`-s` - skip node registration

`-D <type>` - Node type of agent being installed: device, cluster. Default: device

`-U <url>` - Internal url for edge cluster registry. If not specified, this script will auto-detect the value if it is a small, single-node cluster (e.g. k3s or microk8s). For OCP use: image-registry.openshift-image-registry.svc:5000

`-l` - logging verbosity level (0: silent, 1: critical, 2: error, 3: warning, 4: info, 5: debug), the default is (3: warning)

`-f` - use with batch installs to allow the script to install an older version than is currently installed

`-b` - Skip any prompts for user input. Default behavior for batch installs.

`-C` - Install only the horizon-cli package, not the full agent

`--container` - Install the agent in a container

`--namespace` - The namespace that the cluster agent will be installed to. The default is 'openhorizon-agent'

`--namespace-scoped` - The cluster agent will only have namespace scope. The default is 'false'

### anax-in-container

To install more than one agent in a container, the horizon-container command is used (which is included with the horizon-cli package). Instructions can be found below:

[anax-in-container install directions](https://open-horizon.github.io/docs/anax/docs/agent_container_manual_deploy/)

## Package Tree

The script relies on an installation packages tree with the following directory structure:

```text
.
└── horizon-edge-packages
    ├── agent-install.cfg
    ├── agent-install.crt
    ├── agent-install.sh
    ├── agent-uninstall.sh
    ├── amd64_anax.tar.gz
    ├── amd64_anax_k8s.tar.gz
    ├── deployment-template.yml
    ├── amd64_auto-upgrade-cronjob_k8s.tar.gz
    ├── auto-upgrade-cronjob-template.yml
    ├── horizon-2.30.0.x86_64.rpm
    ├── horizon-cli-2.30.0.pkg
    ├── horizon-cli-2.30.0.x86_64.rpm
    ├── horizon-cli.crt
    ├── horizon-cli_2.30.0_arm64.deb
    ├── horizon-cli_2.30.0_amd64.deb
    ├── horizon-cli_2.30.0_armhf.deb
    ├── horizon_2.30.0_arm64.deb
    ├── horizon_2.30.0_amd64.deb
    ├── horizon_2.30.0_armhf.deb
    └── persistentClaim-template.yml
```
