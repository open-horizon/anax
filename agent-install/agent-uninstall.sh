#!/bin/bash

# The script uninstalls Horizon agent on an edge cluster

set -e

DEPLOYMENT_NAME="agent"
SERVICE_ACCOUNT_NAME="agent-service-account"
CLUSTER_ROLE_BINDING_NAME="openhorizon-agent-cluster-rule"
NAMESPACE="openhorizon-agent"
SECRET_NAME="openhorizon-agent-secrets"
CONFIGMAP_NAME="openhorizon-agent-config"
PVC_NAME="openhorizon-agent-pvc"

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

    -v          - show version
    -l          - logging verbosity level (0-5, 5 is verbose)
    -u          - management hub user authorization credentials
    -d          - delete node from the management hub

Example: ./$(basename "$0") -u <hzn-exchange-user-auth> -d
Note: Namespace may be stuck in "Terminating" during deleting. It is a known issue on Kubernetes. Please refer to Kubernetes website to delete namespace manually.

EndOfMessage
}

function show_config() {
	log_debug "show_config() begin"

    echo "Current configuration:"
    echo "HZN_EXCHANGE_USER_AUTH: ${HZN_EXCHANGE_USER_AUTH}"
    echo "Delete node: ${DELETE_EX_NODE}"
    echo "Verbosity is ${VERBOSITY}"

    log_debug "show_config() end"
}

# checks input arguments and env variables specified
function validate_args(){
	log_debug "validate_args() begin"

    log_info "Checking script requirements..."

    # check kubectl is available
    kubectl --help > /dev/null 2>&1
    if [ $? -ne 0 ]; then
    	log_notify "kubectl is not available, please install kubectl and ensure that it is found on your \$PATH. Exiting..."
	exit 1
    fi

    if [[ -z "$HZN_EXCHANGE_USER_AUTH" ]]; then
    	echo "\$HZN_EXCHANGE_USER_AUTH: ${HZN_EXCHANGE_USER_AUTH}"
	log_notify "\$HZN_EXCHANGE_USER_AUTH is not set. Exiting..."
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
    POD_ID=$(kubectl get pod -n ${NAMESPACE} 2> /dev/null | grep "agent-" | cut -d " " -f1 2> /dev/null)
    if [ -n "${POD_ID}" ]; then
        log_info "get pod: ${POD_ID}"
    else
        log_notify "Failed to get pod id, exiting..."
        exit 1
    fi
    log_debug "get_agent_pod_id() end"
}

function unregister() {
    log_debug "unregister() begin"
    log_info "Unregister agent for pod: ${POD_ID}"

    EXPORT_EX_USER_AUTH_CMD="export HZN_EXCHANGE_USER_AUTH=${HZN_EXCHANGE_USER_AUTH}"

    if [[ "$DELETE_EX_NODE" == "true" ]]; then
        log_info "This script will delete the node from the management hub"
	HZN_UNREGISTER_CMD="hzn unregister -rf"
    else
        log_info "This script will NOT delete the node from the management hub"
        HZN_UNREGISTER_CMD="hzn unregister -f"
    fi

    NODE_ORG=$(kubectl exec -it ${POD_ID} -n ${NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn node list | jq -r .organization" | sed 's/\r//g')
    NODE_ID=$(kubectl exec -it ${POD_ID} -n ${NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn node list | jq -r .id" | sed 's/\r//g') 

    kubectl exec -it ${POD_ID} -n ${NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; ${HZN_UNREGISTER_CMD}"

    # verify the node is unregistered
    NODE_STATE=$(kubectl exec -it ${POD_ID} -n ${NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn node list | jq -r .configstate.state" | sed 's/[^a-z]*//g')
    log_debug "NODE config state is ${NODE_STATE}"

    if [[ "$NODE_STATE" != "unconfigured" ]] && [[ "$NODE_STATE" != "unconfiguring" ]]; then
        log_notify "Failed to unregister agent, exiting..."
        exit 1
    fi

    sleep 2

    if [[ "$DELETE_EX_NODE" == "true" ]]; then
        kubectl exec -it ${POD_ID} -n ${NAMESPACE} -- bash -c "${EXPORT_EX_USER_AUTH_CMD}; hzn exchange node list ${NODE_ORG}/${NODE_ID}"
        if [ $? -ne 8 ]; then
            log_notify "Node was not removed from the management hub, exiting..."
            exit 1
        fi
    fi

    log_debug "unregister() end"
}

function deleteAgentResources() {
    log_debug "deleteAgentResources() begin"

    log_info "Deleting agent deployment..."
    kubectl delete deployment $DEPLOYMENT_NAME -n $NAMESPACE
    
    log_info "Deleting configmap..."
    kubectl delete configmap $CONFIGMAP_NAME -n $NAMESPACE

    log_info "Deleting secret..."
    kubectl delete secret $SECRET_NAME -n $NAMESPACE

    log_info "Deleting persistent volume..."
    kubectl delete pvc $PVC_NAME -n $NAMESPACE

    log_info "Deleting clusterrolebinding..."
    kubectl delete clusterrolebinding $CLUSTER_ROLE_BINDING_NAME

    log_info "Deleting serviceaccount..."
    kubectl delete serviceaccount $SERVICE_ACCOUNT_NAME -n $NAMESPACE

    log_info "Deleting namespace..."
    kubectl delete namespace $NAMESPACE --force=true --grace-period=0

    log_debug "deleteAgentResources() end"
}

function uninstall_cluster() {
    show_config
    
    validate_args "$*" "$#"

    get_agent_pod_id
    
    unregister

    deleteAgentResources
}

# Accept the parameters from command line
while getopts "u:hvl:d" opt; do
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
		\?) echo "Invalid option: -$OPTARG"; help; exit 1
		;;
		:) echo "Option -$OPTARG requires an argument"; help; exit 1
		;;
	esac
done

set +e
uninstall_cluster
