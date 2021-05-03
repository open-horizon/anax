#!/bin/bash

if [[ "$NOKUBE" == "1" ]]; then
  echo "Skipping $0"
  exit
fi

# set -x

E2EDEVTEST_TEMPFS=$1
ANAX_SOURCE=$2
EXCH_ROOTPW=$3
DOCKER_TEST_NETWORK=$4

NAME_SPACE="openhorizon-agent"
CONFIGMAP_NAME="agent-configmap-horizon"
SECRET_NAME="agent-secret-cert"
PVC_NAME="agent-pvc-horizon"
WAIT_POD_MAX_TRY=30

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
	sudo snap install microk8s --classic --channel=1.18/stable
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
cp ${ANAX_SOURCE}/anax ${ANAX_SOURCE}/cli/hzn ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube
if [ $? -ne 0 ]; then echo "Failure copying binaries"; exit 1; fi

if [ ${CERT_LOC} -eq "1" ]; then
	cp ${E2EDEVTEST_TEMPFS}/certs/css.crt ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/hub.crt
	if [ $? -ne 0 ]; then echo "Failure copying CSS SSL cert"; exit 1; fi
fi

#
# Generate config files that are specific to the runtime environment.
#
echo "Generate the /etc/default/horizon file based on local network configuration"
EX_IP_MASK=$(docker network inspect ${DOCKER_TEST_NETWORK} | jq -r '.[].Containers | to_entries[] | select (.value.Name == "exchange-api") | .value.IPv4Address')
CSS_IP_MASK=$(docker network inspect ${DOCKER_TEST_NETWORK} | jq -r '.[].Containers | to_entries[] | select (.value.Name == "css-api") | .value.IPv4Address')
EX_IP="$(cut -d'/' -f1 <<<${EX_IP_MASK})"
CSS_IP="$(cut -d'/' -f1 <<<${CSS_IP_MASK})"

if [ "${EX_IP}" == "" ] || [ "${CSS_IP}" == "" ]
then
	echo "Failure obtaining host IP addresses for exchange and CSS"
	exit 1
fi

EX_IP=${EX_IP} CSS_IP=${CSS_IP} envsubst < "${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon.env" > "${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon"
if [ $? -ne 0 ]; then echo "Failure configuring agent env var file"; exit 1; fi

if [ ${CERT_LOC} -eq "1" ]; then
	depl_file="${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/deployment.yaml.tmpl"
else
	# remove HZN_MGMT_HUB_CERT_PATH from the horizon env file
	sed -i '/HZN_MGMT_HUB_CERT_PATH/d' ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon

	depl_file="${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/deployment_nocert.yaml.tmpl"
fi
# create deployment.yaml file
ARCH=${ARCH} envsubst < ${depl_file} > "${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/deployment.yaml"
if [ $? -ne 0 ]; then echo "Failure configuring k8s agent deployment template file"; exit 1; fi

echo "Enable kube dns"
$cprefix microk8s.enable dns
RC=$?
if [ $RC -ne 0 ]
then
        echo "Failure enabling kube dns & storage: $RC"
        exit 1
fi

echo "Enable kube storage"
$cprefix microk8s.enable storage
RC=$?
if [ $RC -ne 0 ]
then
        echo "Failure enabling kube storage: $RC"
        exit 1
fi

#
# Copy the agent container into the local kube container registry so that kube knows where to find it.
#
echo "Move agent container into microk8s container registry"
docker save openhorizon/${ARCH}_anax_k8s:testing > /tmp/agent-in-kube.tar
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
$cprefix microk8s.ctr --namespace k8s.io image import /tmp/agent-in-kube.tar
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

# Create a configmap based on ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon
echo "Create configmap to mount horizon env file"
$cprefix microk8s.kubectl create configmap ${CONFIGMAP_NAME} --from-file=${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon -n ${NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating configmap '${CONFIGMAP_NAME}' to mount horizon env file: $RC"
	$cprefix microk8s.kubectl get configmap ${CONFIGMAP_NAME} -n ${NAME_SPACE}
	exit 1
fi

# Create a secret based on ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/hub.crt
if [ ${CERT_LOC} -eq "1" ]; then
	echo "Create secret to mount cert file"
	$cprefix microk8s.kubectl create secret generic ${SECRET_NAME} --from-file=${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/hub.crt -n ${NAME_SPACE}
	RC=$?
	if [ $RC -ne 0 ]
	then
		echo "Failure creating secret '${SECRET_NAME}' to mount cert file: $RC"
		$cprefix microk8s.kubectl get secret ${SECRET_NAME} -n ${NAME_SPACE}
		exit 1
	fi
fi

# Create a persistent volume claim
echo "Create persistent volume claim to mount db file"
$cprefix microk8s.kubectl apply -f ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/persistent-claim.yaml
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating pvc '${PVC_NAME}' to mount db file: $RC"
	$cprefix microk8s.kubectl get pvc ${PVC_NAME} -n ${NAME_SPACE}
	exit 1
fi

sleep 2

echo "Deploy the agent"
# Debug help = microk8s.kubectl describe pod <pod-name> -n ${NAME_SPACE}
# Debug help = microk8s.kubectl exec <pod-name> -it -n ${NAME_SPACE} /bin/bash
$cprefix microk8s.kubectl apply -f ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/deployment.yaml
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure deploying the agent to kube: $RC"
	$cprefix microk8s.status
	$cprefix microk8s.inspect
	exit 1
fi

echo "Agent deployed to local kube"

sleep 15

echo "Wait for agent pod to run"
i=0
while :
do
	if [ $i -eq $WAIT_POD_MAX_TRY ]; then
		echo "Timeout for waiting pod to become READY"
		exit 1
	else
		if [[ $($cprefix microk8s.kubectl get pods -n ${NAME_SPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
			 echo "waiting for pod: $i"
        		((i++))
			sleep 1
		else
			break
		fi
	fi
done

echo "Configuring agent for policy"

POD=$($cprefix microk8s.kubectl get pod -l app=agent -n ${NAME_SPACE} -o jsonpath="{.items[0].metadata.name}")
if [ $POD == "" ]
then
	echo "Unable to find agent POD"
	exit 1
fi

$cprefix microk8s.kubectl cp $PWD/gov/input_files/k8s_deploy/node.policy.json ${NAME_SPACE}/${POD}:/home/agentuser/.
$cprefix microk8s.kubectl cp $PWD/gov/input_files/k8s_deploy/node_ui.json ${NAME_SPACE}/${POD}:/home/agentuser/.

$cprefix microk8s.kubectl exec ${POD} -it -n ${NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn register -f /home/agentuser/node_ui.json -p e2edev@somecomp.com/sk8s -u root/root:${EXCH_ROOTPW}

echo "Configured agent for policy, waiting for the agbot to start."
