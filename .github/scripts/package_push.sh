#!/bin/bash

if [[ -z "$ANAX_IMAGE_VERSION" ]]; then
    echo "Error: Image version not set in package_push"
    exit 64
fi

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
    -t ${IMAGE_REPO}/${arch}_anax_debian:${ANAX_IMAGE_VERSION} \
    -f Dockerfile.debs.tarball \
    .

# Push docker image
docker push ${IMAGE_REPO}/${arch}_anax_debian:${ANAX_IMAGE_VERSION}

if [[ "$GITHUB_REF" == 'refs/heads/master' ]]; then 
    docker tag ${IMAGE_REPO}/${arch}_anax_debian:${ANAX_IMAGE_VERSION} ${IMAGE_REPO}/${arch}_anax_debian:testing
    docker push ${IMAGE_REPO}/${arch}_anax_debian:testing
fi

# Deal with RPM Package
if [[ ${arch} == 'amd64' || ${arch} == 'ppc64el' ]]; then

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
        -t $IMAGE_REPO/${arch}_anax_rpm:${ANAX_IMAGE_VERSION} \
        -f Dockerfile.rpm.tarball \
        .

    # Push docker image
    docker push ${IMAGE_REPO}/${arch}_anax_rpm:${ANAX_IMAGE_VERSION}

    if [[ "$GITHUB_REF" == 'refs/heads/master' ]]; then 
        docker tag ${IMAGE_REPO}/${arch}_anax_rpm:${ANAX_IMAGE_VERSION} ${IMAGE_REPO}/${arch}_anax_rpm:testing
        docker push ${IMAGE_REPO}/${arch}_anax_rpm:testing
    fi
fi