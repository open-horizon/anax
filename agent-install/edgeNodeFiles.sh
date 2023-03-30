#!/bin/bash

# This script gathers the necessary information and files to install the Horizon agent and register an edge node

function getHznVersion() { local hzn_version=$(hzn version | grep "^Horizon CLI"); echo ${hzn_version##* }; }

# Global constants
SUPPORTED_NODE_TYPES='ARM32-Deb ARM64-Deb AMD64-Deb x86_64-RPM arm64-macOS x86_64-macOS x86_64-Cluster ppc64le-RPM ppc64le-Cluster ALL'
EDGE_CLUSTER_TAR_FILE_NAME='horizon-agent-edge-cluster-files.tar.gz'

# Note: arch must prepend the following 2 variables when used - currently only amd64 and arm64 are built and pushed
AGENT_IMAGE_TAR_FILE='_anax.tar.gz'
AGENT_IMAGE='_anax'

AGENT_K8S_IMAGE_TAR_FILE='amd64_anax_k8s.tar.gz'
AGENT_K8S_IMAGE='amd64_anax_k8s'
AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE='amd64_auto-upgrade-cronjob_k8s.tar.gz'
AUTO_UPGRADE_CRONJOB_K8S_IMAGE='amd64_auto-upgrade-cronjob_k8s'
AGENT_IMAGE_TAG=$(getHznVersion)
DEFAULT_PULL_REGISTRY='docker.io/openhorizon'
MANIFEST_NAME='edgeNodeFiles_manifest'

# Global variables
ALREADY_LOGGED_INTO_REGISTRY='false'
AGENT_FILES_EXPIRATION=''   # in days
SOFTWARE_PACKAGE_VERSION='0'



# Environment variables and their defaults
PACKAGE_NAME=${PACKAGE_NAME:-horizon-edge-packages-4.4.0}
AGENT_NAMESPACE=${AGENT_NAMESPACE:-openhorizon-agent}
# EDGE_CLUSTER_STORAGE_CLASS   # this can also be specified, that default is the empty string

function scriptUsage() {
    cat << EOF

Usage: ./edgeNodeFiles.sh <edge-node-type> [-o <hzn-org-id>] [-c] [-f <directory>] [-t] [-p <package_name>] [-s <edge-cluster-storage-class>] [-i <agent-image-tag>] [-m <agent-namespace>] [-r <registry/repo-path>] [-g <tag>] [-b] [-x <days>]

Parameters:
  Required:
    <edge-node-type>    The type of edge node planned for agent install and registration. Valid values: $SUPPORTED_NODE_TYPES

  Optional:
    -o <hzn-org-id>     The exchange org id that should be used for all edge nodes. If there will be multiple exchange orgs, do not set this.
    -c    Put the gathered files into the Cloud Sync Service (MMS). On the edge nodes agent-install.sh can pull the files from there.
    -f <directory>     The directory to put the gathered files in. Default is current directory.
    -t          Create agentInstallFiles-<edge-node-type>.tar.gz file containing gathered files. If this flag is not set, the gathered files will be placed in the current directory.
    -p <package_name>   The base name of the horizon content tar file (can include a path). Default is $PACKAGE_NAME, which means it will look for $PACKAGE_NAME.tar.gz and expects a standardized directory structure of $PACKAGE_NAME/<OS>/<pkg-type>/<arch>
    -s <edge-cluster-storage-class>   Default storage class to be used for all the edge clusters. If not specified, can be specified when running agent-install.sh. Only applies to node types of <arch>-Cluster.
    -m <agent-namespace>   The edge cluster namespace that the agent will be installed into. Default is $AGENT_NAMESPACE. Only applies to node types of <arch>-Cluster.
    -r <registry/repo-path>     The agent images (both device and cluster) will be pulled from the specified authenticated docker registry. If this flag is set, you must also export the username and password for the registry in REGISTRY_USERNAME and REGISTRY_PASSWORD. Default is $DEFAULT_PULL_REGISTRY.
    -g <tag>    Overwrite the default agent image tag. Default is $AGENT_IMAGE_TAG.
    -b          Get the agent images from the horizon content tar file.
    -x <days>   Sets the expiration field of the versioned files pushed to CSS. Should not be used unless artifacts are being produced and pushed by a CI/CD pipeline. Default is expiration not set.

Required Environment Variables:
    CLUSTER_URL: for example: https://<cluster_CA_domain>:<port-number>

Optional Environment Variables:
    PACKAGE_NAME: The base name of the horizon content tar file (can include a path). Default: $PACKAGE_NAME
    HZN_EXCHANGE_USER_AUTH: The exchange credentials to use to publish artifacts in CSS. Must have access to publish to the IBM org. If not set, will use exchange root credentials.
    AGENT_NAMESPACE: The edge cluster namespace that the agent will be installed into. Default: $AGENT_NAMESPACE
    REGISTRY_USERNAME: The username used to login to the registry supplied with the -r flag.
    REGISTRY_PASSWORD: The password used to login to the registry supplied with the -r flag.
EOF
    exit $1
}

# Echo message and exit
function fatal() {
    : ${1:?} ${2:?}
    local exitCode=$1
    # the rest of the args are the message
    echo "ERROR:" ${@:2}
    exit $exitCode
}

# Check the exit code passed in and exit if non-zero
function chk() {
    local exitCode=$1
    local task=$2
    if [[ $exitCode == 0 ]]; then return; fi
    fatal $exitCode "exit code $exitCode from: $task"
}

# Parse cmd line args
if [[ "$#" = "0" ]]; then
    scriptUsage 1
fi

while (( "$#" )); do
    case "$1" in
        -o) # value of HZN_ORG_ID. Intentionally not using HZN_ORG_ID so they don't accidentally set it from their environment
            ORG_ID=$2
            shift 2
            ;;
        -c) PUT_FILES_IN_CSS='true'
            shift
            ;;
        -f) # directory to move gathered files to
            DIR=$2
            shift 2
            ;;
        -t) # create tar file
            CREATE_TAR_FILE='true'
            shift
            ;;
        -p) # installation media name string
            PACKAGE_NAME=$2
            shift 2
            ;;
        -s) # storage class to use by persistent volume claim in edge cluster
            EDGE_CLUSTER_STORAGE_CLASS=$2
            shift 2
            ;;
        -m) # edge cluster namespace to install agent to
            AGENT_NAMESPACE=$2
            shift 2
            ;;
        -r) # registry to pull agent images from
            PULL_REGISTRY=$2
            shift 2
            ;;
        -g) AGENT_IMAGE_TAG=$2
            shift 2
            ;;
        -b) AGENT_IMAGES_FROM_TAR='true'
            shift
            ;;
        -h) scriptUsage 0
            shift
            ;;
        -x) AGENT_FILES_EXPIRATION=$2
            shift 2
            ;;
        -*) #invalid flag
            echo "ERROR: Unknow flag $1"
            scriptUsage 1
            ;;
        *) # based on "Usage" this should be node type
            EDGE_NODE_TYPE=$1
            shift
            ;;
      esac
done
if [[ -z $EDGE_NODE_TYPE ]]; then
    scriptUsage 1
fi

function checkPrereqsAndInput () {
    echo "Checking system requirements..."
    if ! command -v oc >/dev/null 2>&1; then
        fatal 2 "oc is not installed."
    fi
    echo " - oc installed"

    if ! command -v hzn >/dev/null 2>&1; then
        fatal 2 "hzn is not installed."
    fi
    echo " - hzn installed"

    if [[ $EDGE_NODE_TYPE == 'x86_64-Cluster' || $EDGE_NODE_TYPE == 'ppc64le-Cluster' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        if ! command -v docker >/dev/null 2>&1; then
            fatal 2 "docker is not installed."
        fi
        echo " - docker installed"
    fi
    echo ""

    echo "Checking command arguments ..."
    if [[ $SUPPORTED_NODE_TYPES != *$EDGE_NODE_TYPE* ]]; then
        fatal 1 "Unknown edge node type. Valid values: $SUPPORTED_NODE_TYPES"
    fi
    if [[ -n $DIR && ! -d $DIR ]]; then
        fatal 1 "the value of option '-f' isn't an existing directory ..."
    fi
    if [[ -n $PULL_REGISTRY ]]; then
        if [[ -z $REGISTRY_USERNAME || -z $REGISTRY_PASSWORD ]]; then
            fatal 1 "REGISTRY_USERNAME or REGISTRY_PASSWORD not set. When using the '-r' flag you must supply the username and password for the registry you are attempting to pull the agent images from ..."
        fi
    else
        PULL_REGISTRY=$DEFAULT_PULL_REGISTRY
    fi
    echo ' - Command arguments valid'
    echo ""

    echo "Checking environment variables..."
    if [[ -z $CLUSTER_URL ]]; then
        fatal 1 "CLUSTER_URL environment variable is not set.'"
    fi
    echo " - CLUSTER_URL: $CLUSTER_URL"
    echo ""
}

# Method to store the software package version if it is not set yet
function setSoftwarePackageVersion() { 

       if [[ ! $1 == '' ]] && [[ "${SOFTWARE_PACKAGE_VERSION}" == "0" ]]; then
	       SOFTWARE_PACKAGE_VERSION=$1
       fi
}

# Utility method to add an element to an array
function addElementToArray() { 
       var="$1"
       shift 1
       eval "$var+=($(printf "'%s' " "$@"))"
}

# Add fields common to all upgrade stanzas for an upgrade manifest
function manifestInitUpgradeFields() {

        local __resultvar=$1
        VERSION=$2

        local myinitresult=''
        myinitresult=$(jq --null-input --arg  version $VERSION '{version: $version}' | jq '. + {files: []}')
        eval $__resultvar="'$myinitresult'"
}

# Remove files from previous run, so we know there won't be (for example) multiple versions of the horizon pkgs in the dir
function cleanUpPreviousFiles() {
    echo "Removing any generated files from previous run..."
    rm -f agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt "amd64${AGENT_IMAGE_TAR_FILE}" "arm64${AGENT_IMAGE_TAR_FILE}" "$AGENT_K8S_IMAGE_TAR_FILE" "$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE" deployment-template.yml persistentClaim-template.yml auto-upgrade-cronjob-template.yml horizon*.{deb,rpm,pkg,crt}
    chk $? "removing previous files in $PWD"
    echo
}

# Pull the edge cluster agent image from the package tar file or from a docker registry
function getAgentK8sImageTarFile() {
    local upgradeFiles=$1

    if [[ $AGENT_IMAGES_FROM_TAR == 'true' ]]; then
        local pkgBaseName=${PACKAGE_NAME##*/}   # inside the tar file, the paths start with the base name
        echo "Extracting $pkgBaseName/docker/$AGENT_K8S_IMAGE_TAR_FILE from $PACKAGE_NAME.tar.gz ..."
        tar --strip-components 2 -zxf $PACKAGE_NAME.tar.gz "$pkgBaseName/docker/$AGENT_K8S_IMAGE_TAR_FILE"
        chk $? "extracting $pkgBaseName/docker/$AGENT_K8S_IMAGE_TAR_FILE from $PACKAGE_NAME.tar.gz"
    else
        if [[ ($PULL_REGISTRY != $DEFAULT_PULL_REGISTRY) && ($ALREADY_LOGGED_INTO_REGISTRY == 'false') ]]; then
            echo "Logging into $PULL_REGISTRY ..."
            echo "$REGISTRY_PASSWORD" | docker login -u $REGISTRY_USERNAME --password-stdin $PULL_REGISTRY
            chk $? "logging into edge cluster's registry: $PULL_REGISTRY"
            ALREADY_LOGGED_INTO_REGISTRY='true'
        fi

        echo "Pulling $PULL_REGISTRY/$AGENT_K8S_IMAGE:$AGENT_IMAGE_TAG ..."
        docker pull $PULL_REGISTRY/$AGENT_K8S_IMAGE:$AGENT_IMAGE_TAG
        chk $? "pulling $PULL_REGISTRY/$AGENT_K8S_IMAGE:$AGENT_IMAGE_TAG"

        echo "Saving $AGENT_K8S_IMAGE:$AGENT_IMAGE_TAG to $AGENT_K8S_IMAGE_TAR_FILE ..."
        docker save $PULL_REGISTRY/$AGENT_K8S_IMAGE:$AGENT_IMAGE_TAG | gzip > $AGENT_K8S_IMAGE_TAR_FILE
        chk $? "saving $PULL_REGISTRY/$AGENT_K8S_IMAGE:$AGENT_IMAGE_TAG"
    fi

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        echo "Extracting version from $AGENT_K8S_IMAGE_TAR_FILE ..."
        local version=$(tar -zxOf "$AGENT_K8S_IMAGE_TAR_FILE" manifest.json | jq -r '.[0].RepoTags[0]')   # this gets the full path
        version=${version##*:}   # strip the path and image name from the front
        echo "Version/tag of $AGENT_K8S_IMAGE_TAR_FILE is: $version"
        putOneFileInCss "$AGENT_K8S_IMAGE_TAR_FILE" agent_files false $version
        putOneFileInCss "$AGENT_K8S_IMAGE_TAR_FILE" "agent_software_files-${version}"  true  ${version}

        addElementToArray $upgradeFiles $AGENT_K8S_IMAGE_TAR_FILE
    fi
    echo
}

# Pull the edge cluster agent-auto-upgrade cronjob image from the package tar file or from a docker registry
function getAutoUpgradeCronjobK8sImageTarFile() {
    local upgradeFiles=$1

    if [[ $AGENT_IMAGES_FROM_TAR == 'true' ]]; then
        local pkgBaseName=${PACKAGE_NAME##*/}   # inside the tar file, the paths start with the base name
        echo "Extracting $pkgBaseName/docker/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE from $PACKAGE_NAME.tar.gz ..."
        tar --strip-components 2 -zxf $PACKAGE_NAME.tar.gz "$pkgBaseName/docker/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE"
        chk $? "extracting $pkgBaseName/docker/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE from $PACKAGE_NAME.tar.gz"
    else
        if [[ ($PULL_REGISTRY != $DEFAULT_PULL_REGISTRY) && ($ALREADY_LOGGED_INTO_REGISTRY == 'false') ]]; then
            echo "Logging into $PULL_REGISTRY ..."
            echo "$REGISTRY_PASSWORD" | docker login -u $REGISTRY_USERNAME --password-stdin $PULL_REGISTRY
            chk $? "logging into edge cluster's registry: $PULL_REGISTRY"
            ALREADY_LOGGED_INTO_REGISTRY='true'
        fi

        echo "Pulling $PULL_REGISTRY/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE:$AGENT_IMAGE_TAG ..."
        docker pull $PULL_REGISTRY/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE:$AGENT_IMAGE_TAG
        chk $? "pulling $PULL_REGISTRY/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE:$AGENT_IMAGE_TAG"

        echo "Saving $AUTO_UPGRADE_CRONJOB_K8S_IMAGE:$AGENT_IMAGE_TAG to $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE ..."
        docker save $PULL_REGISTRY/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE:$AGENT_IMAGE_TAG | gzip > $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE
        chk $? "saving $PULL_REGISTRY/$AUTO_UPGRADE_CRONJOB_K8S_IMAGE:$AGENT_IMAGE_TAG"
    fi

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        echo "Extracting version from $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE ..."
        local version=$(tar -zxOf "$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE" manifest.json | jq -r '.[0].RepoTags[0]')   # this gets the full path
        version=${version##*:}   # strip the path and image name from the front
        echo "Version/tag of $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE is: $version"
        putOneFileInCss "$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE" agent_files false $version
        putOneFileInCss "$AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE" "agent_software_files-${version}"  true  ${version}

        addElementToArray $upgradeFiles $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE
    fi
    echo
}

# Put 1 file into CSS in the IBM org as a public object.
function putOneFileInCss() {
    local filename=${1:?} objectType=$2 addExpiration=$3 version=$4
    local resourcename=$(oc get eamhub --no-headers |awk '{printf $1}')

    # First get exchange root creds, if necessary
    if [[ -z $HZN_EXCHANGE_USER_AUTH ]]; then
        echo "Getting exchange root credentials to use to publish to CSS..."
        export HZN_EXCHANGE_USER_AUTH="root/root:$(oc get secret $resourcename-auth -o jsonpath="{.data.exchange-root-pass}" | base64 --decode)"
        chk $? 'getting exchange root creds'
    fi

    echo "Publishing $filename as type $objectType in CSS as a public object in the IBM org..."

    # Build meta-data
    local META_DATA=$(jq --null-input --arg org IBM --arg ID $filename --arg TYPE $objectType '{objectID: $ID, objectType: $TYPE, destinationOrgID: $org}' | jq --argjson SET_TRUE true '. + {public: $SET_TRUE}') 

    if [ ! -z ${version} ]; then 
	    META_DATA=$( echo "${META_DATA}" | jq --arg VERSION ${version} '. + {version: $VERSION}' ) 
    fi 

    if [[ $addExpiration == true ]]; then 
        local exp_time=''
        getExpirationTime exp_time
        if [[ ! "${exp_time}" == "0" ]]; then
            META_DATA=$( echo "${META_DATA}" | jq --arg EXPIRATION ${exp_time} '. + {expiration: $EXPIRATION}' )
        fi
    fi

    echo "${META_DATA}" | hzn mms -o IBM object publish -f $filename   -m-

    local rc=$?
    chk $rc "publishing $filename in CSS as a public object. Ensure HZN_EXCHANGE_USER_AUTH is set to credentials that can publish to the IBM org."
}

# it returns the agent file expiration time beased on $AGENT_FILES_EXPIRATION.
#    return 0 if $AGENT_FILES_EXPIRATION is not set.
function getExpirationTime() {
    local __resultvar=$1

    # Only add if caller passed it a value representing the days before deleting
    local EXP_TIME="0"
    if [ ! -z $AGENT_FILES_EXPIRATION ]; then

        if [[ $OSTYPE == darwin* ]]; then
            local EXPIRATION_TIME=$( date -ju -v +"${AGENT_FILES_EXPIRATION}"d +%Y-%m-%dT%H:%M:%S.00Z )
            if [[ ${#EXPIRATION_TIME} -gt 0  ]]; then
                EXP_TIME="${EXPIRATION_TIME}"
            fi
        else   # linux (deb or rpm) 
            local EXPIRATION_TIME=$( date -u -d "+${AGENT_FILES_EXPIRATION} days" +"%Y-%m-%dT%H:%M:%S.00Z" )
            if [[ ${#EXPIRATION_TIME} -gt 0  ]]; then
                EXP_TIME="${EXPIRATION_TIME}"
            fi
        fi
    fi
    
    eval $__resultvar="'$EXP_TIME'"
}

# Put "total" in CSS for agent_software_files-<version> type
function getAgentFileTotal() {
    ver_in=$1

    # get exchange root creds, if necessary
    if [[ -z $HZN_EXCHANGE_USER_AUTH ]]; then
        echo "Getting exchange root credentials to use to publish to CSS..."
        local resourcename=$(oc get eamhub --no-headers |awk '{printf $1}')
        export HZN_EXCHANGE_USER_AUTH="root/root:$(oc get secret $resourcename-auth -o jsonpath="{.data.exchange-root-pass}" | base64 --decode)"
        chk $? 'getting exchange root creds'
    fi

    objectType="agent_software_files-${ver_in}"
    echo "Getting all the objects with type $objectType from CSS ..."
    output=$(hzn mms -o IBM object list -t $objectType 2>&1)
    if [ $? -ne 0 ]; then
        echo "$output"
        num=0
    else
        # check if the id "total" is already in the list
        echo "$output" | grep '"objectID": "total"' 2>&1 > /dev/null
        has_total=$?

        # get the total number of agents
        num=$(echo "$output" | grep $objectType | wc -l)
        if [ $has_total -eq 0 ]; then
            num=`expr $num - 1`
        fi
    fi

    if [ "$num" != "0" ]; then
        if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
            local META_DATA=$(jq --null-input --arg org IBM --arg ID total --arg TYPE $objectType --arg VERSION ${ver_in} --arg TOTAL $num '{objectID: $ID, objectType: $TYPE, destinationOrgID: $org, version: $VERSION, description: $TOTAL}' | jq --argjson SET_TRUE true '. + {public: $SET_TRUE, noData: $SET_TRUE}') 
            local exp_time=''
            getExpirationTime exp_time
            if [[ ! "${exp_time}" == "0" ]]; then
                META_DATA=$( echo "${META_DATA}" | jq --arg EXPIRATION ${exp_time} '. + {expiration: $EXPIRATION}' )
            fi


            echo "${META_DATA}" | hzn mms -o IBM object publish -m-
            local rc=$?
            chk $rc "publishing 'total' in CSS as a public object. Ensure HZN_EXCHANGE_USER_AUTH is set to credentials that can publish to the IBM org."
        fi
    fi
    echo
}

# Utility file to see if the objectType and objectID already exist in CSS
function test_IsFileInCss() {
	local org=$1 objectType=$2 objectID=$3 

	local resourcename=$(oc get eamhub --no-headers |awk '{printf $1}')
	local USER_AUTH=${HZN_EXCHANGE_USER_AUTH}

        # First get exchange root creds, if necessary
        if [[ -z ${USER_AUTH} ]]; then 
		USER_AUTH="root/root:$(oc get secret $resourcename-auth -o jsonpath="{.data.exchange-root-pass}" | base64 --decode)" 
		chk $? 'getting exchange root creds'
        fi
	
	objects=$( hzn mms object list -u ${USER_AUTH} -o ${org} -t ${objectType}  -i ${objectID} | grep -v "Listing"  | jq '. | length' ) 
	rc=$?  
	if [[ $rc -eq 0  && $objects -gt 0 ]]; then 
		true
		return
	else
		false
		return
	fi
}

# Add specific stanzas to manifest
manifestAppendUpgradeStanza() {
 
        local __resultvar=$1

        manifestJson=$2
        upgradeType=$3
        upgradeJson=$4

        local NEW_MANIFEST=''

        # The jq code is duplicated since wasn't able to find a way to add the types as a variable
        if [ "${upgradeType}" == "configurationUpgrade" ]; then
                NEW_MANIFEST=$(echo "${manifestJson}" | jq --argjson JSON "${upgradeJson}" '. + {configurationUpgrade: $JSON}')
        elif [ "${upgradeType}" == "softwareUpgrade" ]; then
                NEW_MANIFEST=$(echo "${manifestJson}" | jq --argjson JSON "${upgradeJson}" '. + {softwareUpgrade: $JSON}')
        elif [ "${upgradeType}" == "certificateUpgrade" ]; then
                NEW_MANIFEST=$(echo "${manifestJson}" | jq --argjson JSON "${upgradeJson}" '. + {certificateUpgrade: $JSON}')
        fi

        eval $__resultvar="'$NEW_MANIFEST'"
}

# Add files for the upgrade manifest
function manifestAddUpgradeFile() {

        local __resultvar=$1

        JSON=$2
        file=$3

        local addFileResult=''
        addFileResult=$(echo "${JSON}"  | jq --arg FILE $file '.files |= [ $FILE ] + .')
        eval $__resultvar="'$addFileResult'"
}

# Add files for the upgrade manifest
function manifestBuildUpgrade()  {

        local __resultvar=$1
        shift

        local myresult=''

        JSON=$1
        shift 
	
        local filesToUpgrade=("${@}")

        local tmpJson="${JSON}"
        for file in "${filesToUpgrade[@]}"; do
                manifestAddUpgradeFile myresult "${tmpJson}"  ${file}
                tmpJson="${myresult}"
        done

        eval $__resultvar="'${myresult}'"
}

# Function to add type stanza to upgrade manifest
function manifestAddTypeStanza() {

    local __resultvar=$1
    shift 

    upgradeManifest=$1
    shift

    type=$1
    shift

    version=$1
    shift

    upgradeFiles=("${@}")

    local myhorizonresult=$upgradeManifest

    local updatedJson="${upgradeManifest}"

    local upgradeJson=''
    manifestInitUpgradeFields upgradeJson  $version

    manifestBuildUpgrade upgradeJson "${upgradeJson}" "${upgradeFiles[@]}"

    manifestAppendUpgradeStanza myhorizonresult "${upgradeManifest}" $type "${upgradeJson}"

    echo ""
    eval $__resultvar="'$myhorizonresult'"
}


# With the information from the previous functions, create agent-install.cfg
function createAgentInstallConfig () {

    local upgradeFiles=$1

    echo "Creating agent-install.cfg file..."
    HUB_CERT_PATH="agent-install.crt"

    local doUploadConfig='true'
    local doUploadConfig_versioned='true' 

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
            # Only upload cert if it doesn't exist in CSS.. ie. fresh install
            if test_IsFileInCss "IBM"  "agent_files" "agent-install.cfg"; then 
	        doUploadConfig='false' 
            fi

            if test_IsFileInCss "IBM"  "agent_config_files-1.0.0" "agent-install.cfg"; then
	          doUploadConfig_versioned='false'
            fi
    fi

    if [[ $PUT_FILES_IN_CSS != 'true' || ${doUploadConfig} == 'true' || ${doUploadConfig_versioned} == 'true' ]]; then 
	    
	    if [[ $EDGE_NODE_TYPE == 'x86_64-Cluster' || $EDGE_NODE_TYPE == 'ppc64le-Cluster' || $EDGE_NODE_TYPE == 'ALL' ]]; then   # if they chose ALL, the cluster agent-install.cfg is a superset 

		    cat << EndOfContent > agent-install.cfg 
HZN_EXCHANGE_URL=$CLUSTER_URL/edge-exchange/v1
HZN_FSS_CSSURL=$CLUSTER_URL/edge-css/
HZN_AGBOT_URL=$CLUSTER_URL/edge-agbot/
HZN_SDO_SVC_URL=$CLUSTER_URL/edge-sdo-ocs/api
HZN_FDO_SVC_URL=$CLUSTER_URL/edge-fdo-ocs/api
AGENT_NAMESPACE=$AGENT_NAMESPACE
EndOfContent

        	      	# Only include these if they are not empty 
			if [[ -n $EDGE_CLUSTER_STORAGE_CLASS ]]; then 
				echo "EDGE_CLUSTER_STORAGE_CLASS=$EDGE_CLUSTER_STORAGE_CLASS" >> agent-install.cfg 
			fi 
			if [[ -n $ORG_ID ]]; then 
				echo "HZN_ORG_ID=$ORG_ID" >> agent-install.cfg 
			fi 
		
		else   # device 

			cat << EndOfContent > agent-install.cfg
HZN_EXCHANGE_URL=$CLUSTER_URL/edge-exchange/v1
HZN_FSS_CSSURL=$CLUSTER_URL/edge-css/
HZN_AGBOT_URL=$CLUSTER_URL/edge-agbot/
HZN_SDO_SVC_URL=$CLUSTER_URL/edge-sdo-ocs/api
HZN_FDO_SVC_URL=$CLUSTER_URL/edge-fdo-ocs/api
EndOfContent

        		if [[ -n $ORG_ID ]]; then
            			echo "HZN_ORG_ID=$ORG_ID" >> agent-install.cfg
        		fi
    		fi
    		chk $? 'creating agent-install.cfg file'

    		echo "agent-install.cfg file created with content: "
    		cat agent-install.cfg

    fi

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
    	if [[ ${doUploadConfig} == 'true'  ]]; then 
            putOneFileInCss agent-install.cfg agent_files false
	    fi

    	if [[ ${doUploadConfig_versioned} == 'true'  ]]; then 
            putOneFileInCss agent-install.cfg  "agent_config_files-1.0.0" false "1.0.0"
            addElementToArray $upgradeFiles  "agent-install.cfg"
	   fi
    fi

    echo ""
}

# Get the management hub self-signed certificate
function getClusterCert () {

    local upgradeFiles=$1

    echo "Getting the management hub self-signed certificate agent-install.crt..."
    oc get secret management-ingress-ibmcloud-cluster-ca-cert -o jsonpath="{.data['ca\.crt']}" | base64 --decode > agent-install.crt
    chk $? 'getting the management hub self-signed certificate'

    local doUploadCert='true'
    local doUploadCert_versioned='true' 
    # Only upload cert if it doesn't exist in CSS.. ie. fresh install
    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        if test_IsFileInCss "IBM"  "agent_files" "agent-install.crt"; then 
            doUploadCert='false' 
        fi

        if  test_IsFileInCss "IBM"  "agent_cert_files-1.0.0" "agent-install.crt"; then
            doUploadCert_versioned='false'
        fi
    fi

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        if [[ ${doUploadCert} == 'true' ]]; then 
            putOneFileInCss agent-install.crt agent_files false
        fi

        if [[ ${doUploadCert_versioned} == 'true' ]]; then 
            putOneFileInCss agent-install.crt  "agent_cert_files-1.0.0" false  "1.0.0"
            addElementToArray $upgradeFiles  "agent-install.crt"
        fi
    fi
    echo ""
}

# Create 1 horizon pkg tar file, put it into CSS, and then remove the tar file
function putHorizonPkgsInCss() {

    horizonSoftwareFiles=$1
    local opsys=$2 pkgtype=$3 arch=$4

    # Determine the pkgs to put in CSS, and the tar file name
    # Note: at this point there are potentionally other horizonn pkgs too, so we have to be specific about the files that should be included in this tar file
    local pkgWildcard tarFile pkgVersion
    if [[ $pkgtype == 'deb' ]]; then
        pkgWildcard="horizon*_$arch.$pkgtype"
        tarFile="horizon-agent-${opsys}-${pkgtype}-$arch.tar.gz"
        pkgVersion=$(ls horizon_*_$arch.$pkgtype)
        pkgVersion=${pkgVersion#horizon_}
        pkgVersion=${pkgVersion%%_$arch.$pkgtype}
    elif [[ $pkgtype == 'rpm' ]]; then
        pkgWildcard="horizon*.$arch.$pkgtype"
        tarFile="horizon-agent-${opsys}-${pkgtype}-$arch.tar.gz"
        pkgVersion=$(ls horizon-*.$arch.$pkgtype | grep -v 'horizon-cli')
        pkgVersion=${pkgVersion#horizon-}
        pkgVersion=${pkgVersion%%.$arch.$pkgtype}
    elif [[ $opsys == 'macos' ]]; then
        tarFile="horizon-agent-${opsys}-${pkgtype}-$arch.tar.gz"
        # mac pkg might be *.$arch.$pkgtype or just *.$pkgtype
        ls horizon-cli-*.$arch.$pkgtype > /dev/null 2>&1 
        if [[ $? -eq 0 ]]; then
            pkgWildcard="horizon-cli.crt horizon-cli-*.$arch.$pkgtype"
            pkgVersion=$(ls horizon-cli-*.$arch.$pkgtype)
            pkgVersion=${pkgVersion#horizon-cli-}
            pkgVersion=${pkgVersion%%.$arch.$pkgtype}
        else
            pkgWildcard="horizon-cli.crt horizon-cli-*.$pkgtype"
            pkgVersion=$(ls horizon-cli-*.$pkgtype)
            pkgVersion=${pkgVersion#horizon-cli-}
            pkgVersion=${pkgVersion%%.$pkgtype}
        fi
    fi

    # Create the pkg tar file
    tar -zcf "$tarFile" $pkgWildcard   # it is important to NOT quote $pkgWildcard so the wildcard gets expanded
    chk $? "creating $tarFile"

    # Could be some conditions on when to upload files
    local doUploadPkgs='true'
    local doUploadPkgs_versioned='true'

    # Put the tar file in CSS in the IBM org as a public object
    if [[ ${doUploadPkgs} == 'true' ]]; then 
        putOneFileInCss $tarFile "agent_files" false $pkgVersion
    fi
    if [[ ${doUploadPkgs_versioned} == 'true' ]]; then 
        putOneFileInCss $tarFile "agent_software_files-${pkgVersion}" true $pkgVersion

        # Add the tarFile name array for the manifest
        addElementToArray $horizonSoftwareFiles $tarFile 

        # Will set the software package version if not set yet
        setSoftwarePackageVersion ${pkgVersion}
    fi

    # Remove the tar file (it was only needed to put into CSS)
    rm -f "$tarFile"
    chk $? "removing $tarFile"
}

# Get 1 type of horizon packages
function getHorizonPackageFiles() {

    local softwareFiles=$1 opsys=$2 pkgtype=$3 arch=$4
    local pkgBaseName=${PACKAGE_NAME##*/}   # inside the tar file, the paths start with the base name
    echo "Extracting $pkgBaseName/$opsys/$pkgtype/$arch/* from $PACKAGE_NAME.tar.gz ..."
    tar --strip-components 4 -zxf $PACKAGE_NAME.tar.gz $pkgBaseName/$opsys/$pkgtype/$arch
    chk $? "extracting $pkgBaseName/$opsys/$pkgtype/$arch/* from $PACKAGE_NAME.tar.gz"


    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        putHorizonPkgsInCss $softwareFiles $opsys $pkgtype $arch
    fi

    if [[ $opsys == 'macos' ]]; then   #future: do this for all amd64/x86_64

        # Set architectures to match what agent will use
        if [[ $arch == "x86_64" ]]; then
                put_ARCH="amd64"
        else
                put_ARCH=$arch
        fi

        if [[ $AGENT_IMAGES_FROM_TAR == 'true' ]]; then
            tar --strip-components 2 -zxf $PACKAGE_NAME.tar.gz "$pkgBaseName/docker/${put_ARCH}${AGENT_IMAGE_TAR_FILE}"
            chk $? "extracting $pkgBaseName/docker/${put_ARCH}${AGENT_IMAGE_TAR_FILE} from $PACKAGE_NAME.tar.gz"
        else
            if [[ ($PULL_REGISTRY != $DEFAULT_PULL_REGISTRY) && ($ALREADY_LOGGED_INTO_REGISTRY == 'false') ]]; then
                echo "Logging into $PULL_REGISTRY ..."
                echo "$REGISTRY_PASSWORD" | docker login -u $REGISTRY_USERNAME --password-stdin $PULL_REGISTRY
                chk $? "logging into edge cluster's registry: $PULL_REGISTRY"
                ALREADY_LOGGED_INTO_REGISTRY='true'
            fi

            echo "Pulling $PULL_REGISTRY/${put_ARCH}${AGENT_IMAGE}:$AGENT_IMAGE_TAG ..."
            docker pull $PULL_REGISTRY/${put_ARCH}${AGENT_IMAGE}:$AGENT_IMAGE_TAG
            chk $? "pulling $PULL_REGISTRY/${put_ARCH}${AGENT_IMAGE}:$AGENT_IMAGE_TAG"

            echo "Saving ${put_ARCH}${AGENT_IMAGE}:$AGENT_IMAGE_TAG to ${put_ARCH}$AGENT_IMAGE_TAR_FILE ..."
            docker save $PULL_REGISTRY/${put_ARCH}${AGENT_IMAGE}:$AGENT_IMAGE_TAG | gzip > ${put_ARCH}${AGENT_IMAGE_TAR_FILE}
            chk $? "saving $PULL_REGISTRY/${put_ARCH}${AGENT_IMAGE}:$AGENT_IMAGE_TAG"
        fi

        if [[ $PUT_FILES_IN_CSS == 'true' ]]; then

            echo "Extracting version from ${put_ARCH}${AGENT_IMAGE_TAR_FILE} ..."
            local version=$(tar -zxOf "${put_ARCH}${AGENT_IMAGE_TAR_FILE}" manifest.json | jq -r '.[0].RepoTags[0]')   # this gets the full path
            version=${version##*:}   # strip the path and image name from the front
            echo "Version/tag of ${put_ARCH}${AGENT_IMAGE_TAR_FILE} is: $version"
            putOneFileInCss "${put_ARCH}${AGENT_IMAGE_TAR_FILE}" agent_files false $version
            putOneFileInCss "${put_ARCH}${AGENT_IMAGE_TAR_FILE}" "agent_software_files-${version}" true $version

            addElementToArray $softwareFiles "${put_ARCH}${AGENT_IMAGE_TAR_FILE}"
        fi
    fi
}

# Get all of the the horizon packages that they specified
function gatherHorizonPackageFiles() {

    local agentSoftwareFiles=$1

    local opsys pkgtype arch
    if [[ $EDGE_NODE_TYPE == 'ARM32-Deb' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'linux' 'deb' 'armhf'
    fi
    if [[ $EDGE_NODE_TYPE == 'ARM64-Deb' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'linux' 'deb' 'arm64'
    fi
    if [[ $EDGE_NODE_TYPE == 'AMD64-Deb' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'linux' 'deb' 'amd64'
    fi
    if [[ $EDGE_NODE_TYPE == 'x86_64-RPM' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'linux' 'rpm' 'x86_64'
    fi
    if [[ $EDGE_NODE_TYPE == 'x86_64-macOS' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'macos' 'pkg' 'x86_64'
    fi
    if [[ $EDGE_NODE_TYPE == 'arm64-macOS' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'macos' 'pkg' 'arm64'
    fi
    if [[ $EDGE_NODE_TYPE == 'ppc64le-RPM' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles $agentSoftwareFiles 'linux' 'rpm' 'ppc64le'
    fi
    # there are no packages to extract for edge-cluster, because that uses the agent docker image

    echo ""
}

# Get agent-install.sh from where it was installed by horizon-cli
function getAgentInstallScript () {
    local softwareFiles=$1
    local installDir   # where the file has been installed by horizon-cli
    if [[ $OSTYPE == darwin* ]]; then
        installDir='/usr/local/bin'
    else   # linux (deb or rpm)
        installDir='/usr/horizon/bin'
    fi
    local installFile="$installDir/agent-install.sh"
    if [[ ! -f $installFile ]]; then
        fatal 2 "$installFile does not exist"
    fi
    echo "Getting $installFile ..."
    cp "$installFile" .   # should already be executable
    chk $? "Getting $installFile"

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        putOneFileInCss agent-install.sh agent_files false $(getHznVersion)
        putOneFileInCss agent-install.sh agent_software_files-$(getHznVersion) true $(getHznVersion)
        addElementToArray $softwareFiles agent-install.sh
    fi
}

# Get agent-uninstall.sh from where it was install by horizon-cli
function getAgentUninstallScript () {
    local installDir   # where the file has been installed by horizon-cli
    if [[ $OSTYPE == darwin* ]]; then
        installDir='/usr/local/bin'
    else   # linux (deb or rpm)
        installDir='/usr/horizon/bin'
    fi
    local installFile="$installDir/agent-uninstall.sh"
    if [[ ! -f $installFile ]]; then
        fatal 2 "$installFile does not exist"
    fi
    echo "Getting $installFile ..."
    cp "$installFile" .   # should already be executable
    chk $? "Getting $installFile"
}

# Get deployment-template.yml, persistentClaim-template.yml and auto-upgrade-cronjob-template.yml from where it was install by horizon-cli
function getClusterDeployTemplates () {
    local installDir   # where the files have been installed by horizon-cli
    if [[ $OSTYPE == darwin* ]]; then
        installDir='/usr/local/share/horizon/cluster'
    else   # linux (deb or rpm)
        installDir='/usr/horizon/cluster'
    fi
    for f in deployment-template.yml persistentClaim-template.yml auto-upgrade-cronjob-template.yml; do
        local installFile="$installDir/$f"
        if [[ ! -f $installFile ]]; then
            fatal 2 "$installFile does not exist"
        fi
        echo "Getting $installFile ..."
        cp "$installFile" .
    done
}

# Get agent-uninstall.sh, deployment-template.yml, persistentClaim-template.yml and auto-upgrade-cronjob-template.yml and create tar file
function getEdgeClusterFiles() {

    local upgradeFiles=$1
    getAgentUninstallScript
    getClusterDeployTemplates

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        echo "Creating tar file of edge cluster files..."
        tar -zcf $EDGE_CLUSTER_TAR_FILE_NAME agent-uninstall.sh deployment-template.yml persistentClaim-template.yml auto-upgrade-cronjob-template.yml role.yml
        chk $? 'Creating tar file of edge cluster files'
        putOneFileInCss $EDGE_CLUSTER_TAR_FILE_NAME "agent_files" false  $(getHznVersion)
        putOneFileInCss $EDGE_CLUSTER_TAR_FILE_NAME "agent_software_files-$(getHznVersion)" true $(getHznVersion)
        addElementToArray $upgradeFiles $EDGE_CLUSTER_TAR_FILE_NAME
        rm $EDGE_CLUSTER_TAR_FILE_NAME
        chk $? "removing $EDGE_CLUSTER_TAR_FILE_NAME"
    fi
}

# Create a tar file of the gathered files for batch install
function createTarFile () {
    echo "Creating agentInstallFiles-$EDGE_NODE_TYPE.tar.gz file containing gathered files..."

    local files_to_compress
    if [[ $EDGE_NODE_TYPE == 'ALL' ]]; then
        files_to_compress="agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt amd64$AGENT_IMAGE_TAR_FILE arm64${AGENT_IMAGE_TAR_FILE} $AGENT_K8S_IMAGE_TAR_FILE $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE deployment-template.yml persistentClaim-template.yml auto-upgrade-cronjob-template.yml horizon*"
    elif [[ $EDGE_NODE_TYPE == "x86_64-Cluster" || $EDGE_NODE_TYPE == "ppc64le-Cluster" ]]; then
        files_to_compress="agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt $AGENT_K8S_IMAGE_TAR_FILE $AUTO_UPGRADE_CRONJOB_K8S_IMAGE_TAR_FILE deployment-template.yml persistentClaim-template.yml auto-upgrade-cronjob-template.yml"
    elif [[ "$EDGE_NODE_TYPE" == "macOS" ]]; then
        files_to_compress="agent-install.sh agent-install.cfg agent-install.crt horizon-cli* amd64${AGENT_IMAGE_TAR_FILE}"
    else   # linux device
        files_to_compress="agent-install.sh agent-install.cfg agent-install.crt horizon*"
    fi

    echo "tar'ing into agentInstallFiles-$EDGE_NODE_TYPE.tar.gz: $(ls $files_to_compress)"
    tar -czf agentInstallFiles-$EDGE_NODE_TYPE.tar.gz $(ls $files_to_compress)
    chk $? "creating agentInstallFiles-$EDGE_NODE_TYPE.tar.gz file."
    echo ""
}


# When they specify EDGE_NODE_TYPE=ALL we have to do the superset of all of the steps
all_main() {

    local __resultvar=$1
    upgradeManifest=$2

    local mymainresult=$upgradeManifest

    checkPrereqsAndInput

    if [[ -n $DIR ]]; then pushd $DIR; fi   # if they want the files somewhere else, make that our current dir

    cleanUpPreviousFiles

    local upgradeSoftwareFiles=()    # define this here since we need to capture device and cluster filenames and agent-install.sh

    getAgentK8sImageTarFile upgradeSoftwareFiles
    getAutoUpgradeCronjobK8sImageTarFile upgradeSoftwareFiles

    local upgradeConfigFiles=()    
    createAgentInstallConfig upgradeConfigFiles

    configFileLength=${#upgradeConfigFiles[@]}
    if [[ $configFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza mymainresult "${mymainresult}" "configurationUpgrade" "1.0.0" "${upgradeConfigFiles[@]}"
    fi

    local upgradeCertFiles=()    
    getClusterCert upgradeCertFiles
    certFileLength=${#upgradeCertFiles[@]}
    if [[ $certFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza mymainresult "${mymainresult}" "certificateUpgrade" "1.0.0" "${upgradeCertFiles[@]}"
    fi

    gatherHorizonPackageFiles upgradeSoftwareFiles

    getEdgeClusterFiles upgradeSoftwareFiles

    getAgentInstallScript upgradeSoftwareFiles
    softwareFileLength=${#upgradeSoftwareFiles[@]}
    if [[ $softwareFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza mymainresult "${mymainresult}" "softwareUpgrade" "${SOFTWARE_PACKAGE_VERSION}" "${upgradeSoftwareFiles[@]}"
    fi

    echo

    # Note: if they specified they wanted files in CSS, we did that as the files were created

    if [[ $CREATE_TAR_FILE == 'true' ]]; then
        createTarFile
    fi

    if [[ -n $DIR ]]; then popd; fi

    eval $__resultvar="'$mymainresult'"
}

cluster_main() {

    local __resultvar=$1
    upgradeManifest=$2

    local myclustermainresult=$upgradeManifest
    checkPrereqsAndInput

    if [[ -n $DIR ]]; then pushd $DIR; fi   # if they want the files somewhere else, make that our current dir

    cleanUpPreviousFiles

    local upgradeSoftwareFiles=()    # define this here since we need to capture device and cluster filenames and agent-install.sh

    getAgentK8sImageTarFile upgradeSoftwareFiles
    getAutoUpgradeCronjobK8sImageTarFile upgradeSoftwareFiles

    local upgradeConfigFiles=()    
    createAgentInstallConfig upgradeConfigFiles

    configFileLength=${#upgradeConfigFiles[@]}
    if [[ $configFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza myclustermainresult "${myclustermainresult}" "configurationUpgrade" "1.0.0" "${upgradeConfigFiles[@]}"
    fi

    local upgradeCertFiles=()    
    getClusterCert upgradeCertFiles
    certFileLength=${#upgradeCertFiles[@]}
    if [[ $certFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza myclustermainresult "${myclustermainresult}" "certificateUpgrade" "1.0.0" "${upgradeCertFiles[@]}"
    fi

    getEdgeClusterFiles upgradeSoftwareFiles

    getAgentInstallScript upgradeSoftwareFiles
    softwareFileLength=${#upgradeSoftwareFiles[@]}
    if [[ $softwareFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza myclustermainresult "${myclustermainresult}" "softwareUpgrade" "${SOFTWARE_PACKAGE_VERSION}" "${upgradeSoftwareFiles[@]}"
    fi

    echo

    # Note: if they specified they wanted files in CSS, we did that as the files were created

    if [[ $CREATE_TAR_FILE == 'true' ]]; then
        createTarFile
    fi

    if [[ -n $DIR ]]; then popd; fi
    eval $__resultvar="'$myclustermainresult'"
}

device_main() {

    local __resultvar=$1
    upgradeManifest=$2

    local mydevicemainresult=$upgradeManifest

    checkPrereqsAndInput

    if [[ -n $DIR ]]; then pushd $DIR; fi   # if they want the files somewhere else, make that our current dir

    cleanUpPreviousFiles

    local upgradeConfigFiles=()    
    createAgentInstallConfig upgradeConfigFiles

    configFileLength=${#upgradeConfigFiles[@]}
    if [[ $configFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza mydevicemainresult "${mydevicemainresult}" "configurationUpgrade" "1.0.0" "${upgradeConfigFiles[@]}"
    fi

    local upgradeCertFiles=()    
    getClusterCert upgradeCertFiles
    certFileLength=${#upgradeCertFiles[@]}
    if [[ $certFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza mydevicemainresult "${mydevicemainresult}" "certificateUpgrade" "1.0.0" "${upgradeCertFiles[@]}"
    fi

    local upgradeSoftwareFiles=()    # define this here since we need to capture device and cluster filenames and agent-install.sh
    gatherHorizonPackageFiles upgradeSoftwareFiles

    getAgentInstallScript upgradeSoftwareFiles

    softwareFileLength=${#upgradeSoftwareFiles[@]}
    if [[ $softwareFileLength -gt 0 ]]; then 
	    manifestAddTypeStanza mydevicemainresult "${mydevicemainresult}" "softwareUpgrade" "${SOFTWARE_PACKAGE_VERSION}" "${upgradeSoftwareFiles[@]}"
    fi
    echo

    # Note: if they specified they wanted files in CSS, we did that as the files were created

    if [[ $CREATE_TAR_FILE == 'true' ]]; then
        createTarFile
    fi

    if [[ -n $DIR ]]; then popd; fi
    eval $__resultvar="'$mydevicemainresult'"
}

# Publish a manifest for files pushed by this execution
function publishUpgradeManifest() {

    local upgradeManifest=$1 
    local version=$2

    local fileName=${MANIFEST_NAME}_$version
    echo "Generating upgrade manifest"

    echo "${upgradeManifest}" >  ${fileName}
    putOneFileInCss ${fileName} "agent_upgrade_manifests" true $version

    rm -f  ${fileName}
    chk $? "removing ${fileName}"

}

main() {

    # Manifest for upgrade policy
    upgradeManifest="{}"

    if [[ $EDGE_NODE_TYPE == 'ALL' ]]; then
	    all_main upgradeManifest "${upgradeManifest}"
    elif [[ $EDGE_NODE_TYPE == 'x86_64-Cluster' || $EDGE_NODE_TYPE == 'ppc64le-Cluster' ]]; then
	    cluster_main upgradeManifest "${upgradeManifest}"
    else
	    device_main upgradeManifest "${upgradeManifest}"
    fi

    # Publish manifest if artifacts were pushed to CSS which populated the upgradeManifest variable
    if [[ ! "${upgradeManifest}" == "{}" ]]; then
	    publishUpgradeManifest "${upgradeManifest}" "${SOFTWARE_PACKAGE_VERSION}"
        # add a 'total' object so that agbot knows how many agent files are there before updating AgentFileVersion object
        getAgentFileTotal "${SOFTWARE_PACKAGE_VERSION}"
    fi

    echo "edgeNodeFiles.sh completed successfully."
}

main



