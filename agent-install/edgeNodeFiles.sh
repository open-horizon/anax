#!/bin/bash

# This script gathers the necessary information and files to install the Horizon agent and register an edge node

# default agent image tag if it is not specified by script user
AGENT_IMAGE_TAG="4.1.0"
IMAGE_TAR_FILE="amd64_anax_k8s_ubi.tar"
CLUSTER_STORAGE_CLASS="gp2"
PACKAGE_NAME="ibm-eam-4.1.0-x86_64"
AGENT_NAMESPACE="openhorizon-agent"

function scriptUsage () {
	cat << EOF

Usage: ./edgeNodeFiles.sh <edge-node-type> [-k] [-f <directory>] [-t] [-p <package_name>] [-d <distribution>] [-s <edge-cluster-storage-class>] [-i <agent-image-tag>] [-o <hzn-org-id>] [-m <agent-namespace>]

Parameters:
  Required:
    <edge-node-type>    The type of edge node planned for agent install and registration. Valid values: 32-bit-ARM, 64-bit-ARM, x86_64-Linux, macOS, x86_64-Cluster

  Optional:
    -k          Include this flag to create a new $CLUSTER_USER-Edge-Node-API-Key, even if one already exists.
    -f <directory>     The directory to put the gathered files in. Default is current directory.
    -t          Create agentInstallFiles-<edge-node-type>.tar.gz file containing gathered files. If this flag is not set, the gathered files will be placed in the current directory.
    -p <package_name>   The product media bundle name. Default is $PACKAGE_NAME, which means it will look for $PACKAGE_NAME.tar.gz and expects a standardized directory structure of $PACKAGE_NAME/horizon-edge-packages/<PLATFORM>/<OS>/<DISTRO>/<ARCH>
    -d <distribution>	By default 'bionic' and 'buster' packages are used on linux. Use this flag to use 'xenial' or 'stretch' packages. Flag is ignored with macOS and x86_64-Cluster.
    -s <edge-cluster-storage-class>   Storage class used on the edge cluster. Default is gp2. Only applies for node type x86_64-Cluster.
    -i <agent-image-tag>   Docker tag (version) of agent image to deploy to edge cluster. Only applies for node type x86_64-Cluster.
    -o <hzn-org-id>     The exchange org id that should be used on the edge node. Currently on used for node type x86_64-Cluster.
    -m <agent-namespace>   The edge cluster namespace that the agent will be installed into. Default is $AGENT_NAMESPACE. Only applies for node type x86_64-Cluster.

Required Environment Variables:
    CLUSTER_URL	- for example: https://<cluster_CA_domain>:<port-number>
    CLUSTER_USER - Your cluster admin user
    CLUSTER_PW - Your cluster admin password
EOF
	exit 1
}
if [[ "$#" = "0" ]]; then
	scriptUsage
fi

echo "Checking script parameters..."
while (( "$#" )); do
  	case "$1" in
    	-d) # distribution specified
      		if ! ([[ "$2" == "xenial" ]] \
					|| [[ "$2" == "bionic" ]] \
					|| [[ "$2" == "stretch" ]] \
					|| [[ "$2" == "buster" ]]); then
      			echo "ERROR: Unknown linux distribution type."
      			exit 1
      		fi
      		DISTRO=$2
      		shift 2
      		;;
    	-t) # create tar file
      		PACKAGE_FILES=$1
      		shift
     		;;
     	-k) # create api key
      		CREATE_API_KEY=$1
      		shift
     		;;
	#-r) # use edge cluster registry
		#USING_EDGE_CLUSTER_REGISTRY=$1
		#shift
		#;;
	-s) # storage class to use by persistent volume claim in edge cluster
		CLUSTER_STORAGE_CLASS=$2
		shift 2
		;;
	-i) # tag of agent image to deploy to edge cluster
		AGENT_IMAGE_TAG=$2
		shift 2
		;;
	-o) # value of HZN_ORG_ID
		ORG_ID=$2
		shift 2
		;;
	#-n) # value of NODE_ID
		#HZN_NODE_ID=$2
		#shift 2
		#;;
	-m) # edge cluster namespace to install agent to
		AGENT_NAMESPACE=$2
		shift 2
		;;
     	-f) # directory to move gathered files to
		DIR=$2
      		shift 2
      		;;
     	-p) # installation media name string
		PACKAGE_NAME=$2
      		shift 2
      		;;
	-*) #invalid flag
		echo "ERROR: Unknow flag $1"
		scriptUsage
		exit 1
		;;
    	*) # based on "Usage" this should be node type
		if ! ([[ "$1" == "32-bit-ARM" ]] || [[ "$1" == "64-bit-ARM" ]] || [[ "$1" == "x86_64-Linux" ]] || [[ "$1" == "macOS" ]] || [[ "$1" == "x86_64-Cluster" ]]); then
			echo "ERROR: Unknown node type."
			exit 1
		fi
      		EDGE_NODE_TYPE=$1
      		shift
      		;;
  	esac
done
if [ -z $EDGE_NODE_TYPE ]; then
	scriptUsage
fi
echo " - valid parameters"
echo ""


function checkEnvVars () {
	echo "Checking system requirements..."
	cloudctl --help > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		echo "ERROR: cloudctl is not installed."
        	exit 1
    	fi
    	echo " - cloudctl installed"

    	kubectl --help > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		echo "ERROR: kubectl is not installed."
        	exit 1
   	fi
    	echo " - kubectl installed"

	if [[ "$EDGE_NODE_TYPE" == "x86_64-Cluster" ]]; then
    		oc --help > /dev/null 2>&1
		if [ $? -ne 0 ]; then
			echo "ERROR: oc is not installed."
        		exit 1
    		fi
    		echo " - oc installed"

		docker --help > /dev/null 2>&1
		if [ $? -ne 0 ]; then
			echo "ERROR: docker is not installed."
			exit 1
		fi
	fi
    	echo ""

	echo "Checking environment variables..."

	if [ -z $CLUSTER_URL ]; then
		echo "ERROR: CLUSTER_URL environment variable is not set. Can not run 'cloudctl login ...'"
		echo " - CLUSTER_URL=https://<cluster_CA_domain>:<port-number>"
		exit 1

	elif [ -z $CLUSTER_USER ]; then
		echo "ERROR: CLUSTER_USER environment variable is not set. Can not run 'cloudctl login ...'"
		echo " - CLUSTER_USER=<your-cluster-admin-user>"
		exit 1

	elif [ -z $CLUSTER_PW ]; then
		echo "ERROR: CLUSTER_PW environment variable is not set. Can not run 'cloudctl login ...'"
		echo " - CLUSTER_PW=<your-cluster-admin-password>"
		exit 1
	fi
	echo " - CLUSTER_URL set"
	echo " - CLUSTER_USER set"
	echo " - CLUSTER_PW set"

	echo ""
}

function checkParams() {
	: #echo "Checking input paramters ..."
	#if [ -z $HZN_NODE_ID ]; then
	#	echo "ERROR: NODE_ID is not set. Please specify -n <your edge cluster name>"
	#	exit 1
	#fi
	#echo "Using NODE_ID: $HZN_NODE_ID"
	#echo ""
}

function cloudLogin () {
	echo "Connecting to cluster and configure kubectl..."
	echo "cloudctl login -a $CLUSTER_URL -u $CLUSTER_USER -p ******** -n kube-public --skip-ssl-validation"

	cloudctl login -a $CLUSTER_URL -u $CLUSTER_USER -p $CLUSTER_PW -n kube-public --skip-ssl-validation
	if [ $? -ne 0 ]; then
		echo "ERROR: 'cloudctl login' failed. Check if CLUSTER_URL, CLUSTER_USER, and CLUSTER_PW environment variables are set correctly."
        exit 2
    fi
    echo ""
}

# Query the IBM Cloud Pak cluster name
function getClusterName () {
	echo "Getting cluster name..."
	echo "kubectl get configmap -n kube-public ibmcloud-cluster-info -o jsonpath=\"{.data.cluster_name}\""

	CLUSTER_NAME=$(kubectl get configmap -n kube-public ibmcloud-cluster-info -o jsonpath="{.data.cluster_name}")
	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to get cluster name."
        exit 2
    fi

	echo " - Cluster name: $CLUSTER_NAME"
	echo ""
}

# Check if an IBM Cloud Pak platform API key exists
function checkAPIKey () {
	echo "Checking if \"$CLUSTER_USER-Edge-Node-API-Key\" already exists..."
	echo "cloudctl iam api-keys | cut -d' ' -f4 | grep \"$CLUSTER_USER-Edge-Node-API-Key\""

	KEY=$(cloudctl iam api-keys | cut -d' ' -f4 | grep "$CLUSTER_USER-Edge-Node-API-Key")
	if [ -z $KEY ]; then
		echo "\"$CLUSTER_USER-Edge-Node-API-Key\" does not exist. A new one will be created."
        CREATE_NEW_KEY=true
    else
    	echo "\"$CLUSTER_USER-Edge-Node-API-Key\" already exists. Skipping key creation."
    	CREATE_NEW_KEY=false
    fi
    echo ""
}

# Create a IBM Cloud Pak platform API key
function createAPIKey () {
	echo "Creating IBM Cloud Pak platform API key..."
	echo "cloudctl iam api-key-create \"$CLUSTER_USER-Edge-Node-API-Key\" -d \"$CLUSTER_USER-Edge-Node-API-Key\" -f key.txt"

	cloudctl iam api-key-create "$CLUSTER_USER-Edge-Node-API-Key" -d "$CLUSTER_USER-Edge-Node-API-Key" -f key.txt
	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to create API Key."
        exit 2
    fi

    API_KEY=$(cat key.txt | jq -r '.apikey')
    echo " - $CLUSTER_USER-Edge-Node-API-Key: $API_KEY"
    echo ""
}

function getImageFromOcpRegistry() {
    # get OCP_USER, OCP_TOKEN and OCP_DOCKER_HOST
    oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}' > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo "Default route for the OpenShift image registry is not found, creating it ..."
        oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
        if [ $? -ne 0 ]; then
            echo "ERROR: failed to create the default route for the OpenShift image registry, exiting..."
            exit 2
        else
            echo "Default route for the OpenShift image registry created"
			echo ""
        fi
    fi

    OCP_DOCKER_HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
    OCP_USER=$(oc whoami)
    OCP_TOKEN=$(oc whoami -t)

    echo "OCP_DOCKER_HOST=$OCP_DOCKER_HOST"
    echo "OCP_USER=$OCP_USER"
    echo "OCP_TOKEN=$OCP_TOKEN"
    echo ""

    # get the OpenShift certificate
    echo "Getting OpenShift certificate..."
    echo | openssl s_client -connect $OCP_DOCKER_HOST:443 -showcerts | sed -n "/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p" > ocp.crt
    if [ $? -ne 0 ]; then
		echo "ERROR: Failed to get the OpenShift certificate"
2        exit 2
    fi
    echo "Get ocp.crt"
    echo ""

    # Getting image from ocp ....
    if [[ "$OSTYPE" == "linux"* ]]; then
        echo "Detected OS is Linux, adding ocp.crt to docker and restarting docker service..."
        mkdir -p /etc/docker/certs.d/$OCP_DOCKER_HOST
        cp ocp.crt /etc/docker/certs.d/$OCP_DOCKER_HOST
        systemctl restart docker.service
        echo "Docker restarted"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Detected OS is Mac OS, adding ocp.crt to docker and restarting docker service..."
        mkdir -p ~/.docker/certs.d/$OCP_DOCKER_HOST
        cp ocp.crt ~/.docker/certs.d/$OCP_DOCKER_HOST
        osascript -e 'quit app "Docker"'
		sleep 1
        open -a Docker
		# The open cmd above does not wait for docker to fully start, so we have to poll
        printf "Waiting for docker to restart"
		sleep 2   # sometimes the very first docker ps succeeds even tho docker is not ready yet
        while ! docker ps > /dev/null 2>&1; do
            printf '.'
            sleep 2
		done
        echo -e "\nDocker restarted"
    else
        echo "ERROR: Detected OS is $OSTYPE. This script is only supported on Linux or Mac OS, exiting..."
        echo ""
        exit 1
    fi

    # login to OCP registry
    echo "Logging in to OpenShift image registry..."
    echo "$OCP_TOKEN" | docker login -u $OCP_USER --password-stdin $OCP_DOCKER_HOST
    if [ $? -ne 0 ]; then
	echo "ERROR: Failed to login to OpenShift image registry"
        exit 2
    fi

    # Getting image from ocp ....
	OCP_IMAGE=$OCP_DOCKER_HOST/ibmcom/amd64_anax_k8s:$AGENT_IMAGE_TAG
	echo "Pulling image $OCP_IMAGE from OpenShift image registry..."
	docker pull $OCP_IMAGE
	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to pull image from OCP image registry"
2        exit 2
    fi

    # save image to tar file
    echo "Saving agent image to $IMAGE_TAR_FILE..."
    docker save $OCP_IMAGE > $IMAGE_TAR_FILE
    if [ $? -ne 0 ]; then
	echo "ERROR: Failed to save agent image to $IMAGE_TAR_FILE"
        exit 2
    fi
    echo "Agent image saved to $IMAGE_TAR_FILE"
    echo ""
}

function zipAgentImage() {
    echo "Zipping $IMAGE_TAR_FILE..."

    IMAGE_ZIP_FILE="$IMAGE_TAR_FILE.gz"
    tar -czvf $IMAGE_ZIP_FILE $(ls $IMAGE_TAR_FILE)
    if [ $? -ne 0 ]; then
        echo "ERROR: failed to zip $IMAGE_TAR_FILE"
        exit 2
    fi

    rm $IMAGE_TAR_FILE
    echo "$IMAGE_ZIP_FILE created"
    echo ""
}

# With the information from the previous functions, create agent-install.cfg
function createAgentInstallConfig () {
	echo "Creating agent-install.cfg file..."
	HUB_CERT_PATH="agent-install.crt"

if [[ "$EDGE_NODE_TYPE" == "x86_64-Cluster" ]]; then
	cat << EndOfContent > agent-install.cfg
HZN_EXCHANGE_URL=$CLUSTER_URL/edge-exchange/v1/
HZN_FSS_CSSURL=$CLUSTER_URL/edge-css/
HZN_ORG_ID=$ORG_ID
HZN_MGMT_HUB_CERT_PATH=$HUB_CERT_PATH
EDGE_CLUSTER_STORAGE_CLASS=$CLUSTER_STORAGE_CLASS
AGENT_NAMESPACE=$AGENT_NAMESPACE
AGENT_IMAGE_TAG=$AGENT_IMAGE_TAG
EndOfContent

else
	cat << EndOfContent > agent-install.cfg
HZN_EXCHANGE_URL=$CLUSTER_URL/edge-exchange/v1/
HZN_FSS_CSSURL=$CLUSTER_URL/edge-css/
HZN_ORG_ID=$CLUSTER_NAME
EndOfContent

fi
	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to create agent-install.cfg file."
        exit 2
    fi

    echo "agent-install.cfg file created: "
	cat agent-install.cfg
	echo ""
}

# Get the IBM Cloud Pak self-signed certificate
function getClusterCert () {
	echo "Getting the IBM Cloud Pak self-signed certificate agent-install.crt..."
	echo "kubectl -n kube-public get secret ibmcloud-cluster-ca-cert -o jsonpath=\"{.data['ca\.crt']}\" | base64 --decode > agent-install.crt"

	kubectl -n kube-public get secret ibmcloud-cluster-ca-cert -o jsonpath="{.data['ca\.crt']}" | base64 --decode > agent-install.crt
	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to get the IBM Cloud Pak self-signed certificate"
2        exit 2
    fi
    echo ""
}

# Locate the IBM Edge Application Manager node installation content
function gatherHorizonFiles () {
	echo "Locating the IBM Edge Application Manager node installation content for $EDGE_NODE_TYPE node..."
	echo "tar --strip-components n -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/..."
	echo "Dist is $DISTRO"

    # Determine edge node type, and distribution if applicable
    if [[ "$EDGE_NODE_TYPE" == "32-bit-ARM" ]]; then
			if [[ "$DISTRO" == "stretch" ]]; then
				tar --strip-components 6 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/linux/raspbian/stretch/armhf
			else
				tar --strip-components 6 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/linux/raspbian/buster/armhf
			fi
		if [ $? -ne 0 ]; then
			echo "ERROR: Failed to locate the IBM Edge Application Manager node installation content"
        	exit 2
    	fi

	elif [[ "$EDGE_NODE_TYPE" == "64-bit-ARM" ]]; then
		if [[ "$DISTRO" == "xenial" ]]; then
			tar --strip-components 6 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/linux/ubuntu/xenial/arm64
		else
			tar --strip-components 6 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/linux/ubuntu/bionic/arm64
		fi
		if [ $? -ne 0 ]; then
			echo "ERROR: Failed to locate the IBM Edge Application Manager node installation content"
        	exit 2
    	fi

	elif [[ "$EDGE_NODE_TYPE" == "x86_64-Linux" ]]; then
		if [[ "$DISTRO" == "xenial" ]]; then
			tar --strip-components 6 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/linux/ubuntu/xenial/amd64
		else
			tar --strip-components 6 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/linux/ubuntu/bionic/amd64
		fi
		if [ $? -ne 0 ]; then
			echo "ERROR: Failed to locate the IBM Edge Application Manager node installation content"
        	exit 2
    	fi

	elif [[ "$EDGE_NODE_TYPE" == "macOS" ]]; then
		tar --strip-components 3 -zxvf $PACKAGE_NAME.tar.gz $PACKAGE_NAME/horizon-edge-packages/macos
		if [ $? -ne 0 ]; then
			echo "ERROR: Failed to locate the IBM Edge Application Manager node installation content"
        	exit 2
    	fi

	else
		echo "ERROR: Unknown node type."
		exit 1
	fi
	echo ""
}

# Download the latest version of the agent-install.sh script and make it executable
function pullAgentInstallScript () {
	echo "Pulling agent-install.sh script..."

	httpCode=$(curl -w "%{http_code}" --progress-bar -LO https://raw.githubusercontent.com/open-horizon/anax/master/agent-install/agent-install.sh)
	if [[ $httpCode -ne 200 ]]; then
		echo "ERROR: Failed to pull agent-install.sh script from the anax repo, HTTP code: $httpCode"
       	exit 2
    fi
	chmod +x ./agent-install.sh
    echo ""
}

# Download the latest version of the agent-uninstall.sh script and make it executable
function pullAgentUninsallScript () {
	echo "Pulling agent-uninstall.sh script..."
        
	httpCode=$(curl -w "%{http_code}" --progress-bar -LO https://raw.githubusercontent.com/open-horizon/anax/master/agent-install/agent-uninstall.sh)
        if [[ $httpCode -ne 200 ]]; then
                echo "ERROR: Failed to pull agent-uninstall.sh script from the anax repo, HTTP code: $httpCode"
        exit 2
    fi
        chmod +x ./agent-uninstall.sh
    echo ""
}

function pullClusterDeployTemplages () {
	echo "Pulling cluster deploy templates: deployment-template.yml, persistentClaim-template.yml..."

	httpCode=$(curl -w "%{http_code}" --progress-bar -LO https://raw.githubusercontent.com/open-horizon/anax/master/agent-install/k8s/deployment-template.yml)
	if [[ $httpCode -ne 200 ]]; then
		echo "ERROR: Failed to pull deployment-template.yml script from the anax repo, HTTP code: $httpCode"
       	exit 2
    fi

	httpCode=$(curl -w "%{http_code}" --progress-bar -LO https://raw.githubusercontent.com/open-horizon/anax/master/agent-install/k8s/persistentClaim-template.yml)
	if [[ $httpCode -ne 200 ]]; then
		echo "ERROR: Failed to pull persistentClaim-template.yml script from the anax repo, HTTP code: $httpCode"
       	exit 2
    fi
	echo ""
}

# Create a tar file of the gathered files for batch install
function createTarFile () {
	echo "Creating agentInstallFiles-$EDGE_NODE_TYPE.tar.gz file containing gathered files..."

	if [[ "$EDGE_NODE_TYPE" == "x86_64-Cluster" ]]; then
		FILES_TO_COMPRESS="agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt $IMAGE_ZIP_FILE deployment-template.yml persistentClaim-template.yml"
	else
		FILES_TO_COMPRESS="agent-install.sh agent-install.cfg agent-install.crt *horizon*"
	fi
	echo "tar -czvf agentInstallFiles-$EDGE_NODE_TYPE.tar.gz $(ls $FILES_TO_COMPRESS)"

	tar -czvf agentInstallFiles-$EDGE_NODE_TYPE.tar.gz $(ls $FILES_TO_COMPRESS)
	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to create agentInstallFiles-$EDGE_NODE_TYPE.tar.gz file."
       	exit 2
    fi
	echo ""
}

# Move gathered files to specified -f directory
function moveFiles () {
	echo "Moving files to $DIR..."
	if ! [[ -d "$DIR" ]]; then
    	echo "$DIR does not exist, creating it..."
    	mkdir $DIR
	fi

	mv $(ls $FILES_TO_COMPRESS) $DIR
	if [ -f key.txt ]; then
    	mv key.txt $DIR
	fi

	if [ $? -ne 0 ]; then
		echo "ERROR: Failed to move files to $DIR."
       	exit 2
    fi
    echo ""
}

# If an API Key was created, print it out
function printApiKey () {
	echo ""
	echo "************************** Your created API Key ******************************"
	echo ""
	echo "     $CLUSTER_USER-Edge-Node-API-Key: $API_KEY"
	echo ""
	echo "********************* Save this value for future use *************************"
	echo ""
}
cluster_main() {
	checkEnvVars

	checkParams

	cloudLogin

	getImageFromOcpRegistry

	zipAgentImage

	createAgentInstallConfig

	getClusterCert

	pullAgentInstallScript

	pullAgentUninsallScript

	pullClusterDeployTemplages

	if [[ "$PACKAGE_FILES" == "-t" ]]; then
		createTarFile
	fi

	if ! [ -z $DIR ]; then
		moveFiles
	fi
}

device_main() {
	checkEnvVars

	cloudLogin

	getClusterName

	checkAPIKey

	if [[ "$CREATE_API_KEY" == "-k" ]] || [[ "$CREATE_NEW_KEY" == "true" ]]; then
		createAPIKey
	fi

	createAgentInstallConfig

	getClusterCert

	gatherHorizonFiles

	pullAgentInstallScript

	if [[ "$PACKAGE_FILES" == "-t" ]]; then
		createTarFile
	fi

	if ! [ -z $DIR ]; then
		moveFiles
	fi

	if [[ "$CREATE_API_KEY" == "-k" ]] || [[ "$CREATE_NEW_KEY" == "true" ]]; then
		printApiKey
	fi

}

main() {

	if [[ "$EDGE_NODE_TYPE" == "x86_64-Cluster" ]]; then
		cluster_main
	else
		device_main
	fi
}

main


