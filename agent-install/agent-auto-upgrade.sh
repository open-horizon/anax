#!/bin/bash

# Cron job to support automated Horizon agent install and updates.
# It works for device type (native and anax-in-container).
# It is run by root.

# Arguments:
# 1 -- agent port

# Global constants

# logging levels
VERB_ERROR=1
VERB_WARNING=2
VERB_INFO=3
VERB_VERBOSE=4
VERB_DEBUG=5

VERBOSITY=5

AGENT_CERT_FILE_DEFAULT='agent-install.crt'
AGENT_CFG_FILE_DEFAULT='agent-install.cfg'
DEFAULT_VAR_BASE="/var/horizon/nmpdefault"
ROLLBACK_DIR_NAME="backup"
STATUS_FILE_NAME="status.json"
AGENT_WAIT_MAX_SECONDS=30

#====================== functions ======================
# variable that holds the return message for a function
FUNC_RET_MSG=""

function now() {
    echo $(date '+%Y-%m-%d %H:%M:%S')
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
    if [[ $VERBOSITY -ge $log_level ]]; then
        echo -e $(now) "$msg"
    fi
}

# ubuntu raspbian debian
function is_debian_variant() {
    return $(which apt-get > /dev/null 2>&1)
}

# rhel centos fedora
function is_redhat_variant() {
    return $(which dnf > /dev/null 2>&1)
}

# Create a status.json file to save the status.
# It is only called when calling anax API failed to update the status.
# The saved the status will be picked up by the anax node management
# worker when anax starts again.
function write_status_file() {
    local status_file_name=$1
    local status=$2
    local err_msg=$3

    local startTime=0
    local endTime=0
    if [ "$status" == "initiated" ]; then
        startTime=$(date +%s)
    elif [ "$status" == "successful" ]; then
        endTime=$(date +%s)
    elif [ "$status" == "rollback successful" ]; then
        endTime=$(date +%s)
    fi

    cat > $status_file_name << EOF
{
    "status": "$status",
    "startTime": "$startTime",
    "endTime": "$endTime",
    "errorMessage": "$err_msg"
}
EOF
}

# Sets the anax managment status for an 
function set_nodemanagement_status() {
    local nmp=$1
    local status_file_name=$2
    local status=$3
    local err_msg=$4

    #remove old status
    rm -Rf $status_file_name

    if [[ -n "$err_msg" ]]; then
        log_debug "Set node management status for nmp $nmp to \"$status\". Error: \"$err_msg\"."
    else
        log_debug "Set node management status for nmp $nmp to \"$status\"."
    fi

    read -d '' nm_status <<EOF
 {
    "type": "agentUpgrade",
    "status": "$status",
    "errorMessage": "$err_msg"
 }
EOF

    local output=$(echo "$nm_status" | curl -sS -X PUT -w %{http_code} -H "Content-Type: application/json" --data @- "${HORIZON_URL}/nodemanagement/status/$nmp")
    if [ "${output: -3}" != "201" ]; then
        log_error "Failed to set node management status for nmp $nmp to \"$status\". $output"

        # anax is not alive, save the status to the status file so that anax can pick it up later.
        write_status_file "$status_file_name" "$status" "$err_msg"
        return 2
    fi

    log_debug "Done setting node management status for nmp $nmp to \"$status\"."
}

# Make sure the package directionay exists.
# Unpack the packages.
function unpack_packages() {
    local pdir=$1

    FUNC_RET_MSG=""

    if [ ! -d "$pdir" ]; then
        FUNC_RET_MSG="The package directory $pdir does not exist on the host."
        return 2
    fi

    cd $pdir
    for pkg_file in *.tar.gz; do
        tar -zxf $pkg_file
        log_info "Unpacked file $pkg_file."
    done
    cd -
}

# Copy a list of files to the backup_dir preserving the directory structure.
function copy_pkg_files() {
    local backup_dir=$1
    local files=$2
    mkdir -p $backup_dir
    for f in $files; do
        if [ -f "$f" ]; then
            dir=$(dirname "$f")
            if [ ! -d "$backup_dir$dir" ]; then 
                mkdir -p $backup_dir$dir
            fi
            cp -f -p $f $backup_dir$f
        fi
    done
}

# copy the files from backup_dir to host keeping the directory structure.
function restore_pkg_files() {
    local backup_dir=$1
    # remove the last slash
    backup_dir=${backup_dir%/}

    FUNC_RET_MSG=""

    output=$(find "$backup_dir" -type f 2>&1)
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG=$output
        return $rc
    fi

    for f in $output; do
        fn=${f#$backup_dir}
        output=$(cp -f -p $f $fn 2>&1)
        rc=$?
        if [ $rc -ne 0 ]; then
            FUNC_RET_MSG="Failed to copy file $f to $fn. $output"
            return $rc
        fi
    done
}

# Save the agent config and binaries for rollback
function backup_agent_and_cli() {
   local backup_dir=$1

    # clean up old files 
    rm -Rf $backup_dir

    mkdir -p $backup_dir

    FUNC_RET_MSG=""
    OS='unknown'
    if [[ $OSTYPE == linux* ]]; then
        backup_agent_and_cli_linux $backup_dir
        return $?
    elif [[ $OSTYPE == darwin* ]]; then
        backup_agent_and_cli_macos $backup_dir
        return $?
    else
        FUNC_RET_MSG="Unsupported os type: $OSTYPE."
        return 5
    fi
}

# Save the agent config and binaries for rollback on Linux where the agent
# is running on the host.
function backup_agent_and_cli_linux() {
    log_info "Backing up horizon agent/cli binaries and configuration on Linux."
    local backup_dir=$1

    # get a list of files that horion package installs
    # and make a copy of each in the backup_dir
    if is_debian_variant; then 
        output_h=$(dpkg-query -L horizon)
    elif is_redhat_variant; then
        output_h=$(rpm -ql horizon)
    else
        FUNC_RET_MSG="Unsupported Linux disctro."
        return 5
    fi

    if [ $? -eq 0 ]; then
        copy_pkg_files "$backup_dir/horizon" "$output_h"

        # copy other files
        copy_pkg_files "$backup_dir/horizon" "/etc/default/horizon"
        copy_pkg_files "$backup_dir/horizon" "/var/horizon/anax.db"
        
        if [ -f "/etc/default/horizon" ]; then
            cert_file_name=$(grep HZN_MGMT_HUB_CERT_PATH /etc/default/horizon | sed 's/HZN_MGMT_HUB_CERT_PATH=//g')
            if [ -z "$cert_file_name" ]; then
                cert_file_name="/etc/horizon/agent-install.crt"
            fi
            copy_pkg_files "$backup_dir/horizon" "$cert_file_name"
        fi
    fi


    # get a list of files that horion-cli package installs
    # and make a copy of each in the backup_dir
    if is_debian_variant; then 
        output_hcli=$(dpkg-query -L horizon-cli)
    else
        output_hcli=$(rpm -ql horizon-cli)
    fi
    if [ $? -eq 0 ]; then
        copy_pkg_files "$backup_dir/horizon-cli" "$output_hcli"
    fi

    log_debug "End backing up horizon agent/cli binaries and configuration on Linux."
}

# Back up files for horizon-cli, amd64_anax image and agent cofiguration
# files on MacOS
function backup_agent_and_cli_macos() {
    log_info "Backing up horizon agent/cli binaries and configuration on MacOS."
    local backup_dir=$1

    # handle horizon-cli
    hcli_pkg_name=$(pkgutil --pkgs |grep horizon-cli)
    if [ $? -eq 0 ]; then
        # get install dir
        volume=$(pkgutil --pkg-info $hcli_pkg_name | grep "^volume:" | sed 's/volume: //g')
        location=$(pkgutil --pkg-info $hcli_pkg_name | grep "^location:" | sed 's/location: //g')
        install_loc="${volume%/}/${location}"
        install_loc="${install_loc%/}"

        # get files
        hcli_pkg_files=$(pkgutil --files $hcli_pkg_name)

        # get the full pathes for all the files in the package
        hcli_pkg_files_full=""
        for f in $hcli_pkg_files; do
            hcli_pkg_files_full+=" ${install_loc}/${f}"
        done
        
        # copy files
        copy_pkg_files "$backup_dir/horizon-cli" "$hcli_pkg_files_full"
    fi

    # get image and cert file and agent config file   
    copy_pkg_files "$backup_dir/horizon" "/etc/default/horizon"
    if [ -f "/etc/default/horizon" ]; then
        cert_file_name=$(grep HZN_MGMT_HUB_CERT_PATH /etc/default/horizon | sed 's/HZN_MGMT_HUB_CERT_PATH=//g')
        if [ -z "$cert_file_name" ]; then
            cert_file_name="/etc/horizon/agent-install.crt"
        fi
        copy_pkg_files "$backup_dir/horizon" "$cert_file_name"
    fi

    let horizon_num=${agentPort}-8080
    container_info=$(docker inspect horizon${horizon_num})
    if [ $? -eq 0 ]; then
        mkdir -p $backup_dir/container
        echo $container_info > $backup_dir/container/container_info.json
        mkdir -p $backup_dir/container/var/horizon
        docker cp horizon${horizon_num}:/var/horizon/anax.db $backup_dir/container/var/horizon/anax.db
        mkdir -p $backup_dir/container/etc/horizon/
        docker cp horizon${horizon_num}:/etc/horizon/anax.json $backup_dir/container/etc/horizon/anax.json
    fi

    log_debug "End backing up horizon agent/cli binaries and configuration on MacOS."
}

function rollback_agent_and_cli() {
    local backup_dir=$1

    FUNC_RET_MSG=""
    OS='unknown'
    if [[ $OSTYPE == linux* ]]; then
        rollback_agent_and_cli_linux $backup_dir
        return $?
    elif [[ $OSTYPE == darwin* ]]; then
        rollback_agent_and_cli_macos $backup_dir
        return $?
    else
        FUNC_RET_MSG="Unsupported os type: $OSTYPE."
        return 5
    fi
}

function rollback_agent_and_cli_linux() {
    log_info "Rolling back horizon agent/cli binaries and configuration on Linux."

    local backup_dir=$1
    FUNC_RET_MSG=""

    # stop the horizon service
    output=$(systemctl stop horizon 2>&1)
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed to stop the horizon service. $output"
        return $rc
    fi

    # restore the saved files
    restore_pkg_files $backup_dir/horizon
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed restore files for horizon package. $FUNC_RET_MSG"
        return $rc
    fi
    restore_pkg_files $backup_dir/horizon-cli
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed restore files for horizon-cli package. $FUNC_RET_MSG"
        return $rc
    fi

    # restart the horizon service
    output=$(systemctl start horizon 2>&1)
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed to start the horizon service. $output"
        return $rc
    fi

    log_debug "End rolling back horizon agent/cli binaries and configuration on Linux."
}

function rollback_agent_and_cli_macos() {
    log_info "Rolling back horizon agent/cli binaries and configuration on MacOS."

    local backup_dir=$1
    FUNC_RET_MSG=""

    # restore files
    restore_pkg_files $backup_dir/horiozn
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed restore files for horizon package. $FUNC_RET_MSG"
        return $rc
    fi
    restore_pkg_files $backup_dir/horiozn-cli
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed to restore files for horizon-cli package. $FUNC_RET_MSG"
        return $rc
    fi

    # restore files inside the agent container
    let horizon_num=${agentPort}-8080
    container_name=horizon${horizon_num}

    # kill current container
    docker rm -f $container_name

    # start a new container
    if [ ! -f "$backup_dir/container/container_info.json" ]; then
        FUNC_RET_MSG="Failed to find the original docker image for the agent container."
        return 2
    fi
    image=$(echo $backup_dir/container/container_info.json |jq '.[].Config.Image' 2>&1)
    rc=$?
    if [ $rc -ne 0 ]; then
        FUNC_RET_MSG="Failed to get the original agent image name."
        return $rc
    fi

    # remove the quotes from the image name 
    image="${image%\"}"
    image="${image#\"}"

    # start the container
    HC_DOCKER_IMAGE=${image%:*} HC_DOCKER_TAG=${image##*:} horizon-container start $horizon_num

    docker cp $backup_dir/container/var/horizon/anax.db horizon${horizon_num}:/var/horizon/anax.db
    docker cp $backup_dir/container/etc/horizon/anax.json horizon${horizon_num}:/etc/horizon/anax.json

    HC_DOCKER_IMAGE=${image%:*} HC_DOCKER_TAG=${image##*:} horizon-container update $horizon_num

    log_debug "End rolling back horizon agent/cli binaries and configuration on MacOS."
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
        log_error "Horizon agent did not start successfully"
    fi
    log_debug "wait_until_agent_ready() begin"
}

#====================== Main  ======================

# get the directory that this script is located
script_dir=$(cd "$(dirname "$0")" &> /dev/null && pwd)

# get agent port. 
# for anax in container, the agent port is given by the first cmd paremeter
agentPort="8510"
if [ -n "$1" ]; then
	agentPort=$1
fi
export HORIZON_URL="http://localhost:${agentPort}"

# check if the node is registered or not. It will not procceed if not.
hzn_node_list=$(hzn node list 2>/dev/null || true)   # if hzn not installed, hzn_node_list will be empty
log_debug "hzn node list:\n${hzn_node_list}"
node_state=$(jq -r .configstate.state 2>/dev/null <<< $hzn_node_list || true)
if [[ $node_state != 'configured' ]]; then
	log_info "The agent is not registered. Skip."
	exit 0
fi

# the script does not hanlde edge cluster case.
node_type=$(jq -r .nodeType 2>/dev/null <<< $hzn_node_list || true)
if [[ $node_type != 'device' ]]; then
	log_info "The agent is not 'device' type. Skip."
	exit 0
fi

# check if there are pending node auto upgrade job, pick the first one
log_debug "Call Agent to get next agent upgrade task:\n ${nextjob}"
output=$(curl -s -w %{http_code} ${HORIZON_URL}/nodemanagement/nextjob?"type=agentUpgrade&ready=true")
rc="${output: -3}" 
if [ "$rc" != "200" ]; then
    log_error "Failed to get next upgrade job from the agent. http code: ${rc}."
    exit 2
fi
nextjob="${output::-3}"
log_debug "Agent's next agent upgrade task:\n ${nextjob}"

# exit if there are no tasks
num_keys=$(jq -r ".|keys|length" 2>/dev/null <<< $nextjob || true)
if [ "$num_keys" == "0" ]; then
    log_info "No agent upgrade tasks pending. Skip"
    exit 0
fi

# make sure the status is 'downloaded'
nmp_id=$(jq -r '.|keys[0]' 2>/dev/null <<< $nextjob || true)
nm_status=$(jq -r ".\"$nmp_id\".agentUpgrade.status" 2>/dev/null <<< $nextjob || true)
if [ "$nm_status" != "downloaded" ]; then
    log_info "Cannot continue because the current node management stautus is '$nm_status' instead of 'downloaded'."  
    exit 0
fi

# gather node configuration information
node_id=$(jq -r .id 2>/dev/null <<< $hzn_node_list || true)
node_org_id=$(jq -r .organization 2>/dev/null <<< $hzn_node_list || true)
exch_url=$(jq -r .configuration.exchange_api 2>/dev/null <<< $hzn_node_list || true)
log_debug "The exchange url is: $exch_url"

# get package directory
pkg_dir=$(jq -r ".\"$nmp_id\".agentUpgrade.workingDirectory" 2>/dev/null <<< $nextjob || true)
if [ -z "$pkg_dir" ]; then
    pkg_dir="$DEFAULT_VAR_BASE"
fi
#remove last lash and add nmp id to the path
pkg_dir=${pkg_dir%/}
pkg_dir=$pkg_dir/$nmp_id

log_info "Package directory: $pkg_dir"

mkdir -p pkg_dir
status_file="$pkg_dir/$STATUS_FILE_NAME"

# unpack the package files under the package dir
unpack_packages $pkg_dir
if [ $? -ne 0 ]; then
   log_error "Failed unpacking the packages. $FUNC_RET_MSG"
   set_nodemanagement_status "$nmp_id" "$status_file" "failed" "Failed unpacking the packages. $FUNC_RET_MSG"
fi

# save current config and binaries for later in case there is a need for rollback
backup_agent_and_cli "$pkg_dir/$ROLLBACK_DIR_NAME"
if [ $? -ne 0 ]; then
   log_error "Failed backing up the horizon agent and cli. $FUNC_RET_MSG"
   set_nodemanagement_status "$nmp_id" "$status_file" "failed" "Failed backing up the horizon agent and cli. $FUNC_RET_MSG"
fi

# update the management status to 'initiated'
set_nodemanagement_status "$nmp_id" "$status_file" "initiated" "" 

# the agent-install.sh can be found under the package directory.
# if not found, use the one that ships with the current horizon-cli package.
agent_install_sname="${pkg_dir}/agent-install.sh"
if [ ! -f $agent_install_sname ]; then
    agent_install_sname="${script_dir}/agent-install.sh"
fi

# run agent-intall.sh. catch the stdout and stderr in different variables 
unset Std_msg Err_msg RC
cd $pkg_dir
#eval "$(${script_dir}/agent-install.sh -A -i ${pkg_dir} -d ${node_id} -O ${node_org_id} \
#        -c ${pkg_dir}/${AGENT_CERT_FILE_DEFAULT} -k ${pkg_dir}/${AGENT_CFG_FILE_DEFAULT} -s -b -f \
#        2> >(Err_msg=$(cat); typeset -p Err_msg) \
#        > >(Std_msg=$(cat); typeset -p Std_msg); \
#        RC=$?; typeset -p RC)"
eval "$(${agent_install_sname} -A -d ${node_id} -O ${node_org_id} \
        -s -b -f \
        2> >(Err_msg=$(cat); typeset -p Err_msg) \
        > >(Std_msg=$(cat); typeset -p Std_msg); \
        RC=$?; typeset -p RC)"
rc=$RC
# display the agenty-install.sh output
echo "$Std_msg"
#check the return code
if [ $rc -ne 0 ]; then
    #remove the timestamp and word ERROR from err_msg
    if [ -z "$Err_msg" ]; then
       errmsg="Unknown."
    else   
        errmsg=$(echo $Err_msg | cut -d' ' -f4-)
    fi

	log_error "Agent automated upgrade failed. $errmsg"
    set_nodemanagement_status "$nmp_id" "$status_file" "failed" "$errmsg"

    if [ $rc -ge 3 ]; then
        # rolling back
        set_nodemanagement_status "$nmp_id" "$status_file" "rollback started" ""
        rollback_agent_and_cli "$pkg_dir/$ROLLBACK_DIR_NAME"
        if [ $? -ne 0 ]; then
            log_error "Failed to roll back. $FUNC_RET_MSG"
            set_nodemanagement_status "$nmp_id" "$status_file" "rollback failed" "Filed rolling back. $FUNC_RET_MSG"
            exit 3
        else
            # remove backups
            rm -Rf $pkg_dir/$ROLLBACK_DIR_NAME

            wait_until_agent_ready

            # update the management status
            log_info "Rollback successful."
            set_nodemanagement_status "$nmp_id" "$status_file" "rollback successful" ""
        fi
    else
        # update the management status with error message
	    exit 2
    fi
else
    # remove backups
    rm -Rf $pkg_dir/$ROLLBACK_DIR_NAME

    wait_until_agent_ready

    # update the management status to 'successful'
    log_info "Update successful."
    set_nodemanagement_status "$nmp_id" "$status_file" "successful" ""
fi


