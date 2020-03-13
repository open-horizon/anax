#!/bin/bash

set -x

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
$cprefix microk8s.status
RC=$?
if [ $RC -ne 0 ]; then echo "microk8s not running, nothing to clean up."; exit 0; fi

#
# Undeploy everything from the microk8s environment.
#
echo "Undeploy the agent and related constructs"
$cprefix microk8s.kubectl delete deployment agent -n ibm-edge-agent
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent deployment: $RC"; fi

$cprefix microk8s.kubectl delete namespace ibm-edge-agent
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent namespace: $RC"; fi

$cprefix microk8s.ctr image remove docker.io/library/agent-in-kube:local
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent container from container registry: $RC"; fi

#
# Stop the microk8s kube environment.
#
echo "Stopping Kube test environment"
$cprefix microk8s.stop
RC=$?
if [ $RC -ne 0 ]; then echo "Error stopping microk8s: $RC"; fi

#
# Delete the special agent in kube container image.
#
docker rmi agent-in-kube:local

set +x