#!/bin/bash

# This script gathers the necessary information and files to install the Horizon agent and register an edge node

function getHznVersion() { local hzn_version=$(hzn version | grep "^Horizon CLI"); echo ${hzn_version##* }; }

# Global constants
SUPPORTED_NODE_TYPES='ARM32-Deb ARM64-Deb AMD64-Deb x86_64-RPM x86_64-macOS x86_64-Cluster ppc64le-RPM ppc64le-Cluster ALL'
EDGE_CLUSTER_TAR_FILE_NAME='horizon-agent-edge-cluster-files.tar.gz'
AGENT_IMAGE_TAR_FILE='amd64_anax.tar.gz'
AGENT_IMAGE='amd64_anax'
AGENT_K8S_IMAGE_TAR_FILE='amd64_anax_k8s.tar.gz'
AGENT_K8S_IMAGE='amd64_anax_k8s'
AGENT_IMAGE_TAG=$(getHznVersion)
DEFAULT_PULL_REGISTRY='docker.io/openhorizon'
ALREADY_LOGGED_INTO_REGISTRY='false'

# Environment variables and their defaults
PACKAGE_NAME=${PACKAGE_NAME:-horizon-edge-packages-4.2.0}
AGENT_NAMESPACE=${AGENT_NAMESPACE:-openhorizon-agent}
# EDGE_CLUSTER_STORAGE_CLASS   # this can also be specified, that default is the empty string

function scriptUsage() {
    cat << EOF

Usage: ./edgeNodeFiles.sh <edge-node-type> [-o <hzn-org-id>] [-c] [-f <directory>] [-t] [-p <package_name>] [-s <edge-cluster-storage-class>] [-i <agent-image-tag>] [-m <agent-namespace>] [-r <registry/repo-path>] [-g <tag>] [-b]

Parameters:
  Required:
    <edge-node-type>    The type of edge node planned for agent install and registration. Valid values: $SUPPORTED_NODE_TYPES

  Optional:
    -o <hzn-org-id>     The exchange org id that should be used for all edge nodes. If there will be multiple exchange orgs, do not set this.
    -c    Put the gathered files into the Cloud Sync Service (MMS). On the edge nodes agent-install.sh can pull the files from there.
    -f <directory>     The directory to put the gathered files in. Default is current directory.
    -t          Create agentInstallFiles-<edge-node-type>.tar.gz file containing gathered files. If this flag is not set, the gathered files will be placed in the current directory.
    -p <package_name>   The base name of the horizon content tar file (can include a path). Default is $PACKAGE_NAME, which means it will look for $PACKAGE_NAME.tar.gz and expects a standardized directory structure of $PACKAGE_NAME/<OS>/<pkg-type>/<arch>
    -s <edge-cluster-storage-class>   Default storage class to be used for all the edge clusters. If not specified, can be specified when running agnet-install.sh. Only applies to node types of <arch>-Cluster.
    -m <agent-namespace>   The edge cluster namespace that the agent will be installed into. Default is $AGENT_NAMESPACE. Only applies to node types of <arch>-Cluster.
    -r <registry/repo-path>     The agent images (both device and cluster) will be pulled from the specified authenticated docker registry. If this flag is set, you must also export the username and password for the registry in REGISTRY_USERNAME and REGISTRY_PASSWORD. Default is $DEFAULT_PULL_REGISTRY.
    -g <tag>    Overwrite the default agent image tag. Default is $AGENT_IMAGE_TAG.
    -b          Get the agent images from the horizon content tar file.

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

# Remove files from previous run, so we know there won't be (for example) multiple versions of the horizon pkgs in the dir
function cleanUpPreviousFiles() {
    echo "Removing any generated files from previous run..."
    rm -f agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt "$AGENT_IMAGE_TAR_FILE" "$AGENT_K8S_IMAGE_TAR_FILE" deployment-template.yml persistentClaim-template.yml horizon*.{deb,rpm,pkg,crt}
    chk $? "removing previous files in $PWD"
    echo
}

# Pull the edge cluster agent image from the package tar file or from a docker registry
function getAgentK8sImageTarFile() {
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
        putOneFileInCss "$AGENT_K8S_IMAGE_TAR_FILE" $version
    fi
    echo
}

# Put 1 file into CSS in the IBM org as a public object.
function putOneFileInCss() {
    local filename=${1:?} version=$2
    local resourcename=$(oc get eamhub --no-headers |awk '{printf $1}')

    # First get exchange root creds, if necessary
    if [[ -z $HZN_EXCHANGE_USER_AUTH ]]; then
        echo "Getting exchange root credentials to use to publish to CSS..."
        export HZN_EXCHANGE_USER_AUTH="root/root:$(oc get secret $resourcename-auth -o jsonpath="{.data.exchange-root-pass}" | base64 --decode)"
        chk $? 'getting exchange root creds'
    fi

    # Note: when https://github.com/open-horizon/anax/issues/2077 is fixed, we can send the metadata into hzn via stdin
    cat << EOF > ${filename}-meta.json
{
  "objectID": "$filename",
  "objectType": "agent_files",
  "destinationOrgID": "IBM",
  "version": "$version",
  "public": true
}
EOF
    echo "Publishing $filename in CSS as a public object in the IBM org..."
    hzn mms -o IBM object publish -m "${filename}-meta.json" -f $filename
    local rc=$?
    rm -f "${filename}-meta.json"   # clean up metadata file
    chk $rc "publishing $filename in CSS as a public object. Ensure HZN_EXCHANGE_USER_AUTH is set to credentials that can publish to the IBM org."
}

# With the information from the previous functions, create agent-install.cfg
function createAgentInstallConfig () {
    echo "Creating agent-install.cfg file..."
    HUB_CERT_PATH="agent-install.crt"

    if [[ $EDGE_NODE_TYPE == 'x86_64-Cluster' || $EDGE_NODE_TYPE == 'ppc64le-Cluster' || $EDGE_NODE_TYPE == 'ALL' ]]; then   # if they chose ALL, the cluster agent-install.cfg is a superset
        cat << EndOfContent > agent-install.cfg
HZN_EXCHANGE_URL=$CLUSTER_URL/edge-exchange/v1
HZN_FSS_CSSURL=$CLUSTER_URL/edge-css/
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
EndOfContent

        if [[ -n $ORG_ID ]]; then
            echo "HZN_ORG_ID=$ORG_ID" >> agent-install.cfg
        fi
    fi
    chk $? 'creating agent-install.cfg file'

    echo "agent-install.cfg file created with content: "
    cat agent-install.cfg

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        putOneFileInCss agent-install.cfg
    fi
    echo ""
}

# Get the management hub self-signed certificate
function getClusterCert () {
    echo "Getting the management hub self-signed certificate agent-install.crt..."
    oc get secret management-ingress-ibmcloud-cluster-ca-cert -o jsonpath="{.data['ca\.crt']}" | base64 --decode > agent-install.crt
    chk $? 'getting the management hub self-signed certificate'

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        putOneFileInCss agent-install.crt
    fi
    echo ""
}

# Create 1 horizon pkg tar file, put it into CSS, and then remove the tar file
function putHorizonPkgsInCss() {
    local opsys=$1 pkgtype=$2 arch=$3

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
        pkgWildcard="horizon-cli.crt horizon-cli-*.$pkgtype"
        tarFile="horizon-agent-${opsys}-${pkgtype}-$arch.tar.gz"
        pkgVersion=$(ls horizon-cli-*.$pkgtype)
        pkgVersion=${pkgVersion#horizon-cli-}
        pkgVersion=${pkgVersion%%.$pkgtype}
fi

    # Create the pkg tar file
    tar -zcf "$tarFile" $pkgWildcard   # it is important to NOT quote $pkgWildcard so the wildcard gets expanded
    chk $? "creating $tarFile"

    # Put the tar file in CSS in the IBM org as a public object
    putOneFileInCss $tarFile $pkgVersion

    # Remove the tar file (it was only needed to put into CSS)
    rm -f "$tarFile"
    chk $? "removing $tarFile"
}

# Get 1 type of horizon packages
function getHorizonPackageFiles() {
    local opsys=$1 pkgtype=$2 arch=$3
    local pkgBaseName=${PACKAGE_NAME##*/}   # inside the tar file, the paths start with the base name
    echo "Extracting $pkgBaseName/$opsys/$pkgtype/$arch/* from $PACKAGE_NAME.tar.gz ..."
    tar --strip-components 4 -zxf $PACKAGE_NAME.tar.gz $pkgBaseName/$opsys/$pkgtype/$arch
    chk $? "extracting $pkgBaseName/$opsys/$pkgtype/$arch/* from $PACKAGE_NAME.tar.gz"

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        putHorizonPkgsInCss $opsys $pkgtype $arch
    fi

    if [[ $opsys == 'macos' ]]; then   #future: do this for all amd64/x86_64
        if [[ $AGENT_IMAGES_FROM_TAR == 'true' ]]; then
            tar --strip-components 2 -zxf $PACKAGE_NAME.tar.gz "$pkgBaseName/docker/$AGENT_IMAGE_TAR_FILE"
            chk $? "extracting $pkgBaseName/docker/$AGENT_IMAGE_TAR_FILE from $PACKAGE_NAME.tar.gz"
        else
            if [[ ($PULL_REGISTRY != $DEFAULT_PULL_REGISTRY) && ($ALREADY_LOGGED_INTO_REGISTRY == 'false') ]]; then
                echo "Logging into $PULL_REGISTRY ..."
                echo "$REGISTRY_PASSWORD" | docker login -u $REGISTRY_USERNAME --password-stdin $PULL_REGISTRY
                chk $? "logging into edge cluster's registry: $PULL_REGISTRY"
                ALREADY_LOGGED_INTO_REGISTRY='true'
            fi

            echo "Pulling $PULL_REGISTRY/$AGENT_IMAGE:$AGENT_IMAGE_TAG ..."
            docker pull $PULL_REGISTRY/$AGENT_IMAGE:$AGENT_IMAGE_TAG
            chk $? "pulling $PULL_REGISTRY/$AGENT_IMAGE:$AGENT_IMAGE_TAG"

            echo "Saving $AGENT_IMAGE:$AGENT_IMAGE_TAG to $AGENT_IMAGE_TAR_FILE ..."
            docker save $PULL_REGISTRY/$AGENT_IMAGE:$AGENT_IMAGE_TAG | gzip > $AGENT_IMAGE_TAR_FILE
            chk $? "saving $PULL_REGISTRY/$AGENT_IMAGE:$AGENT_IMAGE_TAG"
        fi

        if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
            echo "Extracting version from $AGENT_IMAGE_TAR_FILE ..."
            local version=$(tar -zxOf "$AGENT_IMAGE_TAR_FILE" manifest.json | jq -r '.[0].RepoTags[0]')   # this gets the full path
            version=${version##*:}   # strip the path and image name from the front
            echo "Version/tag of $AGENT_IMAGE_TAR_FILE is: $version"
            putOneFileInCss "$AGENT_IMAGE_TAR_FILE" $version
        fi
    fi
}

# Get all of the the horizon packages that they specified
function gatherHorizonPackageFiles() {
    local opsys pkgtype arch
    if [[ $EDGE_NODE_TYPE == 'ARM32-Deb' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles 'linux' 'deb' 'armhf'
    fi
    if [[ $EDGE_NODE_TYPE == 'ARM64-Deb' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles 'linux' 'deb' 'arm64'
    fi
    if [[ $EDGE_NODE_TYPE == 'AMD64-Deb' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles 'linux' 'deb' 'amd64'
    fi
    if [[ $EDGE_NODE_TYPE == 'x86_64-RPM' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles 'linux' 'rpm' 'x86_64'
    fi
    if [[ $EDGE_NODE_TYPE == 'x86_64-macOS' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles 'macos' 'pkg' 'x86_64'
    fi
    if [[ $EDGE_NODE_TYPE == 'ppc64le-RPM' || $EDGE_NODE_TYPE == 'ALL' ]]; then
        getHorizonPackageFiles 'linux' 'rpm' 'ppc64le'
    fi
    # there are no packages to extract for edge-cluster, because that uses the agent docker image

    echo ""
}

# Get agent-install.sh from where it was installed by horizon-cli
function getAgentInstallScript () {
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
        putOneFileInCss agent-install.sh $(getHznVersion)
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

# Get deployment-template.yml and persistentClaim-template.yml from where it was install by horizon-cli
function getClusterDeployTemplates () {
    local installDir   # where the files have been installed by horizon-cli
    if [[ $OSTYPE == darwin* ]]; then
        installDir='/usr/local/share/horizon/cluster'
    else   # linux (deb or rpm)
        installDir='/usr/horizon/cluster'
    fi
    for f in deployment-template.yml persistentClaim-template.yml; do
        local installFile="$installDir/$f"
        if [[ ! -f $installFile ]]; then
            fatal 2 "$installFile does not exist"
        fi
        echo "Getting $installFile ..."
        cp "$installFile" .
    done
}

# Get agent-uninstall.sh, deployment-template.yml, and persistentClaim-template.yml and create tar file
function getEdgeClusterFiles() {
    getAgentUninstallScript
    getClusterDeployTemplates

    if [[ $PUT_FILES_IN_CSS == 'true' ]]; then
        echo "Creating tar file of edge cluster files..."
        tar -zcf $EDGE_CLUSTER_TAR_FILE_NAME agent-uninstall.sh deployment-template.yml persistentClaim-template.yml
        chk $? 'Creating tar file of edge cluster files'
        putOneFileInCss $EDGE_CLUSTER_TAR_FILE_NAME $(getHznVersion)
        rm $EDGE_CLUSTER_TAR_FILE_NAME
        chk $? "removing $EDGE_CLUSTER_TAR_FILE_NAME"
    fi
}

# Create a tar file of the gathered files for batch install
function createTarFile () {
    echo "Creating agentInstallFiles-$EDGE_NODE_TYPE.tar.gz file containing gathered files..."

    local files_to_compress
    if [[ $EDGE_NODE_TYPE == 'ALL' ]]; then
        files_to_compress="agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt $AGENT_IMAGE_TAR_FILE $AGENT_K8S_IMAGE_TAR_FILE deployment-template.yml persistentClaim-template.yml horizon*"
    elif [[ $EDGE_NODE_TYPE == "x86_64-Cluster" || $EDGE_NODE_TYPE == "ppc64le-Cluster" ]]; then
        files_to_compress="agent-install.sh agent-uninstall.sh agent-install.cfg agent-install.crt $AGENT_K8S_IMAGE_TAR_FILE deployment-template.yml persistentClaim-template.yml"
    elif [[ "$EDGE_NODE_TYPE" == "macOS" ]]; then
        files_to_compress="agent-install.sh agent-install.cfg agent-install.crt horizon-cli* $AGENT_IMAGE_TAR_FILE"
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
    checkPrereqsAndInput

    if [[ -n $DIR ]]; then pushd $DIR; fi   # if they want the files somewhere else, make that our current dir

    cleanUpPreviousFiles

    getAgentK8sImageTarFile

    createAgentInstallConfig

    getClusterCert

    gatherHorizonPackageFiles

    getEdgeClusterFiles

    getAgentInstallScript
    echo

    # Note: if they specified they wanted files in CSS, we did that as the files were created

    if [[ $CREATE_TAR_FILE == 'true' ]]; then
        createTarFile
    fi

    if [[ -n $DIR ]]; then popd; fi
}

cluster_main() {
    checkPrereqsAndInput

    if [[ -n $DIR ]]; then pushd $DIR; fi   # if they want the files somewhere else, make that our current dir

    cleanUpPreviousFiles

    getAgentK8sImageTarFile

    createAgentInstallConfig

    getClusterCert

    getEdgeClusterFiles

    getAgentInstallScript
    echo

    # Note: if they specified they wanted files in CSS, we did that as the files were created

    if [[ $CREATE_TAR_FILE == 'true' ]]; then
        createTarFile
    fi

    if [[ -n $DIR ]]; then popd; fi
}

device_main() {
    checkPrereqsAndInput

    if [[ -n $DIR ]]; then pushd $DIR; fi   # if they want the files somewhere else, make that our current dir

    cleanUpPreviousFiles

    createAgentInstallConfig

    getClusterCert

    gatherHorizonPackageFiles

    getAgentInstallScript
    echo

    # Note: if they specified they wanted files in CSS, we did that as the files were created

    if [[ $CREATE_TAR_FILE == 'true' ]]; then
        createTarFile
    fi

    if [[ -n $DIR ]]; then popd; fi
}

main() {
    if [[ $EDGE_NODE_TYPE == 'ALL' ]]; then
        all_main
    elif [[ $EDGE_NODE_TYPE == 'x86_64-Cluster' || $EDGE_NODE_TYPE == 'ppc64le-Cluster' ]]; then
        cluster_main
    else
        device_main
    fi
    echo "edgeNodeFiles.sh completed successfully."
}

main


