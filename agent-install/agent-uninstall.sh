#!/bin/bash

# The script uninstalls Horizon agent on an edge cluster

set -e

DEPLOYMENT_NAME="agent"
SERVICE_ACCOUNT_NAME="agent-service-account"
CLUSTER_ROLE_BINDING_NAME="openhorizon-agent-cluster-rule"
SECRET_NAME="openhorizon-agent-secrets"
IMAGE_REGISTRY_SECRET_NAME="openhorizon-agent-secrets-docker-cert"
CONFIGMAP_NAME="openhorizon-agent-config"
PVC_NAME="openhorizon-agent-pvc"
CRONJOB_AUTO_UPGRADE_NAME="auto-upgrade-cronjob"
DEFAULT_AGENT_NAMESPACE="openhorizon-agent"
USE_DELETE_FORCE=false
DELETE_TIMEOUT=10 # Default delete timeout

function now() {
	echo `date '+%Y-%m-%d %H:%M:%S'`
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

# Logging
VERB_SILENT=0
#VERB_CRITICAL=1   we always show fatal errors
VERB_ERROR=1
VERB_WARNING=2
VERB_INFO=3
VERB_VERBOSE=4
VERB_DEBUG=5

AGENT_VERBOSITY=3 # Default logging verbosity

function log_fatal() {
    : ${1:?} ${2:?}
    local exitCode=$1
    local msg="$2"
    echo $(now) "ERROR: $msg" >&2   # we always show fatal error msgs
    exit $exitCode
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

function log_verbose() {
    log $VERB_VERBOSE "VERBOSE: $1"
}

function log_debug() {
    log $VERB_DEBUG "DEBUG: $1"
}

function now() {
	echo `date '+%Y-%m-%d %H:%M:%S'`
}

function log() {
    if [ $AGENT_VERBOSITY -ge $1 ]; then
        echo `now` "$2"
    fi
}

function help() {
    cat << EndOfMessage
$(basename "$0") <options>

Uninstall the Horizon agent from an edge cluster.

Options/Flags:
    -v    show version
    -l    Logging verbosity level. Display messages at this level and lower: 1: error, 2: warning, 3: info (default), 4: verbose, 5: debug. Default is 3, info. (equivalent to AGENT_VERBOSITY)
    -u    management hub user authorization credentials (or can set HZN_EXCHANGE_USER_AUTH environment variable)
    -d    delete node from the management hub
    -m    agent namespace to uninstall
    -f    force delete cluster resources
    -t    cluster resource delete timeout (specified timeout should > 0)

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
    echo "Force delete cluster resources: ${USE_DELETE_FORCE}"
    echo "Cluster resource delete timeout: ${DELETE_TIMEOUT}"
    echo "Verbosity is ${AGENT_VERBOSITY}"

    log_debug "show_config() end"
}

# checks input arguments and env variables specified
function validate_args(){
    log_debug "validate_args() begin"

    log_info "Checking script requirements..."

    # check kubectl is available
    if [ "${KUBECTL}" != "" ]; then   # If user set KUBECTL env variable, check that it exists
        if command -v $KUBECTL > /dev/null 2>&1; then
            : # nothing more to do
        else
            log_fatal 2 "$KUBECTL is not available, please install $KUBECTL and ensure that it is found on your \$PATH for edge cluster agent uninstall. Uninstall agent in device is unsupported currently. Exiting..."
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
            log_fatal 2 "kubectl is not available, please install kubectl and ensure that it is found on your \$PATH for edge cluster agent uninstall. Uninstall agent in device is unsupported currently. Exiting..."
        fi
    fi

    # check jq is available
    log_info "Checking if jq is installed..."
    if command -v jq >/dev/null 2>&1; then
        log_info "jq found"
    else
        log_info "jq not found, please install it. Exiting..."
        exit 1
    fi

    if [[ -z "$HZN_EXCHANGE_USER_AUTH" ]]; then
        echo "\$HZN_EXCHANGE_USER_AUTH: ${HZN_EXCHANGE_USER_AUTH}"
        log_info "\$HZN_EXCHANGE_USER_AUTH is not set. Exiting..."
        exit 1
    fi

    if [[ -z "$AGENT_NAMESPACE" ]]; then
        AGENT_NAMESPACE=$DEFAULT_AGENT_NAMESPACE
        echo "\$AGENT_NAMESPACE: ${AGENT_NAMESPACE}"
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

function validate_positive_int() {
    log_debug "validate_positive_int() begin"
    re='^[0-9]+$'
    if [[ $1 =~ $re ]] && (( $1 > 0 )); then
        :
    else
        echo `now` "${1} is not a positive int" >&2; quit 2
    fi

    log_debug "validate_positive_int() end"
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
        	log_info "Failed to get pod id, exiting..."
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
            log_info "Remove node from the management hub..."
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
    
    if [ "$USE_DELETE_FORCE" != true ]; then
        $KUBECTL delete deployment $DEPLOYMENT_NAME -n $AGENT_NAMESPACE --grace-period=$DELETE_TIMEOUT

        $KUBECTL get deployment $DEPLOYMENT_NAME -n $AGENT_NAMESPACE 2>/dev/null
        if [ $? -eq 0 ]; then
            log_info "deployment $DEPLOYMENT_NAME still exist"
            DEPLOYMENT_STILL_EXIST="true"
        fi
    fi

    if [ "$USE_DELETE_FORCE" == true ] || [ "$DEPLOYMENT_STILL_EXIST" == true ]; then
        log_info "Force deleting agent deployment"
        $KUBECTL delete deployment $DEPLOYMENT_NAME -n $AGENT_NAMESPACE --force=true --grace-period=0
    fi

    # give pods sometime to terminate by themselves
    sleep 10

    log_info "Checking if pods are deleted"
    PODS=$($KUBECTL get pod -n $AGENT_NAMESPACE 2>/dev/null)
    if [[ -n "$PODS" ]]; then
        log_info "Pods are not deleted by deleting deployment, delete pods now"
        if [ "$USE_DELETE_FORCE" != true ]; then
            $KUBECTL delete --all pods --namespace=$AGENT_NAMESPACE --grace-period=$DELETE_TIMEOUT

            PODS=$($KUBECTL get pod -n $AGENT_NAMESPACE 2>/dev/null) 
            if [[ -n "$PODS" ]]; then
                log_info "Pods still exist"
                PODS_STILL_EXIST="true"
            fi
        fi

        if [ "$USE_DELETE_FORCE" == true ] || [ "$PODS_STILL_EXIST" == true ]; then
            log_info "Force deleting all the pods under $AGENT_NAMESPACE"
            $KUBECTL delete --all pods --namespace=$AGENT_NAMESPACE --force=true --grace-period=0
            pkill -f anax.service
        fi
    fi

    log_info "Deleting configmap..."
    $KUBECTL delete configmap $CONFIGMAP_NAME -n $AGENT_NAMESPACE
    $KUBECTL delete configmap ${CONFIGMAP_NAME}-backup -n $AGENT_NAMESPACE

    log_info "Deleting secret..."
    $KUBECTL delete secret $SECRET_NAME -n $AGENT_NAMESPACE
    $KUBECTL delete secret $IMAGE_REGISTRY_SECRET_NAME -n $AGENT_NAMESPACE
    $KUBECTL delete secret ${SECRET_NAME}-backup -n $AGENT_NAMESPACE
    set -e

    log_info "Deleting auto-upgrade cronjob..."
    if $KUBECTL get cronjob ${CRONJOB_AUTO_UPGRADE_NAME} -n ${AGENT_NAMESPACE} 2>/dev/null; then
        $KUBECTL delete cronjob $CRONJOB_AUTO_UPGRADE_NAME -n $AGENT_NAMESPACE
    else
        log_info "cronjob ${CRONJOB_AUTO_UPGRADE_NAME} does not exist, skip deleting cronjob"
    fi

    set +e
    log_info "Deleting clusterrolebinding..."
    $KUBECTL delete clusterrolebinding $CLUSTER_ROLE_BINDING_NAME

    log_info "Deleting persistent volume..."
    $KUBECTL delete pvc $PVC_NAME -n $AGENT_NAMESPACE

    log_info "Deleting serviceaccount..."
    $KUBECTL delete serviceaccount $SERVICE_ACCOUNT_NAME -n $AGENT_NAMESPACE

    log_info "Deleting namespace..."
    $KUBECTL delete namespace $AGENT_NAMESPACE --force=true --grace-period=0

    log_info "Deleting cert file from /etc/default/cert ..."
    rm /etc/default/cert/agent-install.crt
    set -e

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
while getopts "u:hvl:dm:ft:" opt; do
	case $opt in
		u) HZN_EXCHANGE_USER_AUTH="$OPTARG"
		;;
		h) help; exit 0
		;;
		v) version
		;;
		l) validate_number_int "$OPTARG"; AGENT_VERBOSITY="$OPTARG"
		;;
		d) DELETE_EX_NODE=true
		;;
		m) AGENT_NAMESPACE="$OPTARG"
		;;
		f) USE_DELETE_FORCE=true
                ;;
		t) validate_positive_int "$OPTARG"; DELETE_TIMEOUT="$OPTARG"
        	;;
		\?) echo "Invalid option: -$OPTARG"; help; exit 1
		;;
		:) echo "Option -$OPTARG requires an argument"; help; exit 1
		;;
	esac
done

set +e
uninstall_cluster
