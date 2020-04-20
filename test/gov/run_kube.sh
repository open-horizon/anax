#!/bin/bash

set -x

AGBOT_TEMPFS=$1
ANAX_SOURCE=$2

NAME_SPACE="openhorizon-agent"
CONFIGMAP_NAME="agent-configmap-horizon"
SECRET_NAME="agent-secret-cert"

isRoot=$(id -u)
cprefix="sudo -E"

if [ "${isRoot}" == "0" ]
then
	cprefix=""
fi

#
# Start the microk8s kube environment. If microk8s isnt installed, then install it.
#
echo "Starting Kube test environment"
$cprefix microk8s.start
RC=$?
sleep 2
if [ $RC -ne 0 ]
then
	echo "Try to install microk8s"
	sudo snap install microk8s --classic --channel=1.14/stable
	IRC=$?
	if [ $IRC -ne 0 ]; then echo "Unable to install microk8s: $IRC"; exit 1; fi

fi

#
# Wait for ready status
#
echo "Waiting for Kube test environment to start"
$cprefix microk8s.status --wait-ready
RC=$?
if [ $RC -ne 0 ]
then
	echo "Error waiting for microk8s to initialize: $RC"
	$cprefix microk8s.status
	$cprefix microk8s.inspect
	exit 1
fi

# Artificial delay that seems to allow time for microk8s to start.
sleep 5

#
# Make sure the necessary services are available in kube.
#
# echo "Enable microk8s services"
# $cprefix microk8s.enable registry
# RC=$?
# if [ $RC -ne 0 ]
# then
# 	echo "Error enabling internal registry: $RC"
# 	$cprefix microk8s.status
# 	exit 1
# fi

#
# Copy binaries and other files that are needed inside the agent container.
#
echo "Grab binaries and config files needed inside the container"
cp ${ANAX_SOURCE}/anax ${ANAX_SOURCE}/cli/hzn ${AGBOT_TEMPFS}/etc/agent-in-kube
if [ $? -ne 0 ]; then echo "Failure copying binaries"; exit 1; fi

cp ${AGBOT_TEMPFS}/certs/css.crt ${AGBOT_TEMPFS}/etc/agent-in-kube/hub.crt
if [ $? -ne 0 ]; then echo "Failure copying CSS SSL cert"; exit 1; fi

#
# Generate config files that are specific to the runtime environment.
#
echo "Generate the /etc/default/horizon file based on local network configuration"
EX_IP_MASK=$(docker network inspect e2edev_test_network | jq -r '.[].Containers | to_entries[] | select (.value.Name == "exchange-api") | .value.IPv4Address')
CSS_IP_MASK=$(docker network inspect e2edev_test_network | jq -r '.[].Containers | to_entries[] | select (.value.Name == "css-api") | .value.IPv4Address')
EX_IP="$(cut -d'/' -f1 <<<${EX_IP_MASK})"
CSS_IP="$(cut -d'/' -f1 <<<${CSS_IP_MASK})"

if [ "${EX_IP}" == "" ] || [ "${CSS_IP}" == "" ]
then
	echo "Failure obtaining host IP addresses for exchange and CSS"
	exit 1
fi

EX_IP=${EX_IP} CSS_IP=${CSS_IP} envsubst < "${AGBOT_TEMPFS}/etc/agent-in-kube/horizon.env" > "${AGBOT_TEMPFS}/etc/agent-in-kube/horizon"
if [ $? -ne 0 ]; then echo "Failure configuring agent env var file"; exit 1; fi

echo "Enable kube dns"
$cprefix microk8s.enable dns
RC=$?
if [ $RC -ne 0 ]
then
        echo "Failure enabling kube dns: $RC"
        exit 1
fi

#
# Copy the agent container into the local kube container registry so that kube knows where to find it.
#
echo "Move agent container into microk8s container registry"
docker save openhorizon/amd64_anax_k8s:testing > /tmp/agent-in-kube.tar
if [ $? -ne 0 ]; then echo "Failure tar-ing agent container to file"; exit 1; fi

#
# Wait for containerd to start
#
echo "Waiting for containerd to start..."
while :
do
	if $cprefix [ -e "/var/snap/microk8s/common/run/containerd.sock" ]
	then
		break
	else
		echo "still waiting for /var/snap/microk8s/common/run/containerd.sock"
		sleep 5
	fi
done

# Debug help - microk8s.ctr images ls
$cprefix microk8s.ctr -n k8s.io image import /tmp/agent-in-kube.tar
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure importing agent container to microk8s container registry: $RC"
	$cprefix microk8s.ctr images ls
	exit 1
fi

#
# Now start deploying the agent, running in it's own namespace.
#
echo "Create namespace for the agent"
$cprefix microk8s.kubectl create namespace ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating namespace for agent container: $RC"
	$cprefix microk8s.kubectl get namespaces
	exit 1
fi

# Create a configmap based on ${AGBOT_TEMPFS}/etc/agent-in-kube/horizon
echo "Create configmap to mount horizon env file"
$cprefix microk8s.kubectl create configmap ${CONFIGMAP_NAME} --from-file=${AGBOT_TEMPFS}/etc/agent-in-kube/horizon -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating configmap '${CONFIGMAP_NAME}' to mount horizon env file: $RC"
	$cprefix microk8s.kubectl get configmap ${CONFIGMAP_NAME} -n ${NAME_SPACE}
	exit 1
fi


# Create a secret based on ${AGBOT_TEMPFS}/etc/agent-in-kube/hub.crt
echo "Create secret to mount cert file"
$cprefix microk8s.kubectl create secret generic ${SECRET_NAME} --from-file=${AGBOT_TEMPFS}/etc/agent-in-kube/hub.crt -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating secret '${SECRET_NAME}' to mount cert file: $RC"
	$cprefix microk8s.kubectl get secret ${SECRET_NAME} -n ${NAME_SPACE}
	exit 1
fi

sleep 2

echo "Deploy the agent"
# Debug help = microk8s.kubectl describe pod <pod-name> -n ${NAME_SPACE}
# Debug help = microk8s.kubectl exec <pod-name> -it -n ${NAME_SPACE} /bin/bash
$cprefix microk8s.kubectl apply -f ${AGBOT_TEMPFS}/etc/agent-in-kube/deployment.yaml
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure deploying the agent to kube: $RC"
	$cprefix microk8s.status
	$cprefix microk8s.inspect
	exit 1
fi

echo "Agent deployed to local kube"
