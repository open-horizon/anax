#!/bin/bash

if [[ "$NOKUBE" == "1" ]]; then
  echo "Skipping $0"
  exit
fi

# set -x

PREFIX="Cluster scoped agent test:"
E2EDEVTEST_TEMPFS=$1
ANAX_SOURCE=$2
EXCH_ROOTPW=$3
DOCKER_TEST_NETWORK=$4

AGENT_NAME_SPACE="agent-namespace"
NAMESPACE_IN_POLICY="ns-in-policy"
SVC_EMBEDDED_NAMESPACE="operator-embedded-ns"
OPERATOR_DEPLOYMENT_NAME="topserviceoperators"
CONFIGMAP_NAME="agent-configmap-horizon"
SECRET_NAME="agent-secret-cert"
PVC_NAME="openhorizon-agent-pvc"
WAIT_POD_MAX_TRY=30

USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

isRoot=$(id -u)
cprefix="sudo -E"
sudoprefix="sudo"

if [ "${isRoot}" == "0" ]
then
	cprefix=""
	sudoprefix=""
fi

#
# Start the microk8s kube environment. If microk8s isnt installed, then install it.
#
echo "Starting Kube test environment with $cprefix microk8s.start"
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
AGBOT_IP_MASK=$(docker network inspect ${DOCKER_TEST_NETWORK} | jq -r '.[].Containers | to_entries[] | select (.value.Name == "agbot") | .value.IPv4Address')
EX_IP="$(cut -d'/' -f1 <<<${EX_IP_MASK})"
CSS_IP="$(cut -d'/' -f1 <<<${CSS_IP_MASK})"
AGBOT_IP="$(cut -d'/' -f1 <<<${AGBOT_IP_MASK})"

if [ "${EX_IP}" == "" ] || [ "${CSS_IP}" == "" ] || [ "${AGBOT_IP}" == "" ]
then
	echo "Failure obtaining host IP addresses for exchange, CSS and agbot"
	exit 1
fi

EX_IP=${EX_IP} CSS_IP=${CSS_IP} AGBOT_IP=${AGBOT_IP} envsubst < "${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon.env" > "${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon"
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
        echo "Failure enabling kube dns: $RC"
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
$cprefix microk8s.ctr --namespace k8s.io image import /tmp/agent-in-kube.tar --base-name docker.io/openhorizon/amd64_anax_k8s
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure importing agent container to microk8s container registry: $RC"
	$cprefix microk8s.ctr images ls
	exit 1
fi

$sudoprefix apparmor_parser -R /var/lib/snapd/apparmor/profiles/snap.microk8s.daemon-containerd
$sudoprefix apparmor_parser -a /var/lib/snapd/apparmor/profiles/snap.microk8s.daemon-containerd

#
# Now start deploying the agent, running in it's own namespace.
#
echo "Create namespace for the agent"
$cprefix microk8s.kubectl create namespace ${AGENT_NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating namespace for agent container: $RC"
	$cprefix microk8s.kubectl get namespaces
	exit 1
fi

# Create a configmap based on ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon
echo "Create configmap to mount horizon env file"
$cprefix microk8s.kubectl create configmap ${CONFIGMAP_NAME} --from-file=${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/horizon -n ${AGENT_NAME_SPACE}
RC=$?
if [ $RC -ne 0 ]
then
	echo "Failure creating configmap '${CONFIGMAP_NAME}' to mount horizon env file: $RC"
	$cprefix microk8s.kubectl get configmap ${CONFIGMAP_NAME} -n ${AGENT_NAME_SPACE}
	exit 1
fi

# Create a secret based on ${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/hub.crt
if [ ${CERT_LOC} -eq "1" ]; then
	echo "Create secret to mount cert file"
	$cprefix microk8s.kubectl create secret generic ${SECRET_NAME} --from-file=${E2EDEVTEST_TEMPFS}/etc/agent-in-kube/hub.crt -n ${AGENT_NAME_SPACE}
	RC=$?
	if [ $RC -ne 0 ]
	then
		echo "Failure creating secret '${SECRET_NAME}' to mount cert file: $RC"
		$cprefix microk8s.kubectl get secret ${SECRET_NAME} -n ${AGENT_NAME_SPACE}
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
	$cprefix microk8s.kubectl get pvc ${PVC_NAME} -n ${AGENT_NAME_SPACE}
	exit 1
fi

sleep 2

echo "Deploy the agent"
# Debug help = microk8s.kubectl describe pod <pod-name> -n ${AGENT_NAME_SPACE}
# Debug help = microk8s.kubectl exec <pod-name> -it -n ${AGENT_NAME_SPACE} /bin/bash
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
		if [[ $($cprefix microk8s.kubectl get pods -n ${AGENT_NAME_SPACE} -l app=agent -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; then
			 echo "waiting for pod: $i"
        		((i++))
			sleep 1
		else
			break
		fi
	fi
done

echo "Configuring agent for policy"

POD=$($cprefix microk8s.kubectl get pod -l app=agent -n ${AGENT_NAME_SPACE} -o jsonpath="{.items[0].metadata.name}")
if [ $POD == "" ]
then
	echo "Unable to find agent POD"
	exit 1
fi

$cprefix microk8s.kubectl cp $PWD/gov/deployment_policies/userdev/bp_k8s_update.json ${AGENT_NAME_SPACE}/${POD}:/home/agentuser/.
$cprefix microk8s.kubectl cp $PWD/gov/deployment_policies/userdev/bp_k8s_embedded_ns_update.json ${AGENT_NAME_SPACE}/${POD}:/home/agentuser/.

$cprefix microk8s.kubectl cp $PWD/gov/input_files/k8s_deploy/topservice-operator/node.policy.json ${AGENT_NAME_SPACE}/${POD}:/home/agentuser/node.policy.k8s.svc1.json
$cprefix microk8s.kubectl cp $PWD/gov/input_files/k8s_deploy/topservice-operator-with-embedded-ns/node.policy.json ${AGENT_NAME_SPACE}/${POD}:/home/agentuser/node.policy.k8s.embedded.svc.json

$cprefix microk8s.kubectl cp $PWD/gov/input_files/k8s_deploy/topservice-operator/node_ui.json ${AGENT_NAME_SPACE}/${POD}:/home/agentuser/node_ui_k8s_svc1.json
$cprefix microk8s.kubectl cp $PWD/gov/input_files/k8s_deploy/topservice-operator-with-embedded-ns/node_ui.json ${AGENT_NAME_SPACE}/${POD}:/home/agentuser/node_ui_k8s_embedded_svc.json


# cluster agent pattern test
# - Failed case:
#   1. pattern with cluster namespce -> unable to register (sk8s-with-cluster-ns)
# - Successful case:
#   1. pattern with empty cluster namespace, no embedded namespace -> service pod in agent namespace (e2edev@somecomp.com/sk8s)
#   2. pattern with empty cluster namespace but has embedded namespace which != agent namespace -> service pod in embedded namespace (sk8s-with-embedded-ns)
# After test, the cluster agent will register with e2edev@somecomp.com/sk8s, service pod will be deployed in "agent-namespace"

# cluster agent policy test
#   1. business policy has no "clusterNamespace", policy constraints match the node, service has embedded ns, service deploy to "operator-embedded-ns" (bp_k8s_embedded_ns)
#   2. business policy has "clusterNamespace": "ns-in-policy", policy constraints match the node, service has embedded ns, service deploy to "ns-in-policy" (update bp_k8s_embedded_ns)
#   3. business policy has no "clusterNamespace", policy constraints match the node, the service deploy to "agent-namespace" (bp_k8s)
#   4. business policy has "clusterNamespace": "ns-in-policy", policy constraints match the node. service deploy to "ns-in-policy" (update bp_k8s)
# After test, the cluster agent will register with userdev/bp_k8s, service pod will be deployed in "ns-in-policy"

AGBOT_URL="$AGBOT_IP:8080"
source gov/verify_edge_cluster.sh
kubecmd="$cprefix microk8s.kubectl"

if [ "${TEST_PATTERNS}" != "" ]; then
	# pattern case
	# pattern name: e2edev@somecomp.com/sk8s-with-cluster-ns
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn register -f /home/agentuser/node_ui_k8s_svc1.json -p e2edev@somecomp.com/sk8s-with-cluster-ns -u root/root:${EXCH_ROOTPW}
	if [ $? -eq 0 ]; then
		echo -e "${PREFIX} cluster agent should return error when register a patter that has non-empty cluster namespace"
  		exit 2
	else
		echo -e "${PREFIX} cluster agent get expected error when register sk8s-with-cluster-ns, which has non-empty cluster namespace"
	fi


	result=$($cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn node list | jq -r '.configstate.state')
	if [ "$result" != "unconfigured" ]; then
		echo -e "${PREFIX} anax-in-kube configstate.state is $result, should be in 'unconfigured' state"
  		exit 2
	else
		echo -e "${PREFIX} cluster agent is in expected 'unconfigured' state"
	fi

	# pattern name: e2edev@somecomp.com/sk8s-with-embedded-ns
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn register -f /home/agentuser/node_ui_k8s_embedded_svc.json -p e2edev@somecomp.com/sk8s-with-embedded-ns -u root/root:${EXCH_ROOTPW}
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to register pattern e2edev@somecomp.com/sk8s-with-embedded-ns"
  		exit 2
	else
		echo -e "${PREFIX} cluster agent registered pattern e2edev@somecomp.com/sk8s-with-embedded-ns, verifying agreement..."
	fi

	# wait 30s for agreement to comeup
	sleep 30
	checkAndWaitForActiveAgreementForPattern "e2edev@somecomp.com/sk8s-with-embedded-ns" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check agreement for e2edev@somecomp.com/sk8s-with-embedded-ns"
  		exit 2
	fi

	checkDeploymentInNamespace "$kubecmd" $OPERATOR_DEPLOYMENT_NAME $SVC_EMBEDDED_NAMESPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check deployment for e2edev@somecomp.com/sk8s-with-embedded-ns"
  		exit 2
	fi

	echo -e "${PREFIX} cluster agent successfully registered with pattern e2edev@somecomp.com/sk8s-with-embedded-ns, unregistering... "
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn unregister -f

	# pattern name: e2edev@somecomp.com/sk8s
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn register -f /home/agentuser/node_ui_k8s_svc1.json -p e2edev@somecomp.com/sk8s -u root/root:${EXCH_ROOTPW}
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to register pattern e2edev@somecomp.com/sk8s"
  		exit 2
	else
		echo -e "${PREFIX} cluster agent registered pattern e2edev@somecomp.com/sk8s, verifying agreement..."
	fi

	sleep 30
	checkAndWaitForActiveAgreementForPattern "e2edev@somecomp.com/sk8s" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check agreement for e2edev@somecomp.com/sk8s"
  		exit 2
	fi
	checkDeploymentInNamespace "$kubecmd" $OPERATOR_DEPLOYMENT_NAME $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check deployment for e2edev@somecomp.com/sk8s"
  		exit 2
	fi

	echo -e "${PREFIX} cluster agent successfully registered with pattern e2edev@somecomp.com/sk8s"
else
	# policy case
	# policy: userdev/bp_k8s_embedded_ns
	echo -e "${PREFIX} cluster agent registers with deployment policy userdev/bp_k8s_embedded_ns"
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn register -f /home/agentuser/node_ui_k8s_embedded_svc.json --policy /home/agentuser/node.policy.k8s.embedded.svc.json -u root/root:${EXCH_ROOTPW}
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to register with deployment policy userdev/bp_k8s_embedded_ns"
  		exit 2
	else
		echo -e "${PREFIX} cluster agent registered with deployment policy userdev/bp_k8s_embedded_ns, verifying agreement..."
	fi

	sleep 30
	echo -e "kubecmd is: $kubecmd" #sudo -E microk8s.kubectl
	checkAndWaitForActiveAgreementForPolicy "userdev/bp_k8s_embedded_ns" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check agreement for userdev/bp_k8s_embedded_ns"
  		exit 2
	fi

	checkDeploymentInNamespace "$kubecmd" $OPERATOR_DEPLOYMENT_NAME $SVC_EMBEDDED_NAMESPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check deployment for userdev/bp_k8s_embedded_ns"
  		exit 2
	fi

	# update policy userdev/bp_k8s_embedded_ns
	echo -e "Updating deployment policy userdev/bp_k8s_embedded_ns to set \"clusterNamespace\": \"$NAMESPACE_IN_POLICY\""
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- /usr/bin/hzn exchange business updatepolicy -f bp_k8s_embedded_ns_update.json bp_k8s_embedded_ns -u $USERDEV_ADMIN_AUTH
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to update deployment policy userdev/bp_k8s_embedded_ns"
  		exit 2
	fi

	echo -e "${PREFIX} sleep 30s to allow cluster agent agreement to be cancelled and re-negotiated"
	sleep 30
	echo -e "${PREFIX} verify agreement is archived for deployment policy userdev/bp_k8s_embedded_ns"
	checkArchivedAgreementForPolicy "userdev/bp_k8s_embedded_ns" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check archived agreement for userdev/bp_k8s_embedded_ns"
  		exit 2
	fi

	echo -e "${PREFIX} verify new agreement is active for deployment policy userdev/bp_k8s_embedded_ns"
	checkAndWaitForActiveAgreementForPolicy "userdev/bp_k8s_embedded_ns" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check agreement for userdev/bp_k8s_embedded_ns"
  		exit 2
	fi

	echo -e "${PREFIX} verify service for deployment policy userdev/bp_k8s_embedded_ns are created under namespace \"$NAMESPACE_IN_POLICY\""
	checkDeploymentInNamespace "$kubecmd" $OPERATOR_DEPLOYMENT_NAME $NAMESPACE_IN_POLICY
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check deployment for userdev/bp_k8s_embedded_ns"
  		exit 2
	fi

	echo -e "${PREFIX} cluster agent successfully registered with deployment policy userdev/bp_k8s_embedded_ns, unregistering... "
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn unregister -f
	
	# policy name: userdev/bp_k8s
	echo -e "${PREFIX} cluster agent registers with deployment policy userdev/bp_k8s"
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- env ARCH=${ARCH} /usr/bin/hzn register -f /home/agentuser/node_ui_k8s_svc1.json --policy /home/agentuser/node.policy.k8s.svc1.json -u root/root:${EXCH_ROOTPW}
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to register with deployment policy userdev/bp_k8s"
  		exit 2
	else
		echo -e "${PREFIX} cluster agent registered with deployment policy userdev/bp_k8s, verifying agreement..."
	fi

	sleep 30
	checkAndWaitForActiveAgreementForPolicy "userdev/bp_k8s" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check agreement for userdev/bp_k8s"
  		exit 2
	fi

	checkDeploymentInNamespace "$kubecmd" $OPERATOR_DEPLOYMENT_NAME $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check deployment for userdev/bp_k8s"
  		exit 2
	fi

	# update policy userdev/bp_k8s
	echo -e "Updating deployment policy userdev/bp_k8s to set \"clusterNamespace\": \"$NAMESPACE_IN_POLICY\""
	$cprefix microk8s.kubectl exec ${POD} -it -n ${AGENT_NAME_SPACE} -- /usr/bin/hzn exchange business updatepolicy -f bp_k8s_update.json bp_k8s -u $USERDEV_ADMIN_AUTH
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to update deployment policy userdev/bp_k8s"
  		exit 2
	fi

	echo -e "${PREFIX} sleep 30s to allow cluster agent agreement to be cancelled and re-negotiated"
	sleep 30
	echo -e "${PREFIX} verify agreement is archived for deployment policy userdev/bp_k8s"
	checkArchivedAgreementForPolicy "userdev/bp_k8s" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check archived agreement for userdev/bp_k8s"
  		exit 2
	fi

	echo -e "${PREFIX} verify new agreement is active for deployment policy userdev/bp_k8s"
	checkAndWaitForActiveAgreementForPolicy "userdev/bp_k8s" $AGBOT_URL "$kubecmd" $POD $AGENT_NAME_SPACE
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check agreement for userdev/bp_k8s"
  		exit 2
	fi

	echo -e "${PREFIX} verify service for deployment policy userdev/bp_k8s are created under namespace \"$NAMESPACE_IN_POLICY\""
	checkDeploymentInNamespace "$kubecmd" $OPERATOR_DEPLOYMENT_NAME $NAMESPACE_IN_POLICY
	if [ $? -ne 0 ]; then
		echo -e "${PREFIX} cluster agent failed to check deployment for userdev/bp_k8s"
  		exit 2
	fi

	echo -e "${PREFIX} cluster agent successfully registered with deployment policy userdev/bp_k8s, service deployed under namespace \"$NAMESPACE_IN_POLICY\""

fi

echo -e "${PREFIX} complete cluster agent test"
