#!/bin/bash

# set -x

NAME_SPACE="agent-namespace"
CONFIGMAP_NAME="agent-configmap-horizon"
SECRET_NAME="agent-secret-cert"
PVC_NAME="openhorizon-agent-pvc"

isRoot=$(id -u)
cprefix="sudo -E"
if [ "${isRoot}" == "0" ]
then
	cprefix=""
fi

#
# Check if microk8s is running.
#

$sudoprefix apparmor_parser -R /var/lib/snapd/apparmor/profiles/snap.microk8s.daemon-containerd
$sudoprefix apparmor_parser -a /var/lib/snapd/apparmor/profiles/snap.microk8s.daemon-containerd

echo "Preparing to cleanup Kube test environment"
OUT=$($cprefix microk8s.status)
RC=$?
if [ $RC -ne 0 ]; then echo "microk8s not running, nothing to clean up."; exit 0; fi

if [[ $OUT == *"microk8s is not running."* ]]; then echo "microk8s not running, nothing to clean up."; exit 0; fi

#
# Stop the agent in k8s gracefully
#
echo "Stopping agent in k8s"

POD=$(microk8s.kubectl get pod -l app=agent -n ${NAME_SPACE} -o jsonpath="{.items[0].metadata.name}")
if [[ ${POD} == "" ]]
then
       echo "Unable to find agent POD"
else
       microk8s.kubectl exec ${POD} -it -n ${NAME_SPACE} -- /usr/bin/hzn unregister -fr

       echo "Stopped agent in k8s."
fi

# echo "Undeploy the agent and related constructs"
# $cprefix microk8s.kubectl delete deployment agent -n ${NAME_SPACE}
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

$cprefix microk8s.kubectl delete pvc ${PVC_NAME} -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting pvc ${PVC_NAME}: $RC"; fi

$cprefix microk8s.kubectl delete namespace ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]; then echo "Error deleting agent namespace ${NAME_SPACE}: $RC"; fi

$cprefix microk8s.ctr --namespace k8s.io image remove docker.io/openhorizon/${ARCH}_anax_k8s:testing
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
