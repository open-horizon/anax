#!/bin/bash

# Installs the Horizon agent on an edge node (device or edge cluster)

set -e   #future: remove?

#====================== Input Gathering ======================

# Global constants

# logging levels
#VERB_FATAL=0   # we always show fata error msgs
VERB_ERROR=1
VERB_WARNING=2
VERB_INFO=3
VERB_VERBOSE=4
VERB_DEBUG=5

# You can add to these lists of supported values by setting/exporting the corresponding <varname>_APPEND variable to a string of space-separated words.
# This allows you to experiment with platforms or variations that are not yet officially tested/supported.
SUPPORTED_DEBIAN_VARIANTS=(ubuntu raspbian debian $SUPPORTED_DEBIAN_VARIANTS_APPEND)   # compared to what our detect_distro() sets DISTRO to
SUPPORTED_DEBIAN_VERSION=(noble bullseye jammy focal bionic buster xenial stretch bookworm $SUPPORTED_DEBIAN_VERSION_APPEND)   # compared to what our detect_distro() sets CODENAME to
SUPPORTED_DEBIAN_ARCH=(amd64 arm64 armhf s390x $SUPPORTED_DEBIAN_ARCH_APPEND)   # compared to dpkg --print-architecture
SUPPORTED_REDHAT_VARIANTS=(rhel redhatenterprise centos fedora $SUPPORTED_REDHAT_VARIANTS_APPEND)   # compared to what our detect_distro() sets DISTRO to
# Note: version 8 and 9 are added because that is what /etc/os-release returns for DISTRO_VERSION_NUM on centos
SUPPORTED_REDHAT_VERSION=(7.6 7.9 8.1 8.2 8.3 8.4 8.5 8.6 8.7 8.8 9.0 9.1 9.2 9.3 9.4 8 9 32 35 36 37 38 39 40 $SUPPORTED_REDHAT_VERSION_APPEND)   # compared to what our detect_distro() sets DISTRO_VERSION_NUM to. For fedora versions see https://fedoraproject.org/wiki/Releases,
SUPPORTED_REDHAT_ARCH=(x86_64 aarch64 ppc64le s390x  riscv64 $SUPPORTED_REDHAT_ARCH_APPEND)     # compared to uname -m

SUPPORTED_EDGE_CLUSTER_ARCH=(amd64 arm64 s390x)
SUPPORTED_ANAX_IN_CONTAINER_ARCH=(amd64 arm64 s390x)

SUPPORTED_OS=(macos linux)   # compared to what our get_os() returns
SUPPORTED_LINUX_DISTRO=(${SUPPORTED_DEBIAN_VARIANTS[@]} ${SUPPORTED_REDHAT_VARIANTS[@]})   # compared to what our detect_distro() sets DISTRO to

HOSTNAME=$(hostname -s)
MAC_PACKAGE_CERT="horizon-cli.crt"
PERMANENT_CERT_PATH='/etc/horizon/agent-install.crt'
ANAX_DEFAULT_PORT=8510
AGENT_CERT_FILE_DEFAULT='agent-install.crt'
AGENT_CFG_FILE_DEFAULT='agent-install.cfg'
CSS_OBJ_PATH_DEFAULT='/api/v1/objects/IBM/agent_files'
CSS_OBJ_AGENT_SOFTWARE_BASE='/api/v1/objects/IBM/agent_software_files'
CSS_OBJ_AGENT_CERT_BASE='/api/v1/objects/IBM/agent_cert_files'
CSS_OBJ_AGENT_CONFIG_BASE='/api/v1/objects/IBM/agent_config_files'
MAX_HTTP_RETRY=5
SLEEP_SECONDS_BETWEEN_RETRY=15
CURL_RETRY_PARMS="--retry 5 --retry-connrefused --retry-max-time 120"

SEMVER_REGEX='^[0-9]+\.[0-9]+(\.[0-9]+)+'   # matches a version like 1.2.3 (must be at least 3 fields). Also allows a bld num on the end like: 1.2.3-RC1

# The following variable will need to have the $ARCH prepended to it before it can be used
DEFAULT_AGENT_IMAGE_TAR_FILE='_anax.tar.gz'

INSTALLED_AGENT_CFG_FILE="/etc/default/horizon"
AGENT_CONTAINER_PORT_BASE=8080

# edge cluster agent deployment
DEFAULT_AGENT_NAMESPACE="openhorizon-agent"
SERVICE_ACCOUNT_NAME="agent-service-account"
CLUSTER_ROLE_BINDING_NAME="openhorizon-agent-cluster-rule"
ROLE_BINDING_NAME="role-binding"
DEPLOYMENT_NAME="agent"
SECRET_NAME="openhorizon-agent-secrets"
IMAGE_PULL_SECRET_NAME="registry-creds"
CRONJOB_AUTO_UPGRADE_NAME="auto-upgrade-cronjob"
IMAGE_REGISTRY_SECRET_NAME="openhorizon-agent-secrets-docker-cert"
CONFIGMAP_NAME="openhorizon-agent-config"
PVC_NAME="openhorizon-agent-pvc"
DEFAULT_PVC_SIZE="10Gi"
GET_RESOURCE_MAX_TRY=5
POD_ID=""
HZN_ENV_FILE="/tmp/agent-install-horizon-env"
DEFAULT_OCP_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY="image-registry.openshift-image-registry.svc:5000"
EDGE_CLUSTER_TAR_FILE_NAME='horizon-agent-edge-cluster-files.tar.gz'
# The following variables will need to have the $ARCH prepended before they can be used
DEFAULT_AGENT_K8S_IMAGE_TAR_FILE='_anax_k8s.tar.gz'
DEFAULT_CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE='_auto-upgrade-cronjob_k8s.tar.gz'
DEFAULT_INIT_CONTAINER_IMAGE_PATH="public.ecr.aws/docker/library/alpine:latest"

# agent upgrade types. To update the certificate only, just do "-G cert" or set AGENT_UPGRADE_TYPES="cert"
UPGRADE_TYPE_SW="software"
UPGRADE_TYPE_CERT="cert"
UPGRADE_TYPE_CFG="config"

# Type of container engine to use; RedHat might have podman
DOCKER_ENGINE="docker"

#0 - not initialized, 1 - supported, 2 - not supported
AGENT_FILE_VERSIONS_STATUS=0

# Script usage info
function usage() {
    local exit_code=$1
    cat <<EndOfMessage
${0##*/} <options>

Install the Horizon agent on an edge device or edge cluster.

Required Input Variables (via flag, environment, or config file):
    HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_ORG_ID, either HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH

Options/Flags:
    -c                  Path to a certificate file. Default: ./$AGENT_CERT_FILE_DEFAULT, if present. If the argument begins with 'css:' (e.g. css:, css:<version>, css:<path>), it will download the certificate file from the MMS. If only 'css:' is specified, the path for the highest certificate file version $CSS_OBJ_AGENT_CERT_BASE-<version> will be added if it can be found. Otherwise the default path $CSS_OBJ_PATH_DEFAULT will be added. (This flag is equivalent to AGENT_CERT_FILE or HZN_MGMT_HUB_CERT_PATH)
    -k                  Path to a configuration file. Default: ./$AGENT_CFG_FILE_DEFAULT, if present. If the argument begins with 'css:' (e.g. css:, css:<version>, css:<path>), it will download the config file from the MMS. If only 'css:' is specified, the path for the highest config file version $CSS_OBJ_AGENT_CONFIG_BASE-<version> will be added if it can be found. Otherwise the default path $CSS_OBJ_PATH_DEFAULT will be added. All other variables for this script can be specified in the config file, except for INPUT_FILE_PATH (and HZN_ORG_ID if -i css: is specified). (This flag is equivalent to AGENT_CFG_FILE)
    -i                  Installation packages/files/image location (default: current directory). If the argument is the URL of an anax git repo release (e.g. https://github.com/open-horizon/anax/releases/download/v1.2.3) it will download the appropriate packages/files from there. If it is anax: or https://github.com/open-horizon/anax/releases , it will default to the latest release. Otherwise, if the argument begins with 'http' or 'https', it will be used as an APT repository (for debian hosts). If the argument begins with 'css:' (e.g. css:, css:<version>, css:<path>), it will download the appropriate files/packages from the MMS. If only 'css:' is specified, the path for the highest package file version $CSS_OBJ_AGENT_SOFTWARE_BASE-<version> will be added if it can be found. Otherwise the default path $CSS_OBJ_PATH_DEFAULT will be added. If the argument is 'remote:', the agent deployment will reference the image in remote image registry. 'remote:' only applies for cluster agent installation. If using 'remote:', values for 'IMAGE_ON_EDGE_CLUSTER_REGISTRY' is required for providing the image container registry info. Values for 'EDGE_CLUSTER_REGISTRY_USERNAME', 'EDGE_CLUSTER_REGISTRY_USERNAME' are required if remote image registry is protected. (This flag is equivalent to INPUT_FILE_PATH)
    -z                  The name of your agent installation tar file. Default: ./agent-install-files.tar.gz (This flag is equivalent to AGENT_INSTALL_ZIP)
    -j                  File location for the public key for an APT repository specified with '-i' (This flag is equivalent to PKG_APT_KEY)
    -t                  Branch to use in the APT repo specified with -i. Default is 'updates' (This flag is equivalent to APT_REPO_BRANCH)
    -O                  The exchange organization id (This flag is equivalent to HZN_ORG_ID)
    -u                  Exchange user authorization credentials (This flag is equivalent to HZN_EXCHANGE_USER_AUTH)
    -a                  Exchange node authorization credentials (This flag is equivalent to HZN_EXCHANGE_NODE_AUTH)
    -d                  The id to register this node with (This flag is equivalent to HZN_NODE_ID, NODE_ID or HZN_DEVICE_ID. NODE_ID is deprecated)
    -p                  Pattern name to register this edge node with. Default: registers node with policy. (This flag is equivalent to HZN_EXCHANGE_PATTERN)
    -n                  Path to a node policy file (This flag is equivalent to HZN_NODE_POLICY)
    -w                  Wait for this edge service to start executing on this node before this script exits. If using a pattern, this value can be '*'. (This flag is equivalent to AGENT_WAIT_FOR_SERVICE)
    -T                  Timeout value (in seconds) for how long to wait for the service to start (This flag is equivalent to AGENT_REGISTRATION_TIMEOUT)
    -o                  Specify an org id for the service specified with '-w'. Defaults to the value of HZN_ORG_ID. (This flag is equivalent to AGENT_WAIT_FOR_SERVICE_ORG)
    -s                  Skip registration, only install the agent (This flag is equivalent to AGENT_SKIP_REGISTRATION)
    -D                  Node type of agent being installed: device, cluster. Default: device. (This flag is equivalent to AGENT_DEPLOY_TYPE)
    -U                  Internal url for edge cluster registry. If not specified, this script will auto-detect the value if it is a small, single-node cluster (e.g. k3s or microk8s). For OCP use: image-registry.openshift-image-registry.svc:5000. (This flag is equivalent to INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY)
    -l                  Logging verbosity level. Display messages at this level and lower: 1: error, 2: warning, 3: info (default), 4: verbose, 5: debug. Default is 3, info. (This flag is equivalent to AGENT_VERBOSITY)
    -f                  Install older version of the horizon agent and CLI packages. (This flag is equivalent to AGENT_OVERWRITE)
    -b                  Skip any prompts for user input (This flag is equivalent to AGENT_SKIP_PROMPT)
    -C                  Install only the horizon-cli package, not the full agent (This flag is equivalent to AGENT_ONLY_CLI)
    -G                  A comma separated list of upgrade types. Supported types are 'software', 'cert' and 'config'. The default is 'software,cert,config'. It is used togetther with --auto-upgrade flag to perform partial agent upgrade. (This flag is equivalent to AGENT_UPGRADE_TYPES)
        --ha-group      Specify the HA group this node will be added to during the node registration, if -s is not specified. (This flag is equivalent to HZN_HA_GROUP)
        --auto-upgrade  Auto agent upgrade. It is used internally by the agent auto upgrade process. (This flag is equivalent to AGENT_AUTO_UPGRADE)
        --container     Install the agent in a container. This is the default behavior for MacOS installations. (This flag is equivalent to AGENT_IN_CONTAINER)
        --namespace     The namespace that the cluster agent will be installed to. The default is 'openhorizon-agent'
        --namespace-scoped The cluster agent will only have namespace scope. The default is 'false'
    -N                  The container number to be upgraded. The default is 1 which means the container name is horizon1. It is used for upgrade only, the HORIZON_URL setting in /etc/horizon/hzn.json will not be changed. (This flag is equivalent to AGENT_CONTAINER_NUMBER)
    -h  --help          This usage

Additional Variables (in environment or config file):
    HZN_AGBOT_URL: The URL that is used for the 'hzn agbot ...' commands.
    HZN_SDO_SVC_URL: The URL that is used for the 'hzn voucher ...' and 'hzn sdo ...' commands. Or,
    HZN_FDO_SVC_URL: The URL that is used for the 'hzn fdo ...' commands.

Additional Edge Device Variables (in environment or config file):
    NODE_ID_MAPPING_FILE: File to map hostname or IP to node id, for bulk install.  Default: node-id-mapping.csv
    AGENT_IMAGE_TAR_FILE: the file name of the device agent docker image in tar.gz format. Default: \${ARCH}$DEFAULT_AGENT_IMAGE_TAR_FILE
    AGENT_WAIT_MAX_SECONDS: Maximum seconds to wait for the Horizon agent to start or stop. Default: 30

Optional Edge Device Environment Variables For Testing New Distros - Not For Production Use
    SUPPORTED_DEBIAN_VARIANTS_APPEND: a debian variant that should be added to the default list: ${SUPPORTED_DEBIAN_VARIANTS[*]}
    SUPPORTED_DEBIAN_VERSION_APPEND: a debian version that should be added to the default list: ${SUPPORTED_DEBIAN_VERSION[*]}
    SUPPORTED_DEBIAN_ARCH_APPEND: a debian architecture that should be added to the default list: ${SUPPORTED_DEBIAN_ARCH[*]}
    SUPPORTED_REDHAT_VARIANTS_APPEND: a Red Hat variant that should be added to the default list: ${SUPPORTED_REDHAT_VARIANTS[*]}
    SUPPORTED_REDHAT_VERSION_APPEND: a Red Hat version that should be added to the default list: ${SUPPORTED_REDHAT_VERSION[*]}
    SUPPORTED_REDHAT_ARCH_APPEND: a Red Hat architecture that should be added to the default list: ${SUPPORTED_REDHAT_ARCH[*]}


Additional Edge Cluster Variables (in environment or config file):
    KUBECTL: specify this value if you have multiple kubectl CLI installed in your enviroment. Otherwise the script will detect in this order: k3s kubectl, microk8s.kubectl, oc, kubectl.
    ENABLE_AUTO_UPGRADE_CRONJOB: specify this value to false to skip installing agent auto upgrade cronjob. Default: true
    IMAGE_ON_EDGE_CLUSTER_REGISTRY: override the agent image path (without tag) if you want it to be different from what this script will default it to, in format: <registry-host>/<repository>/\${ARCH}_anax_k8s
    CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY: override the auto-upgrade-cronjob cronjob image path (without tag) if you want it to be different from what this script will default it to
    INIT_CONTAINER_IMAGE: specify this value if init container is needed and is different from default: public.ecr.aws/docker/library/alpine:latest
    EDGE_CLUSTER_REGISTRY_USERNAME: specify this value if the edge cluster registry requires authentication
    EDGE_CLUSTER_REGISTRY_TOKEN: specify this value if the edge cluster registry requires authentication
    EDGE_CLUSTER_STORAGE_CLASS: the storage class to use for the agent and edge services. Default: gp2
    EDGE_CLUSTER_PVC_SIZE: the requested size in the agent persistent volume to use for the agent. Default: 10Gi
    AGENT_NAMESPACE: The namespace the agent should run in. Default: openhorizon-agent
    AGENT_WAIT_MAX_SECONDS: Maximum seconds to wait for the Horizon agent to start or stop. Default: 30
    AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS: Maximum seconds to wait for the agent deployment rollout status to be successful. Default: 300
    AGENT_K8S_IMAGE_TAR_FILE: the file name of the edge cluster agent docker image in tar.gz format. Default: \${ARCH}$DEFAULT_AGENT_K8S_IMAGE_TAR_FILE
    CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE: the file name of the edge cluster auto-upgrade-cronjob cronjob docker image in tar.gz format. Default: \${ARCH}$DEFAULT_CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE
    AGENT_NAMESPACE: The cluster namespace that the agent will be installed in
    NAMESPACE_SCOPED: specify this value if the edge cluster agent is namespace-scoped agent
EndOfMessage
    exit $exit_code
}

function now() {
    echo $(date '+%Y-%m-%d %H:%M:%S')
}

# Get cmd line args. Each arg corresponds to an env var. The arg value will be stored in ARG_<envvar>. Then the get_variable function will choose the highest precedence variable set.
# Note: could not put this in a function, because getopts would only process the function args
AGENT_VERBOSITY=3   # default until we get it from all of the possible places
if [[ $AGENT_VERBOSITY -ge $VERB_DEBUG ]]; then echo $(now) "getopts begin"; fi

all_args=("$@")
while getopts "c:i:j:p:k:u:d:z:hl:n:sfbw:o:O:T:t:D:a:U:CG:N:-:" opt; do
    case $opt in
    -)
        case "${OPTARG}" in
            ha-group)
                eval nextopt=\${$OPTIND}
                ARG_HZN_HA_GROUP=${nextopt}
                OPTIND=$(( OPTIND + 1 ))
                ;;
            container)
                ARG_AGENT_IN_CONTAINER=true
                ;;
            auto-upgrade)
                ARG_AGENT_AUTO_UPGRADE=true
                ;;
            namespace)
                eval nextopt=\${$OPTIND}
                ARG_AGENT_NAMESPACE=${nextopt}
                OPTIND=$(( OPTIND + 1 ))
                ;;
            namespace-scoped)
                ARG_NAMESPACE_SCOPED=true
                ;;
            help)
                usage 0
                ;;
            *)  echo "Invalid option: --$OPTARG"
                usage 1
                ;;
        esac
        ;;
    c)  ARG_AGENT_CERT_FILE="$OPTARG"
        ;;
    k)  ARG_AGENT_CFG_FILE="$OPTARG"
        ;;
    i)  ARG_INPUT_FILE_PATH="$OPTARG"
        ;;
    z)  ARG_AGENT_INSTALL_ZIP="$OPTARG"
        ;;
    j)  ARG_PKG_APT_KEY="$OPTARG"
        ;;
    t)  ARG_APT_REPO_BRANCH="$OPTARG"
        ;;
    O)  ARG_HZN_ORG_ID="$OPTARG"
        ;;
    u)  ARG_HZN_EXCHANGE_USER_AUTH="$OPTARG"
        ;;
    a)  ARG_HZN_EXCHANGE_NODE_AUTH="$OPTARG"
        ;;
    d)  ARG_HZN_NODE_ID="$OPTARG"
        ;;
    p)  ARG_HZN_EXCHANGE_PATTERN="$OPTARG"
        ;;
    n)  ARG_HZN_NODE_POLICY="$OPTARG"
        ;;
    w)  ARG_AGENT_WAIT_FOR_SERVICE="$OPTARG"
        ;;
    o)  ARG_AGENT_WAIT_FOR_SERVICE_ORG="$OPTARG"
        ;;
    T)  ARG_AGENT_REGISTRATION_TIMEOUT="$OPTARG"
        ;;
    s)  ARG_AGENT_SKIP_REGISTRATION=true
        ;;
    D)  ARG_AGENT_DEPLOY_TYPE="$OPTARG"
        ;;
    U)  ARG_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY="$OPTARG"
        ;;
    l)  ARG_AGENT_VERBOSITY="$OPTARG"
        ;;
    f)  ARG_AGENT_OVERWRITE=true
        ;;
    b)  ARG_AGENT_SKIP_PROMPT=true
        ;;
    C)  ARG_AGENT_ONLY_CLI=true
        ;;
    G)  ARG_AGENT_UPGRADE_TYPES="$OPTARG"
        ;;
    N)  ARG_AGENT_CONTAINER_NUMBER="$OPTARG"
        ;;
    h)  usage 0
        ;;
    \?) echo "Invalid option: -$OPTARG"
        usage 1
        ;;
    :)  echo "Option -$OPTARG requires an argument"
        usage 1
        ;;
    esac
done
if [[ $AGENT_VERBOSITY -ge $VERB_DEBUG ]]; then echo $(now) "getopts end"; fi

#====================== User Input Gathering Functions ======================

function log_fatal() {
    : ${1:?} ${2:?}
    local exitCode=$1
    local msg="$2"
    if [[ $AGENT_AUTO_UPGRADE == 'true' ]]; then
        # output to both stdout and stderr
        echo $(now) "ERROR: $msg" | tee /dev/stderr   # we always show fatal error msgs
    else
        echo $(now) "ERROR: $msg" # we always show fatal error msgs
    fi
    exit $exitCode
}

function log_error() {
    if [[ $AGENT_AUTO_UPGRADE == 'true' ]]; then
        # output to both stdout and stderr
        log $VERB_ERROR "ERROR: $1" | tee /dev/stderr
    else
         log $VERB_ERROR "ERROR: $1"
    fi
}

function log_warning() {
    log $VERB_WARNING "WARNING: $1"
}

function log_info() {
    local msg="$1"
    local nonewline=$2   # optionally 'nonewline'
    if [[ $nonewline == 'nonewline' ]]; then
        printf "$(now) $msg"
    else
        log $VERB_INFO "$msg"
    fi
}

function log_verbose() {
    log $VERB_VERBOSE "VERBOSE: $1"
}

function log_debug() {
    log $VERB_DEBUG "DEBUG: $1"
}

function log() {
    local log_level=$1
    local msg="$2"
    if [[ $AGENT_VERBOSITY -ge $log_level ]]; then
        echo $(now) "$msg"
    fi
}

# Get the value that has been specified by the user for this variable. Precedence: 1) cli flag, 2) env variable, 3) config file, 4) default.
# The side-effect of this function is that it will set the global variable named var_name.
# This function should be called after processing all of the cli args into ARG_<envvarname> and the cfg file values into CFG_<envvarname>
function get_variable() {
    log_debug "get_variable() begin"
    local var_name="$1"   # the name of the env var (not its value)
    local default_value="$2"
    local is_required=${3:-false}   # if set to true, normally default_value will be empty, and it means the user must specify via cli flag, env var, or cfg file

    # These are indirect variable references (http://mywiki.wooledge.org/BashFAQ/006#Indirection) and used because associative arrays are only support in bash 4 or above (macos is currently bash 3)
    local arg_name="ARG_$var_name"
    local cfg_name="CFG_$var_name"

    local from   # where we found the value

    # Look for the value in precedence order, and then set the indirect variable $var_name via the read statement
    if [[ -n ${!arg_name} ]]; then
        IFS= read -r "$var_name" <<<"${!arg_name}"
        from='command line flag'
    elif [[ -n ${!var_name} ]]; then
        :   # env var is already set
        from='environment variable'
    elif [[ -n ${!cfg_name} ]]; then
        IFS= read -r "$var_name" <<<"${!cfg_name}"
        from='configuration file'
    else
        IFS= read -r "$var_name" <<<"$default_value"
        from='default value'
    fi

    if [[ -z ${!var_name} && $is_required == 'true' ]]; then
        log_fatal 1 "A value for $var_name must be specified"
    fi

    if [[ ( $var_name == *"AUTH"* || $var_name == *"TOKEN"* ) && -n ${!var_name} ]]; then
        varValue='******'
    else
        varValue="${!var_name}"
    fi

    if [[ ( $var_name == "HZN_EXCHANGE_URL" ) && -n ${!var_name} ]]; then
        local TMP_HZN_EXCHANGE_URL=$HZN_EXCHANGE_URL
        if [[ "$TMP_HZN_EXCHANGE_URL" != "" ]]; then
            LAST_CHAR=(${TMP_HZN_EXCHANGE_URL: -1})
            # If this URL ends with "/" the curl commands will fail so remove the trailing / if it exists
            if [[ "$LAST_CHAR" == "/" ]]; then
                TMP_HZN_EXCHANGE_URL=${TMP_HZN_EXCHANGE_URL%?}
                log_debug "Changing variable $var_name from ${!var_name} to ${TMP_HZN_EXCHANGE_URL}"
                export HZN_EXCHANGE_URL=${TMP_HZN_EXCHANGE_URL}
                varValue="${!var_name}"
            fi
        fi
    fi

    log_info "${var_name}: $varValue (from $from)"
    log_debug "get_variable() end"
}

# Get the correct css obj download path for the software packages if 'css:version' is specified.
# The path is $CSS_OBJ_AGENT_SOFTWARE_BASE-version
# The input is 'css:', 'css:version' or 'css:path'.
# Please do not add debug messages.
function get_input_file_css_path() {
    local __resultvar=$1

    local input_file_path

    if [[ $INPUT_FILE_PATH == css:* || $INPUT_FILE_PATH == remote:* ]]; then
        # split the input into 2 parts
        local part2
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            part2=${INPUT_FILE_PATH#"css:"}
        elif [[ $INPUT_FILE_PATH == remote:* ]]; then
            part2=${INPUT_FILE_PATH#"remote:"}
        fi

        if [[ -n $part2 ]]; then
            if [[ $part2 == /* ]]; then
                input_file_path=$INPUT_FILE_PATH
            else
                # new path with version
                input_file_path="css:${CSS_OBJ_AGENT_SOFTWARE_BASE}-${part2}"
            fi
        else
            # 'css:' case - need to use AgentFileVersion to decide if we can get the latest cert
            get_agent_file_versions
            if [[ $AGENT_FILE_VERSIONS_STATUS -eq 1 ]] && [ ${#AGENT_SW_VERSIONS[@]} -gt 0 ]; then
                # new way
                input_file_path="css:${CSS_OBJ_AGENT_SOFTWARE_BASE}-${AGENT_SW_VERSIONS[0]}"
            else
                # fall back to old default way
                input_file_path="css:$CSS_OBJ_PATH_DEFAULT"
            fi
        fi
    fi

    eval $__resultvar="'${input_file_path}'"
}

function get_input_file_remote_path() {
    local __resultvar=$1
    local input_file_path

    if [[ $INPUT_FILE_PATH == remote:* ]]; then
        # split the input into 2 parts
        local part2
        if [[ $INPUT_FILE_PATH == remote:* ]]; then
            part2=${INPUT_FILE_PATH#"remote:"}
        fi

        if [[ -n $part2 ]]; then
            input_file_path=$INPUT_FILE_PATH
        else
            # 'remote:' case - need to query image registory to get the highest version
            local highest
            get_agent_version_from_repository highest
            input_file_path="remote:$highest"
        fi
    fi

    eval $__resultvar="'${input_file_path}'"
}

function get_agent_version_from_repository() {
    local __resultvar=$1
    local highest_var

    if [[ -n $EDGE_CLUSTER_REGISTRY_USERNAME && -n $EDGE_CLUSTER_REGISTRY_TOKEN ]]; then
        auth="-u $EDGE_CLUSTER_REGISTRY_USERNAME:$EDGE_CLUSTER_REGISTRY_TOKEN"
    fi

    IFS='/' read -r -a repoarray<<< $IMAGE_ON_EDGE_CLUSTER_REGISTRY
    if [[ "${repoarray[0]}" == *"docker"* ]]; then
        repository_url="https://registry.hub.docker.com/v2/repositories/${repoarray[1]}/${repoarray[2]}/tags"
        agent_versions=$(curl $auth $repository_url 2>/dev/null | jq '.results[]["name"] | select(test("testing|latest") | not)' | sort -rV | tr '\n' ',') # "2.31.0-1495","2.31.0-1492","2.31.0-1021"
    else
        repository_url="https://${repoarray[0]}/v1/repositories/${repoarray[1]}/${repoarray[2]}/tags"
        agent_versions=$(curl $auth $repository_url 2>/dev/null | jq 'keys[]' | sort -rV | tr '\n' ',') # "2.31.0-1495","2.31.0-1492","2.31.0-1021"
    fi
    IFS=',' read -r -a agent_version_array <<< $agent_versions
    highest_var=${agent_version_array[0]}
    if [ -z ${highest_var} ]; then
        get_agent_file_versions
        if [[ $AGENT_FILE_VERSIONS_STATUS -eq 1 ]] && [ ${#AGENT_SW_VERSIONS[@]} -gt 0 ]; then
            highest_var=${AGENT_SW_VERSIONS[0]}
        fi
    fi

    if [ -z ${highest_var} ]; then
        log_fatal 3 "Unable to get image tags, exiting"
    fi

    eval $__resultvar="${highest_var}"
}

# If INPUT_FILE_PATH is a short-hand value, turn it into the long-hand value. There are several variants of INPUT_FILE_PATH. See the -i flag in the usage.
# Side-effect: INPUT_FILE_PATH
function adjust_input_file_path() {
    log_debug "adjust_input_file_path() begin"
    local save_input_file_path=$INPUT_FILE_PATH
    INPUT_FILE_PATH="${INPUT_FILE_PATH%/}"   # remove trailing / if there
    if [[ $INPUT_FILE_PATH == css:* ]]; then
        :
    elif [[ $INPUT_FILE_PATH == 'anax:' ]]; then
        INPUT_FILE_PATH='https://github.com/open-horizon/anax/releases/latest/download'
    elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
        if [[ $INPUT_FILE_PATH == 'https://github.com/open-horizon/anax/releases' ]]; then
            INPUT_FILE_PATH="$INPUT_FILE_PATH/latest/download"   # default to the latest release
        elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases/tag/* ]]; then
            # They probably right-clicked a release and gave us like: https://github.com/open-horizon/anax/releases/tag/v2.27.0-110
            local rel_ver=${INPUT_FILE_PATH#https://github.com/open-horizon/anax/releases/tag/}
            if [[ -n $rel_ver ]]; then
                INPUT_FILE_PATH="https://github.com/open-horizon/anax/releases/download/$rel_ver"
            else
                INPUT_FILE_PATH="https://github.com/open-horizon/anax/releases/latest/download"   # default to the latest release
            fi
        fi
    elif [[ $INPUT_FILE_PATH == http* ]]; then
        log_info "Using INPUT_FILE_PATH value $INPUT_FILE_PATH as an APT repository"
        PKG_APT_REPO="$INPUT_FILE_PATH"
        INPUT_FILE_PATH='.'   # not sure this is necessary
    elif [[ $INPUT_FILE_PATH == remote:* ]]; then
        log_info "Using INPUT_FILE_PATH value $INPUT_FILE_PATH as cluster agent image input path"
    elif [[ ! -d $INPUT_FILE_PATH ]]; then
        log_fatal 1 "INPUT_FILE_PATH directory '$INPUT_FILE_PATH' does not exist"
    fi
    #else we treat it as a local dir that contains the pkgs

    if [[ -z $PKG_APT_REPO ]]; then
        if [[ $INPUT_FILE_PATH != $save_input_file_path ]];then
            log_info "INPUT_FILE_PATH adjusted to: $INPUT_FILE_PATH"
        fi
    fi
    log_debug "adjust_input_file_path() end"
}

# Return what we should use as AGENT_CFG_FILE default if it is not specified: AGENT_CFG_FILE_DEFAULT if it exists, else blank
function get_cfg_file_default() {
    if [[ -f $AGENT_CFG_FILE_DEFAULT ]]; then
        echo "$AGENT_CFG_FILE_DEFAULT"
    fi
    # else return empty string
}

# Returns version number extracted from the anax/releases path, or empty string if can't find it
# Note: this should be called after adjust_input_file_path()
function get_anax_release_version() {
    local input_file_path=${1:?}
    if [[ $input_file_path == https://github.com/open-horizon/anax/releases/latest/download* ]]; then
        echo 'latest'
    elif [[ $input_file_path == https://github.com/open-horizon/anax/releases/download/* ]]; then
        local rel_ver=${input_file_path#https://github.com/open-horizon/anax/releases/download/}
        rel_ver=${rel_ver#v}   # our convention is to add a 'v' at the beginning of releases, so remove it
        if [[ $rel_ver =~ $SEMVER_REGEX ]]; then
            echo "$rel_ver"
        fi
    fi
    #else return empty string, meaning we couldn't find it (so they should probably just use latest)
}

# get the exchange AgentFileVersion object.
function get_agent_file_versions() {
    log_debug "get_agent_file_versions() begin"

    # make sure it is only called once
    if [ $AGENT_FILE_VERSIONS_STATUS -ne 0 ]; then
        return 0
    fi

    # input checking requires either user creds or node creds
    local exch_creds cert_flag
    if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_USER_AUTH"
    elif [[ -n $HZN_EXCHANGE_NODE_AUTH ]]; then exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_NODE_AUTH"
    else
        # no creds available to get the agent file versions
        log_debug "No user or node credentials available to get the agent file versions"
        AGENT_FILE_VERSIONS_STATUS=2
        return 0
    fi

    # make sure HZN_EXCHANGE_URL is set
    if [[ -z $HZN_EXCHANGE_URL ]]; then
        log_fatal 1 "HZN_EXCHANGE_URL is not set."
    fi

    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        cert_flag="--cacert $AGENT_CERT_FILE"
    elif [[ -z $AGENT_CERT_FILE && -f $PERMANENT_CERT_PATH ]]; then
        # using cert from a previous install, see if that works
        cert_flag="--cacert $PERMANENT_CERT_PATH"
    else
        cert_flag="--insecure"
    fi

    local exch_output=$(curl -sSL -w "%{http_code}" ${CURL_RETRY_PARMS} ${cert_flag} ${HZN_EXCHANGE_URL}/orgs/IBM/AgentFileVersion -u ${exch_creds} 2>&1) || true
    log_debug "GET AgentFileVersion: $exch_output"

    if [[ -n $exch_output ]]; then
        local http_code="${exch_output: -3}"
        local len=${#exch_output}
        local output=${exch_output:0: len - 3}
        if [[ $http_code -eq 200 ]]; then
            # The outpit of AgentFileVersion has the following format:
            # {
            #      "agentSoftwareVersions": ["2.30.0", "2.25.4", "1.4.5-123"],
            #      "agentConfigVersions": ["0.0.4", "0.0.3"],
            #      "agentCertVersions": ["0.0.15"],
            # }
            OLDIFS=${IFS}
            IFS=' '
            local tmp=$(echo $output |jq -r '.agentSoftwareVersions[]' | sort -rV |tr '\n' ' ')
            read -a AGENT_SW_VERSIONS <<< $tmp
            tmp=$(echo $output |jq -r '.agentConfigVersions[]' | sort -rV |tr '\n' ' ')
            read -a AGENT_CONFIG_VERSIONS <<< $tmp
            tmp=$(echo $output |jq -r '.agentCertVersions[]' | sort -rV |tr '\n' ' ')
            read -a AGENT_CERT_VERSIONS <<< $tmp
            #0 - not initialized, 1 - supported, 2 - not supported
            AGENT_FILE_VERSIONS_STATUS=1
            IFS=${OLDIFS}
        elif [[ $http_code -eq 404 ]]; then
            # exchange-api version 2.98.0 and older does not support AgentFileVersion API
            AGENT_FILE_VERSIONS_STATUS=2
        else
            log_fatal 1 "Failed to get AgentFileVersion from the exchange. $exch_output"
        fi
    fi

    log_debug "AGENT_FILE_VERSIONS_STATUS: $AGENT_FILE_VERSIONS_STATUS"
    log_debug "AGENT_SW_VERSIONS from the exchange: ${AGENT_SW_VERSIONS[@]} sizs=${#AGENT_SW_VERSIONS[@]}"
    log_debug "AGENT_CERT_VERSIONS from the exchange: ${AGENT_CERT_VERSIONS[@]} sizs=${#AGENT_CERT_VERSIONS[@]}"
    log_debug "AGENT_CONFIG_VERSIONS from the exchange: ${AGENT_CONFIG_VERSIONS[@]} sizs=${#AGENT_CONFIG_VERSIONS[@]}"

    log_debug "get_agent_file_versions() end"
}

# Download the certificate file from CSS if not present
function get_certificate() {
    log_debug "get_certificate() begin"

    local css_cert_path
    get_cert_file_css_path css_cert_path

    if [[ -n $css_cert_path ]]; then
        download_css_file "$css_cert_path"

    fi

    #todo: support the case in which the mgmt hub is using a CA-trusted cert, so we don't need to use a cert at all

    log_debug "get_certificate() end"
}

# get the cert file path beased on the value in AGENT_CERT_FILE_CSS and INPUT_FILE_PATH, will set AGENT_CERT_VERSION
function get_cert_file_css_path() {
    log_debug "get_cert_file_css_path() begin"

    local __resultvar=$1

    local css_path

    local need_latest_cert=false
    if [[ $AGENT_CERT_FILE_CSS == css:* ]]; then
        local part2=${AGENT_CERT_FILE_CSS#"css:"}
        if [[ -n $part2 ]]; then
            if [[ $part2 == /* ]]; then
                # new or old with css path
                css_path=$AGENT_CERT_FILE_CSS
            else
                # new with version
                css_path="css:${CSS_OBJ_AGENT_CERT_BASE}-${part2}"
                AGENT_CERT_VERSION=${part2}
            fi
        else
            # 'css:' case - need to use AgentFileVersion to decide if we can get the latest csert
            need_latest_cert=true
        fi
    elif [[ $INPUT_FILE_PATH == css:* ]]; then
        if [[ -n $AGENT_CERT_FILE && ! -f $AGENT_CERT_FILE ]]; then
            local part2=${INPUT_FILE_PATH#"css:"}
            if [[ -n $part2 ]]; then
                if [[ $part2 == /* ]]; then
                    if [[ "$INPUT_FILE_PATH" == "css:$CSS_OBJ_PATH_DEFAULT" ]]; then
                        # old way with css default path
                        css_path=$INPUT_FILE_PATH
                    else
                        # INPUT_FILE_PATH new way with css path, get the latest cert
                        need_latest_cert=true
                    fi
                else
                    # new with version
                    need_latest_cert=true
                fi
            else
                # 'css:' case - need to use AgentFileVersion to decide if we can get the latest csert
                need_latest_cert=true
            fi
        fi
    fi

    if [ $need_latest_cert == true ]; then
        # get the latest cert versions from AgentFileVersion exchange API
        get_agent_file_versions
        if [[ $AGENT_FILE_VERSIONS_STATUS -eq 1 ]] && [ ${#AGENT_CERT_VERSIONS[@]} -gt 0 ]; then
            # new way
            css_path="css:${CSS_OBJ_AGENT_CERT_BASE}-${AGENT_CERT_VERSIONS[0]}"
            AGENT_CERT_VERSION=${AGENT_CERT_VERSIONS[0]}
        else
            # fall back to old default way
            css_path="css:$CSS_OBJ_PATH_DEFAULT"
        fi
    fi

    css_path="$css_path/$AGENT_CERT_FILE_DEFAULT"
    echo "css_path=$css_path, AGENT_CERT_VERSION=$AGENT_CERT_VERSION"
    eval $__resultvar="'${css_path}'"

    log_debug "get_cert_file_css_path() end"
}

# Download a file from the specified CSS path
function download_css_file() {
    log_debug "download_css_file() begin"
    local css_path=$1   # should be like: css:/api/v1/objects/IBM/agent_files/<file>
    local remote_path="${css_path/#css:/${HZN_FSS_CSSURL%/}}/data"   # replace css: with the value of HZN_FSS_CSSURL and add /data on the end
    local local_file="${css_path##*/}"   # get base name

    # Set creds flag
    local exch_creds cert_flag
    if [[ -z $HZN_FSS_CSSURL ]]; then
        log_fatal 1 "HZN_FSS_CSSURL must be specified"
    fi
    if [[ -z $HZN_ORG_ID ]]; then
        log_fatal 1 "HZN_ORG_ID must be specified"
    fi
    if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then
        exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_USER_AUTH"
    elif [[ -n $HZN_EXCHANGE_NODE_AUTH ]]; then
        exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_NODE_AUTH"
    else
        log_fatal 1 "Either HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH must be specified"
    fi

    # Set cert flag. This is a special case, because sometimes the cert we need is coming from CSS. In that case be creative to try to get it.
    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        # Either we have already downloaded it, or they gave it to us separately
        cert_flag="--cacert $AGENT_CERT_FILE"
    fi

    local remote_cert_path
    if [[ -z $cert_flag && -f $PERMANENT_CERT_PATH ]]; then
        # get the cert file path
        local css_cert_path
        get_cert_file_css_path css_cert_path
        remote_cert_path="${css_cert_path/#css:/${HZN_FSS_CSSURL%/}}/data"

        # Cert from a previous install, see if that works
        log_info "Attempting to download file $remote_cert_path using $PERMANENT_CERT_PATH ..."
        httpCode=$(curl -sSL -w "%{http_code}" ${CURL_RETRY_PARMS} -u "$exch_creds" --cacert "$PERMANENT_CERT_PATH" -o "$AGENT_CERT_FILE" $remote_cert_path 2>/dev/null || true)
        if [[ $? -eq 0 && $httpCode -eq 200 ]]; then
            cert_flag="--cacert $AGENT_CERT_FILE"   # we got it
        fi
    fi

    if [[ -z $cert_flag && $HZN_FSS_CSSURL == https:* ]]; then   #todo: this check still doesn't account for a CA-trusted cert
        # Still didn't find a valid cert. Get the cert from CSS by disabling cert checking
        rm -f "$AGENT_CERT_FILE"   # this probably has the error msg from the previous curl in it

        # get the cert file path
        if [[ -z $remote_cert_path ]]; then
            local css_cert_path
            get_cert_file_css_path css_cert_path
            remote_cert_path="${css_cert_path/#css:/${HZN_FSS_CSSURL%/}}/data"
        fi

        log_info "Downloading file $remote_cert_path using --insecure ..."
        #echo "DEBUG: curl -sSL -w \"%{http_code}\" -u \"$exch_creds\" --insecure -o \"$AGENT_CERT_FILE\" $remote_cert_path || true"   # log_debug isn't set up yet
        httpCode=$(curl -sSL -w "%{http_code}" ${CURL_RETRY_PARMS} -u "$exch_creds" --insecure -o "$AGENT_CERT_FILE" $remote_cert_path || true)
        if [[ $? -ne 0 || $httpCode -ne 200 ]]; then
            local err_msg=$(cat $AGENT_CERT_FILE 2>/dev/null)
            rm -f "$AGENT_CERT_FILE"   # this probably has the error msg from the previous curl in it
            log_fatal 3 "could not download $remote_cert_path: $err_msg"
        fi
        cert_flag="--cacert $AGENT_CERT_FILE"   # we got it
    fi

    # Get the file they asked for
    if [[ $local_file != $AGENT_CERT_FILE ]]; then   # if they asked for the cert, we already got that
        log_info "Downloading file $remote_path ..."
        download_with_retry $remote_path $local_file "$cert_flag" $exch_creds
    fi

    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        local version_from_cert_file
        getCertVersionFromCertFile version_from_cert_file

        if [[ -n $AGENT_CERT_VERSION ]]; then
            if is_linux; then  # Skip writing comment to cert file for MacOS since it failed on M1 machine in mac_trust_cert step
                if [[ -z $version_from_cert_file ]]; then
                    # write AGENT_CERT_VERSION in file comment as ------OpenHorizon Version x.x.x-----
                    version_to_add="-----OpenHorizon Version $AGENT_CERT_VERSION-----"
                    log_debug "add this line $version_to_add to cert file: $AGENT_CERT_FILE"

                    echo "-----OpenHorizon Version $AGENT_CERT_VERSION-----" > tmp-agent-install.crt
                    cat $AGENT_CERT_FILE >> tmp-agent-install.crt
                    mv tmp-agent-install.crt $AGENT_CERT_FILE
                elif [[ "$AGENT_CERT_VERSION" != "$version_from_cert_file" ]]; then
                    # if version in cert != version in css filename, overwrite to use version in css filename
                    sed -i "s#$version_from_cert_file#${AGENT_CERT_VERSION}#g" $AGENT_CERT_FILE
                fi
            fi
        fi
    fi

    log_debug "download_css_file() end"
}

function download_with_retry() {
    log_debug "download_with_retry() begin"
    local url=$1
    local outputFile=$2
    local certFlag=$3
    local cred=$4
    retry=0

    set +e
    while [[ $retry -le $MAX_HTTP_RETRY ]]; do
        if [[ -n "$cred" && -n "$outputFile" ]]; then
            # download from CSS
            httpCode=$(curl -sSL -w "%{http_code}" -u "$cred" $certFlag -o "$outputFile" $url 2>/dev/null)
        else
            # download from github release
            httpCode=$(curl -sSLO -w "%{http_code}" $url 2>/dev/null)
        fi

        curlCode=$?
        if [[ $curlCode -ne 0 || $httpCode =~ .*(503|504).* ]]; then
            log_debug "retry downloading $retry because curlCode is $curlCode, httpCode is $httpCode..."
            count=$((retry + 1))
            retry=$count
            sleep $SLEEP_SECONDS_BETWEEN_RETRY
            continue
        else
            break
        fi
    done
    set -e
    chkHttp $curlCode $httpCode 200 "downloading $url" $outputFile
    log_debug "download_with_retry() end"
}

# returns the cert version from certificate file
function getCertVersionFromCertFile() {
    log_debug "getCertVersionFromCertFile() begin"
    local __resultvar=$1

    local cert_ver_from_file
    while read -r line || [[ -n "$line" ]]; do
        if [[ -z $line || ${line:0:1} != '-' ]]; then continue; fi   # only find the line start with "-"
        content=$(echo $line | sed 's/-----\(.*\)-----/\1/')  # extract the part between ----- and -----
        if [[ -n $content && $content == *"OpenHorizon Version"* ]]; then
            log_debug "cert contains OpenHorizon Version: $content"
            cert_ver_from_file=${content##* }
        else
            continue
        fi
    done < "$AGENT_CERT_FILE"

    log_debug "cert_ver_from_file=$cert_ver_from_file"
    eval $__resultvar="'${cert_ver_from_file}'"
    log_debug "getCertVersionFromCertFile() end"
}

function download_anax_release_file() {
    log_debug "download_anax_release_file() begin"
    local anax_release_path=$1   # should be like: https://github.com/open-horizon/anax/releases/latest/download/<file>
    local local_file="${anax_release_path##*/}"   # get base name
    log_info "Downloading file $anax_release_path ..."
    download_with_retry $anax_release_path $local_file
    log_debug "download_anax_release_file() end"
}

# If necessary, download the cfg file from CSS.
# The input_file_path is either AGENT_CFG_FILE or INPUT_FILE_PATH, the second
# parameter tells which one it is: 1 for AGENT_CFG_FILE, 0 for INPUT_FILE_PATH.
# this function sets AGENT_CONFIG_VERSION
function download_config_file() {
    log_debug "download_config_file() begin"
    local input_file_path=$1   # normally the value of INPUT_FILE_PATH or AGENT_CFG_FILE
    local is_config_path=$2

    if [[ $input_file_path == css:* ]]; then
        local css_path
        local need_latest_cfg=false
        local part2=${input_file_path#"css:"}
        if [[ -n $part2 ]]; then
            if [[ $part2 == /* ]]; then
                if [[ $is_config_path -eq 1 ]]; then
                    # AGENT_CFG_FILE contains path css path, use it
                    css_path=$input_file_path
                else
                    # INPUT_FILE_PATH contains the path, only use it when it is the old default way
                    need_latest_cfg=true
                    if [[ "$input_file_path" == "css:$CSS_OBJ_PATH_DEFAULT" ]]; then
                        # old way with css default path
                        css_path=$input_file_path
                    else
                        # INPUT_FILE_PATH new way with css path, get the latest config
                        need_latest_cfg=true
                    fi
                fi
            else
                if [[ $is_config_path -eq 1 ]]; then
                    # AGENT_CFG_FILE contains the version, use it
                    css_path="css:${CSS_OBJ_AGENT_CONFIG_BASE}-${part2}"
                    AGENT_CONFIG_VERSION=${part2}
                else
                    # INPUT_FILE_PATH contains the version, cannot use it
                    need_latest_cfg=true
                fi
            fi
        else
            # 'css:' case - need to use AgentFileVersion to decide if we can get the latest csert
            need_latest_cfg=true
        fi

        if [ $need_latest_cfg == true ]; then
            # get the latest config versions from AgentFileVersion exchange API
            get_agent_file_versions
            if [[ $AGENT_FILE_VERSIONS_STATUS -eq 1 ]] && [ ${#AGENT_CONFIG_VERSIONS[@]} -gt 0 ]; then
                # new way
                css_path="css:${CSS_OBJ_AGENT_CONFIG_BASE}-${AGENT_CONFIG_VERSIONS[0]}"
                AGENT_CONFIG_VERSION=${AGENT_CONFIG_VERSIONS[0]}
            else
                # fall back to old default way
                css_path="css:$CSS_OBJ_PATH_DEFAULT"
            fi
        fi

        # now do the downloading
        if [[ -n $css_path ]]; then
            download_css_file "$css_path/$AGENT_CFG_FILE_DEFAULT"
        fi
    fi

    echo "AGENT_CONFIG_VERSION=$AGENT_CONFIG_VERSION"

    # the cfg is specific to the instance of the cluster, so not available from anax/release
    #elif [[ $input_file_path == https://github.com/open-horizon/anax/releases* ]]; then
    #    download_anax_release_file "$input_file_path/$AGENT_CFG_FILE_DEFAULT"
    log_debug "download_config_file() end"
}

# Read the configuration file and put each value in CFG_<envvarname>, so each later can be applied with the correct precedence
# Side-effect: sets or adjusts AGENT_CFG_FILE
function read_config_file() {
    log_debug "read_config_file() begin"

    # Get/locate cfg file
    if [[ -n $AGENT_CFG_FILE && -f $AGENT_CFG_FILE ]]; then
        :   # just fall thru this if-else to read the config file
    elif using_remote_input_files 'cfg'; then
        if [[ $AGENT_CFG_FILE == css:*  ]]; then
            download_config_file "$AGENT_CFG_FILE" 1  # in this case AGENT_CFG_FILE is the css object path NOT including the last part (agent-install.cfg)
        else
            if [[ -n $AGENT_CFG_FILE && $AGENT_CFG_FILE != $AGENT_CFG_FILE_DEFAULT ]]; then
                log_fatal 1 "Can not specify both -k (AGENT_CFG_FILE) and -i (INPUT_FILE_PATH)"
            fi
            download_config_file "$INPUT_FILE_PATH" 0
        fi
        AGENT_CFG_FILE=$AGENT_CFG_FILE_DEFAULT   # this is where download_config_file() will put it
    elif [[ -z $AGENT_CFG_FILE ]]; then
        if [[ -f $AGENT_CFG_FILE_DEFAULT ]]; then
            AGENT_CFG_FILE=$AGENT_CFG_FILE_DEFAULT   # Only apply this default if the file is actually there
        else
            log_info "Configuration file not specified. All required input variables must be set via command arguments or the environment."
            return
        fi
    elif [[ ! -f "$AGENT_CFG_FILE" ]]; then
        log_fatal 1 "Configuration file $AGENT_CFG_FILE not found."
    fi

    log_debug "Start to get config version from AGENT_CFG_FILE: $AGENT_CFG_FILE"

    set +e   # Need to disable for the following command so script doesn't terminate
    config_in_file=$( cat ${AGENT_CFG_FILE} | grep "HZN_CONFIG_VERSION" )
    config_version_in_file=${config_in_file#*=}
    set -e
    log_debug "config_version_in_file: $config_version_in_file"

    # Add config version into config file
    if [[ -n $AGENT_CONFIG_VERSION ]]; then
        # write or replace AGENT_CERT_VERSION in file comment as config_version=x.x.x
        if [[ -z $config_in_file ]]; then
            # add
            echo "HZN_CONFIG_VERSION=$AGENT_CONFIG_VERSION" >> $AGENT_CFG_FILE
        elif [[ "$AGENT_CONFIG_VERSION" != "$config_version_in_file" ]]; then
            # replace, overwrite with the agent config file version in css filename
            sed -i -e "s#HZN_CONFIG_VERSION=${config_version_in_file}#HZN_CONFIG_VERSION=${AGENT_CONFIG_VERSION}#g" $AGENT_CFG_FILE
        fi
    fi

    # Read/parse the config file. Note: omitting IFS= because we want leading and trailing whitespace trimmed. Also -n $line handles the case where there is not a newline at the end of the last line.
    log_verbose "Using configuration file: $AGENT_CFG_FILE"
    while read -r line || [[ -n "$line" ]]; do
        if [[ -z $line || ${line:0:1} == '#' ]]; then continue; fi   # ignore empty or commented lines
        #echo "'$line'"
        local var_name="CFG_${line%%=*}"   # the variable name is the line with everything after the 1st = removed
        IFS= read -r "$var_name" <<<"${line#*=}"   # set the variable to the line with everything before the 1st = removed
    done < "$AGENT_CFG_FILE"

    log_debug "read_config_file() end"
}

# make sure that the given input file types has correct values.
# Side-effect: AGENT_CFG_FILE and/or AGENT_CERT_FILE changes to horizon default
# if -G or AGENT_UPGRADE_TYPES are specified and 'config' or 'cert'
# is not included.
function varify_upgrade_types() {
    log_debug "varify_upgrade_types() begin"

    # validate the input values
    for t in ${AGENT_UPGRADE_TYPES//,/ }
    do
        if [ "$t" != "$UPGRADE_TYPE_SW" ] && [ "$t" != "$UPGRADE_TYPE_CERT" ] && [ "$t" != "$UPGRADE_TYPE_CFG" ]; then
            log_fatal 1 "Invalid value in AGENT_UPGRADE_TYPES or -G flag: $AGENT_UPGRADE_TYPES"
        fi
    done

    # use installed config file and certification file if they are not specified
    if ! has_upgrade_type_cfg; then
        if [ -f $INSTALLED_AGENT_CFG_FILE ]; then
            # remove the HZN_MGMT_HUB_CERT_PATH line so that the provided cert file can be used (if any)
            local tmp_dir=$(get_tmp_dir)
            AGENT_CFG_FILE="${tmp_dir}/${AGENT_CERT_FILE_DEFAULT}"
            sed '/HZN_MGMT_HUB_CERT_PATH/d' $INSTALLED_AGENT_CFG_FILE > $AGENT_CFG_FILE
            log_info "AGENT_CFG_FILE is set to $AGENT_CFG_FILE"
        else
            log_fatal 1 "$INSTALLED_AGENT_CFG_FILE file does not exist when'$UPGRADE_TYPE_CFG' is not specified in AGENT_UPGRADE_TYPES or -G flag. Please make sure the agent is installed."
        fi
    fi
    if ! has_upgrade_type_cert; then
        if [ -f $INSTALLED_AGENT_CFG_FILE ]; then
            local horizon_default_cert_file=$(grep -E '^HZN_MGMT_HUB_CERT_PATH=' $INSTALLED_AGENT_CFG_FILE || true)
            horizon_default_cert_file=$(trim_variable "${horizon_default_cert_file#*=}")
            AGENT_CERT_FILE=$horizon_default_cert_file
            log_info "AGENT_CERT_FILE is set to $horizon_default_cert_file"
        else
            log_fatal 1 "$INSTALLED_AGENT_CFG_FILE file does not exist when'$UPGRADE_TYPE_CERT' is not specified in AGENT_UPGRADE_TYPES or -G flag. Please make sure the agent is installed."
        fi
    fi

    log_debug "varify_upgrade_types() end"
}

# check if the upgrade type contains "software"
function has_upgrade_type_sw() {
    for t in ${AGENT_UPGRADE_TYPES//,/ }
    do
        if [ "$t" == "$UPGRADE_TYPE_SW" ]; then
            return 0
        fi
    done
    return 1
}

# check if the upgrade type contains "cert"
function has_upgrade_type_cert() {
    for t in ${AGENT_UPGRADE_TYPES//,/ }
    do
        if [ "$t" == "$UPGRADE_TYPE_CERT" ]; then
            return 0
        fi
    done
    return 1
}

# check if the upgrade type contains "configuration"
function has_upgrade_type_cfg() {
    for t in ${AGENT_UPGRADE_TYPES//,/ }
    do
        if [ "$t" == "$UPGRADE_TYPE_CFG" ]; then
            return 0
        fi
    done
    return 1
}

# Create a temp directory for the agent.
# For anax-in-container case, the directory is /tmp/horizon{i}, where i is the container number.
# For native case, the directory is /tmp
function get_tmp_dir() {
    local container_num=$(get_agent_container_number)
    local tmp_dir="/tmp"
    if [[ $container_num != "0" ]]; then
        tmp_dir="/tmp/horizon${container_num}"
        mkdir -p $tmp_dir
    fi

    echo "$tmp_dir"
}

# Returns the horizon and horizon-cli version.
# It returns empty string for horizon-cli if the hzn command cannot be found.
# It returns empty string if the agent is not up and running.
function get_local_horizon_version() {
    # input/output variables
    local __result_agent_ver=$1
    local __result_cli_ver=$2

    if isCmdInstalled hzn; then
        # hzn is installed, need to check the versions
        local output=$(HZN_LANG=en_US hzn version)
        local cli_version=$(echo "$output" | grep "^Horizon CLI")
        local agent_version=$(echo "$output" | grep "^Horizon Agent")
        cli_version=${cli_version##* }   # remove all of the space-separated words, except the last one
        if $(echo $agent_version | grep -i failed); then
            agent_version=""
        else
            agent_version=${agent_version##* }   # remove all of the space-separated words, except the last one
        fi
    else
        cli_version=""
        agent_version=""
    fi

    eval $__result_agent_ver="'$agent_version'"
    eval $__result_cli_ver="'$cli_version'"
}

# Get all of the input values from cmd line args, env vars, config file, or defaults.
# Side-effect: sets all of the variables as global constants
function get_all_variables() {
    log_debug "get_all_variables() begin"

    OS=$(get_os)
    detect_distro   # if linux, sets: DISTRO, DISTRO_VERSION_NUM, CODENAME
    ARCH=$(get_arch)
    log_info "OS: $OS, Distro: $DISTRO, Distro Release: $DISTRO_VERSION_NUM, Distro Code Name: $CODENAME, Architecture: $ARCH"

    get_variable AGENT_IN_CONTAINER 'false'
    get_variable AGENT_CONTAINER_NUMBER '1'
    get_variable AGENT_AUTO_UPGRADE 'false'
    get_variable AGENT_UPGRADE_TYPES "$UPGRADE_TYPE_SW,$UPGRADE_TYPE_CERT,$UPGRADE_TYPE_CFG" 'false'
    varify_upgrade_types

    # First unpack the zip file (if specified), because the config file could be in there
    get_variable AGENT_INSTALL_ZIP
    if [[ -n $AGENT_INSTALL_ZIP ]]; then
        if [[ -f "$AGENT_INSTALL_ZIP" ]]; then
            rm -f "$AGENT_CFG_FILE_DEFAULT" "$AGENT_CERT_FILE_DEFAULT" horizon*   # clean up files from a previous run
            log_info "Unpacking $AGENT_INSTALL_ZIP ..."
            tar -zxf $AGENT_INSTALL_ZIP
            # now that all of the individual input files are in the local dir, continue like normal
        else
            log_fatal 1 "File $AGENT_INSTALL_ZIP does not exist"
        fi
    fi

    # Next get config file values (cmd line has already been parsed), so get_variable can apply the whole precedence order
    get_variable INPUT_FILE_PATH '.'
    adjust_input_file_path

    get_variable AGENT_CFG_FILE "$(get_cfg_file_default)"

    if [[ -n $AGENT_CFG_FILE && -f $AGENT_CFG_FILE ]] || ! using_remote_input_files 'cfg'; then
        # Read this as soon as possible, so things like HZN_ORG_ID can be specified in the cfg file
        read_config_file
    fi
    get_variable AGENT_VERBOSITY 3
    # need to check this value right now, because we use it immediately
    if [[ $AGENT_VERBOSITY -lt 0 || $AGENT_VERBOSITY -gt $VERB_DEBUG ]]; then
        log_fatal 1 "AGENT_VERBOSITY must be in the range 0 - $VERB_DEBUG"
    fi
    # these are needed to read the cfg file from CSS
    get_variable HZN_EXCHANGE_URL '' 'false'
    get_variable HZN_ORG_ID '' 'true'
    get_variable HZN_MGMT_HUB_CERT_PATH
    get_variable AGENT_CERT_FILE "${HZN_MGMT_HUB_CERT_PATH:-$AGENT_CERT_FILE_DEFAULT}"   # use the default value even if the file doesn't exist yet, because '-i css:'' might create it

    if [[ $AGENT_CERT_FILE == css:* ]]; then
        # Store the css path for the cert file, so we can change AGENT_CERT_FILE to where it will end up locally
        AGENT_CERT_FILE_CSS=$AGENT_CERT_FILE
        AGENT_CERT_FILE=$AGENT_CERT_FILE_DEFAULT
        rm -f $AGENT_CERT_FILE   # they told us to download it, so don't use the file already there
    fi
    if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
        get_variable HZN_FSS_CSSURL '' 'true'
    else
        get_variable HZN_FSS_CSSURL ''
    fi
    get_variable HZN_EXCHANGE_USER_AUTH
    get_variable HZN_EXCHANGE_NODE_AUTH
    if using_remote_input_files 'cfg'; then
        # Now we have enough of the other input variables to do this
        read_config_file   # this will download the cert and cfg
    fi

    # Now that we have the values from cmd line and config file, we can get all of the variables
    get_variable AGENT_SKIP_REGISTRATION 'false'
    get_variable HZN_EXCHANGE_URL '' 'true'
    get_variable HZN_AGBOT_URL   # for now it is optional, since not every other component in the field supports it yet
    get_variable HZN_SDO_SVC_URL   # for now it is optional, since not every other component in the field supports it yet
    get_variable HZN_FDO_SVC_URL   # for now it is optional, since not every other component in the field supports it yet
    get_variable NODE_ID   # deprecated
    get_variable HZN_DEVICE_ID
    get_variable HZN_NODE_ID
    get_variable HZN_CONFIG_VERSION
    get_variable HZN_HA_GROUP
    get_variable HZN_AGENT_PORT
    get_variable HZN_EXCHANGE_PATTERN
    get_variable HZN_NODE_POLICY
    get_variable AGENT_WAIT_FOR_SERVICE
    get_variable AGENT_WAIT_FOR_SERVICE_ORG
    get_variable AGENT_REGISTRATION_TIMEOUT
    get_variable AGENT_OVERWRITE 'false'
    get_variable AGENT_SKIP_PROMPT 'false'
    get_variable AGENT_ONLY_CLI 'false'
    get_variable AGENT_INSTALL_ZIP 'agent-install-files.tar.gz'
    get_variable AGENT_DEPLOY_TYPE 'device'
    get_variable AGENT_WAIT_MAX_SECONDS '30'

    # Update HZN_ENV_FILE to be unique for this installation
    HZN_ENV_FILE="$HZN_ENV_FILE-$HZN_NODE_ID"

    if ! is_cluster && [[ $INPUT_FILE_PATH == remote:* ]]; then
        log_fatal 1 "\$INPUT_FILE_PATH cannot set to 'remote:<version>' if \$AGENT_DEPLOY_TYPE is 'device'"
    fi

    if is_device; then
        get_variable NODE_ID_MAPPING_FILE 'node-id-mapping.csv'
        get_variable PKG_APT_KEY
        get_variable APT_REPO_BRANCH 'updates'

        local image_arch=$(get_image_arch)
        # Currently only support a few architectures for anax-in-container; if anax-in-container specified, check the architecture
	if is_macos; then
                : # do not need to check
        else
                if [[ $AGENT_IN_CONTAINER == 'true' ]]; then
                        check_support "${SUPPORTED_ANAX_IN_CONTAINER_ARCH[*]}" "${image_arch}" 'anax-in-container architectures'
                fi
        fi

        get_variable AGENT_IMAGE_TAR_FILE "${image_arch}${DEFAULT_AGENT_IMAGE_TAR_FILE}"
    elif is_cluster; then
        # check kubectl is available
        if [ "${KUBECTL}" != "" ]; then   # If user set KUBECTL env variable, check that it exists
            if command -v $KUBECTL > /dev/null 2>&1; then
                : # nothing more to do
            else
                log_fatal 2 "$KUBECTL is not available. Please install $KUBECTL and ensure that it is found on your \$PATH"
            fi
        else
            # Nothing specified. Attempt to detect what should be used.
            if command -v k3s > /dev/null 2>&1; then    # k3s needs to be checked before kubectl since k3s creates a symbolic link to kubectl
                KUBECTL="k3s kubectl"
            elif command -v microk8s.kubectl >/dev/null 2>&1; then
                KUBECTL=microk8s.kubectl
            elif command -v oc >/dev/null 2>&1; then
                KUBECTL=oc
            elif command -v kubectl >/dev/null 2>&1; then
                KUBECTL=kubectl
            else
                log_fatal 2 "kubectl is not available. Please install kubectl and ensure that it is found on your \$PATH"
            fi
        fi
        log_info "KUBECTL is set to $KUBECTL"

        # get other variables for cluster agent
        get_variable EDGE_CLUSTER_STORAGE_CLASS 'gp2'
        check_cluster_storage_class "$EDGE_CLUSTER_STORAGE_CLASS"

        get_variable EDGE_CLUSTER_PVC_SIZE "$DEFAULT_PVC_SIZE"
        get_variable AGENT_NAMESPACE "$DEFAULT_AGENT_NAMESPACE"
        get_variable NAMESPACE_SCOPED 'false'
        get_variable USE_EDGE_CLUSTER_REGISTRY 'true'
        get_variable AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS '300'
        get_variable ENABLE_AUTO_UPGRADE_CRONJOB 'true'

        local image_arch=$(get_cluster_image_arch)
        check_support "${SUPPORTED_EDGE_CLUSTER_ARCH[*]}" "${image_arch}" 'kubernetes edge cluster architectures'
        DEFAULT_AGENT_K8S_IMAGE_TAR_FILE=${image_arch}${DEFAULT_AGENT_K8S_IMAGE_TAR_FILE}
        DEFAULT_CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE=${image_arch}${DEFAULT_CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE}

        local default_image_registry_on_edge_cluster
        local default_auto_upgrade_cronjob_image_registry_on_edge_cluster
        isImageVariableRequired=true
        if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
            if [[ $INPUT_FILE_PATH == remote:* ]]; then
                log_fatal 1 "Cannot use local cluster registry if \$INPUT_FILE_PATH is set to 'remote:<version>', please set \$USE_EDGE_CLUSTER_REGISTRY to 'false' or change \$INPUT_FILE_PATH to 'css:'."
            fi

            if [[ $KUBECTL == "microk8s.kubectl" ]]; then
                default_image_registry_on_edge_cluster="localhost:32000/$AGENT_NAMESPACE/${image_arch}_anax_k8s"
                isImageVariableRequired=false
            elif [[ $KUBECTL == "k3s kubectl" ]]; then
                local k3s_registry_endpoint=$($KUBECTL get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
                default_image_registry_on_edge_cluster="$k3s_registry_endpoint/$AGENT_NAMESPACE/${image_arch}_anax_k8s"
                isImageVariableRequired=false
            elif is_ocp_cluster; then
                local ocp_registry_endpoint=$($KUBECTL get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
                default_image_registry_on_edge_cluster="$ocp_registry_endpoint/$AGENT_NAMESPACE/${image_arch}_anax_k8s"
                isImageVariableRequired=false
            fi
	        # image variable $IMAGE_ON_EDGE_CLUSTER_REGISTRY is required

            get_variable INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY
        else
            # need to validate image arch in IMAGE_ON_EDGE_CLUSTER_REGISTRY
            if [[ -z $IMAGE_ON_EDGE_CLUSTER_REGISTRY ]]; then
                log_fatal 1 "A value for \$IMAGE_ON_EDGE_CLUSTER_REGISTRY must be specified"
            fi
            lastpart=$(echo $IMAGE_ON_EDGE_CLUSTER_REGISTRY | cut -d "/" -f 3) # <arch>_anax_k8s
            image_arch_in_param=$(echo $lastpart | cut -d "_" -f 1)
            if [[ "$image_arch" != "$image_arch_in_param" ]]; then
                log_fatal 1 "Cannot use agent image with $image_arch_in_param arch to install on $image_arch cluster, please use agent image with '$image_arch'"
            fi
        fi
        get_variable AGENT_K8S_IMAGE_TAR_FILE "$DEFAULT_AGENT_K8S_IMAGE_TAR_FILE"
        # default_image_registry_on_edge_cluster is not set if use remote image registry
        get_variable IMAGE_ON_EDGE_CLUSTER_REGISTRY "$default_image_registry_on_edge_cluster" ${isImageVariableRequired}
	    log_debug "default_image_registry_on_edge_cluster: $default_image_registry_on_edge_cluster, IMAGE_ON_EDGE_CLUSTER_REGISTRY: $IMAGE_ON_EDGE_CLUSTER_REGISTRY"

        if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
             # set $CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY from $IMAGE_ON_EDGE_CLUSTER_REGISTRY
            auto_upgrade_cronjob_image_registry_on_edge_cluster="${IMAGE_ON_EDGE_CLUSTER_REGISTRY%%_*}_auto-upgrade-cronjob_k8s"
            get_variable CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY "$auto_upgrade_cronjob_image_registry_on_edge_cluster"
            get_variable CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE "$DEFAULT_CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE"
        fi

        get_variable INIT_CONTAINER_IMAGE "$DEFAULT_INIT_CONTAINER_IMAGE_PATH"

        get_variable EDGE_CLUSTER_REGISTRY_USERNAME
        get_variable EDGE_CLUSTER_REGISTRY_TOKEN
        if [[ ( -z $EDGE_CLUSTER_REGISTRY_USERNAME && -n $EDGE_CLUSTER_REGISTRY_TOKEN ) || ( -n $EDGE_CLUSTER_REGISTRY_USERNAME && -z $EDGE_CLUSTER_REGISTRY_TOKEN ) ]]; then
            log_fatal 1 "EDGE_CLUSTER_REGISTRY_USERNAME and EDGE_CLUSTER_REGISTRY_TOKEN should be set/unset together"
        fi
    else
        log_fatal 1 "Invalid AGENT_DEPLOY_TYPE value: $AGENT_DEPLOY_TYPE"
    fi

    # Adjust some of the variable values or add related variables
    # The edge node id can be specified 4 different ways: -d (HZN_NODE_ID), the first part of -a (HZN_EXCHANGE_NODE_AUTH), NODE_ID(deprecated) or HZN_DEVICE_ID. Need to reconcile all of them.
    local node_id   # just used in this section of code to sort out this mess
    # find the 1st occurrence of the user specifying node it
    if [[ -n ${HZN_EXCHANGE_NODE_AUTH%%:*} ]]; then
        node_id=${HZN_EXCHANGE_NODE_AUTH%%:*}
        log_info "Using node id from HZN_EXCHANGE_NODE_AUTH"
    elif [[ -n $HZN_NODE_ID ]]; then
        node_id=$HZN_NODE_ID
        log_info "Using node id from HZN_NODE_ID"
    elif [[ -n $NODE_ID ]]; then
        node_id=$NODE_ID
        log_warning "Using node id from NODE_ID. NODE_ID is deprecated, please use HZN_NODE_ID in the future."
    elif [[ -n $HZN_DEVICE_ID ]]; then
        node_id=$HZN_DEVICE_ID
        log_warning "Using node id from HZN_DEVICE_ID"
    else   # not specified, default it
        #future: we should let 'hzn register' default the node id, but i think there are other parts of this script that depend on it being set
        # Try to get it from a previous installation
        if is_device; then
            node_id=$(grep HZN_NODE_ID $INSTALLED_AGENT_CFG_FILE 2>/dev/null | cut -d'=' -f2)
            if [[ -n $node_id ]]; then
                log_info "Using node id from HZN_NODE_ID in $INSTALLED_AGENT_CFG_FILE: $node_id"
            else
                # if HZN_NODE_ID is not set, look for HZN_DEVICE_ID
                node_id=$(grep HZN_DEVICE_ID $INSTALLED_AGENT_CFG_FILE 2>/dev/null | cut -d'=' -f2)
                if [[ -n $node_id ]]; then
                    log_info "Using node id from HZN_DEVICE_ID in $INSTALLED_AGENT_CFG_FILE: $node_id"
                else
                    node_id=${HOSTNAME}   # default
                    log_info "use hostname as node id"
                fi
            fi
        else    # cluster, read default from configmap
            if $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
                # configmap exist
                node_id=$($KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} -o jsonpath={.data.horizon}  | grep -E '^HZN_NODE_ID=' | cut -d '=' -f2)
                if [[ -n $node_id ]]; then
                    log_info "Using node id from HZN_NODE_ID in configmap ${CONFIGMAP_NAME}: $node_id"
                else
                    node_id=$($KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} -o jsonpath={.data.horizon}  | grep -E '^HZN_DEVICE_ID=' | cut -d '=' -f2)
                    if [[ -n $node_id ]]; then
                        log_info "Using node id from HZN_DEVICE_ID in configmap ${CONFIGMAP_NAME}: $node_id"
                    fi
                fi
            fi

            # if node_id is still not set, use ${HOSTNAME}
            if [[ -z $node_id ]]; then
                if is_device; then
                    node_id=${HOSTNAME}   # default
                    log_info "node_id is not set, use host name as node id"
                else
                    node_id=${AGENT_NAMESPACE}_${HOSTNAME}
                    log_info "node_id is not set, use namespace_hostname as node id"
                fi
            fi
        fi
    fi
    # check if they gave us conflicting values
    if [[ ( -n ${HZN_EXCHANGE_NODE_AUTH%%:*} && ${HZN_EXCHANGE_NODE_AUTH%%:*} != $node_id ) || ( -n $HZN_NODE_ID && $HZN_NODE_ID != $node_id ) || ( -n $NODE_ID && $NODE_ID != $node_id ) || ( -n $HZN_DEVICE_ID && $HZN_DEVICE_ID != $node_id ) ]]; then
        log_fatal 1 "If the edge node id is specified via multiple means (-d (HZN_NODE_ID), -a (HZN_EXCHANGE_NODE_AUTH), HZN_DEVICE_ID, or NODE_ID) they must all be the same value"
    fi
    # regardless of how they specified it to us, we need these variables set for the rest of the script
    NODE_ID=$node_id
    if [[ -z $HZN_EXCHANGE_NODE_AUTH ]]; then
        HZN_EXCHANGE_NODE_AUTH="${node_id}:"   # detault it, hzn register will fill in the token
    fi

    log_debug "get_all_variables() end"
}

# Check the validity of the input variable values that we can
function check_variables() {
    log_debug "check_variables() begin"

    # do not check the exchange credentials because the agent auto upgrade process does not use it.
    if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
        if [[ -z $HZN_EXCHANGE_USER_AUTH ]] && [[ -z ${HZN_EXCHANGE_NODE_AUTH#*:} ]]; then
            log_fatal 1 "If the node token is not specified in HZN_EXCHANGE_NODE_AUTH, then HZN_EXCHANGE_USER_AUTH must be specified"
        fi
    fi

    if [[ -n $HZN_MGMT_HUB_CERT_PATH && -n $AGENT_CERT_FILE && $HZN_MGMT_HUB_CERT_PATH != $AGENT_CERT_FILE ]]; then
        log_fatal 1 "If both HZN_MGMT_HUB_CERT_PATH and AGENT_CERT_FILE are specified they must be equal."
    fi

    # several flags can't be set with AGENT_ONLY_CLI
    if [[ $AGENT_ONLY_CLI == 'true' ]]; then
        if [[ -n $HZN_EXCHANGE_PATTERN || -n $HZN_NODE_POLICY || -n $ARG_AGENT_WAIT_FOR_SERVICE || -n $ARG_AGENT_REGISTRATION_TIMEOUT || -n $ARG_AGENT_WAIT_FOR_SERVICE_ORG || -n $ARG_HZN_HA_GROUP ]]; then
            log_fatal 1 "Since AGENT_ONLY_CLI=='$AGENT_ONLY_CLI', none of these can be set: HZN_EXCHANGE_PATTERN, HZN_NODE_POLICY, AGENT_WAIT_FOR_SERVICE, AGENT_REGISTRATION_TIMEOUT, AGENT_WAIT_FOR_SERVICE_ORG, HZN_HA_GROUP"
        fi
    fi

    # several flags can't be set with AGENT_SKIP_REGISTRATION
    if [[ $AGENT_SKIP_REGISTRATION == 'true' ]]; then
        if [[ -n $HZN_EXCHANGE_PATTERN || -n $HZN_NODE_POLICY || -n $ARG_AGENT_WAIT_FOR_SERVICE || -n $ARG_AGENT_REGISTRATION_TIMEOUT || -n $ARG_AGENT_WAIT_FOR_SERVICE_ORG || -n $ARG_HZN_HA_GROUP ]]; then
            log_fatal 1 "Since AGENT_SKIP_REGISTRATION=='$AGENT_SKIP_REGISTRATION', none of these can be set: HZN_EXCHANGE_PATTERN, HZN_NODE_POLICY, AGENT_WAIT_FOR_SERVICE, AGENT_REGISTRATION_TIMEOUT, AGENT_WAIT_FOR_SERVICE_ORG, HZN_HA_GROUP"
        fi
    fi

    # Policy and pattern are mutually exclusive
    if [[ -n "${HZN_NODE_POLICY}" && -n "${HZN_EXCHANGE_PATTERN}" ]]; then
        log_fatal 1 "HZN_NODE_POLICY and HZN_EXCHANGE_PATTERN can not both be set"
    fi

    # if a node policy is non-empty, check if the file exists
    if [[ -n $HZN_NODE_POLICY && ! -f $HZN_NODE_POLICY ]]; then
        log_fatal 1 "HZN_NODE_POLICY file '$HZN_NODE_POLICY' does not exist"
    fi

    if is_cluster && [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        parts=$(echo $IMAGE_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print NF}')
        if [[ "$parts" != "3" ]]; then
            log_fatal 1 "IMAGE_ON_EDGE_CLUSTER_REGISTRY should be this format: <registry-host>/<registry-repo>/<image-name>"
        fi
    fi

    if is_cluster && [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]] && [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
        parts=$(echo $CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print NF}')
        if [[ "$parts" != "3" ]]; then
            log_fatal 1 "CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY should be this format: <registry-host>/<registry-repo>/<image-name>"
        fi
    fi

    if [[ -n $AGENT_IMAGE_TAR_FILE && $AGENT_IMAGE_TAR_FILE != *.tar.gz ]]; then
        log_fatal 1 "AGENT_IMAGE_TAR_FILE must be in tar.gz format"
    fi

    if [[ -n $AGENT_K8S_IMAGE_TAR_FILE && $AGENT_K8S_IMAGE_TAR_FILE != *.tar.gz ]]; then
        log_fatal 1 "AGENT_K8S_IMAGE_TAR_FILE must be in tar.gz format"
    fi

    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]] && [[ -n $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE && $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE != *.tar.gz ]]; then
        log_fatal 1 "CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE must be in tar.gz format"
    fi
    log_debug "check_variables() begin"
}

#====================== General Functions ======================

# Check the exit code passed in and exit if non-zero
function chk() {
    local exitCode=$1
    local task=$2
    local dontExit=$3   # set to 'continue' to not exit for this error
    if [[ $exitCode == 0 ]]; then return; fi
    log_error "exit code $exitCode from: $task"
    if [[ $dontExit != 'continue' ]]; then
        exit $exitCode
    fi
}

# Check both the exit code and http code passed in and exit if not good
function chkHttp() {
    local exitCode=$1
    local httpCode=$2
    local goodHttpCodes=$3   # space or comma separate list of acceptable http codes
    local task=$4
    local outputFile=$5   # optional: the file that has the curl output in it (which sometimes has the error in it)
    local dontExit=$6   # optional: set to 'continue' to not exit for this error
    chk $exitCode $task
    if [[ -n $httpCode && $goodHttpCodes == *$httpCode* ]]; then return; fi
    # the httpCode was bad, normally in this case the api error msg is in the outputFile
    if [[ -n $outputFile && -s $outputFile ]]; then
        task="$task, stdout: $(cat $outputFile)"
    fi
    log_error "HTTP code $httpCode from: $task"
    if [[ $dontExit != 'continue' ]]; then
        if [[ ! "$httpCode" =~ ^[0-9]+$ ]]; then
            httpCode=5   # some times httpCode is the curl error msg
        fi
        exit $httpCode
    fi
}

# Run a command that does not have a good quiet option, so we have to capture the output and only show it if an error occurs
function runCmdQuietly() {
    # all of the args to this function are the cmd and its args
    set +e
    if [[ $AGENT_VERBOSITY -ge $VERB_VERBOSE ]]; then
        $*
        chk $? "running: $*"
    else
        output=$($* 2>&1)
        local rc=$?
        if [[ $rc -ne 0 ]]; then
            log_fatal $rc "Error running $*: $output"
        fi
    fi
    set -e
}

function is_macos() {
    if [[ $OS == 'macos' ]]; then return 0
    else return 1; fi
}

function is_linux() {
    if [[ $OS == 'linux' ]]; then return 0
    else return 1; fi
}

function is_device() {
    if [[ $AGENT_DEPLOY_TYPE == 'device' ]]; then return 0
    else return 1; fi
}

function is_cluster() {
    if [[ $AGENT_DEPLOY_TYPE == 'cluster' ]]; then return 0
    else return 1; fi
}

function is_small_kube() {
    if $KUBECTL cluster-info | grep -q -E 'Kubernetes .* is running at .*//(127|172|10|192.168)\.'; then return 0
    else return 1; fi
}

function is_ocp_cluster() {
    $KUBECTL get console -n openshift-console >/dev/null 2>&1
    if [[ $? -ne 0 ]]; then return 1 # we couldn't get the default route in openshift-image-registry namespace, so the current cluster is not ocp
    else return 0; fi
}

function is_anax_in_container() {
    if [[ $AGENT_IN_CONTAINER == 'true' ]]; then return 0
    else return 1; fi
}

# return kubernetes server version in format of $major:$minor
function get_kubernetes_version() {
    local major_version
    local minor_version
    local full_version
    major_version=$($KUBECTL version -o json | jq '.serverVersion.major' | sed s/\"//g)
    minor_version=$($KUBECTL version -o json | jq '.serverVersion.minor' | sed s/\"//g)
    if [ "${minor_version:0-1}" == "+" ]; then
        minor_version=${minor_version::-1}
    fi

    full_version="$major_version.$minor_version"
    echo $full_version
}

# compare versions. Return 0 (true) if the 1st version is greater than or equal to the 2nd version
function version_gt_or_equal() {
    local version1=$1
    local version2=$2

    # need the test above, because the test below returns >= because it sorts in ascending order
    test "$(printf '%s\n' "$1" "$2" | sort -V | tail -n 1)" == "$version1"
}

function get_agent_container_number() {
    if [[ $AGENT_IN_CONTAINER == 'true' ]] || is_macos; then
        echo "$AGENT_CONTAINER_NUMBER"
    else
        echo "0"
    fi
}

# Trim leading and trailing whitespace from a variable and return the trimmed value
function trim_variable() {
    local var="$1"
    echo "$var" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

function isDockerContainerRunning() {
    local container="$1"
    if [[ -n $(${DOCKER_ENGINE} ps -q --filter name=$container) ]]; then
        return 0
    else
        return 1
    fi
}

# Returns exit code 0 if the specified cmd is in the path
function isCmdInstalled() {
    local cmd=$1
    command -v $cmd >/dev/null 2>&1
}

# Returns exit code 0 if all of the specified cmds are in the path
function areCmdsInstalled() {
    for c in $*; do
        if ! isCmdInstalled $c; then
            return 1
        fi
    done
    return 0
}

# Verify that the prereq commands we need are installed, or exit with error msg
function confirmCmds() {
    for c in $*; do
        #echo "checking $c..."
        if ! isCmdInstalled $c; then
            log_fatal 2 "$c is not installed but is required"
        fi
    done
}

function ensureWeAreRoot() {
    if [[ $(whoami) != 'root' ]]; then
        log_fatal 2 "must be root to run ${0##*/}. Run 'sudo -sE' and then run ${0##*/}"
    fi
    # or could check: [[ $(id -u) -ne 0 ]]
}

# compare versions.
# Return 0 if the 1st version is equal to the 2nd version
#        1 if the 1st version is greater than the 2nd version
#        2 if the 1st version is less than the 2nd version
function compare_version() {
    local version1=$1
    local version2=$2

    if [[ $version1 == $version2 ]]; then
        return 0
    fi

    local gtval=$(printf '%s\n' "$version1" "$version2" | sort -V | tail -n 1)
    if [ "$gtval" == "$version1" ]; then
        return 1
    else
        return 2
    fi
}

# Return 0 (true) if we are getting this input file from a remote location
function using_remote_input_files() {
    local whichFile=${1:-pkg}   # (optional) Can be: pkg, crt, cfg, yml, uninstall
    if [[ $whichFile == 'crt' ]]; then
        # This file is specific to the instance of the cluster, so only available in CSS
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            return 0
        fi
    elif [[ $whichFile == 'cfg' ]]; then
        # This file is specific to the instance of the cluster, so only available in CSS
        if [[ $INPUT_FILE_PATH == css:* || $AGENT_CFG_FILE == css:* ]]; then
            return 0
        fi
    elif [[ $whichFile == 'pkg' ]]; then
        if [[ $INPUT_FILE_PATH == css:* || $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* || $INPUT_FILE_PATH == remote:* ]]; then
            return 0
        fi
    else   # the other files (yml, uninstall) are available from either
        if [[ $INPUT_FILE_PATH == css:* || $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* || $INPUT_FILE_PATH == remote:* ]]; then
            return 0
        fi
    fi
    return 1
}

# Get the packages from where INPUT_FILE_PATH indicates
# Side-effect: sets PACKAGES
function get_pkgs() {
    log_debug "get_pkgs() begin"

    local input_file_path=$INPUT_FILE_PATH
    if [[ -n $PKG_APT_REPO ]]; then return; fi   # using an APT repo, so we don't deal with local pkgs

    # Download the pkgs, if necessary
    local local_input_file_path='.'   # the place we put the pkgs on this host
    if [[ $INPUT_FILE_PATH == css:* ]]; then
        download_pkgs_from_css
    elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
        download_pkgs_from_anax_release
    else
        local_input_file_path=$INPUT_FILE_PATH   # assume INPUT_FILE_PATH is a local directory and we already have the pkgs there
    fi

    # Ensure we have the pkgs we need
    if is_macos; then
        if ! ls $local_input_file_path/horizon-cli-*.pkg 1>/dev/null 2>&1 || [[ ! -f $local_input_file_path/$MAC_PACKAGE_CERT ]]; then
            log_fatal 2 "Horizon macos packages not found in: $local_input_file_path"
        fi
    elif is_debian_variant && ! ls $local_input_file_path/horizon*_${ARCH}.deb 1>/dev/null 2>&1; then
        log_fatal 2 "Horizon deb packages not found in: $local_input_file_path"
    elif is_redhat_variant && ! ls $local_input_file_path/horizon*.${ARCH}.rpm 1>/dev/null 2>&1; then
        log_fatal 2 "Horizon rpm packages not found in: $local_input_file_path"
    fi
    PACKAGES="$local_input_file_path"

    log_debug "get_pkgs() end"
}

# Download the packages tar file from the anax git repo releases section
function download_pkgs_from_anax_release() {
    log_debug "download_pkgs_from_anax_release() begin"
    # This function is called if INPUT_FILE_PATH starts with at least https://github.com/open-horizon/anax/releases
    # Note: adjust_input_file_path() has already been called, which applies some default (if necessary) to INPUT_FILE_PATH
    local tar_file_name="horizon-agent-${OS}-$(get_pkg_type)-${ARCH}.tar.gz"
    local remote_path="${INPUT_FILE_PATH%/}/$tar_file_name"

    # Download and unpack the package tar file
    log_info "Downloading and unpacking package tar file $remote_path ..."
    download_with_retry $remote_path $tar_file_name
    log_verbose "Download of $remote_path successful, now unpacking it..."
    rm -f horizon*.$(get_pkg_type)   # remove older pkgs so there is no confusion about what is being installed
    tar -zxf $tar_file_name
    rm $tar_file_name   # do not need to leave this around

    log_debug "download_pkgs_from_anax_release() end"
}

# Download the packages tar file from CSS
# Side-effect: changes INPUT_FILE_PATH to '.' after downloading the pkgs to there
function download_pkgs_from_css() {
    log_debug "download_pkgs_from_css() begin"
    # This function is called if INPUT_FILE_PATH starts with css: . We have to add in HZN_FSS_CSSURL to the URL we download from.
    # Note: adjust_input_file_path() has already been called, which applies some default (if necessary) to INPUT_FILE_PATH
    local tar_file_name="horizon-agent-${OS}-$(get_pkg_type)-${ARCH}.tar.gz"

    # Download and unpack the package tar file
    local input_path
    get_input_file_css_path input_path
    download_css_file "$input_path/$tar_file_name"
    log_verbose "Download of $INPUT_FILE_PATH successful, now unpacking it..."
    rm -f horizon*.$(get_pkg_type)   # remove older pkgs so there is no confusion about what is being installed
    tar -zxf $tar_file_name
    rm $tar_file_name   # do not need to leave this around

    log_debug "download_pkgs_from_css() end"
}

# Move the given cert file to a permanent place the cfg file can refer to it. Returns the permanent path. Returns "" if no cert file.
# Used for both linux and mac
function store_cert_file_permanently() {
    # Note: can not put debug statements in this function because it returns a value
    local cert_file=$1
    local abs_certificate
    if [[ ${cert_file:0:1} == "/" && -f $cert_file ]]; then
        # Cert file is already a full path, just refer to it there
        abs_certificate=$cert_file
    elif [[ -n $cert_file && -f $cert_file ]]; then
        # Cert file specified, but relative path. Move it to a permanent place
        abs_certificate="$PERMANENT_CERT_PATH"
        sudo mkdir -p "$(dirname $abs_certificate)"
        chk $? "creating $(dirname $abs_certificate)"
        sudo cp "$cert_file" "$abs_certificate"
        chk $? "moving cert file to $abs_certificate"
    fi
    # else abs_certificate will be empty
    echo "$abs_certificate"
}

# check if the given urls are same. The last / will be trimmed off
# for comparision
function urlEquals() {
    url1=${1%/}
    url2=${2%/}
    if [[ $url1 != $url2 ]]; then
        return 1
    fi
}

# For both device and cluster: Returns true (0) if /etc/default/horizon already has these values
# Side-effects: sets MGMT_HUB_CERT_CHANGED to true or false
function is_horizon_defaults_correct() {
    log_debug "is_horizon_defaults_correct() begin"

    IS_HORIZON_DEFAULTS_CORRECT='false'
    MGMT_HUB_CERT_CHANGED='false'

    local anax_port=$1   # optional
    local cert_file defaults_file

    if is_device; then
        defaults_file=$INSTALLED_AGENT_CFG_FILE
        #if [[ ${AGENT_CERT_FILE:0:1} == '/' && -f $AGENT_CERT_FILE ]]; then
        if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
            cert_file=$AGENT_CERT_FILE   # is_horizon_defaults_correct is called before store_cert_file_permanently, so use the cert file the user specified
        #elif [[ -f $PERMANENT_CERT_PATH ]]; then
            #cert_file=$PERMANENT_CERT_PATH
        # else leave cert_file empty
        fi
    else   # cluster
        if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
            cert_file="/etc/default/cert/$(basename $AGENT_CERT_FILE)"   # this is the name we will give it later when we create the defaults file
        fi   # else leave cert_file empty

        # Have to get the previous defaults from the configmap
        defaults_file="$HZN_ENV_FILE.previous"
        #echo "$KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} -o jsonpath={.data.horizon} 2>/dev/null > $defaults_file"
        $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} -o jsonpath={.data.horizon} 2>/dev/null > "$defaults_file"
        if [[ $? -ne 0 ]]; then
            return 1   # we couldn't get the configmap, so the current defaults are not correct
        fi
    fi

    local horizon_defaults_value
    # FYI, these variables are currently supported in the defaults file: HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_AGBOT_URL, HZN_SDO_SVC_URL (or HZN_FDO_SVC_URL), HZN_DEVICE_ID, HZN_MGMT_HUB_CERT_PATH, HZN_AGENT_PORT, HZN_VAR_BASE, HZN_NO_DYNAMIC_POLL, HZN_MGMT_HUB_CERT_PATH, HZN_ICP_CA_CERT_PATH (deprecated), CMTN_SERVICEOVERRIDE

    # Note: the '|| true' is so not finding the strings won't cause set -e to exit the script
    horizon_defaults_value=$(grep -E '^HZN_EXCHANGE_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if ! urlEquals $horizon_defaults_value $HZN_EXCHANGE_URL; then
        log_info "HZN_EXCHANGE_URL value changed, return"
        return 1
    fi

    horizon_defaults_value=$(grep -E '^HZN_FSS_CSSURL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if ! urlEquals $horizon_defaults_value $HZN_FSS_CSSURL; then
        log_info "HZN_FSS_CSSURL value changed, return"
        return 1
    fi

    # even if HZN_AGBOT_URL is empty in this script, still verify the defaults file is the same
    horizon_defaults_value=$(grep -E '^HZN_AGBOT_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if ! urlEquals $horizon_defaults_value $HZN_AGBOT_URL; then
        log_info "HZN_AGBOT_URL value changed, return"
        return 1
    fi

    # even if HZN_SDO_SVC_URL is empty in this script, still verify the defaults file is the same
    horizon_defaults_value=$(grep -E '^HZN_SDO_SVC_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if ! urlEquals $horizon_defaults_value $HZN_SDO_SVC_URL; then
        log_info "HZN_SDO_SVC_URL value changed, return"
        return 1
    fi

    # even if HZN_FDO_SVC_URL is empty in this script, still verify the defaults file is the same
    horizon_defaults_value=$(grep -E '^HZN_FDO_SVC_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if ! urlEquals $horizon_defaults_value $HZN_FDO_SVC_URL; then
        log_info "HZN_FDO_SVC_URL value changed, return"
        return 1
    fi

    horizon_defaults_value=$(grep -E '^HZN_DEVICE_ID=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if [[ $horizon_defaults_value != $NODE_ID ]]; then
        log_info "HZN_DEVICE_ID value change, return"
        return 1
    fi

    horizon_defaults_value=$(grep -E '^HZN_NODE_ID=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if [[ $horizon_defaults_value != $NODE_ID ]]; then
        log_info "HZN_NODE_ID value changed, return"
        return 1
    fi

    if is_cluster; then
        horizon_defaults_value=$(grep -E '^AGENT_CLUSTER_IMAGE_REGISTRY_HOST=' $defaults_file || true)
        horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
        if [[ -n $horizon_defaults_value ]] && ! urlEquals $horizon_defaults_value $EDGE_CLUSTER_REGISTRY_HOST; then
            log_info "AGENT_CLUSTER_IMAGE_REGISTRY_HOST value changed, return"
            return 1
        fi
    fi

    if [[ -n $cert_file ]]; then
        horizon_defaults_value=$(grep -E '^HZN_MGMT_HUB_CERT_PATH=' $defaults_file || true)
        horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
        if [[ -n $horizon_defaults_value ]] && ! diff -q "$horizon_defaults_value" "$cert_file" >/dev/null; then
            log_info "cert file changed, return"
            MGMT_HUB_CERT_CHANGED='true'
            return 1
        fi   # diff is tolerant of the 2 file names being the same
    fi

    if [[ -n $anax_port ]]; then
        horizon_defaults_value=$(grep -E '^HZN_AGENT_PORT=' $defaults_file || true)
        horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
        if [[ $horizon_defaults_value != $anax_port ]]; then
            log_info "HZN_AGENT_PORT changed, return"
            return 1
        fi
    fi

    if [[ -n $HZN_CONFIG_VERSION ]]; then
        horizon_defaults_value=$(grep -E '^HZN_CONFIG_VERSION=' $defaults_file || true)
        horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
        if [[ $horizon_defaults_value != $HZN_CONFIG_VERSION ]]; then
            log_info "HZN_CONFIG_VERSION changed, return"
            return 1
        fi
    fi

    log_verbose "set IS_HORIZON_DEFAULTS_CORRECT to true and return"
    IS_HORIZON_DEFAULTS_CORRECT='true'
    log_debug "is_horizon_defaults_correct() end"
    return 0
}

# If a variable already exists in /etc/default/horizon, update its value. Otherwise add it to the file.
function add_to_or_update_horizon_defaults() {
    log_debug "add_to_or_update_horizon_defaults() begin"
    local variable=$1 value=$2 filename=$3;
    # Note: we have to use sudo to update the file in case we are on macos, where usually they aren't root, and the defaults file is usually owned by root.

    # First ensure the last line of the file has a newline at the end (or the appends below will have a problem)
    # This tail cmd gets the last char, but cmd substitution deletes any newline, so its value is empty of the last char was a newline.
    if [[ ! -z $(tail -c 1 "$filename") ]]; then
        sudo sh -c "echo >> '$filename'"   # file did not end in newline, so add one
    fi

    # Now update or add the variable
    if grep -q "^$variable=" "$filename"; then
        sudo sed -i.bak "s%^$variable=.*%$variable=$value%g" "$filename"
    else
        sudo sh -c "echo '$variable=$value' >> '$filename'"
    fi
    # this way of doing it in a single line is just kept for reference:
    # grep -q "^$variable=" "$filename" && sudo sed -i.bak "s/^$variable=.*/$variable=$value/" "$filename" || sudo sh -c "echo '$variable=$value' >> '$filename'"
    log_debug "add_to_or_update_horizon_defaults() end"
}

# Create or update /etc/default/horizon file for mac or linux
# Side-effect: sets HORIZON_DEFAULTS_CHANGED to true or false
#              sets MGMT_HUB_CERT_CHANGED to true or false
function create_or_update_horizon_defaults() {
    log_debug "create_or_update_horizon_defaults() begin"
    local anax_port=$1   # optional
    local abs_certificate   # can't call store_cert_file_permanently yet because that could cause is_horizon_defaults_correct
    log_verbose "Permanent location of certificate file: $abs_certificate"

    local defaults_file=$INSTALLED_AGENT_CFG_FILE

    if [[ ! -f $defaults_file ]]; then
        log_info "Creating $defaults_file ..."
        sudo mkdir -p /etc/default
        sudo bash -c "echo -e 'HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}\nHZN_FSS_CSSURL=${HZN_FSS_CSSURL}\nHZN_DEVICE_ID=${NODE_ID}\nHZN_NODE_ID=${NODE_ID}' >$defaults_file"
        if [[ -n $HZN_AGBOT_URL ]]; then
            sudo sh -c "echo 'HZN_AGBOT_URL=$HZN_AGBOT_URL' >> $defaults_file"
        fi
        if [[ -n $HZN_SDO_SVC_URL ]]; then
            sudo sh -c "echo 'HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL' >> $defaults_file"
        fi
        if [[ -n $HZN_FDO_SVC_URL ]]; then
            sudo sh -c "echo 'HZN_FDO_SVC_URL=$HZN_FDO_SVC_URL' >> $defaults_file"
        fi
        abs_certificate=$(store_cert_file_permanently "$AGENT_CERT_FILE")   # can return empty string
        if [[ -n $abs_certificate ]]; then
            sudo sh -c "echo 'HZN_MGMT_HUB_CERT_PATH=$abs_certificate' >> $defaults_file"
        fi
        if [[ -n $anax_port ]]; then
            sudo sh -c "echo 'HZN_AGENT_PORT=$anax_port' >> $defaults_file"
        fi
        if [[ -n $HZN_CONFIG_VERSION ]]; then
            sudo sh -c "echo 'HZN_CONFIG_VERSION=$HZN_CONFIG_VERSION' >> $defaults_file"
        fi
        HORIZON_DEFAULTS_CHANGED='true'
        MGMT_HUB_CERT_CHANGED='true'
    # is_horizon_defaults_correct function sets MGMT_HUB_CERT_CHANGED to true or false
    elif is_horizon_defaults_correct "$anax_port"; then
        log_info "$defaults_file already has the correct values. Not modifying it."
        HORIZON_DEFAULTS_CHANGED='false'
    else
        # File already exists, but isn't correct. Update it (do not overwrite it in case they have other variables in there they want preserved)
        log_info "Updating $defaults_file ..."
        add_to_or_update_horizon_defaults 'HZN_EXCHANGE_URL' "$HZN_EXCHANGE_URL" $defaults_file
        add_to_or_update_horizon_defaults 'HZN_FSS_CSSURL' "$HZN_FSS_CSSURL" $defaults_file
        if [[ -n $HZN_AGBOT_URL ]]; then
            add_to_or_update_horizon_defaults 'HZN_AGBOT_URL' "$HZN_AGBOT_URL" $defaults_file
        fi
        if [[ -n $HZN_SDO_SVC_URL ]]; then
            add_to_or_update_horizon_defaults 'HZN_SDO_SVC_URL' "$HZN_SDO_SVC_URL" $defaults_file
        fi
        if [[ -n $HZN_FDO_SVC_URL ]]; then
            add_to_or_update_horizon_defaults 'HZN_FDO_SVC_URL' "$HZN_FDO_SVC_URL" $defaults_file
        fi
        add_to_or_update_horizon_defaults 'HZN_DEVICE_ID' "$NODE_ID" $defaults_file
        add_to_or_update_horizon_defaults 'HZN_NODE_ID' "$NODE_ID" $defaults_file
        abs_certificate=$(store_cert_file_permanently "$AGENT_CERT_FILE")   # can return empty string
        if [[ -n $abs_certificate ]]; then
            add_to_or_update_horizon_defaults 'HZN_MGMT_HUB_CERT_PATH' "$abs_certificate" $defaults_file
        fi
        if [[ -n $anax_port ]]; then
            add_to_or_update_horizon_defaults 'HZN_AGENT_PORT' "$anax_port" $defaults_file
        fi
        if [[ -n $HZN_CONFIG_VERSION ]]; then
            add_to_or_update_horizon_defaults 'HZN_CONFIG_VERSION' "$HZN_CONFIG_VERSION" $defaults_file
        fi
        HORIZON_DEFAULTS_CHANGED='true'
    fi
    log_debug "create_or_update_horizon_defaults() end"
}

# Have macos trust the horizon-cli pkg cert and the Horizon mgmt hub self-signed cert
function mac_trust_cert() {
    log_debug "mac_trust_cert() begin"
    local cert_file=$1 cert_file_type=$2
    log_info "Importing $cert_file_type into Mac OS keychain..."

    local tmp_dir=$(get_tmp_dir)
    local tmp_file="${tmp_dir}/rights"

    set -x   # echo'ing this cmd because on mac it is usually the 1st sudo cmd and want them to know why they are being prompted for pw

    # save the old permission
    sudo security authorizationdb read com.apple.trust-settings.admin > $tmp_file

    # set the permission to 'allow' to bypass the prompt
    sudo security authorizationdb write com.apple.trust-settings.admin allow

    # add the cerfiticate
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$cert_file"

    # restore the old permission
    sudo security authorizationdb write com.apple.trust-settings.admin < $tmp_file
    rm -f $tmp_file

    { set +x; } 2>/dev/null

    log_debug "mac_trust_cert() end"
}

function install_macos() {
    log_debug "install_macos() begin"

    # set up HORIZON_URL env variable for hzn calls for container case
    local container_num=$(get_agent_container_number)
    local agentPort=$(expr $AGENT_CONTAINER_PORT_BASE + $container_num)
    export HORIZON_URL=http://localhost:${agentPort}

    if has_upgrade_type_sw; then
        if ! isCmdInstalled jq && isCmdInstalled brew; then
            echo "Jq is required, installing it using brew, this could take a minute..."
            runCmdQuietly brew install jq
        fi

        if [[ $AGENT_ONLY_CLI != 'true' ]]; then
            confirmCmds socat docker jq
        fi

        if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
            check_existing_exch_node_is_correct_type "device"
        fi

        if is_agent_registered && (! is_horizon_defaults_correct || ! is_registration_correct); then
            if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
                unregister
            fi
        fi
    fi

    if has_upgrade_type_cert; then
        get_certificate
    fi

    check_exch_url_and_cert

    create_or_update_horizon_defaults

    if [[ has_upgrade_type_cert && $MGMT_HUB_CERT_CHANGED == 'true' ]]; then
        # have macos trust the Horizon mgmt hub self-signed cert
        mac_trust_cert "$AGENT_CERT_FILE" "the management hub certificate"
    fi

    if has_upgrade_type_sw; then
        get_pkgs
        install_mac_horizon-cli
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        start_device_agent_container  $container_num # even if it already running, it restarts it

        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"
    fi

    log_debug "install_macos() end"
}

# Determine what the agent port number should be, verify it is free, and set ANAX_PORT.
# Side-effect: sets ANAX_PORT
function check_and_set_anax_port() {
    log_debug "check_and_set_anax_port() begin"
    local anax_port=$ANAX_DEFAULT_PORT
    if [[ -n $HZN_AGENT_PORT ]]; then
        anax_port=$HZN_AGENT_PORT
    elif [[ -f $INSTALLED_AGENT_CFG_FILE ]]; then
        log_verbose "Trying to get agent port from previous $INSTALLED_AGENT_CFG_FILE file..."
        local prevPort=$(grep HZN_AGENT_PORT $INSTALLED_AGENT_CFG_FILE | cut -d'=' -f2)
        if [[ -n $prevPort ]]; then
            log_verbose "Found agent port in previous $INSTALLED_AGENT_CFG_FILE: $prevPort"
            anax_port=$prevPort
        fi
    fi

    log_verbose "Checking if the agent port ${anax_port} is free..."
    local netStat=$(netstat -nlp | grep tcp | grep $anax_port || true)
    if is_anax_in_container; then
        netStat=$(${DOCKER_ENGINE} ps --filter "publish=$anax_port")
    fi
    if [[ $netStat == *$anax_port* && ! $netStat == *anax* ]]; then
        log_fatal 2 "Another process is listening on Horizon agent port $anax_port. Free the port in order to install Horizon"
    fi
    ANAX_PORT=$anax_port
    log_verbose "Anax port $ANAX_PORT is free, continuing..."
    log_debug "check_and_set_anax_port() end"
}

# Ensure the software prereqs for a debian variant device are installed
function debian_device_install_prereqs() {
    log_debug "debian_device_install_prereqs() begin"

    log_info "Updating apt package index..."
    runCmdQuietly apt-get update -q

    log_info "Installing prerequisites, this could take a minute..."

    if [[ $AGENT_ONLY_CLI == 'true' ]]; then
        runCmdQuietly apt-get install -yqf curl jq cron
    else
        runCmdQuietly apt-get install -yqf curl jq software-properties-common net-tools cron

        if ! isCmdInstalled docker; then
            log_info "Docker is required, installing it..."
            if [ "${DISTRO}" == "raspbian" ]; then
                curl -sSL get.docker.com | sh
            else
                curl -fsSL https://download.docker.com/linux/$DISTRO/gpg | apt-key add -
                add-apt-repository -y "deb [arch=$(dpkg --print-architecture)] https://download.docker.com/linux/$DISTRO $(lsb_release -cs) stable"
                runCmdQuietly apt-get update -q
                runCmdQuietly apt-get install -yqf docker-ce docker-ce-cli containerd.io
            fi
        fi
    fi

    # start cron service if not, cron is used by the agent auto upgrade process
    systemctl start cron

    log_debug "debian_device_install_prereqs() end"
}

# Returns 0 if the deb pkg to be installed is equal to the corresponding pkg already installed
#         1 if the deb pkg to be installed is newer than the corresponding pkg already installed
#         2 if the deb pkg to be installed is older than the corresponding pkg already installed
function compare_deb_pkg() {
    local __local_modified=$1
    local latest_deb_file=${2:?}   # make the decision based on the deb pkg they passed in

    # latest_deb_file is something like /dir/horizon_2.28.0-338_amd64.deb
    local deb_file_version=${latest_deb_file##*/}   # remove the beginning path
    deb_file_version=${deb_file_version%_*.deb}   # remove the ending like _amd64.deb, now left with horizon_2.28.0-338
    if [[ $deb_file_version == horizon-cli* ]]; then
        deb_file_version=${deb_file_version#horizon-cli_}   # remove the pkg name to be left with only the version
        pkg_name='horizon-cli'
    else   # assume it is the horizon pkg
        deb_file_version=${deb_file_version#horizon_}   # remove the pkg name to be left with only the version
        pkg_name='horizon'
    fi

    # Get version of installed horizon deb pkg
    if [[ $(dpkg-query -s $pkg_name 2>/dev/null | grep -E '^Status:' | awk '{print $4}') != 'installed' ]]; then
        log_verbose "The $pkg_name deb pkg is not installed (at least not completely)"
        return 1   # anything is newer than not installed
    fi
    local installed_deb_version=$(dpkg-query -s $pkg_name | grep -E '^Version:' | awk '{print $2}')

    # Get version from binary first if possible
    local local_version=""
    get_local_horizon_version local_agent_ver local_cli_ver
    if [[ $pkg_name == 'horizon' && -n $local_agent_ver ]]; then
        local_version=$local_agent_ver
    elif [[ $pkg_name == 'horizon-cli' && -n $local_cli_ver ]]; then
        local_version=$local_cli_ver
    fi
    log_info "Local agent version is: $local_agent_ver; local cli version is: $local_cli_ver."

    # if the local binary has been manually changed by user or by the agent auto upgrade rollback process
    # and the binary version does not agree with the version in the repository
    local local_ver_modified='false'
    if [[ -n "$local_version" && $local_version != $installed_deb_version ]]; then
        local_ver_modified='true'
    fi
    eval $__local_modified="'$local_ver_modified'"

    log_info "Installed $pkg_name deb package version: $installed_deb_version, Provided $pkg_name deb file version: $deb_file_version"
    local rc=0
    compare_version $deb_file_version $installed_deb_version || rc=$?
    return $rc
}

# Install the deb pkgs on a device
# Side-effect: sets AGENT_WAS_RESTARTED to 'true' or 'false'
function install_debian_device_horizon_pkgs() {
    log_debug "install_debian_device_horizon_pkgs() begin"

    AGENT_WAS_RESTARTED='false'   # only set to true when we are sure
    if [[ -n "$PKG_APT_REPO" ]]; then
        log_info "Installing horizon via the APT repository $PKG_APT_REPO ..."
        if [[ -n "$PKG_APT_KEY" ]]; then
            log_verbose "Adding key $PKG_APT_KEY for APT repository $PKG_APT_REPO"
            apt-key add "$PKG_APT_KEY"
        fi
        log_verbose "Adding $PKG_APT_REPO to /etc/apt/sources.list and installing horizon ..."
        add-apt-repository -y "deb [arch=$(dpkg --print-architecture)] $PKG_APT_REPO $(lsb_release -cs)-$APT_REPO_BRANCH main"
        if [[ $AGENT_ONLY_CLI == 'true' || $AGENT_IN_CONTAINER == 'true' ]]; then
            runCmdQuietly apt-get install -yqf horizon-cli
        else
            runCmdQuietly apt-get install -yqf horizon
            wait_until_agent_ready
        fi
    else
        # Install the horizon pkgs in the PACKAGES dir
        if [[ ${PACKAGES:0:1} != '/' ]]; then
            PACKAGES="$PWD/$PACKAGES"   # to install local pkg files with apt-get install, they must be absolute paths
        fi

        # Get the latest version of the horizon pkg files they gave us to install. sort -V returns the list in ascending order (greatest is last)
        local latest_horizon_cli_file latest_horizon_file latest_files new_pkg
        latest_horizon_cli_file=$(ls -1 $PACKAGES/horizon-cli_*_${ARCH}.deb | sort -V | tail -n 1)
        if [[ -z $latest_horizon_cli_file ]]; then
            log_warning "No horizon-cli deb package found in $PACKAGES"
            return 1
        fi
        latest_files=$latest_horizon_cli_file
        new_pkg=$latest_horizon_cli_file
        if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
            latest_horizon_file=$(ls -1 $PACKAGES/horizon_*_${ARCH}.deb | sort -V | tail -n 1)
            if [[ -z $latest_horizon_file ]]; then
                log_warning "No horizon deb package found in $PACKAGES"
                return 1
            fi
            latest_files="$latest_files $latest_horizon_file"
            new_pkg=$latest_horizon_file
        fi

        local rc=0
        local local_modified=''
        compare_deb_pkg local_modified $new_pkg || rc=$?
        if [ $rc -eq 1 ]; then
            log_info "Installing $latest_files ..."
            runCmdQuietly apt-get install -yqf $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        elif [ $rc -eq 2 ] && [[ "$AGENT_OVERWRITE" == true ]]; then
            log_info "Downgrading to $latest_files because AGENT_OVERWRITE==$AGENT_OVERWRITE ..."
            runCmdQuietly apt-get install -yqf --allow-downgrades $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        elif [[ $local_modified == 'true' ]]; then
            # though the new package and the installed package versions are the same, the local
            # binary has been changed either by user or by the rollback process from agent auto upgrade.
            # The package will be reinstalled
            log_info "Reinstalling $latest_files ..."
            runCmdQuietly apt-get install -yqf --reinstall $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        else
            log_info "Not installing any horizon packages, because the system is already up to date."
        fi
    fi
    log_debug "install_debian_device_horizon_pkgs() end"
}

# Restart the device agent and wait for it to be ready
function restart_device_agent() {
    log_debug "restart_device_agent() begin"
    log_info "Restarting the horizon agent service because $INSTALLED_AGENT_CFG_FILE was modified..."
    systemctl restart horizon.service   # (restart will succeed even if the service was already stopped)
    wait_until_agent_ready
    log_debug "restart_device_agent() end"
}

# Install the agent and register it on a debian variant
function install_debian() {
    log_debug "install_debian() begin"

    if has_upgrade_type_sw; then
        debian_device_install_prereqs
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        check_and_set_anax_port   # sets ANAX_PORT

        if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
            check_existing_exch_node_is_correct_type "device"
        fi

        if is_agent_registered && (! is_horizon_defaults_correct "$ANAX_PORT" || ! is_registration_correct); then
            if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
                unregister
            fi
        fi
    fi

    if has_upgrade_type_cert; then
        get_certificate
    fi

    check_exch_url_and_cert

    create_or_update_horizon_defaults "$ANAX_PORT"

    if has_upgrade_type_sw; then
        get_pkgs

        # Note: the horizon pkg will only write /etc/default/horizon if it doesn't exist, so it won't overwrite what we created/modified above
        install_debian_device_horizon_pkgs
    fi

    set_horizon_url

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        if [[ $HORIZON_DEFAULTS_CHANGED == 'true' && $AGENT_WAS_RESTARTED != 'true' ]]; then
            restart_device_agent   # because the new pkgs were not installed, so that didn't restart the agent
        fi

        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"
    fi

    log_debug "install_debian() end"
}

# Install the agent-in-container and register it on a debian variant
function install_debian_container() {
    log_debug "install_debian_container() begin"

    # set up HORIZON_URL env variable for hzn calls for container case
    local container_num=$(get_agent_container_number)
    local agentPort=$(expr $AGENT_CONTAINER_PORT_BASE + $container_num)
    export HORIZON_URL=http://localhost:${agentPort}

    if has_upgrade_type_sw; then
        debian_device_install_prereqs
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        confirmCmds ${DOCKER_ENGINE} jq
        if is_agent_registered && (! is_horizon_defaults_correct || ! is_registration_correct); then
            if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
                unregister
            fi
        fi
    fi

    if has_upgrade_type_cert; then
        get_certificate
    fi

    check_exch_url_and_cert

    create_or_update_horizon_defaults

    if has_upgrade_type_sw; then
        get_pkgs

        # Note: the horizon pkg will only write /etc/default/horizon if it doesn't exist, so it won't overwrite what we created/modified above
        install_debian_device_horizon_pkgs
        set_horizon_url
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        start_device_agent_container  $container_num # even if it already running, it restarts it
        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"
    fi

    log_debug "install_debian_container() end"
}

# Ensure dnf package manager is installed on host when running in RedHat/CentOS
function install_dnf() {
    log_debug "install_dnf() begin"

    log_info "Checking dnf is installed..."

    if ! isCmdInstalled dnf; then
        log_info "Installing dnf..."
        yum install -y -q dnf
    fi

    log_debug "install_dnf() end"
}

# Ensure the software prereqs for a redhat variant device are installed
function redhat_device_install_prereqs() {
    log_debug "redhat_device_install_prereqs() begin"

    dnf install -yq jq
    rc=$?
    if [[ $rc -ne 0 ]]; then
        # Need EPEL might be needed for jq
        if [[ -z "$(dnf repolist -q epel)" ]]; then
           dnf install -yq https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
        fi
    fi

    # Curl likely already on a RHEL system so only try to install it if missing - caused a problem for RHEL 9.2
    if ! isCmdInstalled curl; then
        dnf install -yq curl
    fi
    dnf install -yq jq cronie

    # cron will be used for agent auto upgrade process
    systemctl start crond

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        dnf install -yq net-tools
        if ! isCmdInstalled ${DOCKER_ENGINE}; then
            # Can't install docker for them on red hat, because they make it difficult. See: https://linuxconfig.org/how-to-install-docker-in-rhel-8
            log_fatal 2 "${DOCKER_ENGINE} is required, but not installed. Install it and rerun this script."
        fi
    fi
    log_debug "redhat_device_install_prereqs() end"
}

# Returns 0 if the rpm pkg to be installed is equal to the corresponding pkg already installed
#         1 if the rpm pkg to be installed is newer than the corresponding pkg already installed
#         2 if the rpm pkg to be installed is older than the corresponding pkg already installed
function compare_rpm_pkg() {
    local __local_modified=$1
    local latest_rpm_file=${2:?}   # make the decision based on the rpm pkg they passed in

    # latest_rpm_file is something like /dir/horizon-2.28.0-338.x86_64.rpm
    local rpm_file_version=${latest_rpm_file##*/}   # remove the beginning path
    rpm_file_version=${rpm_file_version%.*.rpm}   # remove the ending like .x86_64.rpm, now left with horizon-2.28.0-338
    if [[ $rpm_file_version == horizon-cli* ]]; then
        rpm_file_version=${rpm_file_version#horizon-cli-}   # remove the pkg name to be left with only the version
        pkg_name='horizon-cli'
    else   # assume it is the horizon pkg
        rpm_file_version=${rpm_file_version#horizon-}   # remove the pkg name to be left with only the version
        pkg_name='horizon'
    fi

    # Get version of installed horizon rpm pkg
    if ! rpm -q $pkg_name >/dev/null 2>&1; then
        log_verbose "The $pkg_name rpm pkg is not installed"
        return 1   # anything is newer than not installed
    fi
    local installed_rpm_version=$(rpm -q $pkg_name 2>/dev/null)  # will return like: horizon-2.28.0-338.x86_64
    installed_rpm_version=${installed_rpm_version#${pkg_name}-}   # remove the pkg name
    installed_rpm_version=${installed_rpm_version%.*}   # remove the arch, so left with only the version

    # Get version from binary first if possible
    local local_version=""
    get_local_horizon_version local_agent_ver local_cli_ver
    if [[ $pkg_name == 'horizon' && -n $local_agent_ver ]]; then
        local_version=$local_agent_ver
    elif [[ $pkg_name == 'horizon-cli' && -n $local_cli_ver ]]; then
        local_version=$local_cli_ver
    fi
    log_info "Local agent version is: $local_agent_ver; local cli version is: $local_cli_ver."

    # if the local binary has been manually changed by user or by the agent auto upgrade rollback process
    # and the binary version does not agree with the version in the repository
    local local_ver_modified='false'
    if [[ -n "$local_version" && $local_version != $installed_rpm_version ]]; then
        local_ver_modified='true'
    fi
    eval $__local_modified="'$local_ver_modified'"

    log_info "Installed $pkg_name rpm package version: $installed_rpm_version, Provided $pkg_name rpm file version: $rpm_file_version"
    local rc=0
    compare_version $rpm_file_version $installed_rpm_version || rc=$?
    return $rc
}

# Install the rpm pkgs on a redhat variant device
# Side-effect: sets AGENT_WAS_RESTARTED to 'true' or 'false'
function install_redhat_device_horizon_pkgs() {
    log_debug "install_redhat_device_horizon_pkgs() begin"
    AGENT_WAS_RESTARTED='false'   # only set to true when we are sure
    if [[ -n "$PKG_APT_REPO" ]]; then
        log_fatal 1 "Installing horizon RPMs via repository $PKG_APT_REPO is not supported at this time"
        #future: support this
    else
        # Install the horizon pkgs in the PACKAGES dir
        if [[ ${PACKAGES:0:1} != '/' ]]; then
            PACKAGES="$PWD/$PACKAGES"   # to install local pkg files with dnf install, they must be absolute paths
        fi

        # Get the latest version of the horizon pkg files they gave us to install. sort -V returns the list in ascending order (greatest is last)
        local latest_horizon_cli_file latest_horizon_file latest_files new_pkg
        latest_horizon_cli_file=$(ls -1 $PACKAGES/horizon-cli-*.${ARCH}.rpm | sort -V | tail -n 1)
        if [[ -z $latest_horizon_cli_file ]]; then
            log_warning "No horizon-cli rpm package found in $PACKAGES"
            return 1
        fi
        latest_files=$latest_horizon_cli_file
        new_pkg=$latest_horizon_cli_file
        if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
            latest_horizon_file=$(ls -1 $PACKAGES/horizon-*.${ARCH}.rpm | grep -v "$PACKAGES/horizon-cli" | sort -V | tail -n 1)
            if [[ -z $latest_horizon_file ]]; then
                log_warning "No horizon rpm package found in $PACKAGES"
                return 1
            fi
            latest_files="$latest_files $latest_horizon_file"
            new_pkg=$latest_horizon_file
        fi

        local rc=0
        local local_modified=''
        compare_rpm_pkg local_modified $new_pkg || rc=$?
        if [ $rc -eq 1 ]; then
            log_info "Installing $latest_files ..."
            dnf install -yq $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        elif [ $rc -eq 2 ] && [[ "$AGENT_OVERWRITE" == true ]]; then
            log_info "Downgrading to $latest_files because AGENT_OVERWRITE==$AGENT_OVERWRITE ..."
            # Note: dnf automatically detects the specified pkg files are a lower version and downgrades them. If we need to switch to yum, we'll have to use yum downgrade ...
            dnf install -yq $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        elif [[ $local_modified == 'true' ]]; then
            # though the new package and the installed package versions are the same, the local
            # binary has been changed either by user or by the rollback process from agent auto upgrade.
            # The package will be reinstalled
            log_info "Reinstalling $latest_files ..."
            dnf reinstall -yq $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' && $AGENT_IN_CONTAINER != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        else
            log_info "Not installing any horizon packages, because the system is already up to date."
        fi
    fi
    log_debug "install_redhat_device_horizon_pkgs() end"
}

# Install the agent and register it on a redhat variant
function install_redhat() {
    log_debug "install_redhat() begin"

    if has_upgrade_type_sw; then
        log_info "Installing prerequisites, this could take a minute..."
        install_dnf
        redhat_device_install_prereqs
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        check_and_set_anax_port   # sets ANAX_PORT
        if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
            check_existing_exch_node_is_correct_type "device"
        fi

        if is_agent_registered && (! is_horizon_defaults_correct "$ANAX_PORT" || ! is_registration_correct); then
            if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
                unregister
            fi
        fi
    fi

    if has_upgrade_type_cert; then
        get_certificate
    fi

    check_exch_url_and_cert

    create_or_update_horizon_defaults "$ANAX_PORT"

    if has_upgrade_type_sw; then
        get_pkgs
        # Note: the horizon pkg will only write /etc/default/horizon if it doesn't exist, so it won't overwrite what we created/modified above
        install_redhat_device_horizon_pkgs
    fi
    set_horizon_url

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        if [[ $HORIZON_DEFAULTS_CHANGED == 'true' && $AGENT_WAS_RESTARTED != 'true' ]]; then
            restart_device_agent   # because the new pkgs were not installed, so that didn't restart the agent
        fi

        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"
    fi

    log_debug "install_redhat() end"
}

# Install the agent-in-container and register it on a redhat variant
function install_redhat_container() {
    log_debug "install_redhat_container() begin"

    # set up HORIZON_URL env variable for hzn calls for container case
    local container_num=$(get_agent_container_number)
    local agentPort=$(expr $AGENT_CONTAINER_PORT_BASE + $container_num)
    export HORIZON_URL=http://localhost:${agentPort}

    if has_upgrade_type_sw; then
        log_info "Installing prerequisites, this could take a minute..."
        install_dnf
        redhat_device_install_prereqs
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        confirmCmds ${DOCKER_ENGINE} jq
        if is_agent_registered && (! is_horizon_defaults_correct || ! is_registration_correct); then
            if [[ $AGENT_AUTO_UPGRADE != 'true' ]]; then
                unregister
            fi
        fi
    fi

    if has_upgrade_type_cert; then
        get_certificate
    fi

    check_exch_url_and_cert

    create_or_update_horizon_defaults

    if has_upgrade_type_sw; then
        get_pkgs
        # Note: the horizon pkg will only write /etc/default/horizon if it doesn't exist, so it won't overwrite what we created/modified above
        install_redhat_device_horizon_pkgs
        set_horizon_url
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        start_device_agent_container $container_num  # even if it already running, it restarts it
        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"
    fi

    log_debug "install_redhat_container() end"
}

# Install the mac pkg that provides hzn, horizon-container, etc.
# Side-effect: based on the horizon-cli version being installed, it sets HC_DOCKER_TAG to use the same version of the agent container
function install_mac_horizon-cli() {
    log_debug "install_mac_horizon-cli() begin"

    # Get horizon-cli pkg file they gave us to install
    local pkg_file_name=$(ls -1 horizon-cli*.pkg | sort -V | tail -n 1)
    local pkg_file_version=${pkg_file_name#horizon-cli-}   # this removes the 1st part
    set +e
    ls  horizon-cli*.$ARCH.pkg
    if [[ $? -eq 0  ]]; then
        # pkg_file_name might be like horizon-cli-*.arm64.pkg
        pkg_file_version=${pkg_file_version%.$ARCH.pkg}   # remove the ending part
    else
        # or pkg_file_name might be like horizon-cli-*.kg
        pkg_file_version=${pkg_file_version%.pkg}   # remove the ending part
    fi
    set -e
    local pkg_file_version_only=${pkg_file_version%-*}
    local pkg_file_bld_num=${pkg_file_version##*-}
    log_verbose "The package version to be installed: ${pkg_file_version_only}, the build number: $pkg_file_bld_num"
    if [[ -z "$HC_DOCKER_TAG" ]]; then
        if [[ -z "$pkg_file_bld_num" ]]; then
            export HC_DOCKER_TAG="$pkg_file_version_only"
        else
            export HC_DOCKER_TAG="${pkg_file_version_only}-${pkg_file_bld_num}"
        fi
    fi

    log_verbose "Checking installed hzn version..."
    if isCmdInstalled hzn; then
        # hzn is installed, need to check the version
        local installed_version=$(HZN_LANG=en_US hzn version | grep "^Horizon CLI")
        installed_version=${installed_version##* }   # remove all of the space-separated words, except the last one
        log_info "Installed horizon-cli pkg version: $installed_version, Provided horizon pkg file version: $pkg_file_version"
        local rc=0
        compare_version "$pkg_file_version" "$installed_version" || rc=$?
        if [[ ! $installed_version =~ $SEMVER_REGEX ]] || [ $rc -eq 1 ]; then
            log_verbose "Either can not get the installed hzn version, or the given pkg file version is newer"

            # have macos trust the horizon-cli pkg cert
            mac_trust_cert "${PACKAGES}/${MAC_PACKAGE_CERT}" "the horizon-cli package certificate"

            log_info "Installing $PACKAGES/$pkg_file_name ..."
            sudo installer -pkg $PACKAGES/$pkg_file_name -target /
        elif [ $rc -eq 2 ] && [[ "$AGENT_OVERWRITE" == true ]]; then
            log_verbose "The given pkg file version is older than or equal to the installed hzn"

            # have macos trust the horizon-cli pkg cert
            mac_trust_cert "${PACKAGES}/${MAC_PACKAGE_CERT}" "the horizon-cli package certificate"

            log_info "Installing older horizon-cli package ${pkg_file_version} because AGENT_OVERWRITE is set to true ..."
            sudo installer -pkg ${PACKAGES}/$pkg_file_name -target /
        else
            log_info "The installed horizon-cli package is already up to date ($installed_version)"
        fi
    else
        # have macos trust the horizon-cli pkg cert
        mac_trust_cert "${PACKAGES}/${MAC_PACKAGE_CERT}" "the horizon-cli package certificate"

        # hzn not installed
        log_info "Installing $PACKAGES/$pkg_file_name ..."
        sudo installer -pkg ${PACKAGES}/$pkg_file_name -target /
    fi

    # Check if /usr/local/bin is in their path (that's where all the horizon cmds are installed)
    if ! echo $PATH | grep -q -E '(^|:)/usr/local/bin(:|$)'; then
        log_warning "The Horizon executables have been installed in /usr/local/bin, but you do not have that in your path. Add it to your path."
        export PATH="$PATH:/usr/local/bin"   # add it to the path for the rest of this script
    fi

    log_debug "install_mac_horizon-cli() end"
}

# Wait until the given cmd is true
function wait_for() {
    log_debug "wait_for() begin"
    : ${1:?} ${2:?} ${3:?}
    local cmd=$1   # the command (can contain bash syntax) that should return exit code 0 before this function returns
    local stateWaitingFor=$2   # a string describing the state we are waiting for, e.g.: "the Horizon agent container ready"
    local timeoutSecs=$3   # max number of seconds to wait before returning 1 (returns 0 if the state is reached in time)
    local intervalSleep=${4:-2}   # (optional) how long to sleep between each sleep
    log_info "Waiting for state: $stateWaitingFor " 'nonewline'
    local start_agent_check=$(date +%s)
    while ! eval $cmd; do
        local current_agent_check=$(date +%s)
        if ((current_agent_check - start_agent_check > timeoutSecs)); then
            echo
            return 1
        fi
        printf '.'
        sleep $intervalSleep
    done
    echo ''
    log_info "Done: $stateWaitingFor"
    log_debug "wait_for() end"
    return 0
}

# Set the HORIZON_URL variable in /etc/horizon/hzn.json
function set_horizon_url() {
    log_debug "set_horizon_url() begin"
    # not support cluster case
    if is_cluster; then
        return 0
    fi

    local anax_port=""
    local container_num=$(get_agent_container_number)
    if [[ $container_num == "0" ]]; then
        anax_port=$ANAX_PORT
    elif [[ $container_num == "1" ]]; then
        anax_port=8081
    else
        # do not change HORIZON_URL if agent-install.sh is called to upgrade
        # an agent container greater than horizon1.
        return 0
    fi

    if grep HORIZON_URL /etc/horizon/hzn.json; then
        sed -i "s/\"HORIZON_URL\":.*/\"HORIZON_URL\": \"http:\/\/localhost:${anax_port}\"/g" /etc/horizon/hzn.json
    else
        cliCfg=$(cat /etc/horizon/hzn.json | jq --arg url http://localhost:${anax_port} '. + {HORIZON_URL: $url}')
        echo "$cliCfg" > /etc/horizon/hzn.json
    fi
    log_debug "set_horizon_url() end"
}

# Wait until the agent is responding
function wait_until_agent_ready() {
    log_debug "wait_until_agent_ready() begin"

    local agent_port=""
    if is_linux && ! is_cluster && ! is_anax_in_container; then
        # for the native device agent, honor the HZN_AGENT_PORT from /etc/default/horizon
        # for other cases, HZN_AGENT_PORT change is not supported
        if [[ -n $HZN_AGENT_PORT ]]; then
            agent_port=$HZN_AGENT_PORT
        fi
    fi

    if [[ -n $agent_port ]]; then
        if ! wait_for '[[ -n "$(HORIZON_URL=http://localhost:${agent_port} hzn node list 2>/dev/null | jq -r .configuration.preferred_exchange_version 2>/dev/null)" ]]' 'Horizon agent ready' $AGENT_WAIT_MAX_SECONDS; then
            log_fatal 3 "Horizon agent did not start successfully"
        fi
    else
        if ! wait_for '[[ -n "$(hzn node list 2>/dev/null | jq -r .configuration.preferred_exchange_version 2>/dev/null)" ]]' 'Horizon agent ready' $AGENT_WAIT_MAX_SECONDS; then
            log_fatal 3 "Horizon agent did not start successfully"
        fi
    fi
    log_debug "wait_until_agent_ready() begin"
}

# Load a docker system and return the full image path (including tag)
# Note: does not remove the origin tar.gz file.
function load_docker_image() {
    log_debug "load_docker_image() begin"
    local __resultvar=$1
    local tar_file_name=${2:?}
    # Note: only fatal msgs allowed in this function, because it returns a string value
    if [[ ! -f $tar_file_name ]]; then
        log_fatal 3 "Agent docker image tar file $tar_file_name does not exist"
    fi
    if [[ -h $tar_file_name ]]; then
        log_fatal 3 "Can not unpack $tar_file_name because gunzip does not support symbolic links"
    fi

    local rc=0
    local gunzip_message
    gunzip_message=$(gunzip -k ${tar_file_name}  2>&1) || rc=$? # keep the original file
    if [ $rc -ne 0 ]; then
        log_fatal 3 "Exit code $rc from uncompressing ${tar_file_name}. ${gunzip_message}"
    fi

    local loaded_image_message
    loaded_image_message=$(${DOCKER_ENGINE} load --input ${tar_file_name%.gz}) || rc=$?
    rm ${tar_file_name%.gz}   # clean up this temporary file
    if [ $rc -ne 0 ]; then
        log_fatal 3 "Exit code $rc from ${DOCKER_ENGINE} loading ${tar_file_name%.gz}."
    fi

    # loaded_image_message is like: Loaded image: {repo}/{image_name}:{version_number}
    local image_full_path=$(echo $loaded_image_message | awk -F': ' '{print $2}')
    if [[ -z $image_full_path ]]; then
        log_fatal 3 "Could not get agent image path from loaded $tar_file_name"
    fi

    eval $__resultvar="'${image_full_path}'"

    log_debug "load_docker_image() end"
}

# Get the latest agent-in-container started on mac or linux
function start_device_agent_container() {
    log_debug "start_device_agent_container() begin"

    local container_num=$1
    local container_name="horizon${container_num}"

    if ! isCmdInstalled horizon-container; then
        log_fatal 5 "The horizon-container command not found, horizon-cli is not installed or its installation is broken"
    fi

    # Note: install_mac_horizon-cli() sets HC_DOCKER_TAG appropriately
    # In the css case, get amd64_anax.tar.gz from css, docker load it, and set HC_DOCKER_IMAGE and HC_DONT_PULL
    if has_upgrade_type_sw; then
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            local input_path
            get_input_file_css_path input_path
            download_css_file "$input_path/$AGENT_IMAGE_TAR_FILE"
            log_info "Unpacking and docker loading $AGENT_IMAGE_TAR_FILE ..."
            local agent_image_full_path
            load_docker_image agent_image_full_path $AGENT_IMAGE_TAR_FILE

            #rm ${AGENT_IMAGE_TAR_FILE}   # do not remove the file they gave us
            export HC_DONT_PULL=1   # horizon-container should get it straight from the tar file we got from CSS, not try to go to docker hub to get it
            export HC_DOCKER_IMAGE=${agent_image_full_path%:*}   # remove the tag
            export HC_DOCKER_TAG=${agent_image_full_path##*:}   # remove everything but the tag
        elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
            : # we've already set HC_DOCKER_TAG from the horizon-cli version. From there horizon-container naturally does the right thing (pulls it from docker hub)
        elif [[ -f $AGENT_IMAGE_TAR_FILE ]]; then
            # They gave us the agent docker image tar file in the input path
            log_info "Unpacking and docker loading $AGENT_IMAGE_TAR_FILE ..."
            local agent_image_full_path
            load_docker_image agent_image_full_path $AGENT_IMAGE_TAR_FILE

            #rm ${AGENT_IMAGE_TAR_FILE}   # do not remove the file they gave us
            export HC_DONT_PULL=1   # horizon-container should get it straight from the tar file we got from CSS, not try to go to docker hub to get it
            export HC_DOCKER_IMAGE=${agent_image_full_path%:*}   # remove the tag
            export HC_DOCKER_TAG=${agent_image_full_path##*:}   # remove everything but the tag
        fi
        #else let horizon-container do its default thing (run openhorizon/amd64_anax:latest)
    else
        # just a restart, but we have to find the image and version
        container_info=$(${DOCKER_ENGINE} inspect ${container_name})
        if [ $? -eq 0 ]; then
            image=$(echo "$container_info" |jq '.[].Config.Image' 2>&1)
            rc=$?
            if [ $rc -ne 0 ]; then
                log_info "Failed to get the original agent image name."
            else
                # remove the quotes from the image name
                image="${image%\"}"
                image="${image#\"}"

                export HC_DONT_PULL=1
                export HC_DOCKER_IMAGE=${image%:*}
                export HC_DOCKER_TAG=${image##*:}
            fi
        else
            log_info "Failed to inspect the container horizon${container_name}: $container_info"
            # take the default way
        fi
    fi

    if ! isDockerContainerRunning $container_name; then
        if [[ -z $(${DOCKER_ENGINE} ps -aq --filter name=${container_name}) ]]; then
            # horizon services container doesn't exist
            log_info "Starting horizon agent container version $HC_DOCKER_TAG ..."
            horizon-container start $container_num
        else
            # horizon container is stopped but the container exists
            log_info "The horizon agent container was in a stopped state via ${DOCKER_ENGINE}, restarting it..."
            ${DOCKER_ENGINE} start $container_name
            horizon-container update $container_num  # ensure it is running the latest version
        fi
    else
        log_info "The Horizon agent container is running already, restarting it to ensure it is version $HC_DOCKER_TAG ..."
        horizon-container update $container_num  # ensure it is running the latest version
    fi

    wait_until_agent_ready

    log_debug "start_device_agent_container() end"
}

# Stops horizon service container on mac
function stop_agent_container() {
    log_debug "stop_agent_container() begin"
    local container_num=$1
    local container_name="horizon${container_num}"

    if ! isDockerContainerRunning $container_name; then return; fi   # already stopped

    if ! isCmdInstalled horizon-container; then
        log_fatal 3 "Horizon agent container running, but horizon-container command not installed"
    fi

    log_info "Stopping the Horizon agent container..."
    horizon-container stop $container_num

    log_debug "stop_agent_container() end"
}

# Runs the given hzn command directly (for device) or via kubectl exec (for cluster) and returns the output. This enables us to run the same cmd for both (most of the time).
# Note: when a file needs to be passed into the cmd set the flag to read from stdin, then cat the file into this function.
# The full cmd passed in must be a single string, with args quoted within the string as necessary
function agent_exec() {
    local full_cmd=$1
    if is_device; then
        bash -c "$full_cmd"
    else   # cluster
        $KUBECTL exec -i ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "$full_cmd"
    fi
}

# Returns 0 (true) if the node is currently registered
function is_agent_registered() {
    local only_configured=$1   # optional: set to 'true' if configuring or unconfiguring should not be considered registered
    # Verify we have hzn available to us
    if ! agent_exec 'hzn -h >/dev/null 2>&1'; then return 1; fi

    local hzn_node_list=$(agent_exec 'hzn node list' 2>/dev/null || true)   # if hzn not installed, hzn_node_list will be empty
    local node_state=$(jq -r .configstate.state 2>/dev/null <<< $hzn_node_list || true)
    if [[ $only_configured == 'true' && $node_state == 'configured' ]]; then return 0
    elif [[ $only_configured != 'true' && $node_state =~ ^(configured|configuring|unconfiguring)$ ]]; then return 0
    else return 1; fi
}

# Returns 0 (true) if the current registration settings are already the same as what the user asked for
# Note: the caller of this function is taking into account is_horizon_defaults_correct()
function is_registration_correct() {
    log_debug "is_registration_correct() begin"

    if [[ $AGENT_SKIP_REGISTRATION == 'true' ]]; then return 0; fi   # the user doesn't care, so they are correct
    local hzn_node_list=$(agent_exec 'hzn node list' 2>/dev/null || true)   # if hzn not installed, hzn_node_list will be empty
    local reg_node_id=$(jq -r .id 2>/dev/null <<< $hzn_node_list || true)
    local reg_hzn_org_id=$(jq -r .organization 2>/dev/null <<< $hzn_node_list || true)
    local node_state=$(jq -r .configstate.state 2>/dev/null <<< $hzn_node_list || true)
    local reg_pattern=$(jq -r .pattern 2>/dev/null <<< $hzn_node_list || true)
    if [[ $node_state == 'configured' && -n $HZN_EXCHANGE_PATTERN && $reg_pattern == $HZN_EXCHANGE_PATTERN && (-z $NODE_ID || $reg_node_id == $NODE_ID) && $reg_hzn_org_id == $HZN_ORG_ID ]]; then return 0   # pattern case
    elif [[ $node_state == 'configured' && -z $HZN_EXCHANGE_PATTERN && -z $reg_pattern && (-z $NODE_ID || $reg_node_id == $NODE_ID) && $reg_hzn_org_id == $HZN_ORG_ID ]]; then return 0   # policy case (registration() will apply any new policy)
    else return 1; fi
    log_debug "is_registration_correct() end"
}

# Unregister the node, handling problems as necessary
function unregister() {
    log_debug "unregister() begin"

    local hzn_node_list=$(agent_exec 'hzn node list 2>/dev/null' || true)
    local reg_node_id=$(jq -r .id 2>/dev/null <<< $hzn_node_list || true)
    local node_state=$(jq -r .configstate.state 2>/dev/null <<< $hzn_node_list || true)

    # If they didn't specify a node id, try to use the one from the previous registration
    if [[ -z "$NODE_ID" ]]; then
        NODE_ID="$reg_node_id"
        log_info "Using NODE_ID from previous registration: $NODE_ID"
    fi

    local rmExchNodeFlag
    if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then rmExchNodeFlag='-r'; fi   # remove exchange node in case anything needs to be different, but can't recreate the exchange resource w/o HZN_EXCHANGE_USER_AUTH

    local rmAnaxInContainer
    if is_anax_in_container; then rmAnaxInContainer='-C'; fi # perform the deep clean on a container agent if performing anax-in-container install

    if [[ "$node_state" == "configured" ]]; then
        log_info "Unregistering the agent because the current registration settings are not what you want..."
        agent_exec "hzn unregister -f $rmExchNodeFlag"
        local rc=$?
        if [[ $rc -ne 0 ]]; then
            log_info "Unregister failed with exit code $rc. Now trying to unregister with deep clean..."
            agent_exec "hzn unregister -fD $rmExchNodeFlag $rmAnaxInContainer"   # registration is stuck, do a deep clean
            chk $? 'unregistering with deep clean'
        fi
    else
        # configuring, unconfiguring, or some unanticipated state
        log_info "The agent state is $node_state, unregistering it with deep clean..."
        agent_exec "hzn unregister -fD $rmExchNodeFlag $rmAnaxInContainer"   # registration is stuck, do a deep clean
        chk $? 'unregistering with deep clean'
    fi

    log_debug "unregister() end"
}

# For Device and Cluster: register node depending on if registration's requested and if previous registration state matches
function registration() {
    log_debug "registration() begin"
    local skip_reg=$1
    local pattern=$2
    local policy=$3
    local ha_group=$4

    if [[ $skip_reg == 'true' ]]; then return; fi

    local hzn_node_list=$(agent_exec 'hzn node list 2>/dev/null' || true)
    local reg_node_id=$(jq -r .id 2>/dev/null <<< $hzn_node_list || true)

    # Get current node registration state and determine if we need to unregister first
    local node_state=$(jq -r .configstate.state 2>/dev/null <<< $hzn_node_list || true)


    # Register the edge node
    if [[ $node_state == 'configured' ]]; then
        if [[ -z $policy ]] && [[ -z $ha_group ]]; then
            # nothing needs changing in the current registration
            log_info "The current registration settings are correct, keeping them."
        else
            log_info "The node is currently registered."
            if [[ -n $policy ]]; then
                # We only need to update the node policy (we can keep the node registered).
                log_info "Updating the node policy..."
                # Since hzn might be in the edge cluster container, need to pass the policy file's contents in via stdin
                cat $policy | agent_exec "hzn policy update -f-"
            fi

            if [[ -n $ha_group ]]; then
                reg_node_hagr=$(jq -r .ha_group 2>/dev/null <<< $hzn_node_list || true)
                if [[ $reg_node_hagr != $ha_group ]]; then
                    # we can keep the node registered but change the HA group this node is in
                    if [[ -n $reg_node_hagr ]]; then
                        log_warning "The node is currently in a HA group $reg_node_hagr. Please run 'hzn exchange hagroup' command to add this node to HA group $ha_group."
                    else
                        add_node_to_ha_group $ha_group
                    fi
                fi
            fi
        fi
    else
        # Register the node. First get some variables ready
        local user_auth wait_service_flag wait_org_flag timeout_flag node_name pattern_flag
        if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then
            user_auth_mask="-u ********"
            user_auth="-u '$HZN_EXCHANGE_USER_AUTH'"
        fi
        if [[ -n $AGENT_WAIT_FOR_SERVICE ]]; then
            wait_service_flag="-s '$AGENT_WAIT_FOR_SERVICE'"
        fi
        if [[ -n $AGENT_WAIT_FOR_SERVICE_ORG ]]; then
            wait_org_flag="--serviceorg '$AGENT_WAIT_FOR_SERVICE_ORG'"
        fi
        if [[ -n $AGENT_REGISTRATION_TIMEOUT ]]; then
            timeout_flag="-t '$AGENT_REGISTRATION_TIMEOUT'"
        fi
        if is_device; then
            node_name=$HOSTNAME
        else
            node_name=$NODE_ID
        fi
        log_verbose "Using node name: $node_name"
        if [[ -n $pattern ]]; then
            pattern_flag="-p '$pattern'"
        fi

        if [[ -n $ha_group ]]; then
            ha_group_flag="--ha-group '$ha_group'"
        fi

        # Now actually do the registration
        log_info "Registering the edge node..."
        local reg_cmd
        local reg_cmd_mask
        if [[ -n $policy ]]; then
            # Since hzn might be in the edge cluster container, need to pass the policy file's contents in via stdin
            reg_cmd="hzn register -m '${node_name}' -o '$HZN_ORG_ID' $user_auth -n '$HZN_EXCHANGE_NODE_AUTH' $wait_service_flag $wait_org_flag $timeout_flag --policy=- $ha_group_flag"
            reg_cmd_mask="hzn register -m '${node_name}' -o '$HZN_ORG_ID' $user_auth_mask -n ******** $wait_service_flag $wait_org_flag $timeout_flag --policy=$policy $ha_group_flag"
            echo "$reg_cmd_mask"
            cat $policy | agent_exec "$reg_cmd"
        else  # register w/o policy or with pattern
            reg_cmd="hzn register -m '${node_name}' -o '$HZN_ORG_ID' $user_auth -n '$HZN_EXCHANGE_NODE_AUTH' $wait_service_flag $wait_org_flag $timeout_flag $pattern_flag $ha_group_flag"
            reg_cmd_mask="hzn register -m '${node_name}' -o '$HZN_ORG_ID' $user_auth_mask -n ******** $wait_service_flag $wait_org_flag $timeout_flag $pattern_flag $ha_group_flag"
            echo "$reg_cmd_mask"
            agent_exec  "$reg_cmd"
        fi
    fi

    log_debug "registration() end"
}

# This function will add the given node to the given HA group.
# It will create the HA group if it does not exist.
function add_node_to_ha_group() {
    log_debug "add_node_to_ha_group() begin"

    local ha_group_name=$1

    local exch_creds cert_flag
    if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then
        exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_USER_AUTH"
    else
        exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_NODE_AUTH"   # input checking requires either user creds or node creds
    fi

    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        cert_flag="--cacert $AGENT_CERT_FILE"
    fi

    log_info "Getting HA group $ha_group_name..."
    echo "curl -sSL -w %{http_code} ${CURL_RETRY_PARMS} $cert_flag $HZN_EXCHANGE_URL/orgs/$HZN_ORG_ID/hagroups/$ha_group_name -u '*****'"
    local exch_output=$(curl -sSL -w %{http_code} ${CURL_RETRY_PARMS} $cert_flag $HZN_EXCHANGE_URL/orgs/$HZN_ORG_ID/hagroups/$ha_group_name -u "$exch_creds" 2>&1) || true

    if [[ -n "$exch_output" ]]; then
        local http_code="${exch_output: -3}"
        local len=${#exch_output}
        local output=${exch_output:0: len - 3}
        if [[ $http_code -eq 404 ]]; then
            # the group does not exist, create it
            log_info "The HA group $ha_group_name does not exist, creating it with this node as a member..."
            local hagr_def=$(jq --null-input --arg DSCRP "HA group $ha_group_name" '{description: $DSCRP}' | jq --arg N $NODE_ID '.members |= [ $N ] + .')
            local cmd="echo '$hagr_def' | hzn exchange hagroup add -f- '$ha_group_name' -o '$HZN_ORG_ID' -u '$exch_creds'"
            local cmd_mask="echo '$hagr_def' | hzn exchange hagroup add -f- '$ha_group_name' -o '$HZN_ORG_ID' -u '*****'"
            echo "$cmd_mask"
            agent_exec "$cmd"
        elif [[ $http_code -eq 200 ]]; then
            # the group exists, add the node to it
            log_info "The HA group $ha_group_name exists, adding this node to it..."
            local cmd="hzn exchange hagroup member add '$ha_group_name' --node '$NODE_ID' -o '$HZN_ORG_ID' -u '$exch_creds' "
            local cmd_mask="hzn exchange hagroup member add '$ha_group_name' --node '$NODE_ID' -o '$HZN_ORG_ID' -u '*****'"
            echo "$cmd_mask"
            agent_exec "$cmd"
        else
            log_fatal 4 "Failed to get HA group from the exchange. $output"
        fi
    else
        log_fatal 4 "Unable to add node to HA group $ha_group_name."
    fi
    log_debug "add_node_to_ha_group() end"
}

# autocomplete support for CLI
function add_autocomplete() {
    log_debug "add_autocomplete() begin"

    log_verbose "Enabling autocomplete for the hzn command..."

    SHELL_FILE="${SHELL##*/}"
    local autocomplete

    if [[ -f "/etc/bash_completion.d/hzn_bash_autocomplete.sh" ]]; then
        autocomplete="/etc/bash_completion.d/hzn_bash_autocomplete.sh"
    elif [[ -f "/usr/local/share/horizon/hzn_bash_autocomplete.sh" ]]; then
        # This is where the macos horizon-cli pkg puts it
        autocomplete="/usr/local/share/horizon/hzn_bash_autocomplete.sh"
    fi

    if [[ -n "$autocomplete" ]]; then
        if is_linux; then
            grep -q -E "^source ${autocomplete}" ~/.${SHELL_FILE}rc 2>/dev/null || echo -e "\nsource ${autocomplete}" >>~/.${SHELL_FILE}rc
        elif is_macos; then
            # The default terminal app on mac reads .bash_profile instead of .bashrc . But some 3rd part terminal apps read .bashrc, so update that too, if it exists
            grep -q -E "^source ${autocomplete}" ~/.${SHELL_FILE}_profile 2>/dev/null || echo -e "\nsource ${autocomplete}" >>~/.${SHELL_FILE}_profile
            if [[ -f "~/.${SHELL_FILE}rc" ]]; then
                echo -e "\nsource ${autocomplete}" >>~/.${SHELL_FILE}rc
            fi
        fi
    else
        log_verbose "Did not find hzn_bash_autocomplete.sh, skipping it..."
    fi

    log_debug "add_autocomplete() end"
}

# Returns operating system.
function get_os() {
    # OSTYPE is set automatically by the shell
    if [[ $OSTYPE == linux* ]]; then
        echo 'linux'
    elif [[ $OSTYPE == darwin* ]]; then
        echo 'macos'
    else
        echo 'unknown'
    fi
}

# Detects linux distribution name, version, and codename.
# Side-effect: sets DISTRO, DISTRO_VERSION_NUM, CODENAME
function detect_distro() {
    log_debug "detect_distro() begin"

    if ! is_linux && ! is_macos; then return; fi

    if is_macos; then
        DISTRO=$(sw_vers | grep ProductName | awk '{ print$2 }')
        DISTRO_VERSION_NUM=$(sw_vers | grep ProductVersion | awk '{ print$2 }')
        CODENAME=$(awk '/SOFTWARE LICENSE AGREEMENT FOR macOS/' '/System/Library/CoreServices/Setup Assistant.app/Contents/Resources/en.lproj/OSXSoftwareLicense.rtf' | awk -F 'macOS ' '{print $NF}' | awk '{print substr($0, 0, length($0)-1)}')

    elif isCmdInstalled lsb_release; then
        DISTRO=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        DISTRO_VERSION_NUM=$(lsb_release -sr)
        CODENAME=$(lsb_release -sc)

    # need these for redhat variants
    elif [[ -f /etc/os-release ]]; then
        . /etc/os-release
        DISTRO=$ID
        DISTRO_VERSION_NUM=$VERSION_ID
        if is_redhat_variant; then
            CODENAME=$VERSION
            CODENAME=$(echo $CODENAME | awk '{ print $2 }' | tr -d '()')
        else
            CODENAME=$VERSION_CODENAME
        fi
    elif [[ -f /etc/lsb-release ]]; then
        . /etc/lsb-release
        DISTRO=$DISTRIB_ID
        DISTRO_VERSION_NUM=$DISTRIB_RELEASE
        CODENAME=$DISTRIB_CODENAME
    else
        log_fatal 2 "Cannot detect Linux version"
    fi

    log_verbose "Detected distribution: ${DISTRO}, version: ${DISTRO_VERSION_NUM}, codename: ${CODENAME}"

    log_debug "detect_distro() end"
}

function is_debian_variant() {
    : ${DISTRO:?}   # verify this function is not called before DISTRO is set
    if [[ ${SUPPORTED_DEBIAN_VARIANTS[*]} =~ (^|[[:space:]])$DISTRO($|[[:space:]]) ]]; then return 0
    else return 1; fi
}

function is_redhat_variant() {
    : ${DISTRO:?}   # verify this function is not called before DISTRO is set
    if [[ ${SUPPORTED_REDHAT_VARIANTS[*]} =~ (^|[[:space:]])$DISTRO($|[[:space:]]) ]]; then return 0
    else return 1; fi
}

# Returns the extension used for pkgs in this distro
function get_pkg_type() {
    if is_macos; then echo 'pkg'
    elif is_debian_variant; then echo 'deb'
    elif is_redhat_variant; then echo 'rpm'
    fi
}

# Returns hardware architecture the way we want it for the pkgs on linux and mac
function get_arch() {
    if is_linux; then
        if is_debian_variant; then
            dpkg --print-architecture
        elif is_redhat_variant; then
            uname -m
        fi
    elif is_macos; then
        uname -m   # e.g. x86_64 or arm64
    fi
}

# Returns hardware architecture for Docker image names
function get_image_arch() {
    local image_arch=${ARCH:-$(get_arch)}
    if [[ $image_arch == 'x86_64' ]]; then
        image_arch='amd64'
    elif [[ $image_arch == 'aarch64' ]]; then
        image_arch='arm64'
    fi
    echo $image_arch
}

# Returns hardware architecture for the k8s cluster
function get_cluster_image_arch() {
    # We assume that all nodes in the k8s cluster are the same so just look at the 1st one... which will always work even for a small k8s cluster w/ 1 node
    local image_arch=$( $KUBECTL get nodes -o json | jq '.items[0].status.nodeInfo.architecture' | sed 's/"//g' )
    echo $image_arch
}

# check if the storage class exists in the edge cluster
function check_cluster_storage_class() {
    log_debug "check_cluster_storage_class() begin"
    local storage_class=$1
    if $KUBECTL get storageclass ${storage_class} >/dev/null 2>&1; then
        log_verbose "storage class $storage_class exists in the edge cluster"
    else
        log_fatal 2 "storage class $storage_class does not exist in the edge cluster"
    fi
    log_debug "check_cluster_storage_class() end"
}

# checks if OS/distribution/codename/arch is supported
function check_support() {
    log_debug "check_support() begin"
    local supported_list=${1:?} our_value=${2:?} name=${3:?}
    if [[ $supported_list =~ (^|[[:space:]])$our_value($|[[:space:]]) ]]; then
        log_verbose "$our_value is one of the supported $name"
    else
        log_fatal 2 "$our_value is NOT one of the supported $name: $supported_list"
    fi

    log_debug "check_support() end"
}

# Checks if OS, distro, and arch are supported. Also verifies the pkgs exist.
# Side-effect: sets PACKAGES to the dir where the local pkgs are. Also sets: DISTRO, DISTRO_VERSION_NUM, CODENAME, ARCH
function check_device_os() {
    log_debug "check_device_os() begin"

    check_support "${SUPPORTED_OS[*]}" "$OS" 'operating systems'

    if is_linux; then
        ensureWeAreRoot
        check_support "${SUPPORTED_LINUX_DISTRO[*]}" "$DISTRO" 'linux distros'
        if is_debian_variant; then
            check_support "${SUPPORTED_DEBIAN_VERSION[*]}" "$CODENAME" 'debian distro versions'
            check_support "${SUPPORTED_DEBIAN_ARCH[*]}" "$ARCH" 'debian architectures'
        elif is_redhat_variant; then
            check_support "${SUPPORTED_REDHAT_VERSION[*]}" "$DISTRO_VERSION_NUM" 'redhat distro versions'
            check_support "${SUPPORTED_REDHAT_ARCH[*]}" "$ARCH" 'redhat architectures'
        else
            log_fatal 5 "Unrecognized distro: $DISTRO"
        fi
    fi

    log_debug "check_device_os() end"
}

# Find node id in mapping file for this host's hostname or IP (for bulk install)
# Side-effect: sets NODE_ID and BULK_INSTALL
function find_node_id_in_mapping_file() {
    log_debug "find_node_id_in_mapping_file() begin"
    if [[ ! -f $NODE_ID_MAPPING_FILE ]]; then return; fi
    BULK_INSTALL=1   #todo: do we really need this global var, or is setting AGENT_SKIP_PROMPT=true good enough?
    log_debug "found id mapping file $NODE_ID_MAPPING_FILE"
    ID_LINE=$(grep $(hostname) "$NODE_ID_MAPPING_FILE" || [[ $? == 1 ]])
    if [[ -z $ID_LINE ]]; then
        log_debug "Did not find node id with hostname. Trying with ip"
        find_node_ip_address
        for IP in $(echo $NODE_IP); do
            ID_LINE=$(grep "$IP" "$NODE_ID_MAPPING_FILE" || [[ $? == 1 ]])
            if [[ ! "$ID_LINE" == "" ]]; then break; fi
        done
        if [[ ! "$ID_LINE" == "" ]]; then
            NODE_ID=$(echo $ID_LINE | cut -d "," -f 2)
        else
            log_fatal 2 "Failed to find node id in mapping file $NODE_ID_MAPPING_FILE with $(hostname) or $NODE_IP"
        fi
    else
        NODE_ID=$(echo $ID_LINE | cut -d "," -f 2)
    fi
    NODE_ID=$(trim_variable $NODE_ID)
    log_info "Found node id $NODE_ID for this host in mapping file $NODE_ID_MAPPING_FILE"
    log_debug "find_node_id_in_mapping_file() end"
}

function find_node_ip_address() {
    if is_macos; then
        #todo: en0 is not correct in all cases
        NODE_IP=$(ipconfig getifaddr en0)
    else
        NODE_IP=$(hostname -I)
    fi
}

# If node exist in management hub, verify it is correct type (device or cluster)
function check_existing_exch_node_is_correct_type() {
    log_debug "check_existing_exch_node_is_correct_type() begin"

    local expected_type=$1

    log_info "Verifying that node $NODE_ID in the exchange is type $expected_type (if it exists)..."
    local exch_creds cert_flag
    if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_USER_AUTH"
    else exch_creds="$HZN_ORG_ID/$HZN_EXCHANGE_NODE_AUTH"   # input checking requires either user creds or node creds
    fi

    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        cert_flag="--cacert $AGENT_CERT_FILE"
    fi
    local exch_output=$(curl -fsS ${CURL_RETRY_PARMS} $cert_flag $HZN_EXCHANGE_URL/orgs/$HZN_ORG_ID/nodes/$NODE_ID -u "$exch_creds" 2>/dev/null) || true

    if [[ -n "$exch_output" ]]; then
        local exch_node_type=$(echo $exch_output | jq -re '.nodes | .[].nodeType')
        if [[ "$exch_node_type" == "device" ]] && [[ "$expected_type" != "device" ]]; then
            log_fatal 2 "Node id ${NODE_ID} has already been created as nodeType device. Remove the node from the exchange and run this script again."
        elif [[ "$exch_node_type" == "cluster" ]] && [[ "$expected_type" != "cluster" ]]; then
            log_fatal 2 "Node id ${NODE_ID} has already been created as nodeType cluster. Remove the node from the exchange and run this script again."
        fi
    fi

    log_debug "check_existing_exch_node_is_correct_type() end"
}

# make sure the new exchange url and cert are good.
# this function is called after the config file is updated.
function check_exch_url_and_cert() {
    log_debug "check_exch_url_and_cert() begin"

    log_info "Verifying the exchange url and the certificate file..."

    local cert_flag=""
    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        cert_flag="--cacert $AGENT_CERT_FILE"
    fi

    local output=$(curl -w %{http_code} -fsS ${CURL_RETRY_PARMS} $cert_flag $HZN_EXCHANGE_URL/admin/version 2>/dev/null) || true

    local httpCode="${output: -3}"
    if [ $httpCode -ne 200 ]; then
        log_fatal 2 "Failed to verify the exchange url or certificate file."
    fi

    log_debug "check_exch_url_and_cert() end"
}

# Cluster only: to extract agent image tar.gz and load to docker
# Side-effect: sets globals: AGENT_IMAGE, AGENT_IMAGE_VERSION_IN_TAR, IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY INPUT_FILE_PATH
function loadClusterAgentImage() {
    log_debug "loadClusterAgentImage() begin"

    # Get the agent tar file, if necessary
    if using_remote_input_files 'pkg'; then
        if [[ -n $AGENT_K8S_IMAGE_TAR_FILE && $AGENT_K8S_IMAGE_TAR_FILE != $DEFAULT_AGENT_K8S_IMAGE_TAR_FILE ]]; then
            log_fatal 1 "Can not specify both AGENT_K8S_IMAGE_TAR_FILE and -i (INPUT_FILE_PATH)"
        fi
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            local input_path
            get_input_file_css_path input_path
            download_css_file "$input_path/$AGENT_K8S_IMAGE_TAR_FILE"
        elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
            # Get the docker image from docker hub in this case
            local image_tag=$(get_anax_release_version $INPUT_FILE_PATH)   # use the version from INPUT_FILE_PATH, if possible
            if [[ -z $image_tag ]]; then
                image_tag='latest'
            fi
            local image_arch=$(get_cluster_image_arch)
            local image_path="openhorizon/${image_arch}_anax_k8s:$image_tag"
            log_info "Pulling $image_path from docker hub..."
            ${DOCKER_ENGINE} pull "$image_path"
            chk $? "pulling $image_path"
            AGENT_IMAGE=$image_path
            AGENT_IMAGE_VERSION_IN_TAR=${AGENT_IMAGE##*:}
            IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$IMAGE_ON_EDGE_CLUSTER_REGISTRY:$AGENT_IMAGE_VERSION_IN_TAR"
            log_info "IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY is set to $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            return
        elif [[ $INPUT_FILE_PATH == remote:* ]]; then
            # input file path is: "remote:<image_tag>" or "remote:", get the docker image tag INPUT_FILE_PATH
            local input_path
            get_input_file_remote_path input_path
            local image_tag=${input_path##*:} # version
            INPUT_FILE_PATH="remote:$image_tag" # update $INPUT_FILE_PATH

            # local image_tag
            # get_agent_version_from_repository image_tag
            AGENT_IMAGE_VERSION_IN_TAR=$image_tag
            IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$IMAGE_ON_EDGE_CLUSTER_REGISTRY:$AGENT_IMAGE_VERSION_IN_TAR"
            AGENT_IMAGE=$IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY
            log_info "IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY is set to $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            return
        fi
    elif [[ ! -f $AGENT_K8S_IMAGE_TAR_FILE ]]; then
        log_fatal 2 "Edge cluster agent image tar file $AGENT_K8S_IMAGE_TAR_FILE does not exist"
    fi

    log_info "Unpacking and docker loading $AGENT_K8S_IMAGE_TAR_FILE ..."
    local agent_image_full_path
    load_docker_image agent_image_full_path $AGENT_K8S_IMAGE_TAR_FILE
    AGENT_IMAGE=$agent_image_full_path
    chk $? "docker loading $AGENT_K8S_IMAGE_TAR_FILE"
    # AGENT_IMAGE is like: {repo}/{image_name}:{version_number}
    #rm ${AGENT_K8S_IMAGE_TAR_FILE}   # do not remove the file they gave us

    AGENT_IMAGE_VERSION_IN_TAR=${AGENT_IMAGE##*:}
    # use the same tag for the image in the edge cluster registry as the tag they used for the image in the inputted tar file
    IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$IMAGE_ON_EDGE_CLUSTER_REGISTRY:$AGENT_IMAGE_VERSION_IN_TAR"

    log_debug "IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY is set to: $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"

    log_debug "loadClusterAgentImage() end"
}

# Cluster only: to set $EDGE_CLUSTER_REGISTRY_HOST, and login to registry
function getImageRegistryInfo() {
    log_debug "getImageRegistryInfo() begin"
    # split $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY by "/"
    EDGE_CLUSTER_REGISTRY_HOST=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $1}')
    log_info "Edge cluster registry host: $EDGE_CLUSTER_REGISTRY_HOST"

    if [[ $INPUT_FILE_PATH == remote:* || (-z $EDGE_CLUSTER_REGISTRY_USERNAME && -z $EDGE_CLUSTER_REGISTRY_TOKEN) ]]; then
        : # even for a registry in the insecure-registries list, if we don't specify user/pw it will prompt for it
        #docker login $EDGE_CLUSTER_REGISTRY_HOST
    else
        echo "$EDGE_CLUSTER_REGISTRY_TOKEN" | ${DOCKER_ENGINE} login -u $EDGE_CLUSTER_REGISTRY_USERNAME --password-stdin $EDGE_CLUSTER_REGISTRY_HOST
        chk $? "logging into edge cluster's registry: $EDGE_CLUSTER_REGISTRY_HOST"
    fi

    log_debug "getImageRegistryInfo() end"
}

# Cluster only: to push agent and cronjob images to image registry that edge cluster can access
function pushImagesToEdgeClusterRegistry() {
    log_debug "pushImagesToEdgeClusterRegistry() begin"

    log_info "Checking if docker image $AGENT_IMAGE exists on the $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY ..."
    set +e
    ${DOCKER_ENGINE} manifest inspect $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY >/dev/null 2>&1
    rc=$?
    set -e
    if [[ $rc -eq 0 ]]; then
        log_info "$IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY already exists, skip image push"
    else
        log_info "Pushing docker image $AGENT_IMAGE to $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY ..."
        ${DOCKER_ENGINE} tag ${AGENT_IMAGE} ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
        runCmdQuietly ${DOCKER_ENGINE} push ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
        log_verbose "successfully pushed image $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY to edge cluster registry"
    fi

    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
        log_info "Checking if docker image $CRONJOB_AUTO_UPGRADE_IMAGE exists on the $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY ..."
        set +e
        ${DOCKER_ENGINE} manifest inspect $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY >/dev/null 2>&1
        rc=$?
        set -e

        if [[ $rc -eq 0 ]]; then
            log_info "$CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY already exists, skip image push"
        else
            log_info "Pushing docker image $CRONJOB_AUTO_UPGRADE_IMAGE to $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY ..."
            ${DOCKER_ENGINE} tag ${CRONJOB_AUTO_UPGRADE_IMAGE} ${CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
            runCmdQuietly ${DOCKER_ENGINE} push ${CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
            log_verbose "successfully pushed image $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY to edge cluster registry"
        fi
    fi

    log_debug "pushImagesToEdgeClusterRegistry() end"
}

# Cluster only: to extract auto-upgrade-cronjob cronjob image tar.gz and load to docker
# Side-effect: sets globals: CRONJOB_AUTO_UPGRADE_IMAGE, CRONJOB_AUTO_UPGRADE_VERSION_IN_TAR, CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY
function loadClusterAgentAutoUpgradeCronJobImage() {
    log_debug "loadClusterAgentAutoUpgradeCronJobImage() begin"

    # Get the auto-upgrade-cronjob cronjob tar file, if necessary
    if using_remote_input_files 'pkg'; then
        if [[ -n $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE && $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE != $DEFAULT_CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE ]]; then
            log_fatal 1 "Can not specify both CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE and -i (INPUT_FILE_PATH)"
        fi
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            local input_path
            get_input_file_css_path input_path
            download_css_file "$input_path/$CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE"
        elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
            # Get the docker image from docker hub in this case
            local image_tag=$(get_anax_release_version $INPUT_FILE_PATH)   # use the version from INPUT_FILE_PATH, if possible
            if [[ -z $image_tag ]]; then
                image_tag='latest'
            fi
            local image_arch=$(get_cluster_image_arch)
            local image_path="openhorizon/${image_arch}_auto-upgrade-cronjob_k8s:$image_tag"
            log_info "Pulling $image_path from docker hub..."
            ${DOCKER_ENGINE} pull "$image_path"
            chk $? "pulling $image_path"
            CRONJOB_AUTO_UPGRADE_IMAGE=$image_path
            CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR=${CRONJOB_AUTO_UPGRADE_IMAGE##*:}
            CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY:$CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR"
            log_info "CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY is $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            return
        elif [[ $INPUT_FILE_PATH == remote:* ]]; then
            CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR=$AGENT_IMAGE_VERSION_IN_TAR # $AGENT_IMAGE_VERSION_IN_TAR was set in loadClusterAgentImage() function
            CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY:$CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR"
            CRONJOB_AUTO_UPGRADE_IMAGE=$CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY
            log_info "CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY is $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            return
        fi
    elif [[ ! -f $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE ]]; then
        log_fatal 2 "Edge cluster agent cronjob image tar file $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE does not exist"
    fi

    log_info "Unpacking and docker loading $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE ..."
    local cj_image_full_path
    load_docker_image cj_image_full_path $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE
    CRONJOB_AUTO_UPGRADE_IMAGE=$cj_image_full_path
    chk $? "docker loading $CRONJOB_AUTO_UPGRADE_K8S_TAR_FILE"
    # CRONJOB_AUTO_UPGRADE_IMAGE is like: {repo}/{image_name}:{version_number}

    CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR=${CRONJOB_AUTO_UPGRADE_IMAGE##*:}
    # use the same tag for the image in the edge cluster registry as the tag they used for the image in the inputted tar file
    CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY:$CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR"

    log_debug "loadClusterAgentAutoUpgradeCronJobImage() end"
}

# Cluster only: create image pull secrets if use remote private registry, this function is called when USE_EDGE_CLUSTER_REGISTRY=false, and remote registry is private
# Side-effect: set $USE_PRIVATE_REGISTRY
function create_image_pull_secrets() {
    log_debug "create_image_pull_secrets() begin"
    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "false" ]]; then
        if [[ -n $EDGE_CLUSTER_REGISTRY_USERNAME && -n $EDGE_CLUSTER_REGISTRY_TOKEN && -n $EDGE_CLUSTER_REGISTRY_HOST ]]; then
            # if $INPUT_FILE_PATH is "remote:*", we want to avoid the requirment of "docker/podman"
            if [[ $INPUT_FILE_PATH != remote:* ]]; then
                log_verbose "checking if private registry is accessible..."
                echo "$EDGE_CLUSTER_REGISTRY_TOKEN" | ${DOCKER_ENGINE} login -u $EDGE_CLUSTER_REGISTRY_USERNAME --password-stdin $EDGE_CLUSTER_REGISTRY_HOST
                chk $? "logging into remote private registry: $EDGE_CLUSTER_REGISTRY_HOST"
            fi

            log_verbose "checking if secret ${IMAGE_PULL_SECRET_NAME} exist..."
            USE_PRIVATE_REGISTRY="true"

            if $KUBECTL get secret ${IMAGE_PULL_SECRET_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
                $KUBECTL delete secret ${IMAGE_PULL_SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
                chk $? "deleting image pull secret before installing"
            fi

            log_verbose "creating image pull secrets ${IMAGE_PULL_SECRET_NAME}..."
            $KUBECTL create secret docker-registry ${IMAGE_PULL_SECRET_NAME} -n ${AGENT_NAMESPACE} --docker-server=${EDGE_CLUSTER_REGISTRY_HOST} --docker-username=${EDGE_CLUSTER_REGISTRY_USERNAME} --docker-password=${EDGE_CLUSTER_REGISTRY_TOKEN} --docker-email=""
            chk $? "creating image pull secrets ${IMAGE_PULL_SECRET_NAME} from edge cluster registry info"
            log_info "secret ${IMAGE_PULL_SECRET_NAME} created"
        else
            log_info "EDGE_CLUSTER_REGISTRY_USERNAME and/or EDGE_CLUSTER_REGISTRY_TOKEN is not specified, skip creating image pull secrets $IMAGE_PULL_SECRET_NAME"
        fi
    fi

    log_debug "create_image_pull_secrets() end"
}

# Cluster only: check if there is scope conflict
# check if there is another agent deployment in any namespace
#   - NO: continue on agent fresh install
#   - YES:
#       1. same namespace: continue, the scope will be checked in check_agent_deployment_exist (set AGENT_DEPLOYMENT_EXIST_IN_SAME_NAMESPACE=true)
#       2. different namespace: greb 1 deployment and check scope
#           a. current is cluster scoped agent => error
#           b. current is namespace scoped agent:
#               existing agent in other namespace is namespace scope: can proceed to install
#               existing agent in other namespace is cluster scope: error
# Side-effect: set AGENT_DEPLOYMENT_EXIST_IN_SAME_NAMESPACE
function check_cluster_agent_scope() {
    log_debug "check_cluster_agent_scope() begin"
    AGENT_DEPLOYMENT_EXIST_IN_SAME_NAMESPACE="false"

    if $KUBECTL get deployment --field-selector metadata.name=agent -A | grep -E '(^|\s)agent($|\s)' >/dev/null 2>&1; then
        log_debug "Has agent deployment in this cluster"
        # has agent deployment in the cluster
        # check namespace
        namespaces_have_agent=$($KUBECTL get deployment --field-selector metadata.name=agent -A -o jsonpath="{.items[*].metadata.namespace}" | tr -s '[[:space:]]' ',')
        log_info "Already have agent deployment in namespaces: $namespaces_have_agent, checking scope of existing agent"

        if [[ "$namespaces_have_agent" == *"$AGENT_NAMESPACE"* ]]; then
            log_debug "Namespaces array contains current namespace"
            # continue to check_agent_deployment_exist() to check scope
            AGENT_DEPLOYMENT_EXIST_IN_SAME_NAMESPACE="true"
        else
            # has agent in other namespace(s). Pick one agent deployment and check scope
            #   current is cluster scoped agent => error
            #   current is namespace scoped agent:
            #       namespace scope agent in other namespace => can proceed to install
            #       cluster scope agent in other namespace => error
            if ! $NAMESPACE_SCOPED; then
                log_fatal 3 "One or more agents detected in $namespaces_have_agent. A cluster scoped agent cannot be installed to the same cluster that has agent(s) already"
            fi

            IFS="," read -ra namespace_array <<< "$namespaces_have_agent"
            namespace_to_check=${namespace_array[0]}
            local namespace_scoped_env_value_in_use=$($KUBECTL get deployment agent -n ${namespace_to_check} -o json | jq '.spec.template.spec.containers[0].env' | jq -r '.[] | select(.name=="HZN_NAMESPACE_SCOPED").value')
            log_debug "Current HZN_NAMESPACE_SCOPED in agent deployment under namespace $namespace_to_check is: $namespace_scoped_env_value_in_use"
            log_debug "NAMESPACE_SCOPED passed to this script is: $NAMESPACE_SCOPED" # namespace scoped

            if [[ "$namespace_scoped_env_value_in_use" == "" ]] || [[ "$namespace_scoped_env_value_in_use" == "false" ]] ; then
                log_fatal 3 "A cluster scoped agent detected in $namespace_to_check. A namespace scoped agent cannot be installed to the same cluster that has a cluster scoped agent"
            fi
        fi

    else
        # no agent deployment in any namespace, can proceed to install this agent
        log_debug "No agent deployment in any namespace, can proceed to install this agent"
    fi
    log_debug "check_cluster_agent_scope() end"

}

# Cluster only: check if agent deployment exists to determine whether to do agent install or agent update
# Side-effect: sets AGENT_DEPLOYMENT_UPDATE, POD_ID, IS_AGENT_IMAGE_VERSION_SAME, IS_CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_SAME, IS_HORIZON_ORG_ID_SAME
function check_agent_deployment_exist() {
    log_debug "check_agent_deployment_exist() begin"
    IS_AGENT_IMAGE_VERSION_SAME="false"
    IS_CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_SAME="false"
    IS_HORIZON_ORG_ID_SAME="false"

    # has agent deployment, return 0 (true)
    # doesn't have agent deployment, return 1 (false)
    if ! $KUBECTL get deployment ${DEPLOYMENT_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
        # agent deployment doesn't exist in ${AGENT_NAMESPACE}, fresh install
        AGENT_DEPLOYMENT_UPDATE="false"
    else
        # already have an agent deplyment in ${AGENT_NAMESPACE}, check the agent pod status
        if [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent,type!=auto-upgrade-cronjob -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
            # agent deployment does not have agent pod in RUNNING status
            log_fatal 3 "Previous agent pod in not in RUNNING status, please run agent-uninstall.sh to clean up and re-run the agent-install.sh"
        else
            # check 0) agent scope in deployment
            local namespace_scoped_env_value_in_use=$($KUBECTL get deployment agent -n ${AGENT_NAMESPACE} -o json | jq '.spec.template.spec.containers[0].env' | jq -r '.[] | select(.name=="HZN_NAMESPACE_SCOPED").value')
            log_debug "Current HZN_NAMESPACE_SCOPED in agent deployment is $namespace_scoped_env_value_in_use"
            log_debug "NAMESPACE_SCOPED passed to this script is: $NAMESPACE_SCOPED"

            if [[ "$namespace_scoped_env_value_in_use" == "" ]]; then
                namespace_scoped_env_value_in_use="false"
            fi

            if [[ "$namespace_scoped_env_value_in_use" != "$NAMESPACE_SCOPED" ]]; then
                log_fatal 3 "Current agent scope cannot be updated, please run agent-uninstall.sh and re-run agent-install.sh"
            fi

            # check 1) agent image in deployment
            # eg: {image-registry}:5000/{repo}/{image-name}:{version}
            local agent_image_in_use=$($KUBECTL get deployment agent -n ${AGENT_NAMESPACE} -o json | jq -r '.spec.template.spec.containers[0].image')

            # {image-registry}:5000/{repo}
            local agent_image_on_edge_cluster_registry=${agent_image_in_use%:*}
            if [[ "$agent_image_on_edge_cluster_registry" != "$IMAGE_ON_EDGE_CLUSTER_REGISTRY" ]]; then
                log_fatal 3 "Current deployment image registry cannot be updated, please run agent-uninstall.sh and re-run agent-install.sh"
            fi

            local image_pull_secrets_length=$($KUBECTL get deployment agent -n ${AGENT_NAMESPACE} -o json | jq '.spec.template.spec.imagePullSecrets' | jq length)
            local use_image_pull_secrets
            if [[ "$image_pull_secrets_length" == "1" ]]; then
                use_image_pull_secrets="true"
            fi
            if [[ "$use_image_pull_secrets" != "$USE_PRIVATE_REGISTRY" ]]; then
                log_fatal 3 "Current deployment image registry pull secrets info cannot be updated, please run agent-uninstall.sh and re-run agent-install.sh"
            fi

            # {image-name}:{version}
            local agent_image_name_with_tag=$(echo $agent_image_in_use | awk -F'/' '{print $3}')
            # {version}
            local agent_image_version_in_use=$(echo $agent_image_name_with_tag | awk -F':' '{print $2}')

            log_debug "Current agent image version is: $agent_image_version_in_use, agent image version in tar file is: $AGENT_IMAGE_VERSION_IN_TAR"
            AGENT_DEPLOYMENT_UPDATE="true"
            if [[ "$AGENT_IMAGE_VERSION_IN_TAR" == "$agent_image_version_in_use" ]]; then
                IS_AGENT_IMAGE_VERSION_SAME="true"
            fi


            # check 2) auto-upgrade-cronjob cronjob image in cronjob yml
            # eg: {image-registry}:5000/{repo}/{image-name}:{version}
            if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
                local auto_upgrade_cronjob_image_in_use=$($KUBECTL get cronjob ${CRONJOB_AUTO_UPGRADE_NAME} -o jsonpath='{$.spec.jobTemplate.spec.template.spec.containers[:1].image}' -n ${AGENT_NAMESPACE})

                # {image-registry}:5000/{repo}
                local auto_upgrade_cronjob_image_on_edge_cluster_registry=${auto_upgrade_cronjob_image_in_use%:*}
                if [[ "$auto_upgrade_cronjob_image_on_edge_cluster_registry" != "$CRONJOB_AUTO_UPGRADE_IMAGE_ON_EDGE_CLUSTER_REGISTRY" ]]; then
                    log_fatal 3 "Current auto-upgrade-cronjob cronjob image registry cannot be updated, please run agent-uninstall.sh and re-run agent-install.sh"
                fi

                # {image-name}:{version}
                local auto_upgrade_cronjob_image_name_with_tag=$(echo $auto_upgrade_cronjob_image_in_use | awk -F'/' '{print $3}')
                # {version}
                local auto_upgrade_cronjob_image_version_in_use=$(echo $auto_upgrade_cronjob_image_name_with_tag | awk -F':' '{print $2}')

                log_debug "Current auto-upgrade-cronjob cronjob image version is: $auto_upgrade_cronjob_image_version_in_use, auto-upgrade-cronjob cronjob image version in tar file is: $CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR"
                if [[ "$CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_IN_TAR" == "$auto_upgrade_cronjob_image_version_in_use" ]]; then
                    IS_CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_SAME="true"
                fi
            else
                IS_CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_SAME="true"
            fi

            # check 3) HZN_ORG_ID set in deployment
            local horizon_org_id_env_value_in_use=$($KUBECTL get deployment agent -n ${AGENT_NAMESPACE} -o json | jq '.spec.template.spec.containers[0].env' | jq -r '.[] | select(.name=="HZN_ORG_ID").value')
            log_debug "Current HZN_ORG_ID in agent deployment is: $horizon_org_id_env_value_in_use"
            log_debug "HZN_ORG_ID passed to this script is: $HZN_ORG_ID"

            if [[ "$horizon_org_id_env_value_in_use" == "$HZN_ORG_ID" ]]; then
                IS_HORIZON_ORG_ID_SAME="true"
            fi

            POD_ID=$($KUBECTL get pod -l app=agent --field-selector status.phase=Running -n ${AGENT_NAMESPACE} 2>/dev/null | grep "agent-" | cut -d " " -f1 2>/dev/null)
            log_verbose "Previous agent pod is ${POD_ID}, will continue with agent updating in edge cluster"
        fi
    fi

    log_debug "check_agent_deployment_exist() end"
}

# Cluster only: get or verify deployment-template.yml, persistentClaim-template.yml, auto-upgrade-cronjob-template.yml and agent-uninstall.sh
function get_edge_cluster_files() {
    log_debug "get_edge_cluster_files() begin"
    if using_remote_input_files 'yml'; then
        log_verbose "Getting template.yml files and agent-uninstall.sh from $INPUT_FILE_PATH ..."
        if [[ $INPUT_FILE_PATH == css:* || $INPUT_FILE_PATH == remote:* ]]; then
            local input_path
            get_input_file_css_path input_path
            download_css_file "$input_path/$EDGE_CLUSTER_TAR_FILE_NAME"
        elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
            download_anax_release_file "$INPUT_FILE_PATH/$EDGE_CLUSTER_TAR_FILE_NAME"
        fi
        tar -zxf "$EDGE_CLUSTER_TAR_FILE_NAME"
        chk $? "unpacking $EDGE_CLUSTER_TAR_FILE_NAME"
        rm "$EDGE_CLUSTER_TAR_FILE_NAME"
    fi

    for f in deployment-template.yml persistentClaim-template.yml agent-uninstall.sh; do
        if [[ ! -f $f ]]; then
            log_fatal 1 "file $f not found"
        fi
    done

    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]] && [[ ! -f auto-upgrade-cronjob-template.yml ]]; then
        log_fatal 1 "auto-upgrade-cronjob-template.yml not found"
    fi
    log_debug "get_edge_cluster_files() end"
}

# Cluster only: to generate 3 files: deployment.yml, auto-upgrade-cronjob.yml and persistentClaim.yml
function generate_installation_files() {
    log_debug "generate_installation_files() begin"

    get_edge_cluster_files

    log_verbose "Preparing kubernete persistentVolumeClaim file"
    prepare_k8s_pvc_file
    log_verbose "kubernete persistentVolumeClaim file are done."

    if [[ "$IS_AGENT_IMAGE_VERSION_SAME" == "true" ]] && [[ "$IS_HORIZON_ORG_ID_SAME" == "true" ]]; then
        log_verbose "agent image version and value of HZN_ORG_ID are same with existing deployment, skip updating deployment.yml"
    else
        log_verbose "Preparing kubernete deployment files"
        prepare_k8s_deployment_file
        log_verbose "kubernete deployment files are done."
    fi

    if [[ "$IS_CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_SAME" == "true" ]] || [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" != "true" ]] ; then
        log_verbose "auto-upgrade-cronjob cronjob image version is the same with existing deployment, skip updating auto-upgrade-cronjob.yml"
    else
        log_verbose "Preparing kubernete cronjob files"
        prepare_k8s_auto_upgrade_cronjob_file
        log_verbose "kubernete cronjob files are done."
    fi

    log_debug "generate_installation_files() end"
}

# Cluster only: to generate /tmp/agent-install-hzn-env file and copy cert file to /etc/default/cert
function create_horizon_env() {
    log_debug "create_horizon_env() begin"
    if [[ -f $HZN_ENV_FILE ]]; then
        log_verbose "$HZN_ENV_FILE already exists. Will overwrite it..."
        rm $HZN_ENV_FILE
    fi
    if [[ -f $AGENT_CERT_FILE ]]; then
        local cert_name=$(basename ${AGENT_CERT_FILE})
        local cluster_cert_path="/etc/default/cert"
        echo "HZN_MGMT_HUB_CERT_PATH=$cluster_cert_path/$cert_name" >>$HZN_ENV_FILE
    fi

    echo "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}" >>$HZN_ENV_FILE
    echo "HZN_FSS_CSSURL=${HZN_FSS_CSSURL}" >>$HZN_ENV_FILE
    if [[ -n $HZN_AGBOT_URL ]]; then
        echo "HZN_AGBOT_URL=${HZN_AGBOT_URL}" >>$HZN_ENV_FILE
    fi
    if [[ -n $HZN_SDO_SVC_URL ]]; then
        echo "HZN_SDO_SVC_URL=${HZN_SDO_SVC_URL}" >>$HZN_ENV_FILE
    fi
    if [[ -n $HZN_FDO_SVC_URL ]]; then
        echo "HZN_FDO_SVC_URL=${HZN_FDO_SVC_URL}" >>$HZN_ENV_FILE
    fi
    echo "HZN_DEVICE_ID=${NODE_ID}" >>$HZN_ENV_FILE
    echo "HZN_NODE_ID=${NODE_ID}" >> $HZN_ENV_FILE
    echo "HZN_AGENT_PORT=8510" >>$HZN_ENV_FILE
    echo "HZN_CONFIG_VERSION=${HZN_CONFIG_VERSION}" >> $HZN_ENV_FILE
    log_debug "create_horizon_env() end"
}

# Cluster only: to create deployment.yml based on template
function prepare_k8s_deployment_file() {
    log_debug "prepare_k8s_deployment_file() begin"
    # Note: get_edge_cluster_files() already downloaded deployment-template.yml, if necessary

    # InitContainer needs to be removed for ocp because it breaks mounted directory permisson. In ocp, the permission of volume is configured by scc.
    if is_ocp_cluster && [[ $EDGE_CLUSTER_STORAGE_CLASS != ibmc-file* ]] && [[ $EDGE_CLUSTER_STORAGE_CLASS != ibmc-vpc-file* ]]; then
        log_info "remove initContainer"
        sed -i -e '/START_NOT_FOR_OCP/,/END_NOT_FOR_OCP/d' deployment-template.yml
    fi

    if [[ ! -f $AGENT_CERT_FILE ]]; then
        log_debug "agent cert file is not used, remove secret and secret mount section from deployment-template.yml ..."
        sed -i -e '{/START_CERT_VOL/,/END_CERT_VOL/d;}' deployment-template.yml
    fi

    sed -e "s#__AgentNameSpace__#\"${AGENT_NAMESPACE}\"#g" -e "s#__InitContainerImagePath__#${INIT_CONTAINER_IMAGE}#g" -e "s#__NamespaceScoped__#\"${NAMESPACE_SCOPED}\"#g" -e "s#__OrgId__#\"${HZN_ORG_ID}\"#g" deployment-template.yml >deployment.yml
    chk $? 'creating deployment.yml'

    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        sed -i -e '{/START_REMOTE_ICR/,/END_REMOTE_ICR/d;}' deployment.yml # remove imagePullSecrets section from template
        EDGE_CLUSTER_REGISTRY_PROJECT_NAME=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $2}')
        EDGE_CLUSTER_AGENT_IMAGE_AND_TAG=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $3}')
        local image_full_path_on_edge_cluster_registry_internal_url
        if [[ "$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY" == "" ]]; then
            if is_ocp_cluster; then
                # using ocp
                image_full_path_on_edge_cluster_registry_internal_url=$DEFAULT_OCP_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
            else
                image_full_path_on_edge_cluster_registry_internal_url="$IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            fi
        else
            image_full_path_on_edge_cluster_registry_internal_url=$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
        fi
        sed -i -e "s#__ImagePath__#${image_full_path_on_edge_cluster_registry_internal_url}#g" deployment.yml

        # fill out __ImageRegistryHost__ with value of EDGE_CLUSTER_REGISTRY_HOST
        # k3s/microk8s: xx.xx.xxx.xx:5000
        # ocp: default-route-openshift-image-registry.apps.xxxx.xx.xx.xxx.com
        if [[ $KUBECTL == "microk8s.kubectl" ]]; then
            mirok8s_registry_endpoint=$($KUBECTL get service registry -n container-registry | grep registry | awk '{print $3;}'):5000
            log_info "In microk8s cluster, update ImageRegistryHost to $mirok8s_registry_endpoint"
            sed -i -e "s#__ImageRegistryHost__#${mirok8s_registry_endpoint}#g" deployment.yml
        else
            sed -i -e "s#__ImageRegistryHost__#${EDGE_CLUSTER_REGISTRY_HOST}#g" deployment.yml
        fi
    else
        log_info "This agent install on edge cluster is using a remote registry: $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
        ## We want to avoid the requirement of docker/podman if $INPUT_FILE_PATH is "remote:*"
        if [[ $INPUT_FILE_PATH != remote:* ]]; then
            log_info "Checking if image exists in remote registry..."
            set +e
            ${DOCKER_ENGINE} manifest inspect $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY >/dev/null 2>&1
            chk $? "checking existence of image $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            set -e
        fi

        # REMOTE_IMAGE_REGISTRY_PATH is parts before /{arch}_anax_k8s, for example if using quay.io, this value will be quay.io/<username>
        local image_arch=$(get_cluster_image_arch)
        REMOTE_IMAGE_REGISTRY_PATH="${IMAGE_ON_EDGE_CLUSTER_REGISTRY%%/${image_arch}*}"
        log_info "REMOTE_IMAGE_REGISTRY_PATH: $REMOTE_IMAGE_REGISTRY_PATH"
        sed -i -e "s#__ImagePath__#${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}#g" deployment.yml
        sed -i -e "s#__ImageRegistryHost__#${REMOTE_IMAGE_REGISTRY_PATH}#g" deployment.yml

        if [[ "$USE_PRIVATE_REGISTRY" != "true" ]]; then
            log_debug "remote image registry is not private, remove ImagePullSecret..."
            sed -i -e '{/START_REMOTE_ICR/,/END_REMOTE_ICR/d;}' deployment.yml
        fi
    fi

    log_debug "prepare_k8s_deployment_file() end"
}

# Cluster only: to create auto-upgrade-cronjob.yml based on template
function prepare_k8s_auto_upgrade_cronjob_file() {
    log_debug "prepare_k8s_auto_upgrade_cronjob_file() begin"
    # Note: get_edge_cluster_files() already downloaded auto-upgrade-cronjob-template.yml, if necessary

    # check kubernetes version, if >= 1.21, use batch/v1, else use batch/v1beta1
    local kubernetes_api="batch/v1beta1"
    local kubernetes_version_to_compare=1.21
    local kubernetes_version=$(get_kubernetes_version)
    log_debug "kubernetes version is $kubernetes_version"
    if version_gt_or_equal $kubernetes_version $kubernetes_version_to_compare; then
        kubernetes_api="batch/v1"
    fi

    sed -e "s#__KubernetesApi__#${kubernetes_api}#g" -e "s#__ServiceAccount__#\"${SERVICE_ACCOUNT_NAME}\"#g" -e "s#__AgentNameSpace__#\"${AGENT_NAMESPACE}\"#g" auto-upgrade-cronjob-template.yml > auto-upgrade-cronjob.yml
    chk $? 'creating auto-upgrade-cronjob.yml'

    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        sed -i -e '{/START_REMOTE_ICR/,/END_REMOTE_ICR/d;}' auto-upgrade-cronjob.yml
        EDGE_CLUSTER_REGISTRY_PROJECT_NAME=$(echo $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $2}')
        EDGE_CLUSTER_CRONJOB_AUTO_UPGRADE_IMAGE_AND_TAG=$(echo $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $3}')

        local auto_upgrade_cronjob_image_full_path_on_edge_cluster_registry_internal_url
        if [[ "$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY" == "" ]]; then
            # check if using local cluster or remote ocp
            if is_ocp_cluster; then
                auto_upgrade_cronjob_image_full_path_on_edge_cluster_registry_internal_url=$DEFAULT_OCP_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_CRONJOB_AUTO_UPGRADE_IMAGE_AND_TAG
            else
                auto_upgrade_cronjob_image_full_path_on_edge_cluster_registry_internal_url="$CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            fi
        else
            auto_upgrade_cronjob_image_full_path_on_edge_cluster_registry_internal_url=$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
        fi
        sed -i -e "s#__ImagePath__#${auto_upgrade_cronjob_image_full_path_on_edge_cluster_registry_internal_url}#g" auto-upgrade-cronjob.yml
    else
        log_info "This agent install on edge cluster is using a remote registry: $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
        ## We want to avoid the requirement of docker/podman if $INPUT_FILE_PATH is "remote:*"
        if [[ $INPUT_FILE_PATH != remote:* ]]; then
            log_info "Checking if image exists in remote registry..."
            set +e
            ${DOCKER_ENGINE} manifest inspect $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY >/dev/null 2>&1
            chk $? "checking existence of image $CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            set -e
        fi

        sed -i -e "s#__ImagePath__#${CRONJOB_AUTO_UPGRADE_IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}#g" auto-upgrade-cronjob.yml

        if [[ "$USE_PRIVATE_REGISTRY" != "true" ]]; then
            log_debug "remote image registry is not private, remove ImagePullSecret..."
            sed -i -e '{/START_REMOTE_ICR/,/END_REMOTE_ICR/d;}' auto-upgrade-cronjob.yml
        fi
    fi

    log_debug "prepare_k8s_auto_upgrade_cronjob_file() end"
}

# Cluster only: to create persistenClaim.yml based on template
function prepare_k8s_pvc_file() {
    log_debug "prepare_k8s_pvc_file() begin"

    # Note: get_edge_cluster_files() already downloaded deployment-template.yml, if necessary
    local pvc_mode="ReadWriteOnce"
    number_of_nodes=$($KUBECTL get node | grep "Ready" -c)
    if [[ $number_of_nodes -gt 1 ]] && ([[ $EDGE_CLUSTER_STORAGE_CLASS == csi-cephfs* ]] || [[ $EDGE_CLUSTER_STORAGE_CLASS == rook-cephfs* ]] || [[ $EDGE_CLUSTER_STORAGE_CLASS == ibmc-file* ]] || [[ $EDGE_CLUSTER_STORAGE_CLASS == ibmc-vpc-file* ]]); then
        pvc_mode="ReadWriteMany"
    fi

    if [[ -z $CLUSTER_PVC_SIZE ]]; then
        CLUSTER_PVC_SIZE=$DEFAULT_PVC_SIZE
    fi

    sed -e "s#__AgentNameSpace__#${AGENT_NAMESPACE}#g" -e "s/__StorageClass__/\"${EDGE_CLUSTER_STORAGE_CLASS}\"/g" -e "s#__PVCAccessMode__#${pvc_mode}#g" -e "s#__PVCStorageSize__#${CLUSTER_PVC_SIZE}#g" persistentClaim-template.yml >persistentClaim.yml
    chk $? 'creating persistentClaim.yml'

    log_debug "prepare_k8s_pvc_file() end"
}

# Cluster only: to create image stream under namespace for ocp cluster
function create_image_stream() {
    log_info "create_image_stream() begin"

    if ! $KUBECTL get imagestream agent -n ${AGENT_NAMESPACE} 2>/dev/null; then
        $KUBECTL create imagestream agent -n ${AGENT_NAMESPACE}
        chk $? "creating imagestream under namespace ${AGENT_NAMESPACE}"
        log_info "imagestream under namespace ${AGENT_NAMESPACE} created"
    else
        log_info "imagestream under namespace ${AGENT_NAMESPACE} exists, skip creating imagestream"
    fi

    log_info "create_image_stream() end"
}

# Cluster only: to create cluster resources
function create_cluster_resources() {
    log_debug "create_cluster_resources() begin"

    create_namespace
    local namespace_wait_seconds=10
    if ! wait_for '[[ -n $($KUBECTL get namespace ${AGENT_NAMESPACE} 2>/dev/null) ]]' 'agent install namespace created' $namespace_wait_seconds; then
        log_fatal 3 "${AGENT_NAMESPACE} did not created successfully"
    fi
    create_service_account
    create_secret
    create_configmap
    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
        create_cronjobs
    fi
    create_persistent_volume

    log_debug "create_cluster_resources() end"
}

# Cluster only: to update cluster resources
function update_cluster_resources() {
    log_debug "update_cluster_resources() begin"

    update_secret
    update_configmap
    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
        update_cronjobs
    fi

    log_debug "update_cluster_resources() end"
}

# Cluster only: to create namespace that agent will be deployed
function create_namespace() {
    log_debug "create_namespace() begin"
    # check if namespace exists, if not, create
    log_verbose "checking if namespace exist..."

    if ! $KUBECTL get namespace ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "namespace ${AGENT_NAMESPACE} does not exist, creating..."
        log_debug "command: $KUBECTL create namespace ${AGENT_NAMESPACE}"
        $KUBECTL create namespace ${AGENT_NAMESPACE}
        chk $? "creating namespace ${AGENT_NAMESPACE}"
        log_info "namespace ${AGENT_NAMESPACE} created"
    else
        log_info "namespace ${AGENT_NAMESPACE} exists, skip creating namespace"
    fi

    local ocp_supplemental_groups=$($KUBECTL get namespace ${AGENT_NAMESPACE}  -o json | jq -r '.metadata.annotations' | jq '.["openshift.io/sa.scc.supplemental-groups"]')
    local ocp_scc_uid_range=$($KUBECTL get namespace ${AGENT_NAMESPACE}  -o json | jq -r '.metadata.annotations' | jq '.["openshift.io/sa.scc.uid-range"]')
    if [[ -n $ocp_supplemental_groups ]] && [[ "$ocp_supplemental_groups" != "null" ]]  && [[ -n $ocp_scc_uid_range ]] && [[ "$ocp_scc_uid_range" != "null" ]]; then
        # if it has ocp supplementl group and uid range annotation, then update the annotation of namespace
        log_info "update annotation of namespace ${AGENT_NAMESPACE}"
        $KUBECTL annotate namespace ${AGENT_NAMESPACE} openshift.io/sa.scc.uid-range='1000/1000' openshift.io/sa.scc.supplemental-groups='1000/1000' --overwrite
    fi

    log_debug "create_namespace() end"
}

# Cluster only: to create service account for agent namespace and binding to cluster-admin clusterrole
function create_service_account() {
    log_debug "create_service_account() begin"

    log_verbose "checking if serviceaccount exist..."
    if ! $KUBECTL get serviceaccount ${SERVICE_ACCOUNT_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "serviceaccount ${SERVICE_ACCOUNT_NAME} does not exist, creating..."
        $KUBECTL create serviceaccount ${SERVICE_ACCOUNT_NAME} -n ${AGENT_NAMESPACE}
        chk $? "creating service account ${SERVICE_ACCOUNT_NAME}"
        log_info "serviceaccount ${SERVICE_ACCOUNT_NAME} created"

    else
        log_info "serviceaccount ${SERVICE_ACCOUNT_NAME} exists, skip creating serviceaccount"
    fi

    create_cluster_role_binding

    log_debug "create_service_account() end"
}

# Cluster only: to create cluster role binding, bind service account to cluster admin
function create_cluster_role_binding() {
    log_debug "create_cluster_role_binding() begin"

    log_verbose "checking if clusterrolebinding exist..."

    local clusterRoleBindingName=${AGENT_NAMESPACE}-${CLUSTER_ROLE_BINDING_NAME}
    if ! $KUBECTL get clusterrolebinding ${clusterRoleBindingName} 2>/dev/null; then
        log_verbose "Binding ${SERVICE_ACCOUNT_NAME} to cluster admin..."
        $KUBECTL create clusterrolebinding ${clusterRoleBindingName} --serviceaccount=${AGENT_NAMESPACE}:${SERVICE_ACCOUNT_NAME} --clusterrole=cluster-admin
        chk $? "creating clusterrolebinding for ${AGENT_NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
        log_info "clusterrolebinding ${clusterRoleBindingName} created"
    else
        log_info "clusterrolebinding ${clusterRoleBindingName} exists, skip creating clusterrolebinding"
    fi

    log_debug "create_cluster_role_binding() end"
}

# Cluster only: to create secret from cert file for agent deployment
function create_secret() {
    log_debug "create_secrets() begin"

    if [[ ! -f $AGENT_CERT_FILE ]]; then
        log_debug "agent cert file is not used, skip creating secret ..."
        sed -i -e '{/START_CERT_VOL/,/END_CERT_VOL/d;}' deployment-template.yml
    else
        log_verbose "checking if secret ${SECRET_NAME} exist..."

        if ! $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
            log_verbose "creating secret for cert file..."
            $KUBECTL create secret generic ${SECRET_NAME} --from-file=${AGENT_CERT_FILE} -n ${AGENT_NAMESPACE}
            chk $? "creating secret ${SECRET_NAME} from cert file ${AGENT_CERT_FILE}"
            log_info "secret ${SECRET_NAME} created"
        else
            log_info "secret ${SECRET_NAME} exists, skip creating secret"
        fi
    fi
    log_debug "create_secrets() end"
}

# Cluster only: to update secret
function update_secret() {
    log_debug "update_secret() begin"

    if $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
        # secret exists, delete it
        log_verbose "Found secret ${SECRET_NAME} in ${AGENT_NAMESPACE} namespace, deleting the old secret..."
        $KUBECTL delete secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
        chk $? "deleting secret for agent update on cluster"
        log_verbose "Old secret ${SECRET_NAME} in ${AGENT_NAMESPACE} namespace is deleted"
    fi

    create_secret

    log_debug "update_secret() end"
}

# Cluster only: to create configmap based on /tmp/agent-install-horizon-env for agent deployment
function create_configmap() {
    log_debug "create_configmap() begin"

    log_verbose "checking if configmap ${CONFIGMAP_NAME} exist..."

    if ! $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "create configmap from ${HZN_ENV_FILE}..."
        create_horizon_env
        $KUBECTL create configmap ${CONFIGMAP_NAME} --from-file=horizon=${HZN_ENV_FILE} -n ${AGENT_NAMESPACE}
        chk $? "creating configmap ${CONFIGMAP_NAME} from ${HZN_ENV_FILE}"
        log_info "configmap ${CONFIGMAP_NAME} created."
        rm $HZN_ENV_FILE
        chk $? "removing $HZN_ENV_FILE"
    else
        log_info "configmap ${CONFIGMAP_NAME} exists, skip creating configmap"
    fi

    log_debug "create_configmap() end"
}

# Cluster only: to update configmap based on /tmp/agent-install-horizon-env for agent deployment
function update_configmap() {
    log_debug "update_configmap() begin"

    if [[ "$IS_HORIZON_DEFAULTS_CORRECT" == "true" ]]; then
        log_verbose "Values in configmap are same, will skip updating configmap"
    else
        if $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
            # configmap exists, delete it
	        log_verbose "Found configmap ${CONFIGMAP_NAME} in ${AGENT_NAMESPACE} namespace, deleting the old configmap..."
            $KUBECTL delete configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
            chk $? 'deleting the old configmap for agent update on cluster'
            log_verbose "Old configmap ${CONFIGMAP_NAME} in ${AGENT_NAMESPACE} namespace is deleted"
        fi

        create_configmap

    fi

    log_debug "update_configmap() end"
}

# Cluster only: to create cronjobs from {prefix}-cronjob.yml files
function create_cronjobs() {
    log_debug "create_cronjobs() begin"

    # For auto-upgrade-cronjob
    log_verbose "checking if cronjob ${CRONJOB_AUTO_UPGRADE_NAME} exists..."

    if ! $KUBECTL get cronjob ${CRONJOB_AUTO_UPGRADE_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "creating cronjob for auto-upgrade-cronjob.yml file..."
        $KUBECTL create -f auto-upgrade-cronjob.yml -n ${AGENT_NAMESPACE}
        chk $? "creating cronjob ${CRONJOB_AUTO_UPGRADE_NAME} from yml file auto-upgrade-cronjob.yml"
        log_info "cronjob ${CRONJOB_AUTO_UPGRADE_NAME} created"
    else
        log_info "cronjob ${CRONJOB_AUTO_UPGRADE_NAME} exists, skip creating cronjob"
    fi

    log_debug "create_cronjobs() end"
}

# Cluster only: to update cronjobs
function update_cronjobs() {
    log_debug "update_cronjobs() begin"

    # For auto-upgrade-cronjob
    if [[ "$IS_CRONJOB_AUTO_UPGRADE_IMAGE_VERSION_SAME" != "true" ]]; then
        if $KUBECTL get cronjob ${CRONJOB_AUTO_UPGRADE_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
            # cronjob exists, delete it
            log_verbose "Found cronjob ${CRONJOB_AUTO_UPGRADE_NAME} in ${AGENT_NAMESPACE} namespace, deleting the old cronjob..."
            $KUBECTL delete cronjob ${CRONJOB_AUTO_UPGRADE_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
            chk $? "deleting cronjob for auto-upgrade-cronjob on cluster"
            log_verbose "Old cronjob ${CRONJOB_AUTO_UPGRADE_NAME} in ${AGENT_NAMESPACE} namespace is deleted"
        fi
        create_cronjobs
    fi
    log_debug "update_cronjobs() end"
}

# Cluster only: to create persistent volume claim for agent deployment
function create_persistent_volume() {
    log_debug "create_persistent_volume() begin"

    log_verbose "checking if persistent volume claim ${PVC_NAME} exists..."
    if ! $KUBECTL get pvc ${PVC_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "creating persistent volume claim..."
        $KUBECTL apply -f persistentClaim.yml -n ${AGENT_NAMESPACE}
        chk $? 'creating persistent volume claim'
        log_info "persistent volume claim created"
    else
        log_info "persistent volume claim ${PVC_NAME} exists, skip creating persistent volume claim"
    fi

    log_debug "create_persistent_volume() end"
}

# Cluster only: to check secret, configmap, pvc is created. Returns 0 (true) if they are all ready.
function check_resources_for_deployment() {
    log_debug "check_resources_for_deployment() begin"
    # check secrets/configmap/persistent
    if [[ -f $AGENT_CERT_FILE ]]; then
        $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null
        secret_ready=$?
    else
        secret_ready=0
    fi

    $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} >/dev/null
    configmap_ready=$?

    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
        $KUBECTL get cronjob ${CRONJOB_AUTO_UPGRADE_NAME} -n ${AGENT_NAMESPACE} >/dev/null
        auto_upgrade_cronjob_ready=$?
    else
        auto_upgrade_cronjob_ready=0
    fi

    $KUBECTL get pvc ${PVC_NAME} -n ${AGENT_NAMESPACE} >/dev/null
    pvc_ready=$?

    if [[ ${secret_ready} -eq 0 && ${configmap_ready} -eq 0 && ${pvc_ready} -eq 0 && ${auto_upgrade_cronjob_ready} -eq 0 ]]; then
        return 0
    else
        return 1
    fi

    log_debug "check_resources_for_deployment() end"
}

# Cluster only: to create deployment
function create_deployment() {
    log_debug "create_deployment() begin"

    log_verbose "creating deployment under namespace $AGENT_NAMESPACE..."
    $KUBECTL apply -f deployment.yml -n ${AGENT_NAMESPACE}
    chk $? 'creating deployment'

    log_debug "create_deployment() end"
}

# Cluster only: to update deployment
function update_deployment() {
    log_debug "update_deployment() begin"

    if [[ "$IS_AGENT_IMAGE_VERSION_SAME" == "true" ]] && [[ "$IS_HORIZON_ORG_ID_SAME" == "true" ]]; then
        log_info "Agent image version and HZN_ORG_ID are the same. Keeping the existing agent deployment. Deleting agent pod ${POD_ID} in namespace ${AGENT_NAMESPACE} to pickup changes in configmap and secrets"
        $KUBECTL delete pod ${POD_ID} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
        chk $? 'deleting the old pod for agent update on cluster'
    else
        if $KUBECTL get deployment ${DEPLOYMENT_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
            # deployment exists, delete it
            log_verbose "Found deployment ${DEPLOYMENT_NAME} in ${AGENT_NAMESPACE} namespace, deleting it..."
            $KUBECTL delete deployment ${DEPLOYMENT_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
            chk $? 'deleting the old deployment for agent update on cluster'

            # wait for the pod to terminate
            # the 1st arg to wait_for() means keep waiting until the exit code of kubectl is exactly 1
            if wait_for '! ( $KUBECTL -n $AGENT_NAMESPACE get pod $POD_ID >/dev/null 2>&1 || [[ $? -ge 2 ]] )' 'Horizon agent terminated' $AGENT_WAIT_MAX_SECONDS; then
                log_verbose "Horizon agent pod terminated successfully"
            else
                AGENT_POD_STATUS=$($KUBECTL -n $AGENT_NAMESPACE get pod $POD_ID 2>/dev/null | grep -E '^agent-' | awk '{print $3;}')
                if [[ $AGENT_POD_STATUS == "Terminating" ]]; then
                    if [[ $AGENT_SKIP_PROMPT == 'false' ]]; then
                        echo "Agent pod ${POD_ID} is still in Terminating, force delete pod?[y/N]:"
                        read RESPONSE
                        if [[ "$RESPONSE" == 'y' ]]; then
                            log_verbose "Force deleting agent pod ${POD_ID}..."
                            $KUBECTL delete pod ${POD_ID} --force=true --grace-period=0 -n ${AGENT_NAMESPACE}
                            chk $? 'deleting the Terminating pod for agent update on cluster'
                            pkill -f anax.service
                        else
                            log_fatal 3 "Agent pod is not force deleted. Please manually delete the agent pod and run agent-install.sh again"
                        fi
                    else
                        log_verbose "Agent pod ${POD_ID} is still in Terminating, force deleting..."
                        $KUBECTL delete pod ${POD_ID} --force=true --grace-period=0 -n ${AGENT_NAMESPACE}
                        chk $? 'deleting the Terminating pod for agent update on cluster'
                        pkill -f anax.service
                    fi
                else
                    log_fatal 3 "Unexpected status of agent pod $POD_ID: $AGENT_POD_STATUS"
                fi
            fi
        fi

        create_deployment
    fi
    log_debug "update_deployment() end"
}

# Cluster only: to check agent deplyment status
function check_deployment_status() {
    log_debug "check_deployment_status() begin"
    log_info "Waiting up to $AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS seconds for the agent deployment to complete..."

    DEP_STATUS=$($KUBECTL rollout status --timeout=${AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS}s deployment/agent -n ${AGENT_NAMESPACE} | grep "successfully rolled out")
    if [[ -z "$DEP_STATUS" ]]; then
        log_fatal 3 "Deployment rollout status failed"
    fi
    log_debug "check_deployment_status() end"
}

# Cluster only: to get agent pod id
function get_pod_id() {
    log_debug "get_pod_id() begin"

    if ! wait_for '[[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent,type!=auto-upgrade-cronjob -o "jsonpath={..status.conditions[?(@.type==\"Ready\")].status}") == "True" ]]' 'Horizon agent pod ready' $AGENT_WAIT_MAX_SECONDS; then
        log_fatal 3 "Horizon agent pod did not start successfully"
    fi

    if [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent,type!=auto-upgrade-cronjob -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
        log_fatal 3 "Failed to get agent pod in Ready status"
    fi

    POD_ID=$($KUBECTL get pod -l app=agent --field-selector status.phase=Running -n ${AGENT_NAMESPACE} 2>/dev/null | grep "agent-" | cut -d " " -f1 2>/dev/null)
    if [[ -n "${POD_ID}" ]]; then
        log_verbose "got pod: ${POD_ID}"
    else
        log_fatal 3 "Failed to get pod id"
    fi
    log_debug "get_pod_id() end"
}

# Cluster only: to setup agent deployment to use secret contains image registry cert
function setup_cluster_image_registry_cert() {
    log_debug "setup_cluster_image_registry_cert() begin"

    local isUpdate=$1
    if [[ "$isUpdate" == "true" ]]; then
        log_verbose "updating cluster image registry cert in $IMAGE_REGISTRY_SECRET_NAME"
        update_secret_for_image_reigstry_cert
    else
        log_verbose "creating cluster image registry cert in $IMAGE_REGISTRY_SECRET_NAME"
        create_secret_for_image_reigstry_cert
    fi

    $KUBECTL scale --replicas=0 deployment.app/agent -n ${AGENT_NAMESPACE}
    patch_deployment_with_image_registry_volume
    $KUBECTL scale --replicas=1 deployment.app/agent -n ${AGENT_NAMESPACE}

    log_debug "setup_cluster_image_registry_cert() end"
}

# Cluster only: to create image registry cert in secret
function create_secret_for_image_reigstry_cert() {
    log_debug "create_secret_for_image_reigstry_cert() begin"

    log_verbose "checking if secret ${IMAGE_REGISTRY_SECRET_NAME} exist..."

    if ! $KUBECTL get secret ${IMAGE_REGISTRY_SECRET_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        local image_registry_cert_file="/etc/docker/certs.d/$EDGE_CLUSTER_REGISTRY_HOST/ca.crt"
        log_verbose "creating secret for image registry cert file at ${image_registry_cert_file} ..."
        $KUBECTL create secret generic ${IMAGE_REGISTRY_SECRET_NAME} --from-file=${image_registry_cert_file} -n ${AGENT_NAMESPACE}
        chk $? "creating secret ${IMAGE_REGISTRY_SECRET_NAME} from cert file ${image_registry_cert_file}"
        log_info "secret ${IMAGE_REGISTRY_SECRET_NAME} created"
    else
        log_info "secret ${IMAGE_REGISTRY_SECRET_NAME} exists, skip creating secret"
    fi

    log_debug "create_secret_for_image_reigstry_cert() end"
}

# Cluster only: to update image registry cert in secret
function update_secret_for_image_reigstry_cert() {
    log_debug "update_secret_for_image_reigstry_cert() begin"

    if $KUBECTL get secret ${IMAGE_REGISTRY_SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
        # secret exists, delete it
        log_verbose "Find secret ${IMAGE_REGISTRY_SECRET_NAME} in ${AGENT_NAMESPACE} namespace, deleting the old secret..."
        $KUBECTL delete secret ${IMAGE_REGISTRY_SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
        chk $? "deleting secret for agent update on cluster"
        log_verbose "Old secret ${IMAGE_REGISTRY_SECRET_NAME} in ${AGENT_NAMESPACE} namespace is deleted"
    fi

    create_secret_for_image_reigstry_cert

    log_debug "update_secret_for_image_reigstry_cert() end"
}

# Cluster only: to patch the agent deploiyment to use the secret contains image registry
function patch_deployment_with_image_registry_volume() {
    log_debug "patch_deployment_with_image_registry_volume() begin"

    $KUBECTL patch deployment agent -n ${AGENT_NAMESPACE} -p "{\"spec\":{\"template\":{\"spec\":{\"volumes\":[{\"name\": \
    \"agent-docker-cert-volume\",\"secret\":{\"secretName\":\"${IMAGE_REGISTRY_SECRET_NAME}\"}}], \
    \"containers\":[{\"name\":\"anax\",\"volumeMounts\":[{\"mountPath\":\"/etc/docker/certs.d/${EDGE_CLUSTER_REGISTRY_HOST}\" \
    ,\"name\":\"agent-docker-cert-volume\"},{\"mountPath\":\"/etc/docker/certs.d/${DEFAULT_OCP_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY}\" \
    ,\"name\":\"agent-docker-cert-volume\"}],\"env\":[{\"name\":\"SSL_CERT_FILE\",\"value\":\"/etc/docker/certs.d/${EDGE_CLUSTER_REGISTRY_HOST}/ca.crt\"}]}]}}}}"

    log_debug "patch_deployment_with_image_registry_volume() end"
}

# Cluster only: to install/update agent in cluster
function install_update_cluster() {
    log_debug "install_update_cluster() begin"

    if [[ $INPUT_FILE_PATH != remote:* ]]; then
        confirmCmds ${DOCKER_ENGINE}
    fi

    confirmCmds jq

    check_existing_exch_node_is_correct_type "cluster"

    check_cluster_agent_scope   # sets AGENT_DEPLOYMENT_EXIST_IN_SAME_NAMESPACE

    loadClusterAgentImage   # create the cluster agent docker image locally
    if [[ "$ENABLE_AUTO_UPGRADE_CRONJOB" == "true" ]]; then
        loadClusterAgentAutoUpgradeCronJobImage # create the cluster cronjob docker images locally
    fi

    getImageRegistryInfo # set $EDGE_CLUSTER_REGISTRY_HOST, and login to registry

    # push agent and cronjob images to cluster's registry
    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        if is_ocp_cluster; then
            create_namespace
            create_image_stream
        fi
    else
        log_info "Use remote registry"
        create_namespace
        create_image_pull_secrets # create image pull secrets if use private registry (if edge cluster registry username/password are provided), sets USE_PRIVATE_REGISTRY
    fi

    if [[ $INPUT_FILE_PATH != remote:* ]]; then
        pushImagesToEdgeClusterRegistry
    fi

    if [[ "$AGENT_DEPLOYMENT_EXIST_IN_SAME_NAMESPACE" == "true" ]]; then
        check_agent_deployment_exist   # sets AGENT_DEPLOYMENT_UPDATE POD_ID
    fi

    if [[ "$AGENT_DEPLOYMENT_UPDATE" == "true" ]]; then
        log_info "Update agent on edge cluster"
        update_cluster
    else
        log_info "Install agent on edge cluster"
        install_cluster
    fi

    log_debug "install_update_cluster() end"
}

# Cluster only: to install agent in cluster
function install_cluster() {
    log_debug "install_cluster() begin"

    # generate files based on templates
    generate_installation_files

    # create cluster namespace and resources
    create_cluster_resources

    while ! check_resources_for_deployment && [[ ${GET_RESOURCE_MAX_TRY} -gt 0 ]]; do
        count=$((GET_RESOURCE_MAX_TRY - 1))
        GET_RESOURCE_MAX_TRY=$count
    done
    #lily: shouldn't we handle the case where GET_RESOURCE_MAX_TRY reached 0 but the resources still weren't ready?

    # get pod information
    create_deployment
    check_deployment_status

    if is_ocp_cluster && [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        # setup image registry cert. This will patch the running deployment
        local isUpdate='false'
        setup_cluster_image_registry_cert $isUpdate
    fi

    check_deployment_status
    get_pod_id

    # register
    registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"
    log_debug "install_cluster() end"
}

# Cluster only: to update agent in cluster
function update_cluster() {
    log_debug "update_cluster() begin"

    set +e
    is_horizon_defaults_correct "$ANAX_PORT"
    set -e

    if is_agent_registered; then
        if [[ "$IS_HORIZON_DEFAULTS_CORRECT" != "true" ]] || ! is_registration_correct ; then
	        unregister
        fi
    fi

    generate_installation_files

    update_cluster_resources

    while ! check_resources_for_deployment && [[ ${GET_RESOURCE_MAX_TRY} -gt 0 ]]; do
        count=$((GET_RESOURCE_MAX_TRY - 1))
        GET_RESOURCE_MAX_TRY=$count
    done
    #lily: shouldn't we handle the case where GET_RESOURCE_MAX_TRY reached 0 but the resources still weren't ready?

    update_deployment
    check_deployment_status
    if is_ocp_cluster && [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        # setup image registry cert. This will patch the running deployment
        local isUpdate='true'
        setup_cluster_image_registry_cert $isUpdate
    fi

    check_deployment_status
    get_pod_id

    registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY" "$HZN_HA_GROUP"

    log_debug "update_cluster() end"
}


# A RedHat system might use podman instead of docker; default to docker
function get_docker_engine() {

    log_debug "get_docker_engine() begin"

    if is_linux; then
        if isCmdInstalled docker; then
            : # use docker
        elif isCmdInstalled podman;  then
            # podman is installed... lets make sure it is acceptable version (ie > 4.0.0)
            podman_ver=$(podman --version)
            rc=$?
            if [[ $rc -eq 0 ]]; then
               # should be of form 'podman version 4.0.0'
               log_debug "podman version string - ${podman_ver}"
               OLDIFS=${IFS}
               IFS=' '
               read -a podman_ver_array <<< "${podman_ver}"
               if [[ ${#podman_ver_array[@]} -eq 3 ]]; then
                  IFS='.'
                  read -a podman_ver_num_array <<< "${podman_ver_array[2]}"
                  major_version=$(expr "${podman_ver_num_array[0]}" + 0)
                  if [[ $major_version -ge 4 ]]; then
                     DOCKER_ENGINE="podman"
                  fi
               fi
               IFS=${OLDIFS}

               if [[ ${DOCKER_ENGINE} == "podman" ]]; then
                  NetworkBackend=$(podman info -f json | jq '.host.networkBackend' | sed 's/"//g')
                  log_debug "podman network backend is set for ${NetworkBackend}"
                  if [[ "${NetworkBackend}" != "netavark" ]]; then
                     log_warning "Change podman to use the Netavark network stack to support more complex horizon scenarios"
                  fi
               fi
            fi
        fi
    fi

    log_info "DOCKER_ENGINE set to ${DOCKER_ENGINE}"
    log_debug "get_docker_engine() end"
}

#====================== Main Code ======================

# Get and verify all input
# Note: the cmd line args/flags were already parsed at the top of this script (and could not put that in a function, because getopts would only process the function args)
get_all_variables   # this will also output the value of each inputted arg/var
check_variables

find_node_id_in_mapping_file   # for bulk install. Sets NODE_ID if it finds it in mapping file. Also sets BULK_INSTALL

log_info "Node type: ${AGENT_DEPLOY_TYPE}"

get_docker_engine

if is_device; then
    check_device_os   # sets: PACKAGES

    if is_linux; then
        if is_debian_variant; then
            if is_anax_in_container; then
                install_debian_container
            else
                install_debian
            fi
        elif is_redhat_variant; then
            if is_anax_in_container; then
                install_redhat_container
            else
                install_redhat
            fi
        else
            log_fatal 5 "Unrecognized distro: $DISTRO"
        fi
    elif is_macos; then
        install_macos
    else
        log_fatal 5 "Unrecognized OS: $OS"
    fi

    add_autocomplete

elif is_cluster; then
    log_verbose "Install/Update agent on edge cluster"
    set +e
    install_update_cluster
    set -e
else
    log_fatal 1 "AGENT_DEPLOY_TYPE must be 'device' or 'cluster'"
fi
