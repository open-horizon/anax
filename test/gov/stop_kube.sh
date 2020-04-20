#!/bin/bash

set -x

NAME_SPACE="ibm-edge-agent"
CONFIGMAP_NAME="agent-configmap-horizon"
SECRET_NAME="agent-secret-cert"

isRoot=$(id -u)
cprefix="sudo -E"
if [ "${isRoot}" == "0" ]
then
	cprefix=""
fi

#
# Check if microk8s is running.
#
echo "Preparing to cleanup Kube test environment"
OUT=$($cprefix microk8s.status)
RC=$?
if [ $RC -ne 0 ]; then echo "microk8s not running, nothing to clean up."; exit 0; fi

if [[ $OUT == *"microk8s is not running."* ]]; then echo "microk8s not running, nothing to clean up."; exit 0; fi

#
# Undeploy everything from the microk8s environment.
#
echo "Undeploy the agent and related constructs"
$cprefix microk8s.kubectl delete deployment agent -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent deployment: $RC"; fi

$cprefix microk8s.kubectl delete configmap ${CONFIGMAP_NAME} -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting configmap ${CONFIGMAP_NAME}: $RC"; fi

$cprefix microk8s.kubectl delete secret ${SECRET_NAME} -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting secret ${SECRET_NAME}: $RC"; fi

$cprefix microk8s.kubectl delete namespace ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent namespace ${NAME_SPACE}: $RC"; fi

$cprefix microk8s.ctr -n k8s.io image remove docker.io/openhorizon/amd64_anax_k8s:testing
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent container from container registry: $RC"; fi

#
# Stop the microk8s kube environment.
#
echo "Stopping Kube test environment"
$cprefix microk8s.stop
RC=$?
if [ $RC -ne 0 ]; then echo "Error stopping microk8s: $RC"; fi

set +x
