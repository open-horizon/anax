#!/bin/bash

# The script uninstalls Horizon agent on an edge cluster

set -e

DEPLOYMENT_NAME="agent"
SERVICE_ACCOUNT_NAME="agent-service-account"
CLUSTER_ROLE_BINDING_NAME="openhorizon-agent-cluster-rule"
SECRET_NAME="openhorizon-agent-secrets"
CONFIGMAP_NAME="openhorizon-agent-config"
PVC_NAME="openhorizon-agent-pvc"
AGENT_NAMESPACE="openhorizon-agent"

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

VERBOSITY=3 # Default logging verbosity

function log_notify() {
    log $VERB_SILENT "$1"
}

function log_critical() {
    log $VERB_CRITICAL "CRITICAL: $1"
}

function log_error() {
    log $VERB_ERROR "ERROR: $1"
}

function log_warning() {
    log $VERB_WARNING "WARNING: $1"
}

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
        echo `now` "$2" | fold -w80 -s
    fi
}

function help() {
    cat << EndOfMessage
$(basename "$0") <options> -- uninstall agent from edge cluster
where:
    \$HZN_EXCHANGE_USER_AUTH must be defined in environment

    -v          - show version
    -l          - logging verbosity level (0-5, 5 is verbose)
    -u          - management hub user authorization credentials
    -d          - delete node from the management hub
    -m		- agent namespace to uninstall

Example: ./$(basename "$0") -u <hzn-exchange-user-auth> -d
Note: Namespace may be stuck in "Terminating" during deleting. It is a known issue on Kubernetes. Please refer to Kubernetes website to delete namespace manually.

EndOfMessage
}

function show_config() {
	log_debug "show_config() begin"

    echo "Current configuration:"
    echo "HZN_EXCHANGE_USER_AUTH: <specified>"
    echo "Delete node: ${DELETE_EX_NODE}"
    echo "Agent namespace to uninstall: ${AGENT_NAMESPACE}"
    echo "Verbosity is ${VERBOSITY}"

    log_debug "show_config() end"
}

# checks input arguments and env variables specified
function validate_args(){
	log_debug "validate_args() begin"

    log_info "Checking script requirements..."

    # check kubectl is available
    KUBECTL=${KUBECTL:-kubectl}   # the default is kubectl, or what they set in the env var
    if command -v "$KUBECTL" >/dev/null 2>&1; then
	    :   # nothing more to do
    elif command -v microk8s.kubectl >/dev/null 2>&1; then
	    KUBECTL=microk8s.kubectl
    else
	    log_notify "$KUBECTL is not available, please install $KUBECTL and ensure that it is found on your \$PATH for edge cluster agent uninstall. Uninstall agent in device is unsupported currently. Exiting..."
    fi

    # check jq is available
    log_info "Checking if jq is installed..."
    if command -v jq >/dev/null 2>&1; then
	log_info "jq found"
    else
        log_notify "jq not found, please install it. Exiting..."
        exit 1
    fi

    if [[ -z "$HZN_EXCHANGE_USER_AUTH" ]]; then
    	echo "\$HZN_EXCHANGE_USER_AUTH: ${HZN_EXCHANGE_USER_AUTH}"
	log_notify "\$HZN_EXCHANGE_USER_AUTH is not set. Exiting..."
	exit 1
    fi

    if [[ -z "$AGENT_NAMESPACE" ]]; then
        log_notify "AGENT_NAMESPACE is not specified. Please use -m to set. Exiting..."
	exit 1
    fi

    log_info "Check finished successfully"
    log_debug "validate_args() end"
}

function validate_number_int() {
	log_debug "validate_number_int() begin"

	re='^[0-9]+$'
	if [[ $1 =~ $re ]] ; then
   		# integer, validate if it's in a correct range
   		if ! (($1 >= VERB_SILENT && $1 <= VERB_DEBUG)); then
   			echo `now` "The verbosity number is not in range [${VERB_SILENT}; ${VERB_DEBUG}]."
  			quit 2
		fi
   	else
   		echo `now` "The provided verbosity value ${1} is not a number" >&2; quit 2
	fi

	log_debug "validate_number_int() end"
}

function get_agent_pod_id() {
    log_debug "get_agent_pod_id() begin"
    if [[ $($KUBECTL get pods -n ${AGENT_NAMESPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
	    AGENT_POD_READY="false"
    else
	    AGENT_POD_READY="true"
    fi

    if [ "$AGENT_POD_READY" == "true" ]; then
    	POD_ID=$($KUBECTL get pod -n ${AGENT_NAMESPACE} 2> /dev/null | grep "agent-" | cut -d " " -f1 2> /dev/null)
    	if [ -n "${POD_ID}" ]; then
        	log_info "get pod: ${POD_ID}"
    	else
        	log_notify "Failed to get pod id, exiting..."
        	exit 1
    	fi
    fi
    log_debug "get_agent_pod_id() end"
}

function removeNodeFromLocalAndManagementHub() {
    log_debug "removeNodeFromLocalAndManagementHub() begin"
    log_info "Check node status for agent pod: ${POD_ID}"

    EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"
    NODE_INFO=$($KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn node list")
    NODE_STATE=$(echo $NODE_INFO | jq -r .configstate.state | sed 's/[^a-z]*//g')
    NODE_ID=$(echo $NODE_INFO | jq -r .id | sed 's/\r//g')
    log_debug "NODE config state for ${NODE_ID} is ${NODE_STATE}"

    if [[ "$NODE_STATE" != *"unconfigured"* ]] && [[ "$NODE_STATE" != *"unconfiguring"* ]]; then
        if [[ -n $NODE_STATE ]]; then
		log_info "Process with unregister..."
		unregister $NODE_ID
		sleep 2
	else 
		log_info "node state is empty"
	fi
    else
        log_info "Node is not registered, skip unregister..."
	if [[ "$DELETE_EX_NODE" == "true" ]]; then
	    log_info "Remve node from the management hub..."
	    deleteNodeFromManagementHub $NODE_ID
	fi
    fi

    if [[ -n $NODE_STATE ]] && [[ "$DELETE_EX_NODE" == "true" ]]; then
        verifyNodeRemovedFromManagementHub $NODE_ID
    fi

    log_debug "removeNodeFromLocalAndManagementHub() end"
}

function unregister() {
    log_debug "unregister() begin"
    log_info "Unregister agent for pod: ${POD_ID}"

    EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"
    local node_id=$1

    if [[ "$DELETE_EX_NODE" == "true" ]]; then
        log_info "This script will delete the node from the management hub"
	HZN_UNREGISTER_CMD="hzn unregister -rf -t 1"
    else
        log_info "This script will NOT delete the node from the management hub"
        HZN_UNREGISTER_CMD="hzn unregister -f -t 1"
    fi

    set +e
    $KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_UNREGISTER_CMD}"
    set -e

    # verify the node is unregistered
    NODE_STATE=$($KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn node list | jq -r .configstate.state" | sed 's/[^a-z]*//g')
    log_debug "NODE config state is ${NODE_STATE}"

    if [[ "$NODE_STATE" != "unconfigured" ]] && [[ "$NODE_STATE" != "unconfiguring" ]]; then
        log_warning "Failed to unregister agent"
    fi

    log_debug "unregister() end"
}

function deleteNodeFromManagementHub() {
    log_debug "deleteNodeFromManagementHub() begin"

    EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"
    local node_id=$1

    log_info "Deleting node ${node_id} from the management hub..."

    set +e
    $KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn exchange node remove ${node_id} -f"
    set -e

    log_debug "deleteNodeFromManagementHub() end"
}

function verifyNodeRemovedFromManagementHub() {
    log_debug "verifyNodeRemovedFromManagementHub() begin"

    EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"
    local node_id=$1

    log_info "Verifying node ${node_id} is from the management hub..."

    set +e
    $KUBECTL exec -it ${POD_ID} -n ${AGENT_NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn exchange node list ${node_id}" >/dev/null 2>&1
    if [ $? -ne 8 ]; then
	    log_warning "Node was not removed from the management hub"
    fi
    set -e
    log_debug "verifyNodeRemovedFromManagementHub() end"
}

function deleteAgentResources() {
    log_debug "deleteAgentResources() begin"

    set +e
    log_info "Deleting agent deployment..."
    $KUBECTL delete deployment $DEPLOYMENT_NAME -n $AGENT_NAMESPACE --force=true --grace-period=0

    # give pods sometime to terminate by themselves
    sleep 10

    log_info "Force deleting all the pods under $AGNET_NAMESPACE if any stuck in Terminating status"
    $KUBECTL delete --all pods --namespace=$AGENT_NAMESPACE --force=true --grace-period=0
    pkill -f anax.service
    
    log_info "Deleting configmap..."
    $KUBECTL delete configmap $CONFIGMAP_NAME -n $AGENT_NAMESPACE

    log_info "Deleting secret..."
    $KUBECTL delete secret $SECRET_NAME -n $AGENT_NAMESPACE

    log_info "Deleting persistent volume..."
    $KUBECTL delete pvc $PVC_NAME -n $AGENT_NAMESPACE
    set -e

    log_info "Deleting clusterrolebinding..."
    $KUBECTL delete clusterrolebinding $CLUSTER_ROLE_BINDING_NAME

    set +e
    log_info "Deleting serviceaccount..."
    $KUBECTL delete serviceaccount $SERVICE_ACCOUNT_NAME -n $AGENT_NAMESPACE
    set -e

    log_info "Deleting namespace..."
    $KUBECTL delete namespace $AGENT_NAMESPACE --force=true --grace-period=0

    log_debug "deleteAgentResources() end"
}

function uninstall_cluster() {
    show_config
    
    validate_args

    get_agent_pod_id

    if [[ "$AGENT_POD_READY" == "true" ]]; then
    	removeNodeFromLocalAndManagementHub
    fi
    
    deleteAgentResources
}

# Accept the parameters from command line
while getopts "u:hvl:dm:" opt; do
	case $opt in
		u) HZN_EXCHANGE_USER_AUTH="$OPTARG"
		;;
		h) help; exit 0
		;;
		v) version
		;;
		l) validate_number_int "$OPTARG"; VERBOSITY="$OPTARG"
		;;
		d) DELETE_EX_NODE=true
		;;
		m) AGENT_NAMESPACE="$OPTARG"
		;;
		\?) echo "Invalid option: -$OPTARG"; help; exit 1
		;;
		:) echo "Option -$OPTARG requires an argument"; help; exit 1
		;;
	esac
done

set +e
uninstall_cluster
