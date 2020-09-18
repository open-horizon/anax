#!/bin/bash

# Dependencies location
SHUNIT_PATH="."

# Commond expected variables

NODE_STATE_UNCONFIGURED="unconfigured"
NODE_STATE_CONFIGURED="configured"

# path that agent install shell runs under
AGENT_PATH="."
TEST_PATH="test"
CONFIG_DEFAULT="agent-install.cfg"

CERT_DEFAULT="agent-install.crt"

NODE_POLICY="node_policy.json"

PATTERN="IBM/pattern-ibm.helloworld"

AGENT_INSTALL_TIMEOUT=45

SKIP_ENV_TEST=FALSE

# Common functions shared by test cases
prepareConfig(){
    if [ -z "$1" ]; then
        echo "Need to specify a config file name, exiting"
        exit 1
    fi
    cp config/${1} ${AGENT_PATH}/${CONFIG_DEFAULT}
}

removeConfig(){
    echo "Removing ${1}..."
    if [ -z "$1" ]; then
        echo "Need to specify a config file name, exiting"
        exit 1
    fi
    rm -f "$1"
}

showConfig(){
    if [ -z "$1" ]; then
        echo "Need to specify a config file name, exiting"
        exit 1
    fi
    echo "==========================="
    echo "Current config is:"
    cat "$1"
    echo "==========================="
}

prepareCert(){
    if [ -z "$1" ]; then
        echo "Need to specify a cert file name, exiting"
        exit 1
    fi

    cp config/${1} ${AGENT_PATH}/${CERT_DEFAULT}
}

removeCert(){
    echo "Removing ${1}..."
    if [ -z "$1" ]; then
        echo "Need to specify a cert file name, exiting"
        exit 1
    fi
    rm -f "$1"
}

prepareDependencies(){
    echo "Cloning shunit2..."
    git clone https://github.com/kward/shunit2/ "${SHUNIT_PATH}/shunit2"
}

cleanupDependencies(){
    echo "Removing shunit2..."
    rm -rf "${SHUNIT_PATH}/shunit2"
}


uninstallAgent() {
    # remove the cli if it's installed
    if command -v hzn >/dev/null 2>&1 ; then
        hzn unregister -rfD
        sudo apt-get purge --auto-remove horizon horizon-cli -y
    fi

    # cleanup for config and node policies
    if [[ -f "${AGENT_PATH}/${CONFIG_DEFAULT}" ]] ; then
        rm -f "${AGENT_PATH}/${CONFIG_DEFAULT}"
    fi

    if [[ -f "${AGENT_PATH}/${NODE_POLICY}" ]] ; then
        rm -f "${AGENT_PATH}/${NODE_POLICY}"
    fi
}

# Downloads agent installable packages to a node
getEdgePackages() {
    if command -v curl >/dev/null 2>&1; then
		echo "curl found"
	else
        echo "curl not found, installing it as it's reqired to proceed..." 
        apt install -y curl
	fi
    HORIZON_VERSION=$1;
    #if [ -z $HORIZON_VERSION ]; then
    #    export CUR_VERSION=$(curl -s https://raw.githubusercontent.com/open-horizon/horizon-deb-packager/master/VERSION);
    #    HORIZON_VERSION=$CUR_VERSION;
    #    if [ -z "$CUR_VERSION" ]; then
    #        echo "Missing version from the github, exiting...";
    #        return 1;
    #    fi;
    #fi;
    URL=https://na.artifactory.swg-devops.com/artifactory/hyc-edge-team-nightly-debian-local;
    if [ -z $2 ]; then
        PACKAGE_ROOT_DIR=".";
    else
        PACKAGE_ROOT_DIR=${2};
    fi;
    PLATFORM=linux;
    OS=raspbian;
    DISTRO=stretch;
    echo "Downloading version ${HORIZON_VERSION} packages to ${PACKAGE_ROOT_DIR}..."
    for ARCH in armhf;
    do
        FULL_DIR=$PACKAGE_ROOT_DIR;
        mkdir -p $FULL_DIR;
        ( cd $FULL_DIR;
        curl -sSLO -u "$ARTIFACTORY_EMAIL:$ARTIFACTORY_APIKEY" $URL/pool/horizon-cli_${HORIZON_VERSION}_${ARCH}.deb );
        ( cd $FULL_DIR;
        curl -sSLO -u "$ARTIFACTORY_EMAIL:$ARTIFACTORY_APIKEY" $URL/pool/horizon_${HORIZON_VERSION}_${ARCH}.deb );
    done;
    OS=ubuntu;
    for DISTRO in bionic xenial;
    do
        for ARCH in amd64 arm64 armhf ppc64el;
        do
            FULL_DIR=$PACKAGE_ROOT_DIR;
            mkdir -p $FULL_DIR;
            ( cd $FULL_DIR;
            curl -sSLO -u "$ARTIFACTORY_EMAIL:$ARTIFACTORY_APIKEY" $URL/pool/horizon-cli_${HORIZON_VERSION}_${ARCH}.deb );
            ( cd $FULL_DIR;
            curl -sSLO -u "$ARTIFACTORY_EMAIL:$ARTIFACTORY_APIKEY" $URL/pool/horizon_${HORIZON_VERSION}_${ARCH}.deb );
        done;
    done;
    PLATFORM=macos;
    FULL_DIR=$PACKAGE_ROOT_DIR;
    mkdir -p $FULL_DIR;
    ( cd $FULL_DIR;
    curl -sSLO -u "$ARTIFACTORY_EMAIL:$ARTIFACTORY_APIKEY" $URL/pool/horizon-cli_${HORIZON_VERSION}_${ARCH}.deb );
}

preparePackages() {
    cp -r "${1}/linux" "${2}"
}

removeEdgePackages() {
    echo "Removing packages..."
    PACKAGE_ROOT_DIR="$1";
    rm -rf "${PACKAGE_ROOT_DIR}"
}

removeAgentDependencies() {
    sudo apt-get purge --auto-remove docker-ce curl jq -y
}

installAgentDependencies() {
    sudo apt-get install curl jq -y
}

removeVariableFromConfig() {
    sudo sed -i "/${1}/d" "$2"
}

getOS() {
    if [[ "$OSTYPE" == "linux"* ]]; then
        OS="linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
    else
        OS="unknown"
    fi
}

getDistro(){
    if [ -f /etc/os-release ]; then
            . /etc/os-release
            DISTRO=$ID
            VER=$VERSION_ID
            CODENAME=$VERSION_CODENAME
    elif type lsb_release >/dev/null 2>&1; then
            DISTRO=$(lsb_release -si)
            VER=$(lsb_release -sr)
            CODENAME=$(lsb_release -sc)
    elif [ -f /etc/lsb-release ]; then
            . /etc/lsb-release
            DISTRO=$DISTRIB_ID
            VER=$DISTRIB_RELEASE
            CODENAME=$DISTRIB_CODENAME
    else
            echo "Cannot detect Linux version, exiting..."
            exit 1
    fi

    # Raspbian has a codename embedded in a version
    if [[ "$DISTRO" == "raspbian" ]]; then
        CODENAME=$(echo ${VERSION} | sed -e 's/.*(\(.*\))/\1/')
    fi
}

getArch(){
    # detecting architecture
    uname="$(uname -m)"
    if [[ "$uname" =~ "aarch64" ]]; then
        ARCH="arm64"
    elif [[ "$uname" =~ "arm" ]]; then
        ARCH="armhf"
    elif [[ "$uname" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$uname" == "ppc64le" ]]; then
        ARCH="ppc64el"
    else
        (>&2 echo "Unknown architecture $uname")
        exit 1
    fi
}

# Checks if file or directory exists
function checkExist(){
    case $1 in
	f) if ! [[ -f "$2" ]] ; then
			echo "${3} file ${2} doesn't exist"
		    exit 1
		fi
	;;
	d) if ! [[ -d "$2" ]] ; then
			echo "${3} directory ${2} doesn't exist"
	        exit 1
		fi
    ;;
    o) if ! [[ -f "$2" ]] ; then
			echo "${3} file ${2} doesn't exist"
		    echo "Secondary config/switch files were not found, environment switch test disabled."
            return 1
		fi
	;;
	*) echo "not supported"
        exit 1
	;;
	esac

}

checkConfig(){
    checkExist d "config" "Configuration directory"
    checkExist f "config/${CONFIG_DEFAULT}" "Primary configuration"
    checkExist f "config/${CERT_DEFAULT}" "Primary configuration"
    checkExist f "config/${NODE_POLICY}" "Node policy"
    checkExist d "config/switch" "Secondary configuration"
    checkExist o "config/switch/${CONFIG_DEFAULT}" "Secondary configuration"
    checkExist o "config/switch/${CERT_DEFAULT}" "Secondary configuration"
}

checkSecondaryConfig() {
    if ! checkExist o "config/switch/${CONFIG_DEFAULT}" "Secondary configuration" || ! checkExist o "config/switch/${CERT_DEFAULT}" "Secondary configuration" ; then
    return 1
    fi
}

startCount(){
    TEST_START=$(date +%s)
}

stopCount(){
    TEST_END=$(date +%s)
    TEST_ELAPSED=$(( TEST_END - TEST_START ))
    echo "Test case #${TEST_RUNNING} ran for $(( ${TEST_ELAPSED} / 3600 ))h $(( (${TEST_ELAPSED} / 60) % 60 ))m $(( ${TEST_ELAPSED} % 60 ))s"
}
