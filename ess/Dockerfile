ARG RED_HAT_UBI_TYPE=minimal \
    RHEL_VERSION=10 \
    BASE_IMAGE=ubi${RHEL_VERSION}-${RED_HAT_UBI_TYPE} \
    BASE_IMAGE_REGISTRY_UBI=registry.access.redhat.com \
    BASE_IMAGE_TAG_UBI=latest \
    BASE_IMAGE_FROM_UBI=${BASE_IMAGE_REGISTRY_UBI}/${BASE_IMAGE}:${BASE_IMAGE_TAG_UBI} \
    BASE_IMAGE_REGISTRY_ALPINE=docker.io \
    IMAGE_TAG_ALPINE=latest \
    BASE_IMAGE_FROM_ALPINE=${BASE_IMAGE_REGISTRY_ALPINE}/alpine:${IMAGE_TAG_ALPINE} \
    OPENCONTAINERS_IMAGE_AUTHORS="Open Horizon <open-horizon@lists.lfedge.org>" \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION="Provides the edge node side of the Model Management System to be used by the CLI service test tools when also testing object models." \
    OPENCONTAINERS_IMAGE_DOCUMENTATION="https://open-horizon.github.io/" \
    OPENCONTAINERS_IMAGE_LICENSES="Apache 2.0" \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE="https://github.com/open-horizon/anax" \
    OPENCONTAINERS_IMAGE_TITLE="edge-sync-service" \
    OPENCONTAINERS_IMAGE_VENDOR="Open Horizon" \
    OPENCONTAINERS_IMAGE_VERSION \
    SUMMARY="Edge Node Model Management System."

# ---------------------------------------------------------------------------------------------------------------------

FROM ${BASE_IMAGE_FROM_ALPINE} AS ess-alpine

# shadow contains groupadd and adduser commands
ARG BASE_IMAGE_FROM_ALPINE \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_ALPINE} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates openssl shadow" \
    SUMMARY \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE} \
      summary=${SUMMARY} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

RUN mkdir -p /run/user/$UID /var/edge-sync-service; \
    apk upgrade --force-refresh -q -U 1>/dev/null 2>&1 && \
    apk add -q ${REQUIRED_RPMS} 1>/dev/null 2>&1; \
    id -u edge-sync-service 1>/dev/null 2>&1 || ((getent group 4001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 4001 edge-sync-service || addgroup -g 4001 -S edge-sync-service)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 4001 --gid 4001 edge-sync-service || adduser -S -u 4001 -G edge-sync-service edge-sync-service)); \
    apk del -q shadow 1>/dev/null 2>&1 && \
    apk cache clean 1>/dev/null 2>&1; \
    rm -rf /mnt/rootfs/var/cache/* /var/cache/PackageKit /run/user/$UID /tmp/*

# Application binary
COPY --chmod=755 $TARGETPLATFORM/edge-sync-service /usr/local/bin

# Application license
COPY --chmod=644 LICENSE.txt /usr/local/share/licenses/edge-sync-service/

USER 4001:4001

# overwrite a user specific application configuration (no superuser permissions required), and then run the application using user's configuration.
CMD [ "/usr/local/bin/edge-sync-service" ]

# ---------------------------------------------------------------------------------------------------------------------

FROM ${BASE_IMAGE_FROM_UBI} AS ess-ubi

# shadow-utils contains groupadd and adduser commands
ARG BASE_IMAGE_FROM_UBI \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_UBI} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates openssl shadow-utils" \
    SUMMARY \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE} \
      summary=${SUMMARY} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

RUN mkdir -p /run/user/$UID /var/edge-sync-service; \
    microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 && \
    microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_RPMS} 1>/dev/null 2>&1; \
    id -u edge-sync-service 1>/dev/null 2>&1 || ((getent group 4001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 4001 edge-sync-service || addgroup -g 4001 -S edge-sync-service)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 4001 --gid 4001 edge-sync-service || adduser -S -u 4001 -G edge-sync-service edge-sync-service)); \
    microdnf remove --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y shadow-utils 1>/dev/null 2>&1 && \
    microdnf clean all --disableplugin=subscription-manager; \
    rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.* /var/cache/dnf /var/cache/PackageKit /run/user/$UID /tmp/*

# Application binary
COPY --chmod=755 $TARGETPLATFORM/edge-sync-service /usr/local/bin

# Application license
COPY --chmod=644 LICENSE.txt /usr/local/share/licenses/edge-sync-service/

USER 4001:4001

# overwrite a user specific application configuration (no superuser permissions required), and then run the application using user's configuration.
CMD [ "/usr/local/bin/edge-sync-service" ]
