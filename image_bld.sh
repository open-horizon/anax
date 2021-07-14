#! /bin/bash

usage() {
    exitCode=${1:-0}
    cat << EndOfMessage
Usage: ${0##*/} [-c <component>] [-n <image name>] [-l <image labels>] [-d <docker maybe cache>] [-s <image stg>] [-a <arch>]

Building the different component docker images on its supported architectures.

 Flags:
  -c    Component name,such as anax,agbot,anax-k8s,css-docker and ess-docker
  -n    Docker image name
  -l    Docker image lables
  -d    Docker build cache option
  -s    Dokcer image stg
  -a    Architecture to build docker image
  -h    Usage
EndOfMessage
    exit $exitCode
}

#Print the usage
if [ $# -eq 0 ] || [ $# -eq 1 -a "A$1" = "A-h" ]; then
        usage
fi

while getopts 'c:n:l:d:s:a:h:' OPT; do
    case $OPT in
        c)
            COMP_NAME="$OPTARG";;
        n)
            IMAGE_NAME="$OPTARG";;
        l)
            IMAGE_LABELS="$OPTARG";;
        d)
            DOCKER_MAYBE_CACHE="$OPTARG";;
        s)
            IMAGE_STG="$OPTARG";;
        a)
            arch="$OPTARG";;
        h)
            usage;;
        ?)
            echo "Usage: `basename $0` [options]"
    esac
done
shift $(($OPTIND - 1))

# Check the required vars if is defined
if [ "A${COMP_NAME}" == "A" ];then
        echo "Which image you want to build,such as anax, agbot or anax-k8s"
fi
if [ "A${IMAGE_NAME}" == "A" ];then
        echo "Please define the image name."
fi
if [ "A${IMAGE_LABELS}" == "A" ];then
        echo "Please define the image labels."
fi
if [ "A${DOCKER_MAYBE_CACHE}" == "A" ];then
        echo "Please define docker cache option."
fi
if [ "A${IMAGE_STG}" == "A" ];then
        echo "Please define the image tag."
fi
if [ "A${arch}" == "A" ];then
        echo "Please define the image building architecture."
fi

# Special requirements for agbot, anax-k8s ,css-docker, ess-docker image build
if [[ ${arch} != "amd64" && ${COMP_NAME} == "agbot" ]]; then 
	echo "Building the agbot image is only supported on amd64 architectures."
	exit
fi
if [[ ${arch} != "amd64" && ${COMP_NAME} == "css-docker" ]]; then  
        echo "Building the css-docker image is only supported on amd64 architectures."
        exit
fi
if [[ ${arch} == "s390x" && ${COMP_NAME} == "anax-k8s" ]]; then
        echo "Building the anax k8s image is only supported on amd64 ,ppc64el and arm64 architectures."
        exit
fi

if [ ${COMP_NAME} == "anax-k8s" ]; then
	export WORK_DIR=${ANAX_K8S_CONTAINER_DIR}
elif [ ${COMP_NAME} == "css-docker" ]; then
	export WORK_DIR=${CSS_CONTAINER_DIR}
elif [ ${COMP_NAME} == "ess-docker" ]; then
	export WORK_DIR=${ESS_CONTAINER_DIR}
else
	export WORK_DIR=${ANAX_CONTAINER_DIR}
fi

# Normal image build process
if [[ ${arch} == "amd64" || ${arch} == "ppc64el" || ${arch} == "s390x" || ${arch} == "arm64" ]]; then
	
	if [[ ${COMP_NAME} == "css-docker" || ${COMP_NAME} == "ess-docker" ]]; then
		cp -f $(LICENSE_FILE) $(WORK_DIR)
	else
		rm -rf ${WORK_DIR}/anax
		rm -rf ${WORK_DIR}/hzn
		cp ${EXECUTABLE} ${WORK_DIR}
		cp ${CLI_EXECUTABLE} ${WORK_DIR}
		cp -f ${LICENSE_FILE} ${WORK_DIR}
	fi
	cd ${WORK_DIR} && docker build ${DOCKER_MAYBE_CACHE} ${IMAGE_LABELS} -t ${IMAGE_NAME} -f Dockerfile.ubi.${arch} . && \
	docker tag ${IMAGE_NAME} ${IMAGE_STG}
else 
	echo "Building the ${IMAGE_NAME} docker image is not supported on ${arch}"
fi
