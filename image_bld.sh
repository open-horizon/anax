#! /bin/bash

# This script is uesd to build the different images
COMP_NAME=$1
IMAGE_NAME=$2
IMAGE_LABELS=$3
DOCKER_MAYBE_CACHE=$4
IMAGE_STG=$5

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

# Special requirements for agbot, anax-k8s ,css-docker, ess-docker image build
if [[ ${arch} != "amd64" && ${COMP_NAME} == "agbot" ]]; then 
	echo "agbot image building just support on amd64 !!!"
	exit
fi
if [[ ${arch} != "amd64" && ${COMP_NAME} == "css-docker" ]]; then  
        echo "css-docker image building just support on amd64 !!!"
        exit
fi
if [[ ${arch} == "s390x" && ${COMP_NAME} == "anax-k8s" ]]; then
        echo "anax k8s image building just support on amd64 ,ppc64el and arm64 !!!"
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
	echo "Building the anax docker image is not supported on ${arch}"
fi
