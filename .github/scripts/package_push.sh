#!/bin/bash

set -eo pipefail

# Deal with Debian Package First
# Make the temp Dockerfile for the debs only tarball image
## Chose alpine:latest b/c of small size, tried FROM scratch but couldn't run container
touch Dockerfile.debs.tarball
echo "FROM alpine:latest" >> Dockerfile.debs.tarball
echo "ADD ./debs.tar.gz ." >> Dockerfile.debs.tarball

# Make debs tarball
tar --transform 's/.*\/\([^\/]*\/[^\/]*\)$/\1/' -czvf debs.tar.gz ./pkg/deb/debs/*.deb

# Build docker image with only debs tarball
docker build \
    --no-cache \
    -t ${IMAGE_REPO}/${arch}_anax_debian:testing \
    -f Dockerfile.debs.tarball \
    .

if [[ "$GITHUB_REF" == 'refs/heads/master' ]]; then 
    docker push ${IMAGE_REPO}/${arch}_anax_debian:testing
    docker tag ${IMAGE_REPO}/${arch}_anax_debian:testing ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_debian:testing
    docker push ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_debian:testing
else
    # append the branch name to testing tags for when we're building older versions of anax for testing
    docker tag ${IMAGE_REPO}/${arch}_anax_debian:testing ${IMAGE_REPO}/${arch}_anax_debian:testing_${GH_BRANCH}
    docker tag ${IMAGE_REPO}/${arch}_anax_debian:testing ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_debian:testing_${GH_BRANCH}
    docker push ${IMAGE_REPO}/${arch}_anax_debian:testing_${GH_BRANCH}
    docker push ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_debian:testing_${GH_BRANCH}
fi

# Deal with RPM Package
if [[ ${arch} == 'amd64' || ${arch} == 'ppc64el' || ${arch} == 's390x' ]]; then

    # Make the temp Dockerfile for the RPM only tarball image
    touch Dockerfile.rpm.tarball
    echo "FROM alpine:latest" >> Dockerfile.rpm.tarball
    echo "ADD ./rpm.tar.gz ." >> Dockerfile.rpm.tarball

    # More amd64 rpm packages to the right directory
    if [[ ${arch} == 'amd64' ]]; then
        mkdir /home/runner/work/anax/anax/RPMS
        cp /home/runner/rpmbuild/RPMS/x86_64/*.rpm /home/runner/work/anax/anax/RPMS
    fi

    # Make RPM tarball
    tar --transform 's/.*\/\([^\/]*\/[^\/]*\)$/\1/' -czvf rpm.tar.gz /home/runner/work/anax/anax/RPMS/*.rpm

    # Build docker image with only RPM tarball
    docker build \
        --no-cache \
        -t $IMAGE_REPO/${arch}_anax_rpm:testing \
        -f Dockerfile.rpm.tarball \
        .

    if [[ "$GITHUB_REF" == 'refs/heads/master' ]]; then 
        docker push ${IMAGE_REPO}/${arch}_anax_rpm:testing
        docker tag ${IMAGE_REPO}/${arch}_anax_rpm:testing ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_rpm:testing
        docker push ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_rpm:testing
    else
        # append the branch name to testing tags for when we're building older versions of anax for testing
        docker tag ${IMAGE_REPO}/${arch}_anax_rpm:testing ${IMAGE_REPO}/${arch}_anax_rpm:testing_${GH_BRANCH}
        docker tag ${IMAGE_REPO}/${arch}_anax_rpm:testing ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_rpm:testing_${GH_BRANCH}
        docker push ${IMAGE_REPO}/${arch}_anax_rpm:testing_${GH_BRANCH}
        docker push ${GITHUB_CONTAINER_REGISTRY}/${arch}_anax_rpm:testing_${GH_BRANCH}
    fi
fi