#! /bin/bash

#This scripts is used to push the images to repository

IMAGE_NAME=$1
IMAGE_PROD=$2
IMAGE_LATEST=$3

docker pull ${IMAGE}
docker tag ${IMAGE} ${IMAGE_PROD}
docker push ${IMAGE_PROD}
docker tag ${IMAGE} ${IMAGE_LATEST}
docker push ${IMAGE_LATEST}
