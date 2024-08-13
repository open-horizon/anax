#!/bin/bash

# Variables for interfacing with agent pod
KUBECTL="kubectl"

# Timeout value for agent deployment
AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS='300'

# status.json status options
STATUS_FAILED="failed"
STATUS_INITIATED="initiated"
STATUS_ROLLBACK_STARTED="rollback started"
STATUS_ROLLBACK_FAILED="rollback failed"
STATUS_ROLLBACK_SUCCESSFUL="rollback successful"

# Logging levels
VERB_FATAL=0
VERB_ERROR=1
VERB_WARNING=2
VERB_INFO=3
VERB_VERBOSE=4
VERB_DEBUG=5

# Script usage info
function usage() {
    local exit_code=$1
    cat <<EndOfMessage
${0##*/} <options>

Rollback the Horizon agent on an edge cluster to a previous version.

Options/Flags:
    -h      This usage

EndOfMessage
    exit $exit_code
}

# Get current date/timestamp
function now() {
    echo $(date '+%Y-%m-%d %H:%M:%S')
}

# Default logging level
AGENT_VERBOSITY=4

# Get script flags (should never really run unless testing script manually)
if [[ $AGENT_VERBOSITY -ge $VERB_DEBUG ]]; then echo $(now) "getopts begin"; fi
while getopts "c:h:l:" opt; do
    case $opt in
    h)  usage 0
        ;;
    l)  AGENT_VERBOSITY="$OPTARG"
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


# Logging functions

function log_fatal() {
    : ${1:?} ${2:?}
    local exitCode=$1
    local msg="$2"
    echo $(now) "ERROR: $msg" | tee /dev/stderr
    exit $exitCode
}

function log_error() {
    log $VERB_ERROR "ERROR: $1" | tee /dev/stderr     
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

# Utility function for running commands on agent pod
function agent_cmd() {
    $KUBECTL exec -i ${POD_ID} -n ${AGENT_NAMESPACE} -c anax -- bash -c "$@"
}

# Writes cronjob pod logs to /var/horizon/cronjob.log
# Only last 1 million lines of /var/horizon/cronjob.log is kept 
#  - (~100 days worth if schedule is every 15 minutes)
function write_logs() {
    sleep 5 # delay due to syncing issue between k8s and container status
    echo $(now) "CRONJOB LOGS FOR JOB: $CRONJOB_POD_NAME" >> /var/horizon/cronjob.log
    $KUBECTL logs $CRONJOB_POD_NAME -n $AGENT_NAMESPACE >> /var/horizon/cronjob.log
    tail -n 1000000 /var/horizon/cronjob.log > /var/horizon/cronjob.log.tmp
    mv -f /var/horizon/cronjob.log.tmp /var/horizon/cronjob.log
}

# update the error message in status file
function update_error_message() {
    local msg="$1"
    current_err_message=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.errorMessage' | sed 's/\"//g')
    if [[ "$current_err_message" == "" ]]; then
        # update error message in status.json
        echo $(jq --arg updated_message "${msg}" '.agentUpgradePolicyStatus.errorMessage = $updated_message' $STATUS_PATH) > $STATUS_PATH
    elif [[ "$current_err_message" == "null" ]]; then
        # error message field is omitted, add errorMessage
        echo $(jq --arg updated_message "${msg}" '.agentUpgradePolicyStatus += {"errorMessage": $updated_message}' $STATUS_PATH) > $STATUS_PATH
    fi
}

# Sets the status to "rollback failed" and exits with a fatal error
# Also pushes logs to /var/horizon/cronjob.log
function rollback_failed() {
    local msg="$1"
    log_verbose "Setting status to \"$STATUS_ROLLBACK_FAILED\""
    echo $(jq --arg updated_status "$STATUS_ROLLBACK_FAILED" '.agentUpgradePolicyStatus.status = $updated_status' $STATUS_PATH) > $STATUS_PATH
    update_error_message "$msg"
    write_logs
    log_fatal 1 "$msg"
}

# Performs a rollback of the config map
function rollback_configmap() {
    log_debug "rollback_configmap() begin"

    local cmd_output
    local rc

    # Check if a backup exists. If not, keep it and set status.json to "rollback failed"
    log_verbose "Checking for backup config map..."
    local backup_config
    backup_config=$($KUBECTL get configmap -n ${AGENT_NAMESPACE} | grep openhorizon-agent-config-backup)
    if [[ -z $backup_config ]]; then
        rollback_failed "Config Map needs rollback, but \"openhorizon-agent-config-backup\" does not exist in the ${AGENT_NAMESPACE} namespace"
    fi

    # Download the config map backup to a yaml file
    log_verbose "Downloading backup config map to yaml file..."
    cmd_output=$( { $KUBECTL get configmap -n ${AGENT_NAMESPACE} openhorizon-agent-config-backup -o yaml > /tmp/agentbackup/openhorizon-agent-config-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Config map backup could not be retrieved. Error: $cmd_output"
    fi

    # Rename the config map name in the yaml file
    log_verbose "Renaming \"openhorizon-agent-config-backup\" to \"openhorizon-agent-config\"..."
    sed -i 's/openhorizon-agent-config-backup/openhorizon-agent-config/g' /tmp/agentbackup/openhorizon-agent-config-backup.yaml
    
    # Delete the currently deployed config map
    log_verbose "Deleting current config map..."
    cmd_output=$( { $KUBECTL delete configmap -n ${AGENT_NAMESPACE} openhorizon-agent-config; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Config map could not be deleted. Error: $cmd_output"
    fi
    
    # Recreate config map using yaml file
    log_verbose "Creating new config map from backup yaml file..."
    cmd_output=$( { $KUBECTL apply -n ${AGENT_NAMESPACE} -f /tmp/agentbackup/openhorizon-agent-config-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Config map could not be created. Error: $cmd_output"
    fi

    log_debug "rollback_configmap() end"
}

# Performs a rollback of the agent secret
function rollback_secret() {
    log_debug "rollback_secret() begin"

    local cmd_output
    local rc

    # Check if a backup exists. If not, keep it and set status.json to "rollback failed"
    log_verbose "Checking for backup secret..."
    local backup_secret
    backup_secret=$($KUBECTL get secret -n ${AGENT_NAMESPACE} | grep openhorizon-agent-secrets-backup)
    if [[ -z $backup_secret ]]; then
        rollback_failed "Secret needs rollback, but \"openhorizon-agent-secrets-backup\" does not exist in the ${AGENT_NAMESPACE} namespace"
    fi

    # Download the config map backup to a yaml file
    log_verbose "Downloading backup secret to yaml file..."
    cmd_output=$( { $KUBECTL get secret -n ${AGENT_NAMESPACE} openhorizon-agent-secrets-backup -o yaml > /tmp/agentbackup/openhorizon-agent-secrets-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Secret backup could not be retrieved. Error: $cmd_output"
    fi

    # Rename the secret name in the yaml file
    log_verbose "Renaming \"openhorizon-agent-secrets-backup\" to \"openhorizon-agent-secrets\"..."
    sed -i 's/openhorizon-agent-secrets-backup/openhorizon-agent-secrets/g' /tmp/agentbackup/openhorizon-agent-secrets-backup.yaml
    
    # Delete the currently deployed secret
    log_verbose "Deleting current secret..."
    cmd_output=$( { $KUBECTL delete secret -n ${AGENT_NAMESPACE} openhorizon-agent-secrets; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Secret could not be deleted. Error: $cmd_output"
    fi
    
    # Recreate secret using yaml file
    log_verbose "Creating new secret from backup yaml file..."
    cmd_output=$( { $KUBECTL apply -n ${AGENT_NAMESPACE} -f /tmp/agentbackup/openhorizon-agent-secrets-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Secret could not be created. Error: $cmd_output"
    fi

    log_debug "rollback_secret() end"
}

# Checks agent deplyment status
function check_deployment_status() {
    log_debug "check_deployment_status() begin"
    log_info "Waiting up to $AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS seconds for the agent deployment to complete..."

    local dep_status
    dep_status=$($KUBECTL rollout status --timeout=${AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS}s deployment/agent -n ${AGENT_NAMESPACE} | grep "successfully rolled out")
    if [[ -z "$dep_status" ]]; then
        rollback_failed "Deployment rollout status failed"
    fi
    log_debug "check_deployment_status() end"
}

# Performs a rollback of the agent secret
function rollback_agent_image() {
    log_debug "rollback_agent_image() begin"

    local cmd_output
    local rc

    # Determine what version the agent attempted to upgrade from
    local old_image_version
    local current_version
    old_image_version=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.imageVersion.from' | sed 's/\"//g')
    log_debug "Old image version: $old_image_version"
    current_version=$($KUBECTL get deployment -n ${AGENT_NAMESPACE} agent -o=jsonpath='{$..spec.template.spec.containers[0].image}' | sed 's/.*://')

    # Download the agent deployment to a yaml file
    log_verbose "Downloading agent deployment to yaml file..."
    cmd_output=$( { $KUBECTL get deployment -n ${AGENT_NAMESPACE} agent -o yaml > /tmp/agentbackup/deployment.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Agent deployment could not be retrieved. Error: $cmd_output"
    fi

    # Replace the agent version in the yaml file to the version the agent attempted to upgrade from
    current_anax_image_version="_anax_k8s:$current_version"
    old_anax_image_version="_anax_k8s:$old_image_version"
    log_verbose "Downgrading version from $current_version to $old_image_version..."
    sed -i "s/$current_anax_image_version/$old_anax_image_version/g" /tmp/agentbackup/deployment.yaml

    log_debug "New deployment.yaml file: $(cat /tmp/agentbackup/deployment.yaml)"
    
    # Delete the current agent deployment
    log_verbose "Deleting current agent deployment..."
    cmd_output=$( { $KUBECTL delete deployment -n ${AGENT_NAMESPACE} agent; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Agent deployment could not be retrieved. Error: $cmd_output"
    fi
    
    # Redeploy the agent using the yaml file
    log_verbose "Creating new agent deployment from backup yaml file..."
    cmd_output=$( { $KUBECTL apply -n ${AGENT_NAMESPACE} -f /tmp/agentbackup/deployment.yaml; } 2>&1 )
    log_debug "Output from apply command: $cmd_output"
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Agent deployment could not be retrieved. Error: $cmd_output"
    fi

    # Make sure the agent actually gets deployed
    check_deployment_status

    log_debug "rollback_agent_image() end"
}

# Sets $STATUS_PATH to the path of the next agent upgrade job that failed
function get_status_path() {
    log_debug "get_status_path() start"

    # Exit early if /var/horizon/nmp does not exist
    log_verbose "Checking if /var/horizon/nmp directory exists..."
    local auto_upgrade_disabled
    test -d /var/horizon/nmp 2>&1
    auto_upgrade_disabled=$?
    if [ "$auto_upgrade_disabled" -eq "1" ]; then
        log_info "Auto agent upgrade is disabled. /var/horizon/nmp does not exist."
        write_logs
        exit 0
    fi

    # Get the org of the node, exit early if /var/horizon/nmp/{org} does not exist
    log_verbose "Checking if /var/horizon/nmp/{org} directory exists..."
    local nmp_org_arr
    nmp_org_arr=( $(dir /var/horizon/nmp/) )
    local nmp_org_arr_len
    nmp_org_arr_len=$(echo ${#nmp_org_arr[@]})
    if [ "$nmp_org_arr_len" -eq "0" ]; then
        log_info "There are no active auto upgrade jobs. /var/horizon/nmp has no org directory."
        write_logs
        exit 0
    fi
    if [ "$nmp_org_arr_len" -ne "1" ]; then
        write_logs
        log_fatal 1 "/var/horizon/nmp has multiple org directories: ${nmp_org_arr[*]}"
    fi
    local nmp_org
    nmp_org=${nmp_org_arr[0]}

    # Exit early if no NMP subdirectories exist
    log_verbose "Searching NMP subdirectories..."
    local nmp_arr
    nmp_arr=( $(dir /var/horizon/nmp/$nmp_org/) )
    local nmp_arr_len
    nmp_arr_len=$(echo ${#nmp_arr[@]})
    if [ "$nmp_arr_len" -eq "0" ]; then
        log_info "There are no active auto upgrade jobs. /var/horizon/nmp/$nmp_org has no NMP directories."
        write_logs
        exit 0
    fi

    # Loop through NMP subdirectories and check each status.json for newest job that needs rollback
    log_verbose "Getting latest upgrade job status file..."
    local latest
    latest=-1
    local filepath
    filepath=/var/horizon/nmp/$nmp_org
    STATUS_PATH=""
    for nmp_name in "${nmp_arr[@]}"; do

        # Check if status.json ile exists, throw warning if it doesn't and continue
        local status_not_exists
        test -f $filepath/$nmp_name/status.json 2>&1
        status_not_exists=$?
        if [ "$status_not_exists" -eq "1" ]; then
            log_warning "$filepath/$nmp_name exists, but does not contain status.json file, skipping."
        else
            # Get timestamp of status file and convert to seconds, ans get status of file
            local timestamp
            timestamp=$(cat $filepath/$nmp_name/status.json | jq -r '.agentUpgradePolicyStatus.startTime')
            local status
            status=$(cat $filepath/$nmp_name/status.json | jq -r '.agentUpgradePolicyStatus.status')
            local seconds
            seconds=$(date -d $timestamp "+%s")

            # Only set STATUS_PATH to current status file if it is the newest so far, and has a failed status
            if [ "$seconds" -gt "$latest" ]; then
                if [ "$status" = "$STATUS_FAILED" ] || [ "$status" = "$STATUS_INITIATED" ] || [ "$status" = "$STATUS_ROLLBACK_STARTED" ]; then
                    latest=$seconds
                    STATUS_PATH=$filepath/$nmp_name/status.json
                    CURRENT_STATUS=$status
                fi
            fi
        fi
    done

    log_info "STATUS_PATH is $STATUS_PATH"

    # Exit early if status.json does not exist
    local no_active_jobs
    test -f $STATUS_PATH 2>&1
    no_active_jobs=$?
    if [[ "$STATUS_PATH" == "" || "$no_active_jobs" -eq "1" ]]; then
        log_info "There are no active auto upgrade jobs."
        write_logs
        exit 0
    fi

    log_verbose "Found job: $STATUS_PATH"

    log_debug "get_status_path() end"
}

function restart_agent_pod() {
    log_debug "function restart_agent_pod() start"

    log_info "Restarting agent pod..."
    cmd_output=$( { $KUBECTL rollout restart deployment agent -n ${AGENT_NAMESPACE}; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "There was an unexpected error while restarting agent pod: $cmd_output"
    fi

    log_debug "function restart_agent_pod() end"
}

#====================== Main  ======================

log_info "cronjob under namespace: $AGENT_NAMESPACE"

# Sets STATUS_PATH for rest of script
get_status_path

dep_status=$($KUBECTL rollout status --timeout=${AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS}s deployment/agent -n ${AGENT_NAMESPACE} | grep "successfully rolled out")
log_debug "deployment rollout status: $dep_status"

POD_ID=$($KUBECTL get pod -l app=agent,type!=auto-upgrade-cronjob -n ${AGENT_NAMESPACE} 2>/dev/null | grep "agent-" | cut -d " " -f1 2>/dev/null)
pod_status=$($KUBECTL get pods ${POD_ID} -n ${AGENT_NAMESPACE} --no-headers -o custom-columns=":status.phase" | sed -z  's/\n/ /g;s/ //g' )
log_debug "Pod status: $pod_status"

# Check deployment/pod status
# Instantaneous state where both could be running.... 
if [[ "${pod_status}" ==  "RunningRunning"  ]] || [[ "${pod_status}" == "RunningSucceeded" ]]; then
    log_info "Agent pod status is $pod_status; Exiting"
    write_logs
    exit 0
fi

log_info "Checking if there is any pending agent pod..."
if [[ "$pod_status" == *Pending* ]]; then
    log_info "Agent pod is still in pending. Keeping status as \"$CURRENT_STATUS\" and exiting."
    write_logs
    exit 0
fi

if [[ ! -f $STATUS_PATH ]]; then
    log_debug "status file $STATUS_PATH not exist;  Exiting."
    write_logs
    exit 0
fi

json_status=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.status' | sed 's/\"//g') # directory will be deleted by NMP worker if the upgrade is successful
log_debug "Cron Job status: $json_status"
CURRENT_STATUS=$json_status
panic_rollback=false

log_info "Checking if agent is running and deployment is successful..."
if [[ "$pod_status" != "Running" || -z "$dep_status" ]]; then # pod is not running, deployment rollout failed 
    panic_rollback=true

# Should never happen, but if status is "rollback started" and agent is running properly, check agent pod for panic"
elif [[ "$json_status" == "$STATUS_ROLLBACK_STARTED" ]]; then
    log_info "Deployment is successful and rollback was already attempted. Checking agent pod for panic..."
    cmd_output=$(agent_cmd "hzn node list" | grep "nodeType")
    rc=$?

    # If the pod is reachable from hzn node list, just exit.
    if [[ $rc -eq 0 && "$cmd_output" == *"nodeType"*"cluster"* ]]; then
        log_info "Agent pod is running successfully."
        log_verbose "Setting the status to \"$STATUS_ROLLBACK_SUCCESSFUL\"..."
        echo $(jq --arg updated_status "$STATUS_ROLLBACK_SUCCESSFUL" '.agentUpgradePolicyStatus.status = $updated_status' $STATUS_PATH) > $STATUS_PATH
        write_logs
        exit 0
    fi

    # If there is a panic on the agent pod, set status to "rollback failed"
    cmd_output=$(agent_cmd "hzn node list")
    rollback_failed "Rollback was already attempted, but the agent pod is in panic. Output of \"hzn node list\": $cmd_output"

# If status is initiated and agent pod is running, check for panic
elif [[ "$json_status" == "$STATUS_INITIATED" ]]; then
    
    log_info "Deployment is successful but status is still in $CURRENT_STATUS state. Checking agent pod for panic..."

    current_version=$($KUBECTL get deployment -n ${AGENT_NAMESPACE} agent -o=jsonpath='{..image}' | sed 's/.*://')
    old_image_version=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.imageVersion.from' | sed 's/\"//g')

    # Status is initiated and agent pod is running successfully, just exit.
    cmd_output=$(agent_cmd "hzn node list" | grep "nodeType")
    rc=$?
    if [[ $rc -eq 0 && "$cmd_output" == *"nodeType"*"cluster"* ]]; then
        log_info "Deployment is successful and pod is not in panic state. Keeping status as \"$CURRENT_STATUS\" and exiting."
        write_logs
        exit 0

    # Status is initiated but agent pod is in panic state, despite not being upgraded yet
    elif [[ "$old_image_version" == "null" || "$current_version" == "$old_image_version" ]]; then
        # set status to "failed"
        log_info "Agent pod is in panic state and the image version was not updated. Setting status to \"$ROLLBACK_FAILED\" and exiting."
        echo $(jq --arg updated_status "$ROLLBACK_FAILED" '.agentUpgradePolicyStatus.status = $updated_status' $STATUS_PATH) > $STATUS_PATH
        log_debug "Output of \"hzn node list\": $cmd_output"
        write_logs
        log_fatal 1 "Agent pod is in panic state and the image version was not updated."

    # Status is initiated and agent pod is in panic state. Rollback needs to be performed.
    else
        cmd_output=$(agent_cmd "hzn node list")
        log_info "Agent pod is in panic state. Rollback will be performed."
        log_debug "Output of \"hzn node list\": $cmd_output"
        update_error_message "Agent pod is in panic state"
    fi

    panic_rollback=true
fi

# Rollback
log_info "Starting rollback process..."
    
# Set the status to "rollback started" in status.json
log_verbose "Setting the status to \"$STATUS_ROLLBACK_STARTED\"..."
echo $(jq --arg updated_status "$STATUS_ROLLBACK_STARTED" '.agentUpgradePolicyStatus.status = $updated_status' $STATUS_PATH) > $STATUS_PATH
CURRENT_STATUS=$STATUS_ROLLBACK_STARTED

# Configmap:
log_info "Checking config map status..."
config_change=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.configMap.needChange')
config_updated=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.configMap.updated')

log_debug "Config map needs change: $config_change"
log_debug "Config map was updated: $config_updated"

# If the status.json set config_updated to true, perform a rollback of the config map
if [[ "$config_updated" == "true" ]]; then
    if [[ "$config_change" == "false" ]]; then
        log_warning "The config map was updated, but there was no config map change request. Attempting rollback anyway..."
    fi
    log_info "Rollback config map..."
    rollback_configmap
fi

# Secret:
log_info "Checking secrets status..."
secret_change=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.secret.needChange')
secret_updated=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.secret.updated')

log_debug "Secret needs change: $secret_change"
log_debug "Secret was updated: $secret_updated"

# If the status.json set secret_updated to true, perform a rollback of the secret
if [[ "$secret_updated" == "true" ]]; then
    if [[ "$secret_change" == "false" ]]; then
        log_warning "The secret was updated, but there was no secret change request. Attempting rollback anyway..."
    fi
    log_info "Rollback secrets..."
    rollback_secret
fi

# Agent image rollback:
log_info "Checking agent image status..."
image_change=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.imageVersion.needChange')
image_updated=$(cat $STATUS_PATH | jq '.agentUpgradePolicyStatus.k8s.imageVersion.updated')

log_debug "Agent image needs change: $image_change"
log_debug "Agent image was updated: $image_updated"

# If the status.json set image_change to true, perform a rollback of the secret
if [[ $image_updated == "true" || $panic_rollback == "true" ]]; then
    if [[ "$image_change" == "false" ]]; then
        log_error "The agent image was updated, but there was no image change request. Attempting rollback anyway..."
    fi
    log_info "Rollback agent image..."
    rollback_agent_image

# If the pod is not running, simply restart it
elif [[ "$pod_status" != "Running" || "$dep_status" != "Running" ]]; then
    restart_agent_pod
    check_deployment_status
fi

if [[ $image_updated == "false" && $secret_updated == "false" && $config_updated == "false" && $panic_rollback == "false" ]]; then
    log_info "The agent image, config and secret were not updated. Keeping status as \"$CURRENT_STATUS\" and exiting."
    write_logs
    exit 0
fi

# If the rollback ran successfully, update status to "rollback successful". 
log_verbose "Setting the status to \"$STATUS_ROLLBACK_SUCCESSFUL\"..."
echo $(jq --arg updated_status "$STATUS_ROLLBACK_SUCCESSFUL" '.agentUpgradePolicyStatus.status = $updated_status' $STATUS_PATH) > $STATUS_PATH
CURRENT_STATUS=$STATUS_ROLLBACK_SUCCESSFUL

# Delete backup configmap
if [[ "$config_updated" == "true" ]]; then
    log_verbose "Deleting backup config map..."
    cmd_output=$( { $KUBECTL delete configmap -n ${AGENT_NAMESPACE} openhorizon-agent-config-backup; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        log_error "Config map backup could not be deleted. Error: $cmd_output"
    fi
fi

# Delete backup secret
if [[ "$secret_updated" == "true" ]]; then
    log_verbose "Deleting backup secrets..."
    cmd_output=$( { $KUBECTL delete secret -n ${AGENT_NAMESPACE} openhorizon-agent-secrets-backup; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        log_error "Secret backup could not be deleted. Error: $cmd_output"
    fi
fi

log_info "Rollback successful."
write_logs
