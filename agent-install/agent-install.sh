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
SUPPORTED_DEBIAN_VERSION=(focal bionic buster xenial stretch $SUPPORTED_DEBIAN_VERSION_APPEND)   # compared to what our detect_distro() sets CODENAME to
SUPPORTED_DEBIAN_ARCH=(amd64 arm64 armhf $SUPPORTED_DEBIAN_ARCH_APPEND)   # compared to dpkg --print-architecture
SUPPORTED_REDHAT_VARIANTS=(rhel centos $SUPPORTED_REDHAT_VARIANTS_APPEND)   # compared to what our detect_distro() sets DISTRO to
# Note: RHEL 8.3 is not officially supported yet, but is enabled only for testing and tech preview purposes
# Note: version 8 is added because that is what /etc/os-release returns for DISTRO_VERSION_NUM on centos
SUPPORTED_REDHAT_VERSION=(7.6 7.9 8.1 8.2 8.3 8.4 8.5 8 32 35 $SUPPORTED_REDHAT_VERSION_APPEND)   # compared to what our detect_distro() sets DISTRO_VERSION_NUM to. For fedora versions see https://fedoraproject.org/wiki/Releases,
SUPPORTED_REDHAT_ARCH=(x86_64 aarch64 ppc64le riscv64 $SUPPORTED_REDHAT_ARCH_APPEND)     # compared to uname -m

SUPPORTED_OS=(macos linux)   # compared to what our get_os() returns
SUPPORTED_LINUX_DISTRO=(${SUPPORTED_DEBIAN_VARIANTS[@]} ${SUPPORTED_REDHAT_VARIANTS[@]})   # compared to what our detect_distro() sets DISTRO to

HOSTNAME=$(hostname -s)
MAC_PACKAGE_CERT="horizon-cli.crt"
PERMANENT_CERT_PATH='/etc/horizon/agent-install.crt'
ANAX_DEFAULT_PORT=8510
AGENT_CERT_FILE_DEFAULT='agent-install.crt'
AGENT_CFG_FILE_DEFAULT='agent-install.cfg'
CSS_OBJ_PATH_DEFAULT='/api/v1/objects/IBM/agent_files'
SEMVER_REGEX='^[0-9]+\.[0-9]+(\.[0-9]+)+'   # matches a version like 1.2.3 (must be at least 3 fields). Also allows a bld num on the end like: 1.2.3-RC1
DEFAULT_AGENT_IMAGE_TAR_FILE='amd64_anax.tar.gz'

# edge cluster agent deployment
SERVICE_ACCOUNT_NAME="agent-service-account"
CLUSTER_ROLE_BINDING_NAME="openhorizon-agent-cluster-rule"
DEPLOYMENT_NAME="agent"
SECRET_NAME="openhorizon-agent-secrets"
CONFIGMAP_NAME="openhorizon-agent-config"
PVC_NAME="openhorizon-agent-pvc"
GET_RESOURCE_MAX_TRY=5
POD_ID=""
HZN_ENV_FILE="/tmp/agent-install-horizon-env"
DEFAULT_OCP_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY="image-registry.openshift-image-registry.svc:5000"
DEFAULT_AGENT_K8S_IMAGE_TAR_FILE='amd64_anax_k8s.tar.gz'
EDGE_CLUSTER_TAR_FILE_NAME='horizon-agent-edge-cluster-files.tar.gz'


# Type of container engine to use; RedHat might have podman
DOCKER_ENGINE="docker"


# Script usage info
function usage() {
    local exit_code=$1
    cat <<EndOfMessage
${0##*/} <options>

Install the Horizon agent on an edge device or edge cluster.

Required Input Variables (via flag, environment, or config file):
    HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_ORG_ID, either HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH

Options/Flags:
    -c    Path to a certificate file. Default: ./$AGENT_CERT_FILE_DEFAULT, if present. If the argument begins with 'css:' (e.g. css:$CSS_OBJ_PATH_DEFAULT), it will download the config file from the MMS. If only 'css:' is specified, the default path $CSS_OBJ_PATH_DEFAULT will be added. (This flag is equivalent to AGENT_CERT_FILE or HZN_MGMT_HUB_CERT_PATH)
    -k    Path to a configuration file. Default: ./$AGENT_CFG_FILE_DEFAULT, if present. If the argument begins with 'css:' (e.g. css:$CSS_OBJ_PATH_DEFAULT), it will download the config file from the MMS. If only 'css:' is specified, the default path $CSS_OBJ_PATH_DEFAULT will be added. All other variables for this script can be specified in the config file, except for INPUT_FILE_PATH (and HZN_ORG_ID if -i css: is specified). (This flag is equivalent to AGENT_CFG_FILE)
    -i    Installation packages/files location (default: current directory). If the argument is the URL of an anax git repo release (e.g. https://github.com/open-horizon/anax/releases/download/v1.2.3) it will download the appropriate packages/files from there. If it is anax: or https://github.com/open-horizon/anax/releases , it will default to the latest release. Otherwise, if the argument begins with 'http' or 'https', it will be used as an APT repository (for debian hosts). If the argument begins with 'css:' (e.g. css:$CSS_OBJ_PATH_DEFAULT), it will download the appropriate files/packages from the MMS. If only 'css:' is specified, the default path $CSS_OBJ_PATH_DEFAULT will be added. (This flag is equivalent to INPUT_FILE_PATH)
    -z    The name of your agent installation tar file. Default: ./agent-install-files.tar.gz (This flag is equivalent to AGENT_INSTALL_ZIP)
    -j    File location for the public key for an APT repository specified with '-i' (This flag is equivalent to PKG_APT_KEY)
    -t    Branch to use in the APT repo specified with -i. Default is 'updates' (This flag is equivalent to APT_REPO_BRANCH)
    -O    The exchange organization id (This flag is equivalent to HZN_ORG_ID)
    -u    Exchange user authorization credentials (This flag is equivalent to HZN_EXCHANGE_USER_AUTH)
    -a    Exchange node authorization credentials (This flag is equivalent to HZN_EXCHANGE_NODE_AUTH)
    -d    The id to register this node with (This flag is equivalent to HZN_NODE_ID, NODE_ID or HZN_DEVICE_ID. NODE_ID is deprecated)
    -p    Pattern name to register this edge node with. Default: registers node with policy. (This flag is equivalent to HZN_EXCHANGE_PATTERN)
    -n    Path to a node policy file (This flag is equivalent to HZN_NODE_POLICY)
    -w    Wait for this edge service to start executing on this node before this script exits. If using a pattern, this value can be '*'. (This flag is equivalent to AGENT_WAIT_FOR_SERVICE)
    -T    Timeout value (in seconds) for how long to wait for the service to start (This flag is equivalent to AGENT_REGISTRATION_TIMEOUT)
    -o    Specify an org id for the service specified with '-w'. Defaults to the value of HZN_ORG_ID. (This flag is equivalent to AGENT_WAIT_FOR_SERVICE_ORG)
    -s    Skip registration, only install the agent (This flag is equivalent to AGENT_SKIP_REGISTRATION)
    -D    Node type of agent being installed: device, cluster. Default: device. (This flag is equivalent to AGENT_DEPLOY_TYPE)
    -U    Internal url for edge cluster registry. If not specified, this script will auto-detect the value if it is a small, single-node cluster (e.g. k3s or microk8s). For OCP use: image-registry.openshift-image-registry.svc:5000. (This flag is equivalent to INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY)
    -l    Logging verbosity level. Display messages at this level and lower: 1: error, 2: warning, 3: info (default), 4: verbose, 5: debug. Default is 3, info. (This flag is equivalent to AGENT_VERBOSITY)
    -f    Install older version of the horizon agent and CLI packages. (This flag is equivalent to AGENT_OVERWRITE)
    -b    Skip any prompts for user input (This flag is equivalent to AGENT_SKIP_PROMPT)
    -C    Install only the horizon-cli package, not the full agent (This flag is equivalent to AGENT_ONLY_CLI)
    -h    This usage

Additional Variables (in environment or config file):
    HZN_AGBOT_URL: The URL that is used for the 'hzn agbot ...' commands.
    HZN_SDO_SVC_URL: The URL that is used for the 'hzn voucher ...' and 'hzn sdo ...' commands.

Additional Edge Device Variables (in environment or config file):
    NODE_ID_MAPPING_FILE: File to map hostname or IP to node id, for bulk install.  Default: node-id-mapping.csv
    AGENT_IMAGE_TAR_FILE: the file name of the device agent docker image in tar.gz format. Default: $DEFAULT_AGENT_IMAGE_TAR_FILE
    AGENT_WAIT_MAX_SECONDS: Maximum seconds to wait for the Horizon agent to start or stop. Default: 30

Optional Edge Device Environment Variables For Testing New Distros - Not For Production Use
    SUPPORTED_DEBIAN_VARIANTS_APPEND: a debian variant that should be added to the default list: ${SUPPORTED_DEBIAN_VARIANTS[*]}
    SUPPORTED_DEBIAN_VERSION_APPEND: a debian version that should be added to the default list: ${SUPPORTED_DEBIAN_VERSION[*]}
    SUPPORTED_DEBIAN_ARCH_APPEND: a debian architecture that should be added to the default list: ${SUPPORTED_DEBIAN_ARCH[*]}
    SUPPORTED_REDHAT_VARIANTS_APPEND: a Red Hat variant that should be added to the default list: ${SUPPORTED_REDHAT_VARIANTS[*]}
    SUPPORTED_REDHAT_VERSION_APPEND: a Red Hat version that should be added to the default list: ${SUPPORTED_REDHAT_VERSION[*]}
    SUPPORTED_REDHAT_ARCH_APPEND: a Red Hat architecture that should be added to the default list: ${SUPPORTED_REDHAT_ARCH[*]}


Additional Edge Cluster Variables (in environment or config file):
    IMAGE_ON_EDGE_CLUSTER_REGISTRY: override the agent image path (without tag) if you want it to be different from what this script will default it to
    EDGE_CLUSTER_REGISTRY_USERNAME: specify this value if the edge cluster registry requires authentication
    EDGE_CLUSTER_REGISTRY_TOKEN: specify this value if the edge cluster registry requires authentication
    EDGE_CLUSTER_STORAGE_CLASS: the storage class to use for the agent and edge services. Default: gp2
    AGENT_NAMESPACE: The namespace the agent should run in. Default: openhorizon-agent
    AGENT_WAIT_MAX_SECONDS: Maximum seconds to wait for the Horizon agent to start or stop. Default: 30
    AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS: Maximum secods to wait for the agent deployment rollout status to be successful. Default: 75
    AGENT_K8S_IMAGE_TAR_FILE: the file name of the edge cluster agent docker image in tar.gz format. Default: $DEFAULT_AGENT_K8S_IMAGE_TAR_FILE
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
while getopts "c:i:j:p:k:u:d:z:hl:n:sfbw:o:O:T:t:D:a:U:C" opt; do
    case $opt in
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
    echo $(now) "ERROR: $msg" >&2   # we always show fatal error msgs
    exit $exitCode
}

function log_error() {
    log $VERB_ERROR "ERROR: $1" >&2
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
    log_info "${var_name}: $varValue (from $from)"
    log_debug "get_variable() end"
}

# If INPUT_FILE_PATH is a short-hand value, turn it into the long-hand value. There are several variants of INPUT_FILE_PATH. See the -i flag in the usage.
# Side-effect: INPUT_FILE_PATH
function adjust_input_file_path() {
    log_debug "adjust_input_file_path() begin"
    local save_input_file_path=$INPUT_FILE_PATH
    INPUT_FILE_PATH="${INPUT_FILE_PATH%/}"   # remove trailing / if there
    if [[ $INPUT_FILE_PATH == 'css:' ]]; then
        INPUT_FILE_PATH="$INPUT_FILE_PATH$CSS_OBJ_PATH_DEFAULT"
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

# Download the certificate file from CSS if not present
function get_certificate() {
    log_debug "get_certificate() begin"

    if [[ $AGENT_CERT_FILE_CSS == css:* ]]; then
        download_css_file "$AGENT_CERT_FILE_CSS/$AGENT_CERT_FILE_DEFAULT"   # AGENT_CERT_FILE is already set to where it will end up in this case
    elif [[ $INPUT_FILE_PATH == css:* ]]; then
        if [[ -n $AGENT_CERT_FILE && ! -f $AGENT_CERT_FILE ]]; then
            download_css_file "$INPUT_FILE_PATH/$AGENT_CERT_FILE_DEFAULT"
        fi
    fi

    #todo: support the case in which the mgmt hub is using a CA-trusted cert, so we don't need to use a cert at all

    log_debug "get_certificate() end"
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
    local remote_cert_path="${remote_path%/*/data}/$AGENT_CERT_FILE_DEFAULT/data"
    if [[ -n $AGENT_CERT_FILE && -f $AGENT_CERT_FILE ]]; then
        # Either we have already downloaded it, or they gave it to us separately
        cert_flag="--cacert $AGENT_CERT_FILE"
    elif [[ -f $PERMANENT_CERT_PATH ]]; then
        # Cert from a previous install, see if that works
        log_info "Attempting to download file $remote_cert_path using $PERMANENT_CERT_PATH ..."
        httpCode=$(curl -sSL -w "%{http_code}" -u "$exch_creds" --cacert "$PERMANENT_CERT_PATH" -o "$AGENT_CERT_FILE" $remote_cert_path 2>/dev/null || true)
        if [[ $? -eq 0 && $httpCode -eq 200 ]]; then
            cert_flag="--cacert $AGENT_CERT_FILE"   # we got it
        fi
    fi
    if [[ -z $cert_flag && $HZN_FSS_CSSURL == https:* ]]; then   #todo: this check still doesn't account for a CA-trusted cert
        # Still didn't find a valid cert. Get the cert from CSS by disabling cert checking
        rm -f "$AGENT_CERT_FILE"   # this probably has the error msg from the previous curl in it
        log_info "Downloading file $remote_cert_path using --insecure ..."
        #echo "DEBUG: curl -sSL -w \"%{http_code}\" -u \"$exch_creds\" --insecure -o \"$AGENT_CERT_FILE\" $remote_cert_path || true"   # log_debug isn't set up yet
        httpCode=$(curl -sSL -w "%{http_code}" -u "$exch_creds" --insecure -o "$AGENT_CERT_FILE" $remote_cert_path || true)
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
        httpCode=$(curl -sSL -w "%{http_code}" -u "$exch_creds" $cert_flag -o "$local_file" $remote_path)
        chkHttp $? $httpCode 200 "downloading $remote_path" $local_file
    fi
    log_debug "download_css_file() end"
}

function download_anax_release_file() {
    log_debug "download_anax_release_file() begin"
    local anax_release_path=$1   # should be like: https://github.com/open-horizon/anax/releases/latest/download/<file>
    local local_file="${anax_release_path##*/}"   # get base name
    log_info "Downloading file $anax_release_path ..."
    httpCode=$(curl -sSLO -w "%{http_code}" $anax_release_path)
    chkHttp $? $httpCode 200 "downloading $anax_release_path" $local_file
    log_debug "download_anax_release_file() end"
}

# If necessary, download the cfg file from CSS.
function download_config_file() {
    log_debug "download_config_file() begin"
    local input_file_path=$1   # normally the value of INPUT_FILE_PATH or AGENT_CFG_FILE
    if [[ $input_file_path == css:* ]]; then
        download_css_file "$input_file_path/$AGENT_CFG_FILE_DEFAULT"
    # the cfg is specific to the instance of the cluster, so not available from anax/release
    #elif [[ $input_file_path == https://github.com/open-horizon/anax/releases* ]]; then
    #    download_anax_release_file "$input_file_path/$AGENT_CFG_FILE_DEFAULT"
    fi
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
            download_config_file "$AGENT_CFG_FILE"   # in this case AGENT_CFG_FILE is the css object path NOT including the last part (agent-install.cfg)
        else
            if [[ -n $AGENT_CFG_FILE && $AGENT_CFG_FILE != $AGENT_CFG_FILE_DEFAULT ]]; then
                log_fatal 1 "Can not specify both -k (AGENT_CFG_FILE) and -i (INPUT_FILE_PATH)"
            fi
            download_config_file "$INPUT_FILE_PATH"
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

# Get all of the input values from cmd line args, env vars, config file, or defaults.
# Side-effect: sets all of the variables as global constants
function get_all_variables() {
    log_debug "get_all_variables() begin"

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
    if [[ $AGENT_CFG_FILE == 'css:' ]]; then
        AGENT_CFG_FILE="$AGENT_CFG_FILE$CSS_OBJ_PATH_DEFAULT"   # expand the shorthand syntax
    fi
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
    get_variable HZN_ORG_ID '' 'true'
    get_variable HZN_MGMT_HUB_CERT_PATH
    get_variable AGENT_CERT_FILE "${HZN_MGMT_HUB_CERT_PATH:-$AGENT_CERT_FILE_DEFAULT}"   # use the default value even if the file doesn't exist yet, because '-i css:'' might create it
    if [[ $AGENT_CERT_FILE == 'css:' ]]; then
        AGENT_CERT_FILE="$AGENT_CERT_FILE$CSS_OBJ_PATH_DEFAULT"   # expand the shorthand syntax
    fi
    if [[ $AGENT_CERT_FILE == css:* ]]; then
        # Store the css path for the cert file, so we can change AGENT_CERT_FILE to where it will end up locally
        AGENT_CERT_FILE_CSS=$AGENT_CERT_FILE
        AGENT_CERT_FILE=$AGENT_CERT_FILE_DEFAULT
        rm -f $AGENT_CERT_FILE   # they told us to download it, so don't use the file already there
    fi
    get_variable HZN_FSS_CSSURL '' 'true'
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
    get_variable NODE_ID   # deprecated
    get_variable HZN_DEVICE_ID
    get_variable HZN_NODE_ID
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

    OS=$(get_os)
    detect_distro   # if linux, sets: DISTRO, DISTRO_VERSION_NUM, CODENAME
    ARCH=$(get_arch)
    log_info "OS: $OS, Distro: $DISTRO, Distro Release: $DISTRO_VERSION_NUM, Distro Code Name: $CODENAME, Architecture: $ARCH"
    
    if is_cluster; then
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
		elif command -v kubectl >/dev/null 2>&1; then 
			KUBECTL=kubectl 
		else 
			log_fatal 2 "kubectl is not available. Please install kubectl and ensure that it is found on your \$PATH" 
		fi
	fi
    fi

    if is_device; then
        get_variable NODE_ID_MAPPING_FILE 'node-id-mapping.csv'
        get_variable PKG_APT_KEY
        get_variable APT_REPO_BRANCH 'updates'
        get_variable AGENT_IMAGE_TAR_FILE "$DEFAULT_AGENT_IMAGE_TAR_FILE"
    elif is_cluster; then
        get_variable EDGE_CLUSTER_STORAGE_CLASS 'gp2'
        get_variable AGENT_NAMESPACE 'openhorizon-agent'
        USE_EDGE_CLUSTER_REGISTRY='true'   #get_variable USE_EDGE_CLUSTER_REGISTRY 'true'  # currently true is the only supported value
        get_variable AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS '75'

        if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
            local default_image_registry_on_edge_cluster
            if [[ $KUBECTL == "microk8s.kubectl" ]]; then
                default_image_registry_on_edge_cluster="localhost:32000/$AGENT_NAMESPACE/amd64_anax_k8s"
            elif [[ $KUBECTL == "k3s kubectl" ]]; then
                local image_arch=$(get_image_arch)
                local k3s_registry_endpoint=$($KUBECTL get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
                default_image_registry_on_edge_cluster="$k3s_registry_endpoint/$AGENT_NAMESPACE/${image_arch}_anax_k8s"
            else
                local ocp_registry_endpoint=$($KUBECTL get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
                default_image_registry_on_edge_cluster="$ocp_registry_endpoint/$AGENT_NAMESPACE/amd64_anax_k8s"
            fi

            get_variable IMAGE_ON_EDGE_CLUSTER_REGISTRY "$default_image_registry_on_edge_cluster"
            get_variable EDGE_CLUSTER_REGISTRY_USERNAME
            get_variable EDGE_CLUSTER_REGISTRY_TOKEN
            get_variable INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY
            get_variable AGENT_K8S_IMAGE_TAR_FILE "$DEFAULT_AGENT_K8S_IMAGE_TAR_FILE"
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
            node_id=$(grep HZN_NODE_ID /etc/default/horizon 2>/dev/null | cut -d'=' -f2)
            if [[ -n $node_id ]]; then
                log_info "Using node id from HZN_NODE_ID in /etc/default/horizon: $node_id"
            else
                # if HZN_NODE_ID is not set, look for HZN_DEVICE_ID
                node_id=$(grep HZN_DEVICE_ID /etc/default/horizon 2>/dev/null | cut -d'=' -f2)
                if [[ -n $node_id ]]; then 
                    log_info "Using node id from HZN_DEVICE_ID in /etc/default/horizon: $node_id"
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
                node_id=${HOSTNAME}   # default
                log_info "node_id is not set, use host name as node id"
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
    if [[ -z $HZN_EXCHANGE_USER_AUTH ]] && [[ -z ${HZN_EXCHANGE_NODE_AUTH#*:} ]]; then
        log_fatal 1 "If the node token is not specified in HZN_EXCHANGE_NODE_AUTH, then HZN_EXCHANGE_USER_AUTH must be specified"
    fi

    if [[ -n $HZN_MGMT_HUB_CERT_PATH && -n $AGENT_CERT_FILE && $HZN_MGMT_HUB_CERT_PATH != $AGENT_CERT_FILE ]]; then
        log_fatal 1 "If both HZN_MGMT_HUB_CERT_PATH and AGENT_CERT_FILE are specified they must be equal."
    fi

    # Policy and pattern are mutually exclusive
    if [[ -n "${HZN_NODE_POLICY}" && -n "${HZN_EXCHANGE_PATTERN}" ]]; then
        log_fatal 1 "HZN_NODE_POLICY and HZN_EXCHANGE_PATTERN can not both be set"
    fi

    # if a node policy is non-empty, check if the file exists
    if [[ -n $HZN_NODE_POLICY && ! -f $HZN_NODE_POLICY ]]; then
        log_fatal 1 "HZN_NODE_POLICY file '$HZN_NODE_POLICY' does not exist"
    fi

    # several flags can't be set with AGENT_ONLY_CLI
    if [[ $AGENT_ONLY_CLI == 'true' ]]; then
        if [[ -n $HZN_EXCHANGE_PATTERN || -n $HZN_NODE_POLICY || -n $ARG_AGENT_WAIT_FOR_SERVICE || -n $ARG_AGENT_REGISTRATION_TIMEOUT || -n $ARG_AGENT_WAIT_FOR_SERVICE_ORG ]]; then
            log_fatal 1 "Since AGENT_ONLY_CLI=='$AGENT_ONLY_CLI', none of these can be set: HZN_EXCHANGE_PATTERN, HZN_NODE_POLICY, AGENT_WAIT_FOR_SERVICE, AGENT_REGISTRATION_TIMEOUT, AGENT_WAIT_FOR_SERVICE_ORG"
        fi
    fi

    if is_cluster && [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        parts=$(echo $IMAGE_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print NF}')
        if [[ "$parts" != "3" ]]; then
            log_fatal 1 "IMAGE_ON_EDGE_CLUSTER_REGISTRY should be this format: <registry-host>/<registry-repo>/<image-name>"
        fi
    fi

    if [[ -n $AGENT_IMAGE_TAR_FILE && $AGENT_IMAGE_TAR_FILE != *.tar.gz ]]; then
        log_fatal 1 "AGENT_IMAGE_TAR_FILE must be in tar.gz format"
    fi

    if [[ -n $AGENT_K8S_IMAGE_TAR_FILE && $AGENT_K8S_IMAGE_TAR_FILE != *.tar.gz ]]; then
        log_fatal 1 "AGENT_K8S_IMAGE_TAR_FILE must be in tar.gz format"
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
        log_fatal 2 "must be root to run ${0##*/}. Run 'sudo -iE' and then run ${0##*/}"
    fi
    # or could check: [[ $(id -u) -ne 0 ]]
}

# compare versions. Return 0 (true) if the 1st version is greater than the 2nd version
function version_gt() {
    local version1=$1
    local version2=$2
    if [[ $version1 == $version2 ]]; then return 1; fi
    # need the test above, because the test below returns >= because it sorts in ascending order
    test "$(printf '%s\n' "$1" "$2" | sort -V | tail -n 1)" == "$version1"
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
    else   # the other files (pkg, yml, uninstall) are available from either
        if [[ $INPUT_FILE_PATH == css:* || $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
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
    httpCode=$(curl -sSLO -w "%{http_code}" $remote_path)
    chkHttp $? $httpCode 200 "downloading $remote_path" $tar_file_name
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
    download_css_file "$INPUT_FILE_PATH/$tar_file_name"
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

# For both device and cluster: Returns true (0) if /etc/default/horizon already has these values
function is_horizon_defaults_correct() {
    log_debug "is_horizon_defaults_correct() begin"
    IS_HORIZON_DEFAULTS_CORRECT='false'
    local anax_port=$1   # optional
    local cert_file defaults_file

    if is_device; then
        defaults_file='/etc/default/horizon'
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
    # FYI, these variables are currently supported in the defaults file: HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_AGBOT_URL, HZN_SDO_SVC_URL, HZN_DEVICE_ID, HZN_MGMT_HUB_CERT_PATH, HZN_AGENT_PORT, HZN_VAR_BASE, HZN_NO_DYNAMIC_POLL, HZN_MGMT_HUB_CERT_PATH, HZN_ICP_CA_CERT_PATH (deprecated), CMTN_SERVICEOVERRIDE

    # Note: the '|| true' is so not finding the strings won't cause set -e to exit the script
    horizon_defaults_value=$(grep -E '^HZN_EXCHANGE_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if [[ $horizon_defaults_value != $HZN_EXCHANGE_URL ]]; then
        log_info "HZN_EXCHANGE_URL value changed, return"
        return 1
    fi

    horizon_defaults_value=$(grep -E '^HZN_FSS_CSSURL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if [[ $horizon_defaults_value != $HZN_FSS_CSSURL ]]; then 
        log_info "HZN_FSS_CSSURL value changed, return"
        return 1 
    fi

    # even if HZN_AGBOT_URL is empty in this script, still verify the defaults file is the same
    horizon_defaults_value=$(grep -E '^HZN_AGBOT_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if [[ $horizon_defaults_value != $HZN_AGBOT_URL ]]; then 
        log_info "HZN_AGBOT_URL value changed, return"
        return 1
    fi

    # even if HZN_SDO_SVC_URL is empty in this script, still verify the defaults file is the same
    horizon_defaults_value=$(grep -E '^HZN_SDO_SVC_URL=' $defaults_file || true)
    horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
    if [[ $horizon_defaults_value != $HZN_SDO_SVC_URL ]]; then 
        log_info "HZN_SDO_SVC_URL value changed, return"
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

    if [[ -n $cert_file ]]; then
        horizon_defaults_value=$(grep -E '^HZN_MGMT_HUB_CERT_PATH=' $defaults_file || true)
        horizon_defaults_value=$(trim_variable "${horizon_defaults_value#*=}")
        if [[ -n $horizon_defaults_value ]] && ! diff -q "$horizon_defaults_value" "$cert_file" >/dev/null; then 
            log_info "cert file changed, return"
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
function create_or_update_horizon_defaults() {
    log_debug "create_or_update_horizon_defaults() begin"
    local anax_port=$1   # optional
    local abs_certificate   # can't call store_cert_file_permanently yet because that could cause is_horizon_defaults_correct
    log_verbose "Permanent location of certificate file: $abs_certificate"

    if [[ ! -f /etc/default/horizon ]]; then
        log_info "Creating /etc/default/horizon ..."
        sudo mkdir -p /etc/default
        sudo bash -c "echo -e 'HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}\nHZN_FSS_CSSURL=${HZN_FSS_CSSURL}\nHZN_DEVICE_ID=${NODE_ID}\nHZN_NODE_ID=${NODE_ID}' > /etc/default/horizon"
        if [[ -n $HZN_AGBOT_URL ]]; then
            sudo sh -c "echo 'HZN_AGBOT_URL=$HZN_AGBOT_URL' >> /etc/default/horizon"
        fi
        if [[ -n $HZN_SDO_SVC_URL ]]; then
            sudo sh -c "echo 'HZN_SDO_SVC_URL=$HZN_SDO_SVC_URL' >> /etc/default/horizon"
        fi
        abs_certificate=$(store_cert_file_permanently "$AGENT_CERT_FILE")   # can return empty string
        if [[ -n $abs_certificate ]]; then
            sudo sh -c "echo 'HZN_MGMT_HUB_CERT_PATH=$abs_certificate' >> /etc/default/horizon"
        fi
        if [[ -n $anax_port ]]; then
            sudo sh -c "echo 'HZN_AGENT_PORT=$anax_port' >> /etc/default/horizon"
        fi
        HORIZON_DEFAULTS_CHANGED='true'
    elif is_horizon_defaults_correct "$anax_port"; then
        log_info "/etc/default/horizon already has the correct values. Not modifying it."
        HORIZON_DEFAULTS_CHANGED='false'
    else
        # File already exists, but isn't correct. Update it (do not overwrite it in case they have other variables in there they want preserved)
        log_info "Updating /etc/default/horizon ..."
        add_to_or_update_horizon_defaults 'HZN_EXCHANGE_URL' "$HZN_EXCHANGE_URL" /etc/default/horizon
        add_to_or_update_horizon_defaults 'HZN_FSS_CSSURL' "$HZN_FSS_CSSURL" /etc/default/horizon
        if [[ -n $HZN_AGBOT_URL ]]; then
            add_to_or_update_horizon_defaults 'HZN_AGBOT_URL' "$HZN_AGBOT_URL" /etc/default/horizon
        fi
        if [[ -n $HZN_SDO_SVC_URL ]]; then
            add_to_or_update_horizon_defaults 'HZN_SDO_SVC_URL' "$HZN_SDO_SVC_URL" /etc/default/horizon
        fi
        add_to_or_update_horizon_defaults 'HZN_DEVICE_ID' "$NODE_ID" /etc/default/horizon
        add_to_or_update_horizon_defaults 'HZN_NODE_ID' "$NODE_ID" /etc/default/horizon
        abs_certificate=$(store_cert_file_permanently "$AGENT_CERT_FILE")   # can return empty string
        if [[ -n $abs_certificate ]]; then
            add_to_or_update_horizon_defaults 'HZN_MGMT_HUB_CERT_PATH' "$abs_certificate" /etc/default/horizon
        fi
        if [[ -n $anax_port ]]; then
            add_to_or_update_horizon_defaults 'HZN_AGENT_PORT' "$anax_port" /etc/default/horizon
        fi
        HORIZON_DEFAULTS_CHANGED='true'
    fi
    log_debug "create_or_update_horizon_defaults() end"
}

# Have macos trust the horizon-cli pkg cert and the Horizon mgmt hub self-signed cert
function mac_trust_certs() {
    log_debug "mac_trust_certs() begin"
    local mac_pkg_cert_file=$1 mgmt_hub_cert_file=$2
    log_info "Importing the horizon-cli package certificate into Mac OS keychain..."
    set -x   # echo'ing this cmd because on mac it is usually the 1st sudo cmd and want them to know why they are being prompted for pw
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$mac_pkg_cert_file"
    { set +x; } 2>/dev/null
    if [[ -n $mgmt_hub_cert_file ]]; then
        log_info "Importing the management hub certificate into Mac OS keychain..."
        sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$mgmt_hub_cert_file"
    fi
    log_debug "mac_trust_certs() end"
}

function install_macos() {
    log_debug "install_macos() begin"

    if ! isCmdInstalled jq && isCmdInstalled brew; then
            echo "Jq is required, installing it using brew, this could take a minute..."
            runCmdQuietly brew install jq
    fi

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        confirmCmds socat docker jq

        check_existing_exch_node_is_correct_type "device"

        if is_agent_registered && (! is_horizon_defaults_correct || ! is_registration_correct); then
            unregister
        fi
    fi

    get_certificate

    create_or_update_horizon_defaults

    get_pkgs

    mac_trust_certs "${PACKAGES}/${MAC_PACKAGE_CERT}" "$AGENT_CERT_FILE"
    install_mac_horizon-cli

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        start_device_agent_container   # even if it already running, it restarts it

        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"
    fi

    log_debug "install_macos() end"
}

# Determine what the agent port number should be, verify it is free, and set ANAX_PORT.
# Side-effect: sets ANAX_PORT
function check_and_set_anax_port() {
    log_debug "check_and_set_anax_port() begin"
    local anax_port=$ANAX_DEFAULT_PORT
    if [[ -f /etc/default/horizon ]]; then
        log_verbose "Trying to get agent port from previous /etc/default/horizon file..."
        local prevPort=$(grep HZN_AGENT_PORT /etc/default/horizon | cut -d'=' -f2)
        if [[ -n $prevPort ]]; then
            log_verbose "Found agent port in previous /etc/default/horizon: $prevPort"
            anax_port=$prevPort
        fi
    fi

    log_verbose "Checking if the agent port ${anax_port} is free..."
    local netStat=$(netstat -nlp | grep $anax_port || true)
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
        runCmdQuietly apt-get install -yqf curl jq
    else
        runCmdQuietly apt-get install -yqf curl jq software-properties-common net-tools

        if ! isCmdInstalled docker; then
            log_info "Docker is required, installing it..."
            if [ "${DISTRO}" == "raspbian" ]; then
                curl -sSL get.docker.com | sh
            else
                curl -fsSL https://download.docker.com/linux/$DISTRO/gpg | apt-key add -
                add-apt-repository "deb [arch=$(dpkg --print-architecture)] https://download.docker.com/linux/$DISTRO $(lsb_release -cs) stable"
                runCmdQuietly apt-get install -yqf docker-ce docker-ce-cli containerd.io
            fi
        fi
    fi
    log_debug "debian_device_install_prereqs() end"
}

# Returns 0 (true) if the deb pkg to be installed is newer than the corresponding pkg already installed
function is_newer_deb_pkg() {
    local latest_deb_file=${1:?}   # make the decision based on the deb pkg they passed in

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
        return 0   # anything is newer than not installed
    fi
    local installed_deb_version=$(dpkg-query -s $pkg_name | grep -E '^Version:' | awk '{print $2}')

    log_info "Installed $pkg_name deb package version: $installed_deb_version, Provided $pkg_name deb file version: $deb_file_version"
    if version_gt $deb_file_version $installed_deb_version; then return 0
    else return 1; fi
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
        add-apt-repository "deb [arch=$(dpkg --print-architecture)] $PKG_APT_REPO $(lsb_release -cs)-$APT_REPO_BRANCH main"
        if [[ $AGENT_ONLY_CLI == 'true' ]]; then
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
        if [[ $AGENT_ONLY_CLI != 'true' ]]; then
            latest_horizon_file=$(ls -1 $PACKAGES/horizon_*_${ARCH}.deb | sort -V | tail -n 1)
            if [[ -z $latest_horizon_file ]]; then
                log_warning "No horizon deb package found in $PACKAGES"
                return 1
            fi
            latest_files="$latest_files $latest_horizon_file"
            new_pkg=$latest_horizon_file
        fi

        if is_newer_deb_pkg $new_pkg; then
            log_info "Installing $latest_files ..."
            runCmdQuietly apt-get install -yqf $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        elif [[ "$AGENT_OVERWRITE" == true ]]; then
            log_info "Downgrading to $latest_files because AGENT_OVERWRITE==$AGENT_OVERWRITE ..."
            runCmdQuietly apt-get install -yqf --allow-downgrades $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' ]]; then
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
    log_info "Restarting the horizon agent service because /etc/default/horizon was modified..."
    systemctl restart horizon.service   # (restart will succeed even if the service was already stopped)
    wait_until_agent_ready
    log_debug "restart_device_agent() end"
}

# Install the agent and register it on a debian variant
function install_debian() {
    log_debug "install_debian() begin"

    debian_device_install_prereqs

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        check_and_set_anax_port   # sets ANAX_PORT
        check_existing_exch_node_is_correct_type "device"

        if is_agent_registered && (! is_horizon_defaults_correct "$ANAX_PORT" || ! is_registration_correct); then
            unregister
        fi
    fi

    get_certificate

    create_or_update_horizon_defaults "$ANAX_PORT"

    get_pkgs
    # Note: the horizon pkg will only write /etc/default/horizon if it doesn't exist, so it won't overwrite what we created/modified above
    install_debian_device_horizon_pkgs

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        if [[ $HORIZON_DEFAULTS_CHANGED == 'true' && $AGENT_WAS_RESTARTED == 'false' ]]; then
            restart_device_agent   # because the new pkgs were not installed, so that didn't restart the agent
        fi

        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"
    fi

    log_debug "install_debian() end"
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

    dnf install -yq curl jq

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        dnf install -yq net-tools
        if ! isCmdInstalled ${DOCKER_ENGINE}; then
            # Can't install docker for them on red hat, because they make it difficult. See: https://linuxconfig.org/how-to-install-docker-in-rhel-8
            log_fatal 2 "${DOCKER_ENGINE} is required, but not installed. Install it and rerun this script."
        fi
    fi
    log_debug "redhat_device_install_prereqs() end"
}

# Returns 0 (true) if the rpm pkg to be installed is newer than the corresponding pkg already installed
function is_newer_rpm_pkg() {
    local latest_rpm_file=${1:?}   # make the decision based on the rpm pkg they passed in

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
        return 0   # anything is newer than not installed
    fi
    local installed_rpm_version=$(rpm -q $pkg_name 2>/dev/null)  # will return like: horizon-2.28.0-338.x86_64
    installed_rpm_version=${installed_rpm_version#${pkg_name}-}   # remove the pkg name
    installed_rpm_version=${installed_rpm_version%.*}   # remove the arch, so left with only the version

    log_info "Installed $pkg_name rpm package version: $installed_rpm_version, Provided $pkg_name rpm file version: $rpm_file_version"
    if version_gt $rpm_file_version $installed_rpm_version; then return 0
    else return 1; fi
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
        if [[ $AGENT_ONLY_CLI != 'true' ]]; then
            latest_horizon_file=$(ls -1 $PACKAGES/horizon-*.${ARCH}.rpm | grep -v "$PACKAGES/horizon-cli" | sort -V | tail -n 1)
            if [[ -z $latest_horizon_file ]]; then
                log_warning "No horizon rpm package found in $PACKAGES"
                return 1
            fi
            latest_files="$latest_files $latest_horizon_file"
            new_pkg=$latest_horizon_file
        fi
    
        if is_newer_rpm_pkg $new_pkg; then
            log_info "Installing $latest_files ..."
            dnf install -yq $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' ]]; then
                wait_until_agent_ready
                AGENT_WAS_RESTARTED='true'
            fi
        elif [[ "$AGENT_OVERWRITE" == true ]]; then
            log_info "Downgrading to $latest_files because AGENT_OVERWRITE==$AGENT_OVERWRITE ..."
            # Note: dnf automatically detects the specified pkg files are a lower version and downgrades them. If we need to switch to yum, we'll have to use yum downgrade ...
            dnf install -yq $latest_files
            if [[ $AGENT_ONLY_CLI != 'true' ]]; then
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

    log_info "Installing prerequisites, this could take a minute..."
    install_dnf
    redhat_device_install_prereqs

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        check_and_set_anax_port   # sets ANAX_PORT
        check_existing_exch_node_is_correct_type "device"

        if is_agent_registered && (! is_horizon_defaults_correct "$ANAX_PORT" || ! is_registration_correct); then
            unregister
        fi
    fi

    get_certificate

    create_or_update_horizon_defaults "$ANAX_PORT"

    get_pkgs
    # Note: the horizon pkg will only write /etc/default/horizon if it doesn't exist, so it won't overwrite what we created/modified above
    install_redhat_device_horizon_pkgs

    if [[ $AGENT_ONLY_CLI != 'true' ]]; then
        if [[ $HORIZON_DEFAULTS_CHANGED == 'true' && $AGENT_WAS_RESTARTED == 'false' ]]; then
            restart_device_agent   # because the new pkgs were not installed, so that didn't restart the agent
        fi

        registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"
    fi

    log_debug "install_redhat() end"
}

# Install the mac pkg that provides hzn, horizon-container, etc.
# Side-effect: based on the horizon-cli version being installed, it sets HC_DOCKER_TAG to use the same version of the agent container
function install_mac_horizon-cli() {
    log_debug "install_mac_horizon-cli() begin"

    # Get horizon-cli pkg file they gave us to install
    local pkg_file_name=$(ls -1 horizon-cli*.pkg | sort -V | tail -n 1)
    # pkg_file_name is something like horizon-cli-2.27.0-89.pkg
    local pkg_file_version=${pkg_file_name#horizon-cli-}   # this removes the 1st part
    pkg_file_version=${pkg_file_version%.pkg}   # remove the ending part
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
        local installed_version=$(hzn version | grep "^Horizon CLI")
        installed_version=${installed_version##* }   # remove all of the space-separated words, except the last one
        log_info "Installed horizon-cli pkg version: $installed_version, Provided horizon pkg file version: $pkg_file_version"
        if [[ ! $installed_version =~ $SEMVER_REGEX ]] || version_gt "$pkg_file_version" "$installed_version"; then
            log_verbose "Either can not get the installed hzn version, or the given pkg file version is newer"
            log_info "Installing $PACKAGES/$pkg_file_name ..."
            sudo installer -pkg $PACKAGES/$pkg_file_name -target /
        else
            log_verbose "The given pkg file version is older than or equal to the installed hzn"
            if [[ "$AGENT_OVERWRITE" == true ]]; then
                log_info "Installing older horizon-cli package ${pkg_file_version} because AGENT_OVERWRITE is set to true ..."
                sudo installer -pkg ${PACKAGES}/$pkg_file_name -target /
            else
                log_info "The installed horizon-cli package is already up to date ($installed_version)"
            fi
        fi
    else
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
    log_debug "wait_for() begin"
    return 0
}

# Wait until the agent is responding
function wait_until_agent_ready() {
    log_debug "wait_until_agent_ready() begin"
    if ! wait_for '[[ -n "$(hzn node list 2>/dev/null | jq -r .configuration.preferred_exchange_version 2>/dev/null)" ]]' 'Horizon agent ready' $AGENT_WAIT_MAX_SECONDS; then
        log_fatal 3 "Horizon agent did not start successfully"
    fi
    log_debug "wait_until_agent_ready() begin"
}

# Load a docker system and return the full image path (including tag)
# Note: does not remove the origin tar.gz file.
function load_docker_image() {
    local tar_file_name=${1:?}
    # Note: only fatal msgs allowed in this function, because it returns a string value
    if [[ ! -f $tar_file_name ]]; then
        log_fatal 2 "Agent docker image tar file $tar_file_name does not exist"
    fi
    if [[ -h $tar_file_name ]]; then
        log_fatal 2 "Can not unpack $tar_file_name because gunzip does not support symbolic links"
    fi
    gunzip -k $tar_file_name   # keep the original file
    chk $? "uncompressing $tar_file_name"
    local loaded_image_message=$(${DOCKER_ENGINE} load --input ${tar_file_name%.gz})
    chk $? "${DOCKER_ENGINE} loading ${tar_file_name%.gz}"
    rm ${tar_file_name%.gz}   # clean up this temporary file
    # loaded_image_message is like: Loaded image: {repo}/{image_name}:{version_number}
    local image_full_path=$(echo $loaded_image_message | awk -F': ' '{print $2}')
    if [[ -z $image_full_path ]]; then
        log_fatal 3 "Could not get agent image path from loaded $tar_file_name"
    fi
    echo "$image_full_path"
}

# Get the latest agent-in-container started on mac
#future: support agent-in-container on linux too
function start_device_agent_container() {
    log_debug "start_device_agent_container() begin"

    if ! isCmdInstalled horizon-container; then
        log_fatal 5 "The horizon-container command not found, horizon-cli is not installed or its installation is broken"
    fi

    # Note: install_mac_horizon-cli() sets HC_DOCKER_TAG appropriately
    # In the css case, get amd64_anax.tar.gz from css, docker load it, and set HC_DOCKER_IMAGE and HC_DONT_PULL
    if [[ $INPUT_FILE_PATH == css:* ]]; then
        download_css_file "$INPUT_FILE_PATH/$AGENT_IMAGE_TAR_FILE"
        log_info "Unpacking and docker loading $AGENT_IMAGE_TAR_FILE ..."
        local agent_image_full_path=$(load_docker_image $AGENT_IMAGE_TAR_FILE)
        #rm ${AGENT_IMAGE_TAR_FILE}   # do not remove the file they gave us
        export HC_DONT_PULL=1   # horizon-container should get it straight from the tar file we got from CSS, not try to go to docker hub to get it
        export HC_DOCKER_IMAGE=${agent_image_full_path%:*}   # remove the tag
        export HC_DOCKER_TAG=${agent_image_full_path##*:}   # remove everything but the tag
    elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
        : # we've already set HC_DOCKER_TAG from the horizon-cli version. From there horizon-container naturally does the right thing (pulls it from docker hub)
    elif [[ -f $AGENT_IMAGE_TAR_FILE ]]; then
        # They gave us the agent docker image tar file in the input path
        log_info "Unpacking and docker loading $AGENT_IMAGE_TAR_FILE ..."
        local agent_image_full_path=$(load_docker_image $AGENT_IMAGE_TAR_FILE)
        #rm ${AGENT_IMAGE_TAR_FILE}   # do not remove the file they gave us
        export HC_DONT_PULL=1   # horizon-container should get it straight from the tar file we got from CSS, not try to go to docker hub to get it
        export HC_DOCKER_IMAGE=${agent_image_full_path%:*}   # remove the tag
        export HC_DOCKER_TAG=${agent_image_full_path##*:}   # remove everything but the tag
    fi
    #else let horizon-container do its default thing (run openhorizon/amd64_anax:latest)

    if ! isDockerContainerRunning horizon1; then
        if [[ -z $(${DOCKER_ENGINE} ps -aq --filter name=horizon1) ]]; then
            # horizon services container doesn't exist
            log_info "Starting horizon agent container version $HC_DOCKER_TAG ..."
            horizon-container start
        else
            # horizon container is stopped but the container exists
            log_info "The horizon agent container was in a stopped state via docker, restarting it..."
            ${DOCKER_ENGINE} start horizon1
            horizon-container update   # ensure it is running the latest version
        fi
    else
        log_info "The Horizon agent container is running already, restarting it to ensure it is version $HC_DOCKER_TAG ..."
        horizon-container update   # ensure it is running the latest version
    fi

    wait_until_agent_ready

    log_debug "start_device_agent_container() end"
}

# Stops horizon service container on mac
function stop_agent_container() {
    log_debug "stop_agent_container() begin"

    if ! isDockerContainerRunning horizon1; then return; fi   # already stopped

    if ! isCmdInstalled horizon-container; then
        log_fatal 3 "Horizon agent container running, but horizon-container command not installed"
    fi

    log_info "Stopping the Horizon agent container..."
    horizon-container stop

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

    if [[ "$node_state" == "configured" ]]; then
        log_info "Unregistering the agent because the current registration settings are not what you want..."
        agent_exec "hzn unregister -f $rmExchNodeFlag"
        local rc=$?
        if [[ $rc -ne 0 ]]; then
            log_info "Unregister failed with exit code $rc. Now trying to unregister with deep clean..."
            agent_exec "hzn unregister -fD $rmExchNodeFlag"   # registration is stuck, do a deep clean
            chk $? 'unregistering with deep clean'
        fi
    else
        # configuring, unconfiguring, or some unanticipated state
        log_info "The agent state is $node_state, unregistering it with deep clean..."
        agent_exec "hzn unregister -fD $rmExchNodeFlag"   # registration is stuck, do a deep clean
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

    if [[ $skip_reg == 'true' ]]; then return; fi

    local hzn_node_list=$(agent_exec 'hzn node list 2>/dev/null' || true)
    local reg_node_id=$(jq -r .id 2>/dev/null <<< $hzn_node_list || true)

    # Get current node registration state and determine if we need to unregister first
    local node_state=$(jq -r .configstate.state 2>/dev/null <<< $hzn_node_list || true)

    # Register the edge node
    if [[ $node_state == 'configured' ]]; then
        if [[ -n $policy ]]; then
            # We only need to update the node policy (we can keep the node registered).
            log_info "The current registration settings are correct, keeping them, except updating the node policy..."
            # Since hzn might be in the edge cluster container, need to pass the policy file's contents in via stdin
            cat $policy | agent_exec "hzn policy update -f-"
        else   # nothing needs changing in the current registration
            log_info "The current registration settings are correct, keeping them."
        fi
    else
        # Register the node. First get some variables ready
        local user_auth wait_service_flag wait_org_flag timeout_flag node_name pattern_flag
        if [[ -n $HZN_EXCHANGE_USER_AUTH ]]; then
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

        # Now actually do the registration
        log_info "Registering the edge node..."
        local reg_cmd
        if [[ -n $policy ]]; then
            # Since hzn might be in the edge cluster container, need to pass the policy file's contents in via stdin
            reg_cmd="hzn register -m '${node_name}' -o '$HZN_ORG_ID' $user_auth -n '$HZN_EXCHANGE_NODE_AUTH' $wait_service_flag $wait_org_flag $timeout_flag --policy=-"
            echo "$reg_cmd"
            cat $policy | agent_exec "$reg_cmd"
        else  # register w/o policy or with pattern
            reg_cmd="hzn register -m '${node_name}' -o '$HZN_ORG_ID' $user_auth -n '$HZN_EXCHANGE_NODE_AUTH' $wait_service_flag $wait_org_flag $timeout_flag $pattern_flag"
            echo "$reg_cmd"
            agent_exec  "$reg_cmd"
        fi
    fi

    log_debug "registration() end"
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

    if ! is_linux; then return; fi

    if isCmdInstalled lsb_release; then
        DISTRO=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        DISTRO_VERSION_NUM=$(lsb_release -sr)
        CODENAME=$(lsb_release -sc)

    # need these for redhat variants
    elif [[ -f /etc/os-release ]]; then
        . /etc/os-release
        DISTRO=$ID
        DISTRO_VERSION_NUM=$VERSION_ID
        CODENAME=$VERSION_CODENAME   # is empty for RHEL
    elif [[ -f /etc/lsb-release ]]; then
        . /etc/lsb-release
        DISTRO=$DISTRIB_ID
        DISTRO_VERSION_NUM=$DISTRIB_RELEASE
        CODENAME=$DISTRIB_CODENAME
    else
        log_fatal 2 "Cannot detect Linux version"
    fi

    log_verbose "Detected distribution: ${DISTRO}, verison: ${DISTRO_VERSION_NUM}, codename: ${CODENAME}"

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
    if is_macos; then echo 'pkg'  # Test macos first since DISTRO is not set for macos and results in an error if debian or redhat is checked first 
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
        uname -m   # e.g. x86_64. We don't currently use ARCH on macos
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
    local exch_output=$(curl -fsS $cert_flag $HZN_EXCHANGE_URL/orgs/$HZN_ORG_ID/nodes/$NODE_ID -u "$exch_creds" 2>/dev/null) || true

    if [[ -n "$exch_output" ]]; then
        local exch_node_type=$(echo $exch_output | jq -re '.nodes | .[].nodeType')
        if [[ "$exch_node_type" == "device" ]] && [[ "$expected_type" != "device" ]]; then
            log_fatal 2 "Node id ${NODE_ID} has already been created as nodeType device. Remove the node from the exchange and run this script again."
        elif [[ "$exch_node_type" == "cluster" ]] && [[ "$expected_type" != "cluster" ]]; then
            log_fatal 2 "Node id ${NODE_ID} has already been created as nodeType clutser. Remove the node from the exchange and run this script again."
        fi
    fi

    log_debug "check_existing_exch_node_is_correct_type() end"
}

# Cluster only: to extract agent image tar.gz and load to docker
# Side-effect: sets globals: AGENT_IMAGE, AGENT_IMAGE_VERSION_IN_TAR, IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY
function loadClusterAgentImage() {
    log_debug "loadClusterAgentImage() begin"

    # Get the agent tar file, if necessary
    if using_remote_input_files 'pkg'; then
        if [[ -n $AGENT_K8S_IMAGE_TAR_FILE && $AGENT_K8S_IMAGE_TAR_FILE != $DEFAULT_AGENT_K8S_IMAGE_TAR_FILE ]]; then
            log_fatal 1 "Can not specify both AGENT_K8S_IMAGE_TAR_FILE and -i (INPUT_FILE_PATH)"
        fi
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            download_css_file "$INPUT_FILE_PATH/$AGENT_K8S_IMAGE_TAR_FILE"
        elif [[ $INPUT_FILE_PATH == https://github.com/open-horizon/anax/releases* ]]; then
            # Get the docker image from docker hub in this case
            local image_tag=$(get_anax_release_version $INPUT_FILE_PATH)   # use the version from INPUT_FILE_PATH, if possible
            if [[ -z $image_tag ]]; then
                image_tag='latest'
            fi
            local image_arch=$(get_image_arch)
            local image_path="openhorizon/${image_arch}_anax_k8s:$image_tag"
            log_info "Pulling $image_path from docker hub..."
            ${DOCKER_ENGINE} pull "$image_path"
            chk $? "pulling $image_path"
            AGENT_IMAGE=$image_path
            AGENT_IMAGE_VERSION_IN_TAR=${AGENT_IMAGE##*:}
            IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$IMAGE_ON_EDGE_CLUSTER_REGISTRY:$AGENT_IMAGE_VERSION_IN_TAR"
            return
        fi
    elif [[ ! -f $AGENT_K8S_IMAGE_TAR_FILE ]]; then
        log_fatal 2 "Edge cluster agent image tar file $AGENT_K8S_IMAGE_TAR_FILE does not exist"
    fi

    log_info "Unpacking and docker loading $AGENT_K8S_IMAGE_TAR_FILE ..."
    AGENT_IMAGE=$(load_docker_image $AGENT_K8S_IMAGE_TAR_FILE)
    chk $? "docker loading $AGENT_K8S_IMAGE_TAR_FILE"
    # AGENT_IMAGE is like: {repo}/{image_name}:{version_number}
    #rm ${AGENT_K8S_IMAGE_TAR_FILE}   # do not remove the file they gave us

    AGENT_IMAGE_VERSION_IN_TAR=${AGENT_IMAGE##*:}
    # use the same tag for the image in the edge cluster registry as the tag they used for the image in the inputted tar file
    IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY="$IMAGE_ON_EDGE_CLUSTER_REGISTRY:$AGENT_IMAGE_VERSION_IN_TAR"

    log_debug "loadClusterAgentImage() end"
}

# Cluster only: to push agent image to image registry that edge cluster can access
function pushImageToEdgeClusterRegistry() {
    log_debug "pushImageToEdgeClusterRegistry() begin"

    # split $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY by "/"
    EDGE_CLUSTER_REGISTRY_HOST=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $1}')
    log_info "Edge cluster registry host: $EDGE_CLUSTER_REGISTRY_HOST"

    if [[ -z $EDGE_CLUSTER_REGISTRY_USERNAME && -z $EDGE_CLUSTER_REGISTRY_TOKEN ]]; then
        : # even for a registry in the insecure-registries list, if we don't specify user/pw it will prompt for it
        #docker login $EDGE_CLUSTER_REGISTRY_HOST
    else
        echo "$EDGE_CLUSTER_REGISTRY_TOKEN" | ${DOCKER_ENGINE} login -u $EDGE_CLUSTER_REGISTRY_USERNAME --password-stdin $EDGE_CLUSTER_REGISTRY_HOST
        chk $? "logging into edge cluster's registry: $EDGE_CLUSTER_REGISTRY_HOST"
    fi

    log_info "Pushing docker image $AGENT_IMAGE to $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY ..."
    ${DOCKER_ENGINE} tag ${AGENT_IMAGE} ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
    runCmdQuietly ${DOCKER_ENGINE} push ${IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY}
    log_verbose "successfully pushed image $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY to edge cluster registry"

    log_debug "pushImageToEdgeClusterRegistry() end"
}

# Cluster only: check if agent deployment exists to determine whether to do agent install or agent update
# Side-effect: sets AGENT_DEPLOYMENT_UPDATE, POD_ID, IS_AGENT_IMAGE_VERSION_SAME, IS_HORIZON_ORG_ID_SAME
function check_agent_deployment_exist() {
    log_debug "check_agent_deployment_exist() begin"
    IS_AGENT_IMAGE_VERSION_SAME="false"
    IS_HORIZON_ORG_ID_SAME="false"

    if ! $KUBECTL get deployment ${DEPLOYMENT_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
        # agent deployment doesn't exist in ${AGENT_NAMESPACE}, fresh install
        AGENT_DEPLOYMENT_UPDATE="false"
    else
        # already have an agent deplyment in ${AGENT_NAMESPACE}, check the agent pod status
        if [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
            # agent deployment does not have agent pod in RUNNING status
            log_fatal 3 "Previous agent pod in not in RUNNING status, please run agent-uninstall.sh to clean up and re-run the agent-install.sh"
        else
            # check 1) agent image in deployment
            # eg: {image-registry}:5000/{repo}/{image-name}:{version}
            local agent_image_in_use=$($KUBECTL get deployment agent -o jsonpath='{$.spec.template.spec.containers[:1].image}' -n ${AGENT_NAMESPACE})
            # {image-name}:{version}
            local agent_image_name_with_tag=$(echo $agent_image_in_use | awk -F'/' '{print $3}')
            # {version}
            local agent_image_version_in_use=$(echo $agent_image_name_with_tag | awk -F':' '{print $2}')

            log_debug "Current agent image version is: $agent_image_version_in_use, agent image version in tar file is: $AGENT_IMAGE_VERSION_IN_TAR"
            AGENT_DEPLOYMENT_UPDATE="true"
            if [[ "$AGENT_IMAGE_VERSION_IN_TAR" == "$agent_image_version_in_use" ]]; then
                IS_AGENT_IMAGE_VERSION_SAME="true"
            fi

            # check HZN_ORG_ID set in deployment
            local horizon_org_id_env_name_in_use=$($KUBECTL get deployment agent -n ${AGENT_NAMESPACE} -o jsonpath='{.spec.template.spec.containers[0].env[0].name}')
            local horizon_org_id_env_value_in_use=$($KUBECTL get deployment agent -n ${AGENT_NAMESPACE} -o jsonpath='{.spec.template.spec.containers[0].env[0].value}')
            log_debug "Current HZN_ORG_ID in agent deployment is: env_name: $horizon_org_id_env_name_in_use, env_value: $horizon_org_id_env_value_in_use"
            log_debug "HZN_ORG_ID passed to this script is: env_name: HZN_ORG_ID, env_value: $HZN_ORG_ID"

            if [[ "$horizon_org_id_env_name_in_use" == "HZN_ORG_ID" ]] && [[ "$horizon_org_id_env_value_in_use" == "$HZN_ORG_ID" ]]; then
                IS_HORIZON_ORG_ID_SAME="true"
            fi

            POD_ID=$($KUBECTL get pod -l app=agent --field-selector status.phase=Running -n ${AGENT_NAMESPACE} 2>/dev/null | grep "agent-" | cut -d " " -f1 2>/dev/null)
            log_verbose "Previous agent pod is ${POD_ID}, will continue with agent updating in edge cluster"
        fi
    fi

    log_debug "check_agent_deployment_exist() end"
}

# Cluster only: get or verify deployment-template.yml, persistentClaim-template.yml, and agent-uninstall.sh
function get_edge_cluster_files() {
    log_debug "get_edge_cluster_files() begin"
    if using_remote_input_files 'yml'; then
        log_verbose "Getting template.yml files and agent-uninstall.sh from $INPUT_FILE_PATH ..."
        if [[ $INPUT_FILE_PATH == css:* ]]; then
            download_css_file "$INPUT_FILE_PATH/$EDGE_CLUSTER_TAR_FILE_NAME"
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

    log_debug "get_edge_cluster_files() end"
}

# Cluster only: to generate 2 files: deployment.yml and persistentClaim.yml
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

    log_debug "generate_installation_files() end"
}

# Cluster only: to generate /tmp/agent-install-hzn-env file and copy cert file to /etc/default/cert
function create_horizon_env() {
    log_debug "create_horizon_env() begin"
    if [[ -f $HZN_ENV_FILE ]]; then
        log_verbose "$HZN_ENV_FILE already exists. Will overwrite it..."
        rm $HZN_ENV_FILE
    fi
    local cert_name=$(basename ${AGENT_CERT_FILE})
    local cluster_cert_path="/etc/default/cert"
    log_verbose "copy cert file to $cluster_cert_path ..."
    mkdir -p $cluster_cert_path && cp ${AGENT_CERT_FILE} $cluster_cert_path

    echo "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}" >>$HZN_ENV_FILE
    echo "HZN_FSS_CSSURL=${HZN_FSS_CSSURL}" >>$HZN_ENV_FILE
    if [[ -n $HZN_AGBOT_URL ]]; then
        echo "HZN_AGBOT_URL=${HZN_AGBOT_URL}" >>$HZN_ENV_FILE
    fi
    if [[ -n $HZN_SDO_SVC_URL ]]; then
        echo "HZN_SDO_SVC_URL=${HZN_SDO_SVC_URL}" >>$HZN_ENV_FILE
    fi
    echo "HZN_DEVICE_ID=${NODE_ID}" >>$HZN_ENV_FILE
    echo "HZN_NODE_ID=${NODE_ID}" >> $HZN_ENV_FILE
    echo "HZN_MGMT_HUB_CERT_PATH=$cluster_cert_path/$cert_name" >>$HZN_ENV_FILE
    echo "HZN_AGENT_PORT=8510" >>$HZN_ENV_FILE
    log_debug "create_horizon_env() end"
}

# Cluster only: to create deployment.yml based on template
function prepare_k8s_deployment_file() {
    log_debug "prepare_k8s_deployment_file() begin"

    # Note: get_edge_cluster_files() already downloaded deployment-template.yml, if necessary

    sed -e "s#__AgentNameSpace__#${AGENT_NAMESPACE}#g" -e "s#__OrgId__#\"${HZN_ORG_ID}\"#g" deployment-template.yml >deployment.yml
    chk $? 'creating deployment.yml'

    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        EDGE_CLUSTER_REGISTRY_PROJECT_NAME=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $2}')
        EDGE_CLUSTER_AGENT_IMAGE_AND_TAG=$(echo $IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY | awk -F'/' '{print $3}')

        local image_full_path_on_edge_cluster_registry_internal_url
        if [[ "$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY" == "" ]]; then
            # check if using local cluster or remote ocp
            if $KUBECTL cluster-info | grep -q -E 'Kubernetes .* is running at .*//(127|172|10|192.168)\.'; then
                # using small kube
                image_full_path_on_edge_cluster_registry_internal_url="$IMAGE_FULL_PATH_ON_EDGE_CLUSTER_REGISTRY"
            else
                # using ocp
                image_full_path_on_edge_cluster_registry_internal_url=$DEFAULT_OCP_INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
            fi
        else
            image_full_path_on_edge_cluster_registry_internal_url=$INTERNAL_URL_FOR_EDGE_CLUSTER_REGISTRY/$EDGE_CLUSTER_REGISTRY_PROJECT_NAME/$EDGE_CLUSTER_AGENT_IMAGE_AND_TAG
        fi
        sed -i -e "s#__ImagePath__#${image_full_path_on_edge_cluster_registry_internal_url}#g" deployment.yml
    else
        log_fatal 1 "Agent install on edge cluster requires using an edge cluster registry"
        #sed -i -e "s#__ImagePath__#${AGENT_IMAGE}#g" deployment.yml
    fi

    log_debug "prepare_k8s_deployment_file() end"
}

# Cluster only: to create persistenClaim.yml based on template
function prepare_k8s_pvc_file() {
    log_debug "prepare_k8s_pvc_file() begin"

    # Note: get_edge_cluster_files() already downloaded deployment-template.yml, if necessary

    sed -e "s#__AgentNameSpace__#${AGENT_NAMESPACE}#g" -e "s/__StorageClass__/\"${EDGE_CLUSTER_STORAGE_CLASS}\"/g" persistentClaim-template.yml >persistentClaim.yml
    chk $? 'creating persistentClaim.yml'

    log_debug "prepare_k8s_pvc_file() end"
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
    create_persistent_volume

    log_debug "create_cluster_resources() end"
}

# Cluster only: to update cluster resources
function update_cluster_resources() {
    log_debug "update_cluster_resources() begin"

    update_secret
    update_configmap

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

    log_debug "create_namespace() end"
}

# Cluster only: to create service account for agent namespace and binding to cluster-admin clusterrole
function create_service_account() {
    log_debug "create_service_account() begin"

    log_verbose "checking if serviceaccont exist..."
    if ! $KUBECTL get serviceaccount ${SERVICE_ACCOUNT_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "serviceaccount ${SERVICE_ACCOUNT_NAME} does not exist, creating..."
        $KUBECTL create serviceaccount ${SERVICE_ACCOUNT_NAME} -n ${AGENT_NAMESPACE}
        chk $? "creating service account ${SERVICE_ACCOUNT_NAME}"
        log_info "serviceaccount ${SERVICE_ACCOUNT_NAME} created"

    else
        log_info "serviceaccount ${SERVICE_ACCOUNT_NAME} exists, skip creating serviceaccount"
    fi

    log_verbose "checking if clusterrolebinding exist..."

    if ! $KUBECTL get clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} 2>/dev/null; then
        log_verbose "Binding ${SERVICE_ACCOUNT_NAME} to cluster admin..."
        $KUBECTL create clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} --serviceaccount=${AGENT_NAMESPACE}:${SERVICE_ACCOUNT_NAME} --clusterrole=cluster-admin
        chk $? "creating clusterrolebinding for ${AGENT_NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
        log_info "clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} created"
    else
        log_info "clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME} exists, skip creating clusterrolebinding"
    fi
    log_debug "create_service_account() end"
}

# Cluster only: to create secret from cert file for agent deployment
function create_secret() {
    log_debug "create_secrets() begin"

    log_verbose "checking if secret ${SECRET_NAME} exist..."

    if ! $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        log_verbose "creating secret for cert file..."
        $KUBECTL create secret generic ${SECRET_NAME} --from-file=${AGENT_CERT_FILE} -n ${AGENT_NAMESPACE}
        chk $? "creating secret ${SECRET_NAME} from cert file ${AGENT_CERT_FILE}"
        log_info "secret ${SECRET_NAME} created"
    else
        log_info "secret ${SECRET_NAME} exists, skip creating secret"
    fi

    log_debug "create_secrets() end"
}

# Cluster only: to update secret
function update_secret() {
    log_debug "update_secret() begin"

    if $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1; then
        # secret exists, delete it
        log_verbose "Find secret ${SECRET_NAME} in ${AGENT_NAMESPACE} namespace, deleting the old secret..."
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
	        log_verbose "Find configmap ${CONFIGMAP_NAME} in ${AGENT_NAMESPACE} namespace, deleting the old configmap..."
            $KUBECTL delete configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} >/dev/null 2>&1
            chk $? 'deleting the old configmap for agent update on cluster'
            log_verbose "Old configmap ${CONFIGMAP_NAME} in ${AGENT_NAMESPACE} namespace is deleted"
        fi

        create_configmap

    fi

    log_debug "update_configmap() end"
}

# Cluster only: to create persistent volume claim for agent deployment
function create_persistent_volume() {
    log_debug "create_persistent_volume() begin"

    log_verbose "checking if persistent volume claim ${PVC_NAME} exist..."
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
    $KUBECTL get secret ${SECRET_NAME} -n ${AGENT_NAMESPACE} >/dev/null
    secret_ready=$?

    $KUBECTL get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAMESPACE} >/dev/null
    configmap_ready=$?

    $KUBECTL get pvc ${PVC_NAME} -n ${AGENT_NAMESPACE} >/dev/null
    pvc_ready=$?

    if [[ ${secret_ready} -eq 0 && ${configmap_ready} -eq 0 && ${pvc_ready} -eq 0 ]]; then
        return 0
    else
        return 1
    fi

    log_debug "check_resources_for_deployment() end"
}

# Cluster only: to create deployment
function create_deployment() {
    log_debug "create_deployment() begin"

    log_verbose "creating deployment..."
    $KUBECTL apply -f deployment.yml -n ${AGENT_NAMESPACE}
    chk $? 'creating deployment'

    log_debug "create_deployment() end"
}

# Cluster only: to update deployment
function update_deployment() {
    log_debug "update_deployment() begin"

    if [[ "$IS_AGENT_IMAGE_VERSION_SAME" == "true" ]] && [[ "$IS_HORIZON_ORG_ID_SAME" == "true" ]]; then
        log_info "Agent image version and HZN_ORG_ID are the same. Keeping the existing agent deployment."
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
    log_debug "check_resource_for_deployment() begin"
    log_info "Waiting up to $AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS seconds for the agent deployment to complete..."

    DEP_STATUS=$($KUBECTL rollout status --timeout=${AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS}s deployment/agent -n ${AGENT_NAMESPACE} | grep "successfully rolled out")
    if [[ -z "$DEP_STATUS" ]]; then
        log_fatal 3 "Deployment rollout status failed"
    fi
    log_debug "check_resource_for_deployment() end"
}

# Cluster only: to get agent pod id
function get_pod_id() {
    log_debug "get_pod_id() begin"

    if ! wait_for '[[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent -o "jsonpath={..status.conditions[?(@.type==\"Ready\")].status}") == "True" ]]' 'Horizon agent pod ready' $AGENT_WAIT_MAX_SECONDS; then
        log_fatal 3 "Horizon agent pod did not start successfully"
    fi

    if [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
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

# Cluster only: to install/update agent in cluster
function install_update_cluster() {
    log_debug "install_update_cluster() begin"
    confirmCmds ${DOCKER_ENGINE} jq

    check_existing_exch_node_is_correct_type "cluster"

    loadClusterAgentImage   # create the cluster agent docker image locally

    # push agent image to cluster's registry
    if [[ "$USE_EDGE_CLUSTER_REGISTRY" == "true" ]]; then
        pushImageToEdgeClusterRegistry
    fi

    check_agent_deployment_exist   # sets AGENT_DEPLOYMENT_UPDATE
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
    confirmCmds ${DOCKER_ENGINE} jq

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
    get_pod_id

    # register
    registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"
    log_debug "install_cluster() end"
}

# Cluster only: to update agent in cluster
function update_cluster() {
    log_debug "update_cluster() begin"

    set +e
    is_horizon_defaults_correct "$ANAX_PORT"
    set -e

    if is_agent_registered ; then
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
    get_pod_id

    registration "$AGENT_SKIP_REGISTRATION" "$HZN_EXCHANGE_PATTERN" "$HZN_NODE_POLICY"

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
            install_debian
        elif is_redhat_variant; then
            install_redhat
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
