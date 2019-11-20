# Edge node agent-install

## Overview

This script:

* Verifies prerequisites and configuration information
* Installs the agent packages appropriate for the edge node
* Optionally registers the edge node to run a deployment pattern of services

## Requirements

Currently supported OS and architectures:

* Ubuntu 16.04, 18.04
  * armhf, arm64, amd64, ppc64el
* Raspbian Stretch
  * armhf
* macOS
  * amd64

## Description

This script first checks all its arguments and variables. If a required environment variable is not defined, the script will use the value in the configuration file, if present. If required information is not specified, the script exits.

The script then detects the active operation system, version, and architecture, and begins the agent installation process.

Before starting the installation, `agent-install.sh` checks if the node is already registered. In that case, it queries if a user wants to overwrite the current node configuration. If the response is `yes`, the node is unregistered, and the packages and configuration are updated.

On Linux, the script updates the apt repositories, and verifies the required prerequisite software is installed, installing it if needed and available. On macOS, if the prerequisite software is missing, the script asks the user to install the missing packages and exits.

The **horizon** and **horizon-cli** packages will be installed or updated.

After installation and configuration is complete, a node is created in the Exchange, and is registered with the pattern, if specified.

## Usage

Example: `./agent-install.sh`

### Configuration File

By default it will look for a configuration file called `agent-install.cfg` in the same directory. The config file should contain:

```bash
HZN_EXCHANGE_URL="https://<ICP_IP>:8443/ec-exchange/v1/"
HZN_FSS_CSSURL="https://<ICP_IP>:8443/ec-css/"
HZN_ORG_ID="<your_ICP_cluster>"
HZN_EXCHANGE_USER_AUTH="iamapikey:<your_ICP_API_key>"
HZN_EXCHANGE_PATTERN="IBM/pattern-ibm.helloworld"
```

### Certificate File

By default it will use the certificate file called `agent-install.crt` in the same directory, if it exists.

### Environment Variables

Environment variables with the same names as the variables in the config file above can be used to override the config file values.

In addition, the environment variable `CERTIFICATE` can be set to specify the location of the certificate file.

### Command Line Flags

Command line flags override the corresponding environment variables or config file variables:

`-k <path>` - path to the configuration file. Default: `./agent-install.cfg`

`-c <path>` - path to the CA certificate file from your ICP instance. Default: `./agent-install.crt`

`-i <path>` - path to the packages. Default: current directory

`-p <pattern>` - pattern name to register this node with. Default: register the node

`-n <path>` - path to a node policy file

`-s` - skip node registration

`-v` - show version

`-l` - logging verbosity level (0-5, 5 is verbose; default is 3.)

## Package Tree

The script relies on an installation packages tree with the following directory structure:

```text
.
├── linux
│   ├── raspbian
│   │   └── stretch
│   │       └── armhf
│   │           ├── bluehorizon_2.23.29~ppa~raspbian.stretch_all.deb
│   │           ├── horizon-cli_2.23.29~ppa~raspbian.stretch_armhf.deb
│   │           └── horizon_2.23.29~ppa~raspbian.stretch_armhf.deb
│   └── ubuntu
│       ├── bionic
│       │   ├── amd64
│       │   │   ├── bluehorizon_2.23.29~ppa~ubuntu.bionic_all.deb
│       │   │   ├── horizon-cli_2.23.29~ppa~ubuntu.bionic_amd64.deb
│       │   │   └── horizon_2.23.29~ppa~ubuntu.bionic_amd64.deb
│       │   ├── arm64
│       │   │   ├── bluehorizon_2.23.29~ppa~ubuntu.bionic_all.deb
│       │   │   ├── horizon-cli_2.23.29~ppa~ubuntu.bionic_arm64.deb
│       │   │   └── horizon_2.23.29~ppa~ubuntu.bionic_arm64.deb
│       │   ├── armhf
│       │   │   ├── bluehorizon_2.23.29~ppa~ubuntu.bionic_all.deb
│       │   │   ├── horizon-cli_2.23.29~ppa~ubuntu.bionic_armhf.deb
│       │   │   └── horizon_2.23.29~ppa~ubuntu.bionic_armhf.deb
│       │   └── ppc64el
│       │       ├── bluehorizon_2.23.29~ppa~ubuntu.bionic_all.deb
│       │       ├── horizon-cli_2.23.29~ppa~ubuntu.bionic_ppc64el.deb
│       │       └── horizon_2.23.29~ppa~ubuntu.bionic_ppc64el.deb
│       └── xenial
│           ├── amd64
│           │   ├── bluehorizon_2.23.29~ppa~ubuntu.xenial_all.deb
│           │   ├── horizon-cli_2.23.29~ppa~ubuntu.xenial_amd64.deb
│           │   └── horizon_2.23.29~ppa~ubuntu.xenial_amd64.deb
│           ├── arm64
│           │   ├── bluehorizon_2.23.29~ppa~ubuntu.xenial_all.deb
│           │   ├── horizon-cli_2.23.29~ppa~ubuntu.xenial_arm64.deb
│           │   └── horizon_2.23.29~ppa~ubuntu.xenial_arm64.deb
│           ├── armhf
│           │   ├── bluehorizon_2.23.29~ppa~ubuntu.xenial_all.deb
│           │   ├── horizon-cli_2.23.29~ppa~ubuntu.xenial_armhf.deb
│           │   └── horizon_2.23.29~ppa~ubuntu.xenial_armhf.deb
│           └── ppc64el
│               ├── bluehorizon_2.23.29~ppa~ubuntu.xenial_all.deb
│               ├── horizon-cli_2.23.29~ppa~ubuntu.xenial_ppc64el.deb
│               └── horizon_2.23.29~ppa~ubuntu.xenial_ppc64el.deb
└── macos
    ├── horizon-cli-2.23.29.pkg
    └── horizon-cli.crt
```
