# Edge node agent-install 

## Overview

This script:

* Verifies prerequisites and configuration information
* Installs the agent packages appropriate for the edge node
* Optionally registers the edge node with a given pattern or node policy
* Optionally waits for a specified service to begin executing on the edge node

## Requirements

Currently supported OS and architectures:

* Ubuntu bionic, xenial
  * arm64, amd64
* Raspbian buster, stretch
  * armhf
* Debian buster, stretch
  * amd64
* macOS
  * amd64

## Description

This script needs to be run with `sudo -s` to work properly.

The script will first extract files from a given tar file specified with `-z`. This script checks all its arguments and variables. If a required environment variable is not defined, the script will use the value in the configuration file, if present. If required information is not specified, the script exits. If non-required information is missing, the script will continue with a warning.

The script then detects the active operation system, version, and architecture, and begins the agent installation process. If there is a node id mapping file, the script will check for its node id using its hostname then ip address if its hostname is not found. If a node id mapping file is found this is assumed to be a batch install and there will be no user prompts.

Before starting the installation, `agent-install.sh` checks if the node is already registered. In that case, it queries if a user wants to overwrite the current node configuration. If the response is `yes`, the node is unregistered, and the packages and configuration are updated. If it is a batch install, or the user responds `no` to the prompt, the script will install without overwriting the existing node configuration.

On Linux, the script updates the apt repositories, and verifies the required prerequisite software is installed, installing it if needed and available. On macOS, if the prerequisite software is missing, the script will attempt to install the missing software. If that fails, the script asks the user to install the missing packages and exits.

If the packages being installed are an older version than those currently installed, the script will prompt the user if they want to continue. If it is a batch install, the older version will not be installed unless `-f` was specified. If the package version is equal to the currently installed version, the script will show a warning and exit.

The **horizon** and **horizon-cli** packages will be installed or updated.

After installation and configuration is complete, a node is created in the Exchange, and is registered with the pattern or policy, if specified. If -w <service name> is specified, the script will wait until the specified service begins executing on the node.

## Usage

Example: `sudo -s ./agent-install.sh -i . -u $HZN_EXCHANGE_USER_AUTH -p IBM/pattern-ibm.helloworld -w ibm.helloworld -o IBM -z $AGENT_TAR_FILE`

### Configuration File

By default it will look for a configuration file called `agent-install.cfg` in the same directory. The config file should contain:

```bash
HZN_EXCHANGE_URL="https://<ICP_IP>:8443/ec-exchange/v1/"
HZN_FSS_CSSURL="https://<ICP_IP>:8443/ec-css/"
HZN_ORG_ID="<your_ICP_cluster>"
CERTIFICATE="/etc/horizon/agent-install.crt"
HZN_EXCHANGE_USER_AUTH="iamapikey:<api key>"
```

### Certificate File

By default it will use the certificate file called `agent-install.crt` in the same directory, if it exists. This location can be overwritten in the config file.

### Environment Variables

Environment variables with the same names as the variables in the config file above can be used to override the config file values. Variables set by flags take the highest precedence.

### Command Line Flags

Command line flags override the corresponding environment variables or config file variables:

`-z <name>` - specifies the name of your agent installation tar file. Default is ./agent-install-files.tar.gz

`-u iamapikey:<your api key>` -  specifies your exchange credentials

`-c <path>` - path to the CA certificate file from your ICP instance. Default: `./agent-install.crt`

`-i <path>` - path to the packages. Specify `https://github.com/open-horizon/anax/releases` to get latest packages from open horizon anax github repository. Default: current directory

`-p <pattern>` - pattern name to register this node with. Default: register the node

`-n <path>` - path to a node policy file

`-w <service name>` - causes the script to wait up to 60 seconds until agreements are made and the service is deployed

`-o <org name>` - specifies the org of the service that you specified with the -w flag. If it is not set, the default is your edge device org.

`-f` - use with batch installs to allow the script to install an older version than is currently installed

`-d <node id>` - the node id to register with. For individual not batch install only.

`-s` - skip node registration

`-v` - show version

`-l` - logging verbosity level (0: silent, 1: critical, 2: error, 3: warning, 4: info, 5: debug), the default is (3: warning)

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
    ├── horizon-2.27.0-173.x86_64.rpm
    ├── horizon-cli-2.27.0-173.pkg
    ├── horizon-cli-2.27.0-173.x86_64.rpm
    ├── horizon-cli.crt
    ├── horizon-cli_2.27.0-167_arm64.deb
    ├── horizon-cli_2.27.0-173_amd64.deb
    ├── horizon-cli_2.27.0-173_armhf.deb
    ├── horizon_2.27.0-167_arm64.deb
    ├── horizon_2.27.0-173_amd64.deb
    ├── horizon_2.27.0-173_armhf.deb
    └── persistentClaim-template.yml
```
