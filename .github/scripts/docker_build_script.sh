#!/bin/bash

# Stops the Makefile from pushing the images to dockerhub so we can control when the 'testing' tag is pushed
export IMAGE_OVERRIDE="true"

# Makes and pushes arch_cloud-sync-service and arch_edge-sync-service images
if [[ ${arch} == 'amd64' || ${arch} == 'ppc64el' || ${arch} == 'arm64' ]]; then
    make fss-package
fi

# Makes and pushes amd64_agbot image
if [[ ${arch} == 'amd64' ]]; then
    make agbot-package
fi

# Specify if we should use buildx for multiarch, github runner is amd64 so we only need this for arm and ppc
if [[ ${arch} == 'arm64' || ${arch} == 'ppc64el' ]]; then
    export USE_DOCKER_BUILDX=true
fi

make anax-package                       # Makes and pushes arch_anax
make anax-k8s-package                   # Makes and pushes arch_anax_k8s
make auto-upgrade-cronjob-k8s-package   # Makes and pushes arch_auto-upgrade-cronjob-k8s

# Outputs created docker images for viewing
echo "**************"
docker images
echo "**************" 