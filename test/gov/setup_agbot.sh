#!/bin/bash
echo "setup_agbot.sh start"

export ARCH=${ARCH}

if [ -z $PREBUILT_DOCKER_REG_USER ]; then
    echo "You must specify the Docker Registry User ID"
    exit 1
elif [ -z $PREBUILT_DOCKER_REG_PW ]; then
    echo "You must specify the Docker Registry User Password/Token"
    exit 1
elif [ -z $PREBUILT_DOCKER_REG_URL ]; then
    echo "You must specify the Docker Registry URL"
    exit 1
fi

echo "Docker Registry login"
echo ${PREBUILT_DOCKER_REG_PW} | docker login -u=${PREBUILT_DOCKER_REG_USER} --password-stdin ${PREBUILT_DOCKER_REG_URL}

echo "Pulling anax_k8s and ESS from Docker Registry..."
docker pull ${PREBUILT_DOCKER_REG_URL}/${ARCH}_anax_k8s:${PREBUILT_ANAX_VERSION}
docker pull ${PREBUILT_DOCKER_REG_URL}/${ARCH}_edge-sync-service:${PREBUILT_ESS_VERSION}

echo "Tagging ESS to openhorizon/${ARCH}_edge-sync-service:latest"
docker tag ${PREBUILT_DOCKER_REG_URL}/${ARCH}_edge-sync-service:${PREBUILT_ESS_VERSION} openhorizon/${ARCH}_edge-sync-service:testing

echo "Creating (but not starting/running) anax container..."
id=$(docker create ${PREBUILT_DOCKER_REG_URL}/${ARCH}_anax_k8s:${PREBUILT_ANAX_VERSION})

echo "Doing container copy out of binaries from anax container: $id"
docker cp $id:/usr/bin/hzn $ANAX_SOURCE/cli/.
docker cp $id:/usr/horizon/bin/anax $ANAX_SOURCE/.

echo "Removing anax container"
docker rm -v ${id}

echo "setup_agbot.sh finished"
