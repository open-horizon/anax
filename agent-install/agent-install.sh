#!/bin/bash

# The script installs Horizon agent on an edge node

set -e


SCRIPT_VERSION="1.1.1"

SUPPORTED_OS=( "macos" "linux" )
SUPPORTED_LINUX_DISTRO=( "ubuntu" "raspbian" "debian" )
SUPPORTED_LINUX_VERSION=( "bionic" "buster" "xenial" "stretch" )
SUPPORTED_ARCH=( "amd64" "arm64" "armhf" )

# Defaults
PKG_PATH="."
PKG_TREE_IGNORE=false
SKIP_REGISTRATION=false
CFG="agent-install.cfg"
OVERWRITE=false
SKIP_PROMPT=false
HZN_NODE_POLICY=""
AGENT_INSTALL_ZIP="agent-install-files.tar.gz"
NODE_ID_MAPPING_FILE="node-id-mapping.csv"
CERTIFICATE_DEFAULT="agent-install.crt"
BATCH_INSTALL=0
DEPLOY_TYPE="device" # "cluster" for deploy on edge cluster

# agent deployment
SERVICE_ACCOUNT_NAME="agent-service-account"
CLUSTER_ROLE_BINDING_NAME="openhorizon-agent-cluster-rule"
SECRET_NAME="openhorizon-agent-secrets"
CONFIGMAP_NAME="openhorizon-agent-config"
PVC_NAME="openhorizon-agent-pvc"
RESOURCE_READY=0
GET_RESOURCE_MAX_TRY=5
WAIT_POD_MAX_TRY=20
POD_ID=""
HZN_ENV_FILE="/tmp/agent-install-horizon-env"
USE_EDGE_CLUSTER_REGISTRY="true"
DEFAULT_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY="image-registry.openshift-image-registry.svc:5000"


VERBOSITY=3 # Default logging verbosity

#future: decide if we are going to use this or check_empty()
REQUIRED_PARAMS=( "HZN_EXCHANGE_URL" "HZN_FSS_CSSURL" "HZN_ORG_ID" )
# the default values are passed into each call of get_variable()
#REQUIRED_VALUE_FLAG="REQUIRED_FROM_USER"
#DEFAULTS=( "${REQUIRED_VALUE_FLAG}" "${REQUIRED_VALUE_FLAG}" "${REQUIRED_VALUE_FLAG}" )

# certificate for the CLI package on MacOS
MAC_PACKAGE_CERT="horizon-cli.crt"

# Script help
function help() {
     cat << EndOfMessage
$(basename "$0") <options> -- installing Horizon software

Required Input Variables (in environment or config file):
    HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_ORG_ID, either HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH
    
Input Variables Specific to Cluster Node Type (in environment or config file):
    IMAGE_ON_EDGE_CLUSTER_REGISTRY: (required) agent image path (without tag) in edge cluster registry. For OCP: <registry-host>/<agent-project>/amd64_anax_k8s, for microsk8s: localhost:32000/<agent-namespace>/amd64_anax_k8s
    EDGE_CLUSTER_STORAGE_CLASS: (optional) the storage class to use for the agent and edge services. (Storage class must already be created.)

Parameters:
    -c    path to a certificate file
    -k    path to a configuration file (if not specified, uses agent-install.cfg in current directory, if present)
    -p    pattern name to register with (if not specified, registers node w/o pattern)
    -i    installation packages location (if not specified, uses current directory). if the argument begins with 'http' or 'https', will use as an apt repository
    -j    file location for the public key for an apt repository specified with '-i'
    -t    set a branch to use in the apt repo specified with -i. default is 'updates'
    -n    path to a node policy file
    -s    skip registration
    -v    show version
    -l    logging verbosity level (0: silent, 1: critical, 2: error, 3: warning, 4: info, 5: debug), the default is (3: warning)
    -u    exchange user authorization credentials
    -a    exchange node authorization credentials
    -d    the id to register this node with
    -f    install older version without prompt. overwrite configured node without prompt.
    -b    skip any prompts for user input
    -w    wait for the named service to start executing on this node
    -o    specify an org id for the service specified with '-w'
    -z    specify the name of your agent installation tar file. Default is ./agent-install-files.tar.gz
    -D    specify node type (device, cluster. If not specifed, uses device by default).
    -U    specify internal url for edge cluster registry (If not specified, this script will detect if cluster is local. Use "image-registry.openshift-image-registry.svc:5000" by default for ocp image registry)

Example: ./$(basename "$0") -i <path_to_package(s)>

EndOfMessage
}

function version() {
	echo "$(basename "$0") version: ${SCRIPT_VERSION}"
	exit 0
}

# Exit handling
function quit(){
  case $1 in
    1) echo "Exiting..."; exit 1
    ;;
    2) echo "Input error, exiting..."; exit 2
    ;;
    *) exit
    ;;
  esac
}

function now() {
	echo `date '+%Y-%m-%d %H:%M:%S'`
}

# Logging
VERB_SILENT=0
VERB_CRITICAL=1
VERB_ERROR=2
VERB_WARNING=3
VERB_INFO=4
VERB_DEBUG=5

# Always printed
function log_notify() {
    log $VERB_SILENT "$1"
}

function log_critical() {
    log $VERB_CRITICAL "CRITICAL ERROR: $1"
}

function log_error() {
    log $VERB_ERROR "ERROR: $1"
}

# The current default highest output level
function log_warning() {
    log $VERB_WARNING "WARNING: $1"
}

# This is more like verbose
function log_info() {
    log $VERB_INFO "INFO: $1"
}

function log_debug() {
    log $VERB_DEBUG "DEBUG: $1"
}

function now() {
	echo `date '+%Y-%m-%d %H:%M:%S'`
}

function log() {
    if [ $VERBOSITY -ge $1 ]; then
        #echo `now` "$2" | fold -w80 -s
        echo `now` "$2"
    fi
}

# Get the specified input variable. Precedence: If env variable is defined uses it, if not check in the config file
function get_variable() {
	log_debug "get_variable() begin"

    local var_to_check=$1
    local config_file=$2
    local default_val="$3"   # optional

	if [[ -n "${!var_to_check}" ]]; then
		# do not display the value of secrets
		if [[ $var_to_check == *"AUTH"* ]] || [[ $var_to_check == *"TOKEN"* ]]; then
			varValue='******'
		else
			varValue="${!var_to_check}"
		fi
		log_notify "${var_to_check}: $varValue (from environment)"
	elif [[ -f "$config_file" ]] ; then
		# Variable not defined in the environment, look in the config file
		# Note: we already checked for the existence of the config file before get_variable() is called, and printed a warning if not found, so don't have to do that here
		log_debug "The ${var_to_check} is missed in environment/not specified with command line, looking for it in the config file ${config_file} ..."
		if [ -z "$(grep ${var_to_check} ${config_file} | grep "^#")" ] && ! [ -z "$(grep ${var_to_check} ${config_file} | cut -d'=' -f2 | cut -d'"' -f2)" ]; then
			# found variable in the config file
			ref=${var_to_check}
			IFS= read -r "$ref" <<< $(grep ${var_to_check} ${config_file} | cut -d'=' -f2 | cut -d'"' -f2)
			if [[ $var_to_check == *"AUTH"* ]] || [[ $var_to_check == *"TOKEN"* ]]; then
				varValue='******'
			else
				varValue="${!var_to_check}"
			fi
			log_notify "${var_to_check}: $varValue (from ${config_file})"
		fi
	elif [[ -n "$default_val" ]]; then
		# A default value was passed into this function, use that
		ref=${var_to_check}
		IFS= read -r "$ref" <<< "$default_val"
		log_notify "${var_to_check}: ${!var_to_check} (default)"
	fi

	if [[ -z "${!var_to_check}" ]]; then
		# Did not find a value anywhere. See if it is required
		if [[ " ${REQUIRED_PARAMS[*]} " == *" ${var_to_check} "* ]]; then
			log_notify "${var_to_check} is required but was not set in either the environment or ${config_file}, exiting..."
			exit 1
		else
			log_notify "${var_to_check}: (not found)"
		fi
	fi

	log_debug "get_variable() end"
}

# validates if mutually exclusive arguments are mutually exclusive
function validate_mutual_ex() {
	log_debug "validate_mutual_ex() begin"
    local policy=$1
    local pattern=$2

	if [[ -n "${!policy}" && -n "${!pattern}" ]]; then
		echo "Both ${policy}=${!policy} and ${pattern}=${!pattern} mutually exlusive parameters are defined, exiting..."
		exit 1
	fi

	log_debug "validate_mutual_ex() end"
}

function validate_number_int() {
	log_debug "validate_number_int() begin"
    local verbosity_int_val=$1

	re='^[0-9]+$'
	if [[ $verbosity_int_val =~ $re ]] ; then
   		# integer, validate if it's in a correct range
   		if ! (($verbosity_int_val >= VERB_SILENT && $verbosity_int_val <= VERB_DEBUG)); then
   			echo `now` "The verbosity number is not in range [${VERB_SILENT}; ${VERB_DEBUG}]."
  			quit 2
		fi
   	else
   		echo `now` "The provided verbosity value ${verbosity_int_val} is not a number" >&2; quit 2
	fi

	log_debug "validate_number_int() end"
}

# Checks input arguments and env variables specified. Side effect: sets KUBECTL to the kubectl cmd on this host
function validate_args(){
	log_debug "validate_args() begin"

    log_info "Checking script arguments..."

    if [ "${DEPLOY_TYPE}" == "device" ]; then
    	# preliminary check for script arguments
    	check_empty "$PKG_PATH" "path to installation packages"
    	if [[ ${PKG_PATH:0:4} == "http" ]]; then
	    PKG_APT_REPO="$PKG_PATH"
	    if [[ "${PKG_APT_REPO: -1}" == "/" ]]; then
		    PKG_APT_REPO=$(echo "$PKG_APT_REPO" | sed 's/\/$//')
	    fi
	    PKG_PATH="."
    	else
	    PKG_PATH=$(echo "$PKG_PATH" | sed 's/\/$//')
	    check_exist d "$PKG_PATH" "The package installation"
    	fi
    else
      # check kubectl is available
      KUBECTL=${KUBECTL:-kubectl}   # the default is kubectl, or what they set in the env var
      if command -v "$KUBECTL" >/dev/null 2>&1; then
        :   # nothing more to do
      elif command -v microk8s.kubectl >/dev/null 2>&1; then
        KUBECTL=microk8s.kubectl
      else
                log_notify "$KUBECTL is not available, please install $KUBECTL and ensure that it is found on your \$PATH. Exiting..."
		exit 1
      fi

      check_installed "docker" "Docker"
      check_installed "jq" "jq"
    fi

    check_empty "$SKIP_REGISTRATION" "registration flag"
    log_info "Check finished successfully"

    log_info "Checking configuration..."
    # read and validate configuration
	if [[ -f "$CFG" ]]; then
		log_info "Using configuration file: $CFG"
	else
		log_warning "Configuration file $CFG not found. All required input variables must be set in the environment."
	fi
    get_variable HZN_EXCHANGE_URL $CFG
    check_empty "$HZN_EXCHANGE_URL" "Exchange URL"
    get_variable HZN_FSS_CSSURL $CFG
    check_empty "$HZN_FSS_CSSURL" "FSS_CSS URL"
    get_variable HZN_ORG_ID $CFG
    check_empty "$HZN_ORG_ID" "ORG ID"
    get_variable HZN_EXCHANGE_NODE_AUTH $CFG
    #check_empty $HZN_EXCHANGE_NODE_AUTH "Exchange Node Auth"
    get_variable HZN_EXCHANGE_USER_AUTH $CFG
    #check_empty $HZN_EXCHANGE_USER_AUTH "Exchange User Auth"
    get_variable NODE_ID $CFG

    # If USER_AUTH and NODE_AUTH are unset, exit with error code of 1
	if [[ -z $HZN_EXCHANGE_USER_AUTH ]] && [[ -z $HZN_EXCHANGE_NODE_AUTH ]]; then
		help; exit 1
	fi

	# if NODE_AUTH is set, get the node id to save in /etc/default/horizon
	if [[ -n $HZN_EXCHANGE_NODE_AUTH ]]; then
		DEVICE_ID=${HZN_EXCHANGE_NODE_AUTH%%:*}
	else
		DEVICE_ID=${HOSTNAME}
	fi

    if [ "${DEPLOY_TYPE}" == "cluster" ]; then
	if [[ "$NODE_ID" == "" ]]; then
		log_notify "The NODE_ID value is empty. Please set NODE_ID to edge cluster name in agent-install.cfg or as environment variable. Exiting..."
		exit 1
	fi

	get_variable EDGE_CLUSTER_STORAGE_CLASS $CFG 'gp2'

	get_variable AGENT_NAMESPACE $CFG 'openhorizon-agent'

	get_variable AGENT_IMAGE_TAG $CFG
	check_empty "$AGENT_IMAGE_TAG" "Agent image tag"

	# $USE_EDGE_CLUSTER_REGISTRY is set to true by default
	if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
		get_variable EDGE_CLUSTER_REGISTRY_USERNAME $CFG
		get_variable EDGE_CLUSTER_REGISTRY_TOKEN $CFG
		get_variable IMAGE_ON_EDGE_CLUSTER_REGISTRY $CFG
		check_empty "$IMAGE_ON_EDGE_CLUSTER_REGISTRY" "Image on edge cluster registry"
		parts=$(echo $IMAGE_ON_EDGE_CLUSTER_REGISTRY|awk -F'/' '{print NF}')
    		if [ "$parts" != "3" ]; then
			log_notify "\$IMAGE_ON_EDGE_CLUSTER_REGISTRY should be this format: <registry-host>/<registry-repo>/<image-name>"
			exit 1
		fi
		IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY=$IMAGE_ON_EDGE_CLUSTER_REGISTRY:$AGENT_IMAGE_TAG
		get_variable INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY $CFG
	fi

    fi
    get_variable CERTIFICATE $CFG
    get_variable HZN_MGMT_HUB_CERT_PATH $CFG
    if [[ "$CERTIFICATE" == "" ]]; then
	    if [[ "$HZN_MGMT_HUB_CERT_PATH" != "" ]]; then
		    CERTIFICATE=$HZN_MGMT_HUB_CERT_PATH
	    elif [ -f "$CERTIFICATE_DEFAULT" ]; then
		    CERTIFICATE="$CERTIFICATE_DEFAULT"
	    fi
    fi

    get_variable HZN_EXCHANGE_PATTERN $CFG

    get_variable HZN_NODE_POLICY $CFG
    # check on mutual exclusive params (node policy and pattern name)
	validate_mutual_ex "HZN_NODE_POLICY" "HZN_EXCHANGE_PATTERN"

	# if a node policy is non-empty, check if the file exists
	if [[ -n  $HZN_NODE_POLICY ]]; then
		check_exist f "$HZN_NODE_POLICY" "The node policy"
	fi

    if [ "${DEPLOY_TYPE}" == "device" ]; then
    	if [[ -z "$WAIT_FOR_SERVICE_ORG" ]] && [[ -n "$WAIT_FOR_SERVICE" ]]; then
    		log_error "Must specify service with -w to use with -o organization. Ignoring -o flag."
		unset WAIT_FOR_SERVICE_ORG
    	fi
    fi

    log_info "Check finished successfully"
    log_debug "validate_args() end"
}

function show_config() {
	log_debug "show_config() begin"

	# Unless verbose, only show cmd line args (env var/cfg vars were already displayed)
    echo "Installation packages location: ${PKG_PATH}"
    echo "Ignore package tree: ${PKG_TREE_IGNORE}"
    echo "Node policy: ${HZN_NODE_POLICY}"
    echo "NODE_ID"=${NODE_ID}
	echo "Image Full Path On Edge Cluster Registry: ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}"
	echo "Internal URL for Edge Cluster Registry: ${INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY}"

	if [[ $VERBOSITY -lt $VERB_INFO ]]; then
		return
	fi

    echo "Current configuration:"
    echo "Certification file: ${CERTIFICATE}"
    echo "Configuration file: ${CFG}"
    echo "Pattern name: ${HZN_EXCHANGE_PATTERN}"
    echo "Skip registration: ${SKIP_REGISTRATION}"
    echo "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}"
    echo "HZN_FSS_CSSURL=${HZN_FSS_CSSURL}"
    echo "HZN_ORG_ID=${HZN_ORG_ID}"
    echo "HZN_EXCHANGE_USER_AUTH=<specified>"
    echo "Verbosity is ${VERBOSITY}"
    echo "Agent in Edge Cluster config:"
    echo "AGENT_NAMESPACE: ${AGENT_NAMESPACE}"
    echo "Edge Cluster Storage Class: ${EDGE_CLUSTER_STORAGE_CLASS}"
    echo "Using Edge Cluster Registry: ${USE_EDGE_CLUSTER_REGISTRY}"
    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
	echo "Edge Cluster Registry Username: ${EDGE_CLUSTER_REGISTRY_USERNAME}"
	if [[ -z $EDGE_CLUSTER_REGISTRY_TOKEN ]]; then
		echo "Edge Cluster Registry Token:"
	else
		echo "Edge Cluster Registry Token: <specified>"
	fi
	echo "Image On Edge Cluster Registry: ${IMAGE_ON_EDGE_CLUSTER_REGISTRY}"
    fi

    log_debug "show_config() end"
}

function check_installed() {
	log_debug "check_installed() begin"
    local prog=$1
    local prog_name=$2
    local brew_installer=$3

    if command -v "$prog" >/dev/null 2>&1; then
        log_info "${prog_name} is installed"
    elif [[ $brew_installer != "" ]]; then
      if command -v "$brew_installer" >/dev/null 2>&1; then
        log_notify "${prog_name} not found. Attempting to install with ${brew_installer}"
        set -x
        $brew_installer install "$prog_name"
        { set +x; } 2>/dev/null
      fi
      if command -v "$prog" >/dev/null 2>&1; then
        log_info "${prog_name} is now installed"
      else
        log_info "Failed to install ${prog_name} with ${brew_installer}. Please install ${prog_name}"
      fi
    else
        log_notify "${prog_name} not found, please install it"
        quit 1
    fi

    log_debug "check_installed() end"
}

# compare versions
function version_gt() {
    local version=$1
	test "$(printf '%s\n' "$@" | sort -V | head -n 1)" != "$version";
}

# create /etc/default/horizon file for mac or linux
function create_config() {
    if [[ -n "${HZN_EXCHANGE_URL}" ]] && [[ -n "${HZN_FSS_CSSURL}" ]]; then
            log_info "Found environment variables HZN_EXCHANGE_URL and HZN_FSS_CSSURL, updating horizon config..."
            set -x
		if [ -z "$CERTIFICATE" ]; then
			sudo sed -i.bak -e "s~^HZN_EXCHANGE_URL=[^ ]*~HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}~g" \
				-e "s~^HZN_DEVICE_ID=[^ ]*~HZN_DEVICE_ID=${DEVICE_ID}~g" \
				-e "s~^HZN_FSS_CSSURL=[^ ]*~HZN_FSS_CSSURL=${HZN_FSS_CSSURL}~g" /etc/default/horizon
		else
			if [[ ${CERTIFICATE:0:1} != "/" ]]; then
                set -x
				sudo cp $CERTIFICATE /etc/horizon/agent-install.crt
                { set +x; } 2>/dev/null
				ABS_CERTIFICATE=/etc/horizon/agent-install.crt
			else
				ABS_CERTIFICATE=${CERTIFICATE}
			fi
			sudo sed -i.bak -e "s~^HZN_EXCHANGE_URL=[^ ]*~HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}~g" \
				-e "s~^HZN_FSS_CSSURL=[^ ]*~HZN_FSS_CSSURL=${HZN_FSS_CSSURL}~g" \
				-e "s~^HZN_DEVICE_ID=[^ ]*~HZN_DEVICE_ID=${DEVICE_ID}~g" \
				-e "s~^HZN_MGMT_HUB_CERT_PATH=[^ ]*~HZN_MGMT_HUB_CERT_PATH=${ABS_CERTIFICATE}~g" /etc/default/horizon
		fi
            { set +x; } 2>/dev/null
            log_info "Config updated"
    fi
}

function install_macos() {
    log_debug "install_macos() begin"

    log_notify "Installing agent on ${OS}..."

    log_info "Checking ${OS} specific prerequisites..."
    check_installed "socat" "socat"
    check_installed "docker" "Docker"
    check_installed "jq" "jq" "brew"

    # Setting up a certificate
    log_info "Importing the horizon-cli package certificate into Mac OS keychain..."
    set -x

    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ${PACKAGES}/${MAC_PACKAGE_CERT}
    { set +x; } 2>/dev/null
	if [[ "$CERTIFICATE" != "" ]]; then
		log_info "Configuring an edge node to trust the ICP certificate ..."
		set -x
		sudo cp $CERTIFICATE /private/etc/horizon/agent-install.crt
		CERTIFICATE=/private/etc/horizon/agent-install.crt
		sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$CERTIFICATE"
		{ set +x; } 2>/dev/null
	fi

	PKG_NAME=$(find . -name "horizon-cli*\.pkg" | sort -V | tail -n 1 | cut -d "/" -f 2)
	log_info "Detecting packages version..."
	PACKAGE_VERSION=$(echo ${PACKAGES}/$PKG_NAME | cut -d'-' -f3 | cut -d'.' -f1-3)
	ICP_VERSION=$(echo ${PACKAGES}/$PKG_NAME | cut -d'-' -f4 | cut -d'.' -f1-3)

	log_info "The packages version is ${PACKAGE_VERSION}"
	log_info "The ICP version is ${ICP_VERSION}"
	if [[ -z "$ICP_VERSION" ]]; then
		export HC_DOCKER_TAG="$PACKAGE_VERSION"
	else
		export HC_DOCKER_TAG="${PACKAGE_VERSION}-${ICP_VERSION}"
	fi

	log_debug "Setting up the agent container tag on Mac..."
    log_debug "HC_DOCKER_TAG is ${HC_DOCKER_TAG}"

    log_info "Checking if hzn is installed..."
    if command -v hzn >/dev/null 2>&1; then
    	# if hzn is installed, need to check the current setup
		log_info "hzn found, checking setup..."
		AGENT_VERSION=$(hzn version | grep "^Horizon Agent" | sed 's/^.*: //' | cut -d'-' -f1)
		log_info "Found Agent version is ${AGENT_VERSION}"
		re='^[0-9]+([.][0-9]+)+([.][0-9]+)'
		if ! [[ $AGENT_VERSION =~ $re ]] ; then
			log_info "Something's wrong. Can't get the agent verison, installing it..."
			set -x
	        sudo installer -pkg ${PACKAGES}/$PKG_NAME -target /
	        { set +x; } 2>/dev/null
		else
			# compare version for installing and what we have
			log_info "Comparing agent and packages versions..."
			if [ "$AGENT_VERSION" = "$PACKAGE_VERSION" ] && [ ! "$OVERWRITE" = true ]; then
				log_info "Versions are equal: agent is ${AGENT_VERSION} and packages are ${PACKAGE_VERSION}. Don't need to install"
			else
				if version_gt "$AGENT_VERSION" "$PACKAGE_VERSION"; then
					log_info "Installed agent ${AGENT_VERSION} is newer than the packages ${PACKAGE_VERSION}"
					if [ ! "$OVERWRITE" = true ] && [[ $SKIP_PROMPT == 'false' ]] ; then
						if [ $BATCH_INSTALL -eq 1 ]; then
							exit 1
						fi
						echo "The installed agent is newer than one you're trying to install, continue?[y/N]:"
						read RESPONSE
						if [ ! "$RESPONSE" == 'y' ]; then
							echo "Exiting at users request"
							exit
						fi
					fi
					log_notify "Installing older packages ${PACKAGE_VERSION}..."
					set -x
        			sudo installer -pkg ${PACKAGES}/$PKG_NAME -target /
        			{ set +x; } 2>/dev/null
				else
					log_info "Installed agent is ${AGENT_VERSION}, package is ${PACKAGE_VERSION}"
					log_notify "Installing newer package (${PACKAGE_VERSION}) ..."
					set -x
        			sudo installer -pkg ${PACKAGES}/$PKG_NAME -target /
        			{ set +x; } 2>/dev/null
				fi
			fi
		fi
	else
        log_notify "hzn not found, installing it..."
        set -x
        sudo installer -pkg ${PACKAGES}/$PKG_NAME -target /
        { set +x; } 2>/dev/null
	fi

	start_horizon_service

	process_node

    # configuring agent inside the container
    HZN_CONFIG=/etc/default/horizon
    log_info "Configuring ${HZN_CONFIG} file for the agent container..."
    HZN_CONFIG_DIR=$(dirname "${HZN_CONFIG}")
    if ! [[ -f "$HZN_CONFIG" ]] ; then
	    log_info "$HZN_CONFIG file doesn't exist, creating..."
	    # check if the directory exists
	    if ! [[ -d "$(dirname "${HZN_CONFIG}")" ]] ; then
		    log_info "The directory ${HZN_CONFIG_DIR} doesn't exist, creating..."
            set -x
		    sudo mkdir -p "$HZN_CONFIG_DIR"
            { set +x; } 2>/dev/null
	    fi
	    log_info "Creating ${HZN_CONFIG} file..."
        set -x
	if [ -z "$CERTIFICATE" ]; then
		printf "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL} \nHZN_FSS_CSSURL=${HZN_FSS_CSSURL} \
			\nHZN_DEVICE_ID=${DEVICE_ID}"  | sudo tee "$HZN_CONFIG"
	else
		if [[ ${CERTIFICATE:0:1} != "/" ]]; then
			#ABS_CERTIFICATE=$(pwd)/${CERTIFICATE}
			sudo cp $CERTIFICATE /private/etc/horizon/agent-install.crt
			ABS_CERTIFICATE=/private/etc/horizon/agent-install.crt
		else
			ABS_CERTIFICATE=${CERTIFICATE}
		fi
		printf "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL} \nHZN_FSS_CSSURL=${HZN_FSS_CSSURL} \
			\nHZN_DEVICE_ID=${DEVICE_ID} \nHZN_MGMT_HUB_CERT_PATH=${ABS_CERTIFICATE}"  | sudo tee "$HZN_CONFIG"
	fi

        { set +x; } 2>/dev/null
        log_info "Config created"
    else
        create_config
    fi

	start_horizon_service

	create_node

	registration "$SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"

    log_debug "install_macos() end"
}

function install_linux(){
    log_debug "install_linux() begin"
    log_notify "Installing agent on ${DISTRO}, version ${CODENAME}, architecture ${ARCH}"

    ANAX_PORT=8510

    if [[ "$OS" == "linux" ]]; then
        if [ -f /etc/default/horizon ]; then
            log_info "Getting agent port from /etc/default/horizon file..."
            anaxPort=$(grep HZN_AGENT_PORT /etc/default/horizon |cut -d'=' -f2)
            if [[ "$anaxPort" == "" ]]; then
                log_info "Cannot detect agent port as /etc/default/horizon does not contain HZN_AGENT_PORT, using ${ANAX_PORT} instead"
            else
                ANAX_PORT=$anaxPort
            fi
        else
            log_info "Cannot detect agent port as /etc/default/horizon cannot be found, using ${ANAX_PORT} instead"
        fi
    fi

	log_info "Checking if the agent port ${ANAX_PORT} is free..."
	local netStat=`netstat -nlp | grep $ANAX_PORT`
	if [[ $netStat == *$ANAX_PORT* ]]; then
		log_info "Something is running on ${ANAX_PORT}..."
		if [[ ! $netStat == *anax* ]]; then
			log_notify "It's not anax, please free the port in order to install horizon, exiting..."
			exit 1
		else
			log_info "It's anax, continuing..."
		fi
	else
		log_info "Anax port ${ANAX_PORT} is free, continuing..."
	fi

    log_info "Checking if curl is installed..."
    if command -v curl >/dev/null 2>&1; then
		log_info "curl found"
	else
        log_info "curl not found, installing it..."
        set -x
        apt install -y curl
        { set +x; } 2>/dev/null
        log_info "curl installed"
	fi

    log_info "Checking if jq is installed..."
	if command -v jq >/dev/null 2>&1; then
		log_info "jq found"
	else
        log_info "jq not found, installing it..."
        set -x
        apt install -y jq
        { set +x; } 2>/dev/null
        log_info "jq installed"
	fi

    log_info "Checking if docker is installed..."
    if command -v docker >/dev/null 2>&1; then
        log_info "docker found"
    else
        log_info "docker not found, please install docker and rerun the agent_install script."
        exit 1
    fi

    if [[ -n "$PKG_APT_REPO" ]]; then
	    if [[ -n "$PKG_APT_KEY" ]]; then
		    log_info "Adding key $PKG_APT_KEY"
		    set -x
		    apt-key add "$PKG_APT_KEY"
		    { set +x; } 2>/dev/null
	    fi
	    if [[ -z "$APT_REPO_BRANCH" ]]; then
		    APT_REPO_BRANCH="updates"
	    fi
	    log_info "Adding $PKG_APT_REPO to /etc/sources to install with apt"
	    set -x
	    add-apt-repository "deb $PKG_APT_REPO ${CODENAME}-$APT_REPO_BRANCH main"
	    apt-get install bluehorizon -y -f
	    { set +x; } 2>/dev/null
    else
    	log_info "Checking if hzn is installed..."
    	if command -v hzn >/dev/null 2>&1; then
    		# if hzn is installed, need to check the current setup
		log_info "hzn found, checking setup..."
		AGENT_VERSION=$(hzn version | grep "^Horizon Agent" | sed 's/^.*: //' | cut -d'-' -f1)
		log_info "Found Agent version is ${AGENT_VERSION}"
		re='^[0-9]+([.][0-9]+)+([.][0-9]+)'
		if ! [[ $AGENT_VERSION =~ $re ]] ; then
			log_notify "Something's wrong. Can't get the agent verison, installing it..."
			set -x
	        set +e
	        dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
	        set -e
	        { set +x; } 2>/dev/null
        	log_notify "Resolving any dependency errors..."
        	set -x
        	apt update && apt-get install -y -f
        	{ set +x; } 2>/dev/null
		else
			# compare version for installing and what we have
			PACKAGE_VERSION=$(ls ${PACKAGES} | grep horizon-cli | cut -d'_' -f2 | cut -d'~' -f1)
			log_info "The packages version is ${PACKAGE_VERSION}"
			log_info "Comparing agent and packages versions..."
			if [ "$AGENT_VERSION" = "$PACKAGE_VERSION" ] && [ ! "$OVERWRITE" = true ]; then
				log_notify "Versions are equal: agent is ${AGENT_VERSION} and packages are ${PACKAGE_VERSION}. Don't need to install"
			else
				if version_gt "$AGENT_VERSION" "$PACKAGE_VERSION" ; then
					log_notify "Installed agent ${AGENT_VERSION} is newer than the packages ${PACKAGE_VERSION}"
					if [ ! "$OVERWRITE" = true ] && [[ $SKIP_PROMPT == 'false' ]] ; then
						if [ $BATCH_INSTALL -eq 1 ]; then
							exit 1
						fi
						echo "The installed agent is newer than one you're trying to install, continue?[y/N]:"
						read RESPONSE
						if [ ! "$RESPONSE" == 'y' ]; then
							echo "Exiting at users request"
							exit
						fi
					fi
					log_notify "Installing older packages ${PACKAGE_VERSION}..."
					set -x
		        	set +e
		        	dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
		        	set -e
		        	{ set +x; } 2>/dev/null
		        	log_notify "Resolving any dependency errors..."
		        	set -x
		        	apt update && apt-get install -y -f
		        	{ set +x; } 2>/dev/null
				else
					log_info "Installed agent is ${AGENT_VERSION}, package is ${PACKAGE_VERSION}"
					log_notify "Installing newer package (${PACKAGE_VERSION}) ..."
					set -x
		        	set +e
		        	dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
		        	set -e
		        	{ set +x; } 2>/dev/null
		        	log_notify "Resolving any dependency errors..."
		        	set -x
		        	apt update && apt-get install -y -f
		        	{ set +x; } 2>/dev/null
				fi
			fi
		fi
	else
        log_notify "hzn not found, installing it..."
        set -x
        set +e
        dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
        set -e
        { set +x; } 2>/dev/null
        log_notify "Resolving any dependency errors..."
        set -x
        apt update && apt-get install -y -f
        { set +x; } 2>/dev/null
	fi
    fi

    if [[ -f "/etc/horizon/anax.json" ]]; then
	    while read line; do
        	if [[ $(echo $line | grep "APIListen")  != "" ]]; then
    			if [[ $(echo $line | cut -d ":" -f 3 | cut -d "\"" -f 1 ) != "$ANAX_PORT" ]]; then
            			ANAX_PORT=$(echo $line | cut -d ":" -f 3 | cut -d "\"" -f 1 )
				log_info "Using anax port $ANAX_PORT"
    			fi
		break
		fi
    	    done </etc/horizon/anax.json
    fi

    process_node

    check_exist f "/etc/default/horizon" "horizon configuration"
    # The /etc/default/horizon creates upon horizon deb packages installation
    create_config

    log_info "Restarting the service..."
    set -x
    systemctl restart horizon.service
    { set +x; } 2>/dev/null

    start_anax_service_check=`date +%s`

    while [ -z "$(curl -sm 10 http://localhost:$ANAX_PORT/status | jq -r .configuration.exchange_version)" ] ; do
   		current_anax_service_check=`date +%s`
		log_notify "the service is not ready, will retry in 1 second"
		if (( current_anax_service_check - start_anax_service_check > 60 )); then
			log_notify "anax service timeout of 60 seconds occured"
			exit 1
		fi
		sleep 1
	done

    log_notify "The service is ready"

    create_node

    registration "$SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"

    log_debug "install_linux() end"
}

# start horizon service container on mac
function start_horizon_service(){
	log_debug "start_horizon_service() begin"

	if command -v horizon-container >/dev/null 2>&1; then
		if [[ -z $(docker ps -q --filter name=horizon1) ]]; then
			# horizn services container is not running

			if [[ -z $(docker ps -aq --filter name=horizon1) ]]; then
				# horizon services container doesn't exist
		    	log_info "Starting horizon services..."
		    	set -x
		    	horizon-container start
		    	{ set +x; } 2>/dev/null
			else
				# horizon services are shutdown but the container exists
				docker start horizon1
			fi

		   	start_horizon_container_check=`date +%s`

		    while [ -z "$(hzn node list | jq -r .configuration.preferred_exchange_version 2>/dev/null)" ] ; do
		    	current_horizon_container_check=`date +%s`
				log_info "the horizon-container with anax is not ready, retry in 10 seconds"
				if (( current_horizon_container_check - start_horizon_container_check > 300 )); then
					log_notify "horizon container timeout of 60 seconds occured"
					exit 1
				fi
				sleep 10
			done

			log_info "The horizon-container is ready"
		else
			log_info "The horizon-container is running already..."
		fi
	else
        log_notify "horizon-container not found, hzn is not installed or its installation is broken, exiting..."
        exit 1
	fi


	log_debug "start_horizon_service() end"
}

# stops horizon service container on mac
function stop_horizon_service(){
	log_debug "stop_horizon_service() begin"

	# check if the horizon-container script exists
    if command -v horizon-container >/dev/null 2>&1; then
		# horizon-container script is installed
        if ! [[ -z $(docker ps -q --filter name=horizon1) ]]; then
			log_info "Stopping the Horizon services container...."
			set -x
            horizon-container stop
            { set +x; } 2>/dev/null
        fi
	else
        log_notify "horizon-container not found, hzn is not installed or its installation is broken, exiting..."
        exit 1
	fi

	log_debug "stop_horizon_service() end"
}

function process_node(){
	log_debug "process_node() begin"
  if [ -z "$OVERWRITE_NODE" ]; then
    OVERWRITE_NODE=$OVERWRITE
  fi

	# Checking node state
	NODE_STATE=$(hzn node list | jq -r .configstate.state)
	WORKLOADS=$(hzn agreement list | jq -r .[])
	if [[ "$NODE_ID" == "" ]] && [[ ! $OVERWRITE_NODE == "true" ]]; then
		NODE_ID=$(hzn node list | jq -r .id)
		log_notify "Registering node with existing id $NODE_ID"
	fi

	if [ "$NODE_STATE" = "configured" ]; then
		# node is registered
		log_info "Node is registered, state is ${NODE_STATE}"
		if [ -z "$WORKLOADS" ]; then
		 	# w/o pattern currently
			if [[ -z "$HZN_EXCHANGE_PATTERN" ]] && [[ -z "$HZN_NODE_POLICY" ]]; then
				log_info "Neither a pattern nor node policy has not been specified, skipping registration..."
		 	else
				if [[ -n "$HZN_EXCHANGE_PATTERN" ]]; then
					log_info "There's no workloads running, but ${HZN_EXCHANGE_PATTERN} pattern has been specified"
					log_info "Unregistering the node and register it again with the new ${HZN_EXCHANGE_PATTERN} pattern..."
				fi
				if [[ -n "$HZN_NODE_POLICY" ]]; then
					log_info "There's no workloads running, but ${HZN_NODE_POLICY} node policy has been specified"
					log_info "Unregistering the node and register it again with the new ${HZN_NODE_POLICY} node policy..."
				fi
				set -x
    			hzn unregister -rf
    			{ set +x; } 2>/dev/null
				# if mac, need to stop the horizon services container
				if [[ "$OS" == "macos" ]]; then
					stop_horizon_service
				fi
    		fi
		else
			# with a pattern currently
			log_notify "The node currently has workload(s) (check them with hzn agreement list)"
			if [[ -z "$HZN_EXCHANGE_PATTERN" ]] && [[ -z "$HZN_NODE_POLICY" ]]; then
				log_info "Neither a pattern nor node policy has been specified"
				if [[ ! "$OVERWRITE_NODE" = "true" ]] && [ $BATCH_INSTALL -eq 0 ] && [[ $SKIP_PROMPT == 'false' ]] ; then
					echo "Do you want to unregister node and register it without pattern or node policy, continue?[y/N]:"
					read RESPONSE
					if [ ! "$RESPONSE" == 'y' ]; then
						echo "Exiting at users request"
						exit
					fi
				fi
				log_notify "Unregistering the node and register it again without pattern or node policy..."
			else
				if [[ -n "$HZN_EXCHANGE_PATTERN" ]]; then
					log_notify "${HZN_EXCHANGE_PATTERN} pattern has been specified"
				fi
				if [[ -n "$HZN_NODE_POLICY" ]]; then
					log_notify "${HZN_NODE_POLICY} node policy has been specified"
				fi
				if [[ "$OVERWRITE_NODE" != "true" ]] && [ $BATCH_INSTALL -eq 0 ] && [[ $SKIP_PROMPT == 'false' ]] ; then
					if [[ -n "$HZN_EXCHANGE_PATTERN" ]]; then
						echo "Do you want to unregister and register it with a new ${HZN_EXCHANGE_PATTERN} pattern, continue?[y/N]:"
					fi
					if [[ -n "$HZN_NODE_POLICY" ]]; then
						echo "Do you want to unregister and register it with a new ${HZN_NODE_POLICY} node policy, continue?[y/N]:"
					fi
					read RESPONSE
					if [ ! "$RESPONSE" == 'y' ]; then
						echo "Exiting at users request"
						exit
					fi
				fi
				if [[ -n "$HZN_EXCHANGE_PATTERN" ]]; then
					log_notify "Unregistering the node and register it again with the new ${HZN_EXCHANGE_PATTERN} pattern..."
				fi
				if [[ -n "$HZN_NODE_POLICY" ]]; then
					log_notify "Unregistering the node and register it again with the new ${HZN_NODE_POLICY} node policy..."
				fi
			fi
		 	set -x
    		hzn unregister -rf
    		{ set +x; } 2>/dev/null
			# if mac, need to stop the horizon services container
			if [[ "$OS" == "macos" ]]; then
				stop_horizon_service
			fi
		fi
	else
		log_info "Node is not registered, state is ${NODE_STATE}"

		# if mac, need to stop the horizon services container
		if [[ "$OS" == "macos" ]]; then
			stop_horizon_service
		fi
	fi

	log_debug "process_node() end"

}

# For Device and Cluster: creates node
function create_node(){
	log_debug "create_node() begin"

    if [ "${DEPLOY_TYPE}" == "cluster" ]; then
	NODE_NAME=$NODE_ID
    else
	NODE_NAME=$HOSTNAME
    fi

    log_info "Node name is $NODE_NAME"
    if [ -z "$HZN_EXCHANGE_NODE_AUTH" ]; then
        log_info "HZN_EXCHANGE_NODE_AUTH is not defined, creating it..."
        if [ "${DEPLOY_TYPE}" == "device" ]; then
		if [[ "$OS" == "linux" ]]; then
            		if [ -f /etc/default/horizon ]; then
              			if [[ "$NODE_ID" == "" ]]; then
                			log_info "Getting node id from /etc/default/horizon file..."
                			NODE_ID=$(grep HZN_DEVICE_ID /etc/default/horizon |cut -d'=' -f2)
                			if [[ "$NODE_ID" == "" ]]; then
                    				NODE_ID=$HOSTNAME
                			fi
              			fi
            		else
                		log_info "Cannot detect node id as /etc/default/horizon cannot be found, using ${NODE_NAME} hostname instead"
                		NODE_ID=$NODE_NAME
            		fi
        	elif [[ "$OS" == "macos" ]]; then
            		log_info "Using hostname as node id..."
            		NODE_ID=$NODE_NAME
        	fi
	fi
        log_info "Node id is $NODE_ID"

        log_info "Generating node token..."
        HZN_NODE_TOKEN=$(cat /dev/urandom | env LC_CTYPE=C tr -dc 'a-zA-Z0-9' | fold -w 45 | head -n 1)
        log_notify "Generated node token is ${HZN_NODE_TOKEN}"
        HZN_EXCHANGE_NODE_AUTH="${NODE_ID}:${HZN_NODE_TOKEN}"
        log_info "HZN_EXCHANGE_NODE_AUTH for a node is ${HZN_EXCHANGE_NODE_AUTH}"
    else
        log_notify "Found HZN_EXCHANGE_NODE_AUTH variable, using it..."
    fi

    if [ "${DEPLOY_TYPE}" == "cluster" ]; then
	log_notify "Creating a node..."
	EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"
	HZN_EX_NODE_CREATE_CMD="hzn exchange node create -n \"$HZN_EXCHANGE_NODE_AUTH\" -m \"$NODE_NAME\" -o \"$HZN_ORG_ID\" -u \"$HZN_EXCHANGE_USER_AUTH\" -T \"cluster\""
	log_info "AGENT POD ID: ${POD_ID}"
	$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_EX_NODE_CREATE_CMD}"

	log_notify "Verifying a node..."
	HZN_EX_NODE_CFM_CMD="hzn exchange node confirm -n \"$HZN_EXCHANGE_NODE_AUTH\" -o \"$HZN_ORG_ID\""
	$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_EX_NODE_CFM_CMD}"
    fi

    log_debug "create_node() end"
}

# For Device and Cluster: register node depending on if registration's requested and pattern name or policy file
function registration() {
	log_debug "registration() begin"
    local skip_reg=$1
    local pattern=$2
    local policy=$3

	if [ "${DEPLOY_TYPE}" == "device" ]; then
		NODE_STATE=$(hzn node list | jq -r .configstate.state)

		if [ "$NODE_STATE" = "configured" ]; then
			log_info "Node is registered already, skipping registration..."
			return 0
		fi

		WAIT_FOR_SERVICE_ARG=""
		if [[ "$WAIT_FOR_SERVICE" != "" ]]; then
			if [[ "$WAIT_FOR_SERVICE_ORG" != "" ]]; then
				WAIT_FOR_SERVICE_ARG=" -s $WAIT_FOR_SERVICE --serviceorg $WAIT_FOR_SERVICE_ORG "
			else
				WAIT_FOR_SERVICE_ARG=" -s $WAIT_FOR_SERVICE "
			fi
		fi
	fi

	if [ "${DEPLOY_TYPE}" == "device" ]; then
    		NODE_NAME=$HOSTNAME
	elif [ "${DEPLOY_TYPE}" == "cluster" ]; then
		NODE_NAME=$NODE_ID
		EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"
	fi
    log_info "Node name is $NODE_NAME"
    if [ "$skip_reg" = true ] ; then
        log_notify "Skipping registration as it was specified with -s"
    else
        log_notify "Registering node..."
        if [[ -z "${2}" ]]; then
        	if [[ -z "${3}" ]]; then
        		log_info "Neither a pattern nor node policy were not specified, registering without it..."
            		if [ "${DEPLOY_TYPE}" == "device" ]; then
				set -x
            			hzn register -m "${NODE_NAME}" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" -n "$HZN_EXCHANGE_NODE_AUTH" $WAIT_FOR_SERVICE_ARG
            			{ set +x; } 2>/dev/null
			elif [ "${DEPLOY_TYPE}" == "cluster" ]; then
				HZN_REGISTER_CMD="hzn register -n \"$HZN_EXCHANGE_NODE_AUTH\" -m \"$NODE_NAME\" -o \"$HZN_ORG_ID\" -u \"$HZN_EXCHANGE_USER_AUTH\""
				log_info "AGENT POD ID: ${POD_ID}"
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_REGISTER_CMD}"
			fi
                else
        		log_info "Node policy ${HZN_NODE_POLICY} was specified, registering..."
            		if [ "${DEPLOY_TYPE}" == "device" ]; then
				set -x
            			hzn register -m "${NODE_NAME}" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" -n "$HZN_EXCHANGE_NODE_AUTH" --policy "$policy" $WAIT_FOR_SERVICE_ARG
            			{ set +x; } 2>/dev/null
			elif [ "${DEPLOY_TYPE}" == "cluster" ]; then
				# copy policy file to /home/agentuser inside k8s container
				log_info "Copying policy file $policy to pod container..."
				POLICY_CONTENT=$(cat $policy | sed 's/\r/\n/')
				POLICY_FILE_NAME=$(basename "$policy")
				POLICY_FILE_IN_POD="/home/agentuser/${POLICY_FILE_NAME}"
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "echo '${POLICY_CONTENT}' >> ${POLICY_FILE_IN_POD}"

				# check if policy file exists
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "ls ${POLICY_FILE_IN_POD}"
				if [ $? -ne 0 ]; then
					log_notify "Failed to copy policy file $policy into pod container, existing..."
					exit 1
				else
					log_info "Copied policy file $policy to ${POLICY_FILE_IN_POD} inside pod container"
				fi

				HZN_REGISTER_CMD="hzn register -n \"$HZN_EXCHANGE_NODE_AUTH\" -m \"$NODE_NAME\" -o \"$HZN_ORG_ID\" -u \"$HZN_EXCHANGE_USER_AUTH\" --policy \"$POLICY_FILE_IN_POD\""
				log_info "AGENT POD ID: ${POD_ID}"
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_REGISTER_CMD}"
			fi
                fi
        else
        	if [[ -z "${policy}" ]]; then
        		log_info "Registering node with ${pattern} pattern"
			if [ "${DEPLOY_TYPE}" == "device" ]; then
            			set -x
            			hzn register -p "$pattern" -m "${NODE_NAME}" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" -n "$HZN_EXCHANGE_NODE_AUTH" $WAIT_FOR_SERVICE_ARG
            			{ set +x; } 2>/dev/null
			elif [ "${DEPLOY_TYPE}" == "cluster" ]; then
				HZN_REGISTER_CMD="hzn register -p \"$pattern\" -n \"$HZN_EXCHANGE_NODE_AUTH\" -m \"$NODE_NAME\" -o \"$HZN_ORG_ID\" -u \"$HZN_EXCHANGE_USER_AUTH\""
				log_info "AGENT POD ID: ${POD_ID}"
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_REGISTER_CMD}"
			fi
        	else
        		log_info "Pattern ${pattern} and policy ${policy} were specified. However, pattern registration will override the policy, registering..."
            		if [ "${DEPLOY_TYPE}" == "device" ]; then
				set -x
           	 		hzn register -p "$pattern" -m "${NODE_NAME}" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" -n "$HZN_EXCHANGE_NODE_AUTH" --policy "$policy" $WAIT_FOR_SERVICE_ARG
            			{ set +x; } 2>/dev/null
			elif [ "${DEPLOY_TYPE}" == "cluster" ]; then
				log_info "Copying policy file $policy to pod container..."
				POLICY_CONTENT=$(cat $policy)
				POLICY_FILE_NAME=$(basename "$policy")
				POLICY_FILE_IN_POD="/home/agentuser/${POLICY_FILE_NAME}"
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "echo '${POLICY_CONTENT}' >> ${POLICY_FILE_IN_POD}"

				HZN_REGISTER_CMD="hzn register -p \"$pattern\" -n \"$HZN_EXCHANGE_NODE_AUTH\" -m \"$NODE_NAME\" -o \"$HZN_ORG_ID\" -u \"$HZN_EXCHANGE_USER_AUTH\" --policy \"$POLICY_FILE_IN_POD\""
				log_info "AGENT POD ID: ${POD_ID}"
				$KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_REGISTER_CMD}"
			fi
                fi
        fi
    fi

    log_debug "registration() end"
}

#future: remove this function and instead add to REQUIRED_PARAMS so get_variable() takes care of this
function check_empty() {
	log_debug "check_empty() begin"
    local env_var_val=$1
    local env_var_desc=$2

    if [ -z "$env_var_val" ]; then
        log_notify "The ${env_var_desc} value is empty, exiting..."
        exit 1
    fi

    log_debug "check_empty() end"
}

# checks if file or directory exists
function check_exist() {
	log_debug "check_exist() begin"
    local flag=$1
    local val=$2
    local name=$3

    case $flag in
	f) if ! [[ -f "$val" ]] ; then
			log_notify "${name} file ${val} doesn't exist"
		    exit 1
		fi
	;;
	d) if ! [[ -d "$val" ]] ; then
			log_notify "${name} directory ${val} doesn't exist"
	        exit 1
		fi
    ;;
    w) if ! ls ${val} 1> /dev/null 2>&1 ; then
			log_notify "${name} files ${val} do not exist"
	        exit 1
	    fi
	;;
	*) echo "not supported"
        exit 1
	;;
	esac

	log_debug "check_exist() end"
}

# autocomplete support for CLI
function add_autocomplete() {
	log_debug "add_autocomplete() begin"

	log_info "Enabling autocomplete for the CLI commands..."

	SHELL_FILE="${SHELL##*/}"

    if [ -f "/etc/bash_completion.d/hzn_bash_autocomplete.sh" ]; then
        AUTOCOMPLETE="/etc/bash_completion.d/hzn_bash_autocomplete.sh"
    elif [ -f "/usr/local/share/horizon/hzn_bash_autocomplete.sh" ]; then
        # backward compatibility support
        AUTOCOMPLETE="/usr/local/share/horizon/hzn_bash_autocomplete.sh"
    fi

    if [[ -n "$AUTOCOMPLETE" ]]; then
    	if [ -f ~/.${SHELL_FILE}rc ]; then
            grep -q "^source ${AUTOCOMPLETE}" ~/.${SHELL_FILE}rc || \
            echo "source ${AUTOCOMPLETE}" >> ~/.${SHELL_FILE}rc
    	else
	    echo "source ${AUTOCOMPLETE}" > ~/.${SHELL_FILE}rc
    	fi
    else
        log_info "There's no an autocomplete script expected, skipping it..."
    fi

	log_debug "add_autocomplete() end"
}

# detects operating system.
function detect_os() {
    log_debug "detect_os() begin"

    if [[ "$OSTYPE" == "linux"* ]]; then
        OS="linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
    else
        OS="unknown"
    fi

    log_info "Detected OS is ${OS}"

    log_debug "detect_os() end"
}

# detects linux distribution name, version, and codename
function detect_distro() {
    log_debug "detect_distro() begin"

    if [ -f /etc/os-release ]; then
            . /etc/os-release
            DISTRO=$ID
            VER=$VERSION_ID
            CODENAME=$VERSION_CODENAME
    elif type lsb_release >/dev/null 2>&1; then
            DISTRO=$(lsb_release -si)
            VER=$(lsb_release -sr)
            CODENAME=$(lsb_release -sc)
    elif [ -f /etc/lsb-release ]; then
            . /etc/lsb-release
            DISTRO=$DISTRIB_ID
            VER=$DISTRIB_RELEASE
            CODENAME=$DISTRIB_CODENAME
    else
            log_notify "Cannot detect Linux version, exiting..."
            exit 1
    fi

    # Raspbian has a codename embedded in a version
    if [[ "$DISTRO" == "raspbian" ]]; then
        CODENAME=$(echo ${VERSION} | sed -e 's/.*(\(.*\))/\1/')
    fi

    log_info "Detected distribution is ${DISTRO}, verison is ${VER}, codename is ${CODENAME}"

    log_debug "detect_distro() end"
}

# detects hardware architecture on linux
function detect_arch() {
    log_debug "detect_arch() begin"

    # detecting architecture
    uname="$(uname -m)"
    if [[ "$uname" =~ "aarch64" ]]; then
        ARCH="arm64"
    elif [[ "$uname" =~ "arm" ]]; then
        ARCH="armhf"
    elif [[ "$uname" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$uname" == "ppc64le" ]]; then
        ARCH="ppc64el"
    else
        (>&2 echo "Unknown architecture $uname")
        exit 1
    fi

    log_info "Detected architecture is ${ARCH}"

    log_debug "detect_arch() end"
}

# checks if OS/distribution/codename/arch is supported
function check_support() {
    log_debug "check_support() begin"

    # checks if OS, distro or arch is supported

    if [[ ! "${1}" = *"${2}"* ]]; then
        echo "Supported components are: "
        for i in "${1}"; do echo -n "${i} "; done
        echo ""
        log_notify "The detected ${2} is not supported, exiting..."
        exit 1
    else
        log_info "The detected ${2} is supported"
    fi

    log_debug "check_support() end"
}

# checks if requirements are met
function check_requirements() {
    log_debug "check_requirements() begin"

    detect_os

    log_info "Checking support of detected OS..."
    check_support "${SUPPORTED_OS[*]}" "$OS"

    if [ "$OS" = "linux" ]; then
        detect_distro
        log_info "Checking support of detected Linux distribution..."
        check_support "${SUPPORTED_LINUX_DISTRO[*]}" "$DISTRO"
        log_info "Checking support of detected Linux version/codename..."
        check_support "${SUPPORTED_LINUX_VERSION[*]}" "$CODENAME"
        detect_arch
        log_info "Checking support of detected architecture..."
        check_support "${SUPPORTED_ARCH[*]}" "$ARCH"

	if [[ -z "$PKG_APT_REPO" ]]; then
        	log_info "Checking the path with packages..."

        	if [ "$PKG_TREE_IGNORE" = true ] ; then
        		# ignoring the package tree, checking the current dir
        		PACKAGES="${PKG_PATH}"
      		else
        		# checking the package tree for linux
        		PACKAGES="${PKG_PATH}/${OS}/${DISTRO}/${CODENAME}/${ARCH}"
        	fi

        	log_info "Checking path with packages ${PACKAGES}"
        	check_exist w "${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb" "Linux installation"
	fi

        if [ $(id -u) -ne 0 ]; then
	        log_notify "Please run script with the root priveleges by running 'sudo -s' command first"
            quit 1
        fi

    	elif [ "$OS" = "macos" ]; then
	    if [[ -z "$PKG_APT_REPO" ]]; then
    		log_info "Checking the path with packages..."

    		if [ "$PKG_TREE_IGNORE" = true ] ; then
      			# ignoring the package tree, checking the current dir
      			PACKAGES="${PKG_PATH}"
      		else
      			# checking the package tree for macos
      			PACKAGES="${PKG_PATH}/${OS}"
      		fi

      		log_info "Checking path with packages ${PACKAGES}"
      		check_exist w "${PACKAGES}/horizon-cli-*.pkg" "MacOS installation"
      		check_exist f "${PACKAGES}/${MAC_PACKAGE_CERT}" "The CLI package certificate"
	fi
    fi

    log_debug "check_requirements() end"
}

function check_node_state() {
	log_debug "check_node_state() begin"

	if command -v hzn >/dev/null 2>&1; then
		local NODE_STATE=$(hzn node list | jq -r .configstate.state)
		log_info "Current node state is: ${NODE_STATE}"

		if [ $BATCH_INSTALL -eq 0 ] && [[ "$NODE_STATE" = "configured" ]] && [[ ! $OVERWRITE = "true" ]]; then
			# node is configured need to ask what to do
			log_notify "Your node is registered"
			OVERWRITE_NODE=true
			log_notify "The configuration will be overwritten..."
		elif [[ "$NODE_STATE" = "unconfigured" ]]; then
			# node is unconfigured
			log_info "The node is in unconfigured state, continuing..."
		fi
	else
		log_info "The hzn doesn't seem to be installed, continuing..."
	fi

	log_debug "check_node_state() end"
}

# removes agent-install files and deb packages before bulk install
function device_cleanup_agent_files() {
    log_debug "device_cleanup_agent_files() begin"

    files_to_remove=( 'agent-install.cfg' *'horizon'* )

    set +e
    for file in "${files_to_remove[@]}"; do
        rm -f $file
    done
    set -e

    log_debug "device_cleanup_agent_files() end"
}

function unzip_install_files() {
	if [ -f $AGENT_INSTALL_ZIP ]; then
		tar -zxf $AGENT_INSTALL_ZIP
	else
		log_error "Agent install tar file $AGENT_INSTALL_ZIP does not exist."
	fi
}

function find_node_id() {
	log_debug "start find_node_id"
	if [ -f $NODE_ID_MAPPING_FILE ]; then
		BATCH_INSTALL=1
		log_debug "found id mapping file $NODE_ID_MAPPING_FILE"
		ID_LINE=$(grep $(hostname) "$NODE_ID_MAPPING_FILE" || [[ $? == 1 ]] )
		if [ -z $ID_LINE ]; then
			log_debug "Did not find node id with hostname. Trying with ip"
			find_node_ip_address
			for IP in $(echo $NODE_IP); do
				ID_LINE=$(grep "$IP" "$NODE_ID_MAPPING_FILE" || [[ $? == 1 ]] )
				if [[ ! "$ID_LINE" = "" ]]; then break; fi
			done
			if [[ ! "$ID_LINE" = "" ]]; then
				NODE_ID=$(echo $ID_LINE | cut -d "," -f 2)
			else
				log_notify "Failed to find node id in mapping file $NODE_ID_MAPPING_FILE with $(hostname) or $NODE_IP"
				exit 1
			fi
		else
			NODE_ID=$(echo $ID_LINE | cut -d "," -f 2)
		fi
	fi
	log_debug "finished find_node_id"
}

function find_node_ip_address() {
	if [[ "$OS" == "macos" ]]; then
        NODE_IP=$(ipconfig getifaddr en0)
    else
        NODE_IP=$(hostname -I)
    fi
}

# Cluster only: check if node exist in management hub
function check_node_exist() {
    log_debug "check_node_exist() begin"

    if [[ $HZN_EXCHANGE_USER_AUTH == *"iamapikey"* ]]; then
        EXCH_CREDS=$HZN_ORG_ID/$HZN_EXCHANGE_USER_AUTH
    else
        EXCH_CREDS=$HZN_EXCHANGE_USER_AUTH
    fi

    if [[ $CERTIFICATE != "" ]]; then
        EXCH_OUTPUT=$(curl -fs --cacert $CERTIFICATE $HZN_EXCHANGE_URL/orgs/$HZN_ORG_ID/nodes/$NODE_ID -u $EXCH_CREDS) || true
        else
                EXCH_OUTPUT=$(curl -fs $HZN_EXCHANGE_URL/orgs/$HZN_ORG_ID/nodes/$NODE_ID -u $EXCH_CREDS) || true
        fi

        if [[ "$EXCH_OUTPUT" != "" ]]; then
                EXCH_NODE_TYPE=$(echo $EXCH_OUTPUT | jq -e '.nodes | .[].nodeType' | sed 's/"//g')
                if [[ "$EXCH_NODE_TYPE" = "device" ]]; then
                	log_notify "Node id ${NODE_ID} is already been created as nodeType Device"
            		exit 1
        	else
            		log_notify "Node id ${NODE_ID} is already been created as nodeType Clutser"
			exit 1
                fi
        fi

    log_debug "check_node_exist() end"
}

# Cluster only: to extract agent image tar.gz and load to docker
function getImageInfo() {
    log_debug "getImageInfo() begin"

    tar xvzf amd64_anax_k8s_ubi.tar.gz
    if [ $? -ne 0 ]; then
        log_notify "failed to uncompress agent image from amd64_anax_k8s_ubi.tar.gz, exiting..."
        exit 1
    fi

    LOADED_IMAGE_MESSAGE=$(docker load --input amd64_anax_k8s_ubi.tar)
    AGENT_IMAGE=$(echo $LOADED_IMAGE_MESSAGE|awk -F': ' '{print $2}')

    if [ -z $AGENT_IMAGE ]; then
	    log_notify "Agent image is empty, exiting..."
	    exit 1
    fi
    log_info "Got agent image: $AGENT_IMAGE"

    log_debug "getImageInfo() end"
}

# Cluster only: to push agent image to image registry that edge cluster can access
function pushImageToEdgeClusterRegistry() {
    log_debug "pushImageToEdgeClusterRegistry() begin"

    # split $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY by "/"
    EDGE_CLUSTER_REGISTRY_HOST=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY|awk -F'/' '{print $1}')
    log_notify "Edge cluster registy host: $EDGE_CLUSTER_REGISTRY_HOST"
    
    if [ -z $EDGE_CLUSTER_REGISTRY_USERNAME ] && [ -z $EDGE_CLUSTER_REGISTRY_TOKEN ]; then
		:  # even for a registry in the insecure-registries list, if we don't specify user/pw it will prompt for it
	    #docker login $EDGE_CLUSTER_REGISTRY_HOST
    else
	    echo "$EDGE_CLUSTER_REGISTRY_TOKEN" | docker login -u $EDGE_CLUSTER_REGISTRY_USERNAME --password-stdin $EDGE_CLUSTER_REGISTRY_HOST
    fi
    
    if [ $? -ne 0 ]; then
	    log_notify "Failed to login to edge cluster's registry: $EDGE_CLUSTER_REGISTRY_HOST, exiting..."
            exit 1
    fi

    docker tag ${AGENT_IMAGE} ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
    docker push ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
    if [ $? -ne 0 ]; then
        log_notify "Failed to push image ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY} to edge cluster's registry, exiting..."
        exit 1
    fi
    log_info "successfully pushed image $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY to edge cluster registry"

    log_debug "pushImageToEdgeClusterRegistry() end"
}

# Cluster only: to generate 3 files: /tmp/agent-install-horizon-env, deployment.yml and persistentClaim.yml
function generate_installation_files() {
    log_debug "generate_installation_files() begin"

    log_info "Preparing horizon environment file."
    generate_horizon_env
    log_info "Horizon environment file is done."

    log_info "Preparing kubernete persistentVolumeClaim file"
    prepare_k8s_pvc_file
    log_info "kubernete persistentVolumeClaim file are done."

    log_info "Preparing kubernete development files"
    prepare_k8s_development_file
    log_info "kubernete development files are done."

    log_debug "generate_installation_files() end"
}

# Cluster only: to generate /tmp/agent-install-hzn-env file
function generate_horizon_env() {
    log_debug "generate_horizon_env() begin"
    if [ -e $HZN_ENV_FILE ]; then
        log_info "$HZN_ENV_FILE already exists. This script will overwrite it"
	rm $HZN_ENV_FILE
    fi
    
    create_horizon_env

    log_debug "generate_horizon_env() end"
}

# Cluster only: to generate /tmp/agent-install-hzn-env file
function create_horizon_env() {
    log_debug "create_horizon_env() begin"
    cert_name=$(basename ${CERTIFICATE})
    echo "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}" >> $HZN_ENV_FILE
    echo "HZN_FSS_CSSURL=${HZN_FSS_CSSURL}" >> $HZN_ENV_FILE
    echo "HZN_DEVICE_ID=${NODE_ID}" >> $HZN_ENV_FILE
    echo "HZN_MGMT_HUB_CERT_PATH=/etc/default/cert/$cert_name" >> $HZN_ENV_FILE
    echo "HZN_AGENT_PORT=8510" >> $HZN_ENV_FILE
    log_debug "create_horizon_env() end"
}

# Cluster only: to delete /tmp/agent-install-hzn-env file
function cleanup_cluster_config_files() {
    log_debug "cleanup_cluster_config_files() begin"
    rm $HZN_ENV_FILE
    if [ $? -ne 0 ]; then
	    log_notify "Failed to remove $HZN_ENV_FILE, please remove it mannually"
    fi
    log_debug "cleanup_cluster_config_files() end"
}

# Cluster only: to create deployment.yml based on template
function prepare_k8s_development_file() {
    log_debug "prepare_k8s_development_file() begin"

    sed -e "s#__AgentNameSpace__#${AGENT_NAMESPACE}#g" -e "s#__OrgId__#\"${HZN_ORG_ID}\"#g" deployment-template.yml > deployment.yml

    if [ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]; then
			EDGE_CLUSTER_REGISTRY_PROJECT_NAME=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY|awk -F'/' '{print $2}')
			EDGE_CLUSTER_AGENT_IMAGE_AND_TAG=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY|awk -F'/' '{print $3}')

			if [[ "$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY" == "" ]]; then
				# check if using local cluster or remote ocp
				if $KUBECTL cluster-info | grep -q -E 'Kubernetes master.*//(127|172|10|192.168)\.'; then
					# using small kube
					sed -i -e "s#__ImagePath__#${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}#g" deployment.yml
				else
					# using ocp
					IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY_INTERNAL_URL=$DEFAULT_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
					sed -i -e "s#__ImagePath__#${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY_INTERNAL_URL}#g" deployment.yml
				fi
			else
				IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY_INTERNAL_URL=$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
				sed -i -e "s#__ImagePath__#${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY_INTERNAL_URL}#g" deployment.yml
			fi		
    else
		log_notify "Agent install on edge cluster requires to use an edge cluster registry"
		exit 1
        #sed -i -e "s#__ImagePath__#${AGENT_IMAGE}#g" deployment.yml
    fi

    log_debug "prepare_k8s_development_file() end"
}

# Cluster only: to create persistenClaim.yml based on template
function prepare_k8s_pvc_file() {
	log_debug "prepare_k8s_pvc_file() begin"

	sed -e "s#__AgentNameSpace__#${AGENT_NAMESPACE}#g" -e "s/__StorageClass__/\"${EDGE_CLUSTER_STORAGE_CLASS}\"/g" persistentClaim-template.yml > persistentClaim.yml

	log_debug "prepare_k8s_pvc_file() end"
}

# Cluster only: to create cluster resources
function create_cluster_resources() {
	log_debug "create_cluster_resources() begin"

	create_namespace
	sleep 2
	create_service_account
	create_secret
	create_configmap
	create_persistent_volume

	log_debug "create_cluster_resources() end"
}

# Cluster only: to create namespace that agent will be deployed
function create_namespace() {
    log_debug "create_namespace() begin"
    # check if namespace exist, if not, create
    log_info "checking if namespace exist..."

    $KUBECTL get namespace ${AGENT_NAMESPACE} 2>/dev/null
    local ret=$?
    if [ $ret -ne 0 ]; then
        log_info "namespace ${AGENT_NAMESPACE} does not exist, creating..."
        log_debug "command: $KUBECTL create namespace ${AGENT_NAMESPACE}"
        $KUBECTL create namespace ${AGENT_NAMESPACE}
        if [ $? -ne 0 ]; then
            log_notify "Failed to create namespace ${AGENT_NAMESPACE}, exiting..."
            exit 1
        fi
	log_notify "namespace ${AGENT_NAMESPACE} created"
    else
        log_notify "namespace ${AGENT_NAMESPACE} exists, skip creating namespace"
    fi

    log_debug "create_namespace() end"
}

# Cluster only: to create service account for agent namespace and binding to cluster-admin clusterrole
function create_service_account() {
	log_debug "create_service_account() begin"

	log_info "checking if serviceaccont exist..."
	$KUBECTL get serviceaccount ${SERVICE_ACCOUNT_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null
	
	if [ $? -ne 0 ]; then
		log_info "serviceaccount ${SERVICE_ACCOUNT_NAME} does not exist, creating..."
		$KUBECTL create serviceaccount ${SERVICE_ACCOUNT_NAME} -n ${AGENT_NAMESPACE}
		if [ $? -ne 0 ]; then
        		log_notify "Failed to create service account ${SERVICE_ACCOUNT_NAME}, exiting..."
        		exit 1
    		fi
		log_notify "serviceaccount ${SERVICE_ACCOUNT_NAME} created"

	else
		log_notify "serviceaccount ${SERVICE_ACCOUNT_NAME} exists, skip creating serviceaccount"
	fi

	log_info "checking if clusterrolebinding exist..."
	$KUBECTL get clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} 2>/dev/null
	if [ $? -ne 0 ]; then
		log_info "Binding ${SERVICE_ACCOUNT_NAME} to cluster admin..."
		$KUBECTL create clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} --serviceaccount=${AGENT_NAMESPACE}:${SERVICE_ACCOUNT_NAME} --clusterrole=cluster-admin
		if [ $? -ne 0 ]; then
        		log_notify "Failed to create clusterrolebinding for ${AGENT_NAMESPACE}:${SERVICE_ACCOUNT_NAME}, exiting..."
        		exit 1
    		fi
		log_notify "clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} created"
	else
		log_notify "clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} exists, skip creating clusterrolebinding"
	fi
	log_debug "create_service_account() end"
}

# Cluster only: to create secret from cert file for agent deployment
function create_secret() {
    log_debug "create_secrets() begin"

    log_info "checking if secret ${SECRET_NAME} exist..."
    $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null
    if [ $? -ne 0 ]; then
    	log_info "creating secret for cert file..."
    	$KUBECTL create secret generic ${SECRET_NAME} --from-file=${CERTIFICATE} -n ${AGENT_NAMESPACE}
    	if [ $? -ne 0 ]; then
        	log_notify "Failed to create secret ${SECRET_NAME} from cert file ${CERTIFICATE}, exiting..."
        	exit 1
    	fi
    	log_notify "secret ${SECRET_NAME} created"
    else
        log_notify "secret ${SECRET_NAME} exists, skip creating secret"
    fi

    log_debug "create_secrets() end"
}

# Cluster only: to create configmap based on /tmp/agent-install-horizon-env for agent deployment
function create_configmap() {
    log_debug "create_configmap() begin"

    log_info "checking if configmap ${CONFIGMAP_NAME} exist..."
    $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null
    if [ $? -ne 0 ]; then
    	log_info "create configmap from ${HZN_ENV_FILE}..."
    	$KUBECTL create configmap ${CONFIGMAP_NAME} --from-file=horizon=${HZN_ENV_FILE} -n ${AGENT_NAMESPACE}
    	if [ $? -ne 0 ]; then
        	log_notify "Failed to create configmap ${CONFIGMAP_NAME} from ${HZN_ENV_FILE}, exiting..."
        	exit 1
    	fi
    	log_notify "configmap ${CONFIGMAP_NAME} created."
    else
        log_notify "configmap ${CONFIGMAP_NAME} exists, skip creating configmap"
    fi

    log_debug "create_configmap() end"
}

# Cluster only: to create persistent volume claim for agent deployment
function create_persistent_volume() {
    log_debug "create_persistent_volume() begin"

    log_info "checking if persistent volume claim ${PVC_NAME} exist..."
    $KUBECTL get pvc ${PVC_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null
    if [ $? -ne 0 ]; then
    	log_info "creating persistent volume claim..."
    	$KUBECTL apply -f persistentClaim.yml -n ${AGENT_NAMESPACE}
    	if [ $? -ne 0 ]; then
        	log_notify "Failed to create persistent volume claim, exiting..."
        	exit 1
    	fi
    	log_notify "persistent volume claim created"
    else
        log_notify "persistent volume claim ${PVC_NAME} exists, skip creating persistent volume claim"
    fi

    log_debug "create_persistent_volume() end"
}

# Cluster only: to check secret, configmap, pvc is created
function check_resources_for_deployment() {
    log_debug "check_resource_for_deployment() begin"
    # check secrets/configmap/persistent
    $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} > /dev/null
    secret_ready=$?

    $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} > /dev/null
    configmap_ready=$?

    $KUBECTL get pvc ${PVC_NAME} -n ${AGENT_NAMESPACE} > /dev/null
    pvc_ready=$?

    if [[ ${secret_ready} -eq 0 ]] && [[ ${configmap_ready} -eq 0 ]] && [[ ${pvc_ready} -eq 0 ]]; then
        RESOURCE_READY=1
    else
        RESOURCE_READY=0
    fi

    log_debug "check_resource_for_deployment() end"
}

# Cluster only: to create deployment
function create_deployment() {
    log_debug "create_deployment() begin"

    log_info "creating deployment..."
    $KUBECTL apply -f deployment.yml -n ${AGENT_NAMESPACE}
    if [ $? -ne 0 ]; then
        log_notify "Failed to create deployment, exiting..."
        exit 1
    fi

    log_debug "create_deployment() end"
}

# Cluster only: to check agent deplyment status
function check_deployment_status() {
    log_debug "check_resource_for_deployment() begin"
    local timeout=75s
    log_notify "Waiting up to $timeout for the agent deployment to complete..."

    DEP_STATUS=$($KUBECTL rollout status --timeout=${timeout} deployment/agent -n ${AGENT_NAMESPACE} | grep "successfully rolled out" )
    if [ -z "$DEP_STATUS" ]; then
        log_notify "Deployment rollout status failed"
        exit 1
    fi
    log_debug "check_resource_for_deployment() end"
}

# Cluster only: to get agent pod id
function get_pod_id() {
    log_debug "get_pod_id() begin"

    local i=0
    while [[ $i -le $WAIT_POD_MAX_TRY ]] && [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do
        log_notify "waiting for pod: $i"
	((i++))

        sleep 1
    done

    if [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
         log_notify "Failed to get agent pod in Ready status"
	 exit 1
    fi

    POD_ID=$($KUBECTL get pod -l app=agent --field-selector status.phase=Running -n ${AGENT_NAMESPACE} 2> /dev/null | grep "agent-" | cut -d " " -f1 2> /dev/null)
    if [ -n "${POD_ID}" ]; then
        log_info "get pod: ${POD_ID}"
    else
        log_notify "Failed to get pod id"
        exit 1
    fi
    log_debug "get_pod_id() end"
}

# Cluster only: to install agent in cluster
function install_cluster() {
	check_node_exist

	getImageInfo

	# push agent image to cluster's registry
	if [ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]; then
		pushImageToEdgeClusterRegistry
	fi

	# generate files based on templates
	generate_installation_files

	# create cluster namespace and resources
	create_cluster_resources

	while [ -z ${RESOURCE_READY} ] && [ ${GET_RESOURCE_MAX_TRY} -gt 0 ]
	do
		check_resources_for_deployment
		count=$((GET_RESOURCE_MAX_TRY-1))
		GET_RESOURCE_MAX_TRY=$count
	done

	# get pod information
	create_deployment
	check_deployment_status
	get_pod_id

	set -e
	create_node

	# register
	registration "$SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"
	set +e

	cleanup_cluster_config_files

}

# Accept the parameters from command line
while getopts "c:i:j:p:k:u:d:z:hvl:n:sfbw:o:t:D:a:U:" opt; do
	case $opt in
		c) CERTIFICATE="$OPTARG"
		;;
		i) PKG_PATH="$OPTARG" PKG_TREE_IGNORE=true
		;;
		j) PKG_APT_KEY="$OPTARG"
		;;
		p) HZN_EXCHANGE_PATTERN="$OPTARG"
		;;
		k) CFG="$OPTARG"
		;;
		u) HZN_EXCHANGE_USER_AUTH="$OPTARG"
		;;
		a) HZN_EXCHANGE_NODE_AUTH="$OPTARG"
		;;
		d) NODE_ID="$OPTARG"
		;;
		z) AGENT_INSTALL_ZIP="$OPTARG"
		;;
		h) help; exit 0
		;;
		v) version
		;;
		l) validate_number_int "$OPTARG"; VERBOSITY="$OPTARG"
		;;
		n) HZN_NODE_POLICY="$OPTARG"
		;;
		s) SKIP_REGISTRATION=true
		;;
		f) OVERWRITE=true
		;;
		b) SKIP_PROMPT=true
		;;
		w) WAIT_FOR_SERVICE="$OPTARG"
		;;
		o) WAIT_FOR_SERVICE_ORG="$OPTARG"
		;;
		t) APT_REPO_BRANCH="$OPTARG"
		;;
		D) DEPLOY_TYPE="$OPTARG"
		;;
		U) INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY="$OPTARG"
		;;
		\?) echo "Invalid option: -$OPTARG"; help; exit 1
		;;
		:) echo "Option -$OPTARG requires an argument"; help; exit 1
		;;
	esac
done

# Temporary patch to accept -d id:token
if [[ $NODE_ID =~ : && -n ${NODE_ID#*:} ]]; then   # tests if NODE_ID contains a colon and there is text after it
 	HZN_EXCHANGE_NODE_AUTH="$NODE_ID"
 	NODE_ID=${NODE_ID%%:*}   # strip the text after the colon
fi

if [ -f "$AGENT_INSTALL_ZIP" ]; then
    device_cleanup_agent_files
	unzip_install_files
	find_node_id
	NODE_ID=$(echo "$NODE_ID" | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*$//' )
	if [[ $NODE_ID != "" ]]; then
		log_info "Found node id $NODE_ID"
	fi
fi

# checking the supplied arguments
validate_args "$*" "$#"
# showing current configuration
show_config

log_notify "Node type is: ${DEPLOY_TYPE}"
if [ "${DEPLOY_TYPE}" == "device" ]; then
	# checking if the requirements are met
	check_requirements

	check_node_state

	if [[ "$OS" == "linux" ]]; then
		log_notify "Detection results: OS is ${OS}, distribution is ${DISTRO}, release is ${CODENAME}, architecture is ${ARCH}"
		install_${OS} ${OS} ${DISTRO} ${CODENAME} ${ARCH}
	elif [[ "$OS" == "macos" ]]; then
		log_notify "Detection results: OS is ${OS}"
		install_${OS}
	fi

	add_autocomplete

elif [ "${DEPLOY_TYPE}" == "cluster" ]; then
	log_info "Install agent on edge cluster"
	set +e
	install_cluster
else
	log_error "node type only support 'device' or 'cluster'"
fi
