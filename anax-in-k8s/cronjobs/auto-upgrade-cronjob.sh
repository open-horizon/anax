#!/bin/bash

KUBECTL="kubectl"
AGENT_NAMESPACE="openhorizon-agent"
POD_ID=$($KUBECTL get pod -l app=agent -n ${AGENT_NAMESPACE} 2>/dev/null | grep "agent-" | cut -d " " -f1 2>/dev/null)

AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS='75'

# logging levels
#VERB_FATAL=0   # we always show fata error msgs
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

function now() {
    echo $(date '+%Y-%m-%d %H:%M:%S')
}

AGENT_VERBOSITY=5   # default

if [[ $AGENT_VERBOSITY -ge $VERB_DEBUG ]]; then echo $(now) "getopts begin"; fi
while getopts "c:h" opt; do
    case $opt in
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

function agent_cmd() {
    $KUBECTL exec -i ${POD_ID} -n ${AGENT_NAMESPACE} -c anax -- bash -c "$@"
}

# Sets the status to "rollback failed" and exits with a fatal error
function rollback_failed() {
    local msg="$1"
    log_verbose "Setting status to \"rollback failed\""
    agent_cmd "sed -i 's/\"status\":.*/\"status\": \"rollback failed\",/g' /var/horizon/status.json"
    log_fatal 1 "$msg"
}

# Performs a rollback of the config map
function rollback_configmap() {
    log_debug "rollback_configmap() begin"

    # Check if a backup exists. If not, keep it and set status.json to "rollback failed"
    log_verbose "Checking for backup config map..."
    backup_config=$($KUBECTL get configmap -n ${AGENT_NAMESPACE} | grep openhorizon-agent-config-backup)
    if [[ -z $backup_config ]]; then
        rollback_failed "Config Map needs rollback, but \"openhorizon-agent-config-backup\" does not exist in the ${AGENT_NAMESPACE} namespace"
    fi

    # Download the config map backup to a yaml file
    log_verbose "Downloading backup config map to yaml file..."
    cmd_output=$( { $KUBECTL get configmap -n ${AGENT_NAMESPACE} openhorizon-agent-config-backup -o yaml > /tmp/openhorizon-agent-config-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Config map backup could not be retrieved. Error: $cmd_output"
    fi

    # Rename the config map name in the yaml file
    log_verbose "Renaming \"openhorizon-agent-config-backup\" to \"openhorizon-agent-config\"..."
    sed -i 's/openhorizon-agent-config-backup/openhorizon-agent-config/g' /tmp/openhorizon-agent-config-backup.yaml
    
    # Delete the currently deployed config map
    log_verbose "Deleting current config map..."
    cmd_output=$( { $KUBECTL delete configmap -n ${AGENT_NAMESPACE} openhorizon-agent-config; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Config map could not be deleted. Error: $cmd_output"
    fi
    
    # Recreate config map using yaml file
    log_verbose "Creating new config map from backup yaml file..."
    cmd_output=$( { $KUBECTL apply -n ${AGENT_NAMESPACE} -f /tmp/openhorizon-agent-config-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Config map could not be created. Error: $cmd_output"
    fi

    log_debug "rollback_configmap() end"
}

# Performs a rollback of the agent secret
function rollback_secret() {
    log_debug "rollback_secret() begin"

    # Check if a backup exists. If not, keep it and set status.json to "rollback failed"
    log_verbose "Checking for backup secret..."
    backup_secret=$($KUBECTL get secret -n ${AGENT_NAMESPACE} | grep openhorizon-agent-secrets-backup)
    if [[ -z $backup_secret ]]; then
        rollback_failed "Secret needs rollback, but \"openhorizon-agent-secrets-backup\" does not exist in the ${AGENT_NAMESPACE} namespace"
    fi

    # Download the config map backup to a yaml file
    log_verbose "Downloading backup secret to yaml file..."
    cmd_output=$( { $KUBECTL get secret -n ${AGENT_NAMESPACE} openhorizon-agent-secrets-backup -o yaml > /tmp/openhorizon-agent-secrets-backup.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Secret backup could not be retrieved. Error: $cmd_output"
    fi

    # Rename the secret name in the yaml file
    log_verbose "Renaming \"openhorizon-agent-secrets-backup\" to \"openhorizon-agent-secrets\"..."
    sed -i 's/openhorizon-agent-secrets-backup/openhorizon-agent-secrets/g' /tmp/openhorizon-agent-secrets-backup.yaml
    
    # Delete the currently deployed secret
    log_verbose "Deleting current secret..."
    cmd_output=$( { $KUBECTL delete secret -n ${AGENT_NAMESPACE} openhorizon-agent-secrets; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Secret could not be deleted. Error: $cmd_output"
    fi
    
    # Recreate secret using yaml file
    log_verbose "Creating new secret from backup yaml file..."
    cmd_output=$( { $KUBECTL apply -n ${AGENT_NAMESPACE} -f /tmp/openhorizon-agent-secrets-backup.yaml; } 2>&1 )
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

    dep_status=$($KUBECTL rollout status --timeout=${AGENT_DEPLOYMENT_STATUS_TIMEOUT_SECONDS}s deployment/agent -n ${AGENT_NAMESPACE} | grep "successfully rolled out")
    if [[ -z "$dep_status" ]]; then
        rollback_failed "Deployment rollout status failed"
    fi
    log_debug "check_deployment_status() end"
}

# Performs a rollback of the agent secret
function rollback_agent_image() {
    log_debug "rollback_agent_image() begin"

    # Determine what version the agent attempted to upgrade from
    log_verbose "Checking agent image version change..."
    old_image_version=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.image.from'")
    log_debug "Old image version: $old_image_version"
    new_image_version=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.image.to'")
    log_debug "New image version: $new_image_version"

    # Download the agent deployment to a yaml file
    log_verbose "Dowloading agent deployment to yaml file..."
    cmd_output=$( { $KUBECTL get deployment -n ${AGENT_NAMESPACE} agent -o yaml > /tmp/deployment.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Agent deployment could not be retrieved. Error: $cmd_output"
    fi

    # Replace the agent version in the yaml file to the version the agent attempted to upgrade from
    log_verbose "Updating version from $new_image_version to $old_image_version..."
    sed -i "s/amd64_anax_k8s:$new_image_version/amd64_anax_k8s:$old_image_version/g" deployment.yaml
    
    # Delete the current agent deployment
    log_verbose "Deleting current agent deployment..."
    cmd_output=$( { $KUBECTL delete deployment -n ${AGENT_NAMESPACE} agent; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Agent deployment could not be retrieved. Error: $cmd_output"
    fi
    
    # Redeploy the agent using the yaml file
    log_verbose "Creating new agent deployment from backup yaml file..."
    cmd_output=$( { $KUBECTL apply -n ${AGENT_NAMESPACE} -f deployment.yaml; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "Agent deployment could not be retrieved. Error: $cmd_output"
    fi

    # Make sure the agent actually gets deployed
    check_deployment_status

    log_debug "rollback_agent_image() end"
}

#====================== Main  ======================

# Check agent deployment/pod status and status.json
pod_status=$($KUBECTL get pods ${POD_ID} --no-headers -o custom-columns=":status.phase")
log_debug "Pod status: $pod_status"
dep_status=$($KUBECTL rollout status deployment/agent -n ${AGENT_NAMESPACE} | awk '{ print $3 }' | sed 's/successfully/Running/g')
log_debug "Deployment status: $dep_status"
json_status=$(agent_cmd "cat /var/horizon/status.json | jq '.status' | sed 's/\"//g'")
log_debug "Cron Job status: $json_status"

# Exit early if cron job already executed to success or failure
if [[ "$json_status" == "rollback failed" ]]; then
    rollback_failed "Rollback was already initiated, but it failed. Deployment status: $dep_status, Pod status: $pod_status"
elif [[ "$json_status" == "rollback successful" ]]; then
    log_info "Rollback already completed successfully."
    exit 0
fi

log_info "Checking if agent is running and deployment is successful..."
if [[ "$pod_status" != "Running" || "$dep_status" != "Running" ]]; then
    
    # If k8s deployment status is "error" and if status.json has "rollback started" -> set status to "rollback failed" (stop here)
    log_info "Agent is not running. Checking if rollback was already attempted..."
    if [[ "$json_status" == "rollback started" ]]; then
        rollback_failed "Rollback was already initiated, but it failed. Deployment status: $dep_status"

    # If k8s deployment status is "error" and status.json is not "initiated" status -> throw error
    log_debug "Checking if agent upgrade was initiated..."
    elif [[ "$json_status" != "initiated" ]]; then
        log_info "The agent is up-to-date."
        exit 0
    fi
fi

# Rollback
log_info "Starting rollback process..."
    
# Set the status to "rollback started" in status.json
log_verbose "Setting the status to \"rollback started\"..."
agent_cmd "sed -i 's/\"status\":.*/\"status\": \"rollback started\",/g' /var/horizon/status.json"

# Configmap:
log_info "Checking config map status..."
config_change=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.configMap.needChange'")
config_updated=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.configMap.updated'")

log_debug "Config map needs change: $config_change"
log_debug "Config map was updated: $config_updated"

# If the status.json set config_updated to true, perform a rollback of the config map
if [[ "$config_updated" == "true" ]]; then
    if [[ "$config_change" == "false" ]]; then
        log_error "The config map was updated, but there was no config map change request. Attempting rollback anyway..."
    fi
    log_info "Rollback config map..."
    rollback_configmap
fi

# Secret:
log_info "Checking secrets status..."
secret_change=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.secret.needChange'")
secret_updated=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.secret.updated'")

log_debug "Secret needs change: $secret_change"
log_debug "Secret was updated: $secret_updated"

# If the status.json set secret_updated to true, perform a rollback of the secret
if [[ "$secret_updated" == "true" ]]; then
    if [[ "$secret_change" == "false" ]]; then
        log_error "The secret was updated, but there was no secret change request. Attempting rollback anyway..."
    fi
    log_info "Rollback secrets..."
    rollback_secret
fi

# Agent image rollback:
log_info "Checking agent image status..."
image_change=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.image.needChange'")
image_updated=$(agent_cmd "cat /var/horizon/status.json | jq '.k8s.image.updated'")

log_debug "Agent image needs change: $image_change"
log_debug "Agent image was updated: $image_updated"

# If the status.json set image_change to true, perform a rollback of the secret
if [[ $image_updated == "true" ]]; then
    if [[ "$image_change" == "false" ]]; then
        log_error "The agent image was updated, but there was no image change request. Attempting rollback anyway..."
    fi
    log_info "Rollback agent image..."
    rollback_agent_image

# If the pod is not running, simply restart it
elif [[ "$pod_status" != "Running" || "$dep_status" != "Running" ]]; then
    log_info "Restarting agent pod..."
    cmd_output=$( { $KUBECTL rollout restart deployment agent -n ${AGENT_NAMESPACE}; } 2>&1 )
    rc=$?
    if [[ $rc != 0 ]]; then
        rollback_failed "There was an unexpected error while restarting agent pod: $cmd_output"
    fi
fi

# If the rollback ran successfully, update status to "rollback successful". 
log_verbose "Setting the status to \"rollback successful\"..."
agent_cmd "sed -i 's/\"status\":.*/\"status\": \"rollback successful\",/g' /var/horizon/status.json"

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
