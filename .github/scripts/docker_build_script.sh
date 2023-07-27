#!/bin/bash

# Makes and pushes arch_cloud-sync-service and arch_edge-sync-service images
if [[ ${arch} == 'amd64' || ${arch} == 'ppc64el' || ${arch} == 'arm64' || ${arch} == 's390x' ]]; then
    make ess-docker-image
    make css-docker-image
fi

# Makes and pushes amd64_agbot image
if [[ ${arch} == 'amd64' ]]; then
    make agbot-image
fi

# Specify if we should use buildx for multiarch, github runner is amd64 so we only need this for arm and ppc
if [[ ${arch} == 'arm64' || ${arch} == 'ppc64el' || ${arch} == 's390x' ]]; then
    export USE_DOCKER_BUILDX=true
fi

make anax-image                         # Makes and pushes arch_anax
make anax-k8s-image                     # Makes and pushes arch_anax_k8s
make auto-upgrade-cronjob-k8s-image     # Makes and pushes arch_auto-upgrade-cronjob-k8s

# Outputs created docker images for viewing
echo "**************"
docker images
echo "**************" 