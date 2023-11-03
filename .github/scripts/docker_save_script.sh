#!/bin/bash

if [[ -z "$ANAX_IMAGE_VERSION" ]]; then
    echo "::error file=docker_save_script.sh::Anax Image Version is unset, check the 'Configure Version Variables' step"
    exit 1
fi
if [[ -z "$CSS_IMAGE_VERSION" ]]; then
    echo "::error file=docker_save_script.sh::CSS Image Version is unset, check the 'Configure Version Variables' step"
    exit 1
fi
if [[ -z "$ESS_IMAGE_VERSION" ]]; then
    echo "::error file=docker_save_script.sh::ESS Image Version is unset, check the 'Configure Version Variables' step"
    exit 1
fi

# Names of the images created for each architecture
if [[ ${arch} == 'amd64' ]]; then
    images=('amd64_agbot' 'amd64_anax' 'amd64_anax_k8s' 'amd64_auto-upgrade-cronjob_k8s' 'amd64_cloud-sync-service' 'amd64_edge-sync-service')
elif [[ ${arch} == 'ppc64el' ]]; then
    images=('ppc64el_anax' 'ppc64el_anax_k8s' 'ppc64el_auto-upgrade-cronjob_k8s' 'ppc64el_edge-sync-service')
elif [[ ${arch} == 'arm64' ]]; then
    images=('arm64_anax' 'arm64_anax_k8s' 'arm64_auto-upgrade-cronjob_k8s' 'arm64_edge-sync-service')
elif [[ ${arch} == 's390x' ]]; then
    images=('s390x_anax' 's390x_anax_k8s' 's390x_auto-upgrade-cronjob_k8s' 's390x_edge-sync-service')
fi

# Save those images
for image in "${images[@]}"; do

    if [[ ${image} == *"cloud-sync-service"* ]]; then
        VERSION=${CSS_IMAGE_VERSION}
    elif [[ ${image} == *"edge-sync-service"* ]]; then
        VERSION=${ESS_IMAGE_VERSION}
    else
        VERSION=${ANAX_IMAGE_VERSION}
    fi

    docker tag ${IMAGE_REPO}/${image}:testing ${IMAGE_REPO}/${image}:${VERSION}
    docker save ${IMAGE_REPO}/${image}:${VERSION} | gzip > ${image}.tar.gz

done