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
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT="The Agreement Bot scans all the edge nodes in the system initiating deployment of services and model to all eligible nodes." \
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT="A container which holds the edge node agent, to be used in environments where there is no operating system package that can install the agent natively." \
    OPENCONTAINERS_IMAGE_DOCUMENTATION="https://open-horizon.github.io/" \
    OPENCONTAINERS_IMAGE_LICENSES="Apache 2.0" \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE="https://github.com/open-horizon/anax" \
    OPENCONTAINERS_IMAGE_TITLE_AGBOT="agbot" \
    OPENCONTAINERS_IMAGE_TITLE_AGENT="anax" \
    OPENCONTAINERS_IMAGE_VENDOR="Open Horizon" \
    OPENCONTAINERS_IMAGE_VERSION \
    SUMMARY_AGBOT="The deployment engine." \
    SUMMARY_AGENT="The agent in a general purpose container."

# Agent Images
# ---------------------------------------------------------------------------------------------------------------------


FROM ${BASE_IMAGE_FROM_ALPINE} AS agent-alpine

ARG BASE_IMAGE_FROM_ALPINE \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_ALPINE} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE_AGENT \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="bash ca-certificates docker docker-cli docker-engine gzip iptables jq openssl procps-ng psmisc shadow sudo tar vim" \
    SUMMARY_AGENT \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE_AGENT} \
      summary=${SUMMARY_AGENT} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE_AGENT} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

# TODO: Image updates and docker
RUN mkdir -p /run/user/$UID /licenses /usr/horizon/bin /usr/horizon/web /var/horizon /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust; \
    apk --force-refresh -q -U upgrade 1>/dev/null 2>&1; \
    apk add -q ${REQUIRED_PACKAGES} 1>/dev/null 2>&1; \
    id -u anax 1>/dev/null 2>&1 || ((getent group 5001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 5001 anax || addgroup -g 5001 -S anax)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 5001 --gid 5001 anax || adduser -S -u 5001 -G anax,docker anax)); \
    apk del -q shadow 1>/dev/null 2>&1 \
    && apk cache clean 1>/dev/null 2>&1; \
    mkdir -p /home/anax/.config/anax \
    && touch /home/anax/.config/anax/anax.json \
    && chown -R 5001:5001 /home/anax/.config/anax

# add license file
COPY --chmod=644 LICENSE.txt /usr/share/licenses/anax/
COPY --chmod=644 LICENSE.txt /usr/share/licenses/hzn/

# copy the horizon configurations
# COPY --chmod=644 anax-in-container/config/anax.json anax-in-container/config/anax.json.tmpl anax-in-container/config/hzn.json /etc/horizon/
COPY --chmod=644 anax-in-container/config/anax.json anax-in-container/config/hzn.json /etc/horizon/

# copy anax and hzn binaries
COPY --chmod=755 $TARGETPLATFORM/anax $TARGETPLATFORM/hzn /usr/local/bin/

VOLUME /etc/horizon \
       /var/horizon

# TODO: non-root user
# USER 5001:5001

# TODO: Temporary directory needs to move to /run/user/$UID/horizon to isolate file contents from other users/processes in the container.
ENV ANAX_LOG_LEVEL=3 \
    DB_PATH=/home/anax/var/local/anax/ \
    DOCKER_NAME=horizon1 \
    HZN_VAR_RUN_BASE=/tmp/horizon/horizon1

# You can add a 2nd arg to this on the docker run cmd or the CMD statement in another dockerfile, to configure a specific environment
# CMD (/usr/bin/envsubst < /etc/horizon/anax.json.tmpl > $HOME/.config/anax/anax.json); sudo -E "anax -v $ANAX_LOG_LEVEL -logtostderr -config $HOME/.config/anax/anax.json"
CMD anax -v $ANAX_LOG_LEVEL -logtostderr -config /etc/horizon/anax.json
# ---------------------------------------------------------------------------------------------------------------------
FROM ${BASE_IMAGE_FROM_UBI} AS agent-ubi9

ARG BASE_IMAGE_FROM_UBI \
    DOCKER_REPO_DOMAIN="https://download.docker.com/linux/rhel/docker-ce.repo" \
    DOCKER_REPO_DOMAIN_GPG="https://download.docker.com/linux/rhel/gpg" \
    DOCKER_VER \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_UBI} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE_AGENT \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates containerd.io docker-ce${DOCKER_VER} docker-ce-cli${DOCKER_VER} gettext gzip iptables jq openssl procps-ng psmisc shadow-utils sudo tar vim-minimal" \
    REQUIRED_PACKAGES_PPC64LE="ca-certificates gettext gzip iptables jq openssl procps-ng psmisc shadow-utils sudo tar vim-minimal" \
    SUMMARY_AGENT \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE_AGENT} \
      summary=${SUMMARY_AGENT} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE_AGENT} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

# The anax binary (secrets manager code) shells out to groupadd, groupdel (from shadow-utils), pkill (from procps-ng)
# The anax.service calls jq (from jq) and killall (from psmisc)
# anax does not use iptables directly but the github.com/coreos/go-iptables/iptables dependency needs the directory structure
# Install docker cli, which requires tar / gunzip to unpack, then remove tar / gzip packages
# Anax administrates Docker containers and firewall rules on the system (container) and requires elevated privleges. This is the wheel group in the Fedora family of distributions.
# Create required directories
#&& echo "5001 ALL=(ALL) NOPASSWD: /usr/local/bin/anax" >> /etc/sudoers; \
RUN mkdir -p /run/user/$UID /licenses /usr/horizon/bin /usr/horizon/web /var/horizon /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust; \
    (if [[ $TARGETPLATFORM != 'linux/ppc64le' ]]; \
       then (rpm --import $DOCKER_REPO_DOMAIN_GPG && curl -fsSL $DOCKER_REPO_DOMAIN -o /etc/yum.repos.d/docker-ce.repo \
             && microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
             && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES} 1>/dev/null 2>&1) \
     else (microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
           && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES_PPC64LE} 1>/dev/null 2>&1 \
           && dockerLatest=$(curl -fsSL "https://download.docker.com/linux/static/stable/ppc64le" | tail -n 2 | grep -oE "docker-[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+(-([[:digit:]]+|ce))*\.tgz" | tail -n 1) \
           && (curl -fsSL "https://download.docker.com/linux/static/stable/ppc64le/${dockerLatest}" | tar xvz --strip-components=1 -C /usr/bin) \
           && (docker &)) \
     fi); \
    (id -u anax 1>/dev/null 2>&1 || ((getent group 5001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 5001 anax || addgroup -g 5001 -S anax)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 5001 --gid 5001 anax || adduser -S -u 5001 -G anax,docker anax))); \
    microdnf remove --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y shadow-utils 1>/dev/null 2>&1 \
    && microdnf clean all --disableplugin=subscription-manager \
    && rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.* /var/cache/dnf /var/cache/PackageKit /run/user/$UID /tmp/*; \
    mkdir -p /home/anax/.config/anax \
    && touch /home/anax/.config/anax/anax.json \
    && chown -R 5001:5001 /home/anax/.config/anax

# add license file
COPY --chmod=644 LICENSE.txt /usr/share/licenses/anax/
COPY --chmod=644 LICENSE.txt /usr/share/licenses/hzn/

# copy the horizon configurations
#COPY --chmod=644 anax-in-container/config/anax.json anax-in-container/config/anax.json.tmpl anax-in-container/config/hzn.json /etc/horizon/
COPY --chmod=644 anax-in-container/config/anax.json anax-in-container/config/hzn.json /etc/horizon/

# copy anax and hzn binaries
COPY --chmod=755 $TARGETPLATFORM/anax $TARGETPLATFORM/hzn /usr/local/bin/

VOLUME /etc/horizon \
       /var/horizon

# TODO: non-root user
#USER 5001:5001

# TODO: Temporary directory needs to move to /run/user/$UID/horizon to isolate file contents from other users/processes in the container.
ENV ANAX_LOG_LEVEL=3 \
    DB_PATH=/home/anax/var/local/anax/ \
    DOCKER_NAME=horizon1 \
    HZN_VAR_RUN_BASE=/tmp/horizon/horizon1

# You can add a 2nd arg to this on the docker run cmd or the CMD statement in another dockerfile, to configure a specific environment
# CMD (/usr/bin/envsubst < /etc/horizon/anax.json.tmpl > $HOME/.config/anax/anax.json); sudo -E "anax -v $ANAX_LOG_LEVEL -logtostderr -config $HOME/.config/anax/anax.json"
CMD anax -v $ANAX_LOG_LEVEL -logtostderr -config /etc/horizon/anax.json
# ---------------------------------------------------------------------------------------------------------------------
# UBI 10 Requires either, the use of nftables directly (as a replacement for iptables), or firewalld over top nftables.
# TODO: Implement firewall rule sets for nftables and/or firewalld.
FROM ${BASE_IMAGE_FROM_UBI} AS agent-ubi10

ARG BASE_IMAGE_FROM_UBI \
    DOCKER_REPO_DOMAIN="https://download.docker.com/linux/rhel/docker-ce.repo" \
    DOCKER_REPO_DOMAIN_GPG="https://download.docker.com/linux/rhel/gpg" \
    DOCKER_VER \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_UBI} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE_AGENT \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates containerd.io docker-ce${DOCKER_VER} docker-ce-cli${DOCKER_VER} gzip iptables jq openssl procps-ng psmisc shadow-utils tar vim-minimal" \
    REQUIRED_PACKAGES_PPC64LE="ca-certificates gzip iptables jq openssl procps-ng psmisc shadow-utils tar vim-minimal" \
    SUMMARY_AGENT \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE_AGENT} \
      summary=${SUMMARY_AGENT} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGENT} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE_AGENT} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

# The anax binary (secrets manager code) shells out to groupadd, groupdel (from shadow-utils), pkill (from procps-ng)
# The anax.service calls jq (from jq) and killall (from psmisc)
# anax does not use iptables directly but the github.com/coreos/go-iptables/iptables dependency needs the directory structure
# Install docker cli, which requires tar / gunzip to unpack, then remove tar / gzip packages
# Create required directories
RUN mkdir -p /run/user/$UID /licenses /usr/horizon/bin /usr/horizon/web /var/horizon /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust \
    && (if [[ $TARGETPLATFORM == 'linux/ppc64le' ]]; \
        then (microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
        && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES_PPC64LE} 1>/dev/null 2>&1 \
        && dockerLatest=$(curl -fsSL "https://download.docker.com/linux/static/stable/ppc64le" | tail -n 2 | grep -oE "docker-[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+(-([[:digit:]]+|ce))*\.tgz" | tail -n 1) \
        && (curl -fsSL "https://download.docker.com/linux/static/stable/ppc64le/${dockerLatest}" | tar xvz --strip-components=1 -C /usr/bin) \
        && (docker &)) \
    elif [[ $TARGETPLATFORM == 'linux/s390x' ]]; \
        then (microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
        && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES_PPC64LE} 1>/dev/null 2>&1 \
        && dockerLatest=$(curl -fsSL "https://download.docker.com/linux/static/stable/s390x" | tail -n 2 | grep -oE "docker-[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+(-([[:digit:]]+|ce))*\.tgz" | tail -n 1) \
        && (curl -fsSL "https://download.docker.com/linux/static/stable/s390x/${dockerLatest}" | tar xvz --strip-components=1 -C /usr/bin) \
        && (docker &)) \
    else (rpm --import $DOCKER_REPO_DOMAIN_GPG && curl -fsSL $DOCKER_REPO_DOMAIN -o /etc/yum.repos.d/docker-ce.repo \
          && microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
          && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES} 1>/dev/null 2>&1) \
    fi) \
    (id -u anax 1>/dev/null 2>&1 || ((getent group 5001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 5001 anax || addgroup -g 5001 -S anax)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 5001 --gid 5001 anax || adduser -S -u 5001 -G anax,docker anax))); \
    microdnf remove --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y shadow-utils 1>/dev/null 2>&1 \
    && microdnf clean all --disableplugin=subscription-manager \
    && rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.* /var/cache/dnf /var/cache/PackageKit /run/user/$UID /tmp/*

# add license file
COPY --chmod=644 LICENSE.txt /usr/share/licenses/anax/
COPY --chmod=644 LICENSE.txt /usr/share/licenses/hzn/

# copy the horizon configurations
COPY --chmod=644 anax-in-container/config/anax.json anax-in-container/config/hzn.json /etc/horizon/

# copy anax and hzn binaries
COPY --chmod=755 $TARGETPLATFORM/anax $TARGETPLATFORM/hzn /usr/local/bin/

VOLUME /etc/horizon \
       /var/horizon

# You can add a 2nd arg to this on the docker run cmd or the CMD statement in another dockerfile, to configure a specific environment
CMD anax -v $ANAX_LOG_LEVEL -logtostderr -config /etc/horizon/anax.json


# Agreement Bot Images
# ---------------------------------------------------------------------------------------------------------------------


FROM ${BASE_IMAGE_FROM_ALPINE} AS agbot-alpine

ARG BASE_IMAGE_FROM_ALPINE \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_ALPINE} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE_AGBOT \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates iptables jq openssl procps-ng psmisc shadow" \
    SUMMARY_AGBOT \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE_AGBOT} \
      summary=${SUMMARY_AGBOT} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE_AGBOT} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

# TODO: Image updates and docker
RUN mkdir -p /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust /run/user/$UID /licenses /usr/horizon/bin /var/horizon/msgKey /usr/horizon/web; \
    apk --force-refresh -q -U upgrade 1>/dev/null 2>&1 \
    && apk add -q ${REQUIRED_PACKAGES} 1>/dev/null 2>&1; \
    id -u agbot 1>/dev/null 2>&1 || ((getent group 2001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 2001 agbot || addgroup -g 2001 -S agbot)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 2001 --gid 2001 agbot || adduser -S -u 2001 -G agbot,docker agbot)); \
    apk del -q shadow 1>/dev/null 2>&1 \
    && apk cache clean 1>/dev/null 2>&1; \
    rm -rf /mnt/rootfs/var/cache/* /run/user/$UID /tmp/*; \
    mkdir -p /home/agbot/policy.d \
    && chown 2001:2001 /home/agbot/policy.d

# add license file
COPY --chmod=644 LICENSE.txt /usr/share/licenses/anax/
COPY --chmod=644 LICENSE.txt /usr/share/licenses/hzn/

# copy the horizon configurations
COPY --chmod=644 anax-in-container/config/agbot.json.tmpl anax-in-container/config/hzn.json /etc/horizon/

# copy anax and hzn binaries
# TODO: Move anax binary to /usr/local/bin from /usr/horizon/bin
# TODO: Move hzn binary to /usr/local/bin from /usr/bin
COPY --chmod=755 $TARGETPLATFORM/anax anax-in-container/script/agbot_start.sh /usr/horizon/bin/
COPY --chmod=755 $TARGETPLATFORM/hzn /usr/bin/

USER 2001:2001

# Run the application
CMD ["/usr/horizon/bin/agbot_start.sh"]
# ---------------------------------------------------------------------------------------------------------------------
FROM ${BASE_IMAGE_FROM_UBI} AS agbot-ubi9

ARG BASE_IMAGE_FROM_UBI \
    DOCKER_REPO_DOMAIN="https://download.docker.com/linux/rhel/docker-ce.repo" \
    DOCKER_REPO_DOMAIN_GPG="https://download.docker.com/linux/rhel/gpg" \
    DOCKER_VER \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_UBI} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT \
    OPENCONTAINERS_IMAGE_DOCUMENTATION \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE_AGBOT \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates gettext iptables jq openssl procps-ng psmisc shadow-utils" \
    SUMMARY_AGBOT \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE_AGBOT} \
      summary=${SUMMARY_AGBOT} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE_AGBOT} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

# The anax binary (secrets manager code) shells out to groupadd, groupdel (from shadow-utils), pkill (from procps-ng)
# The anax.service calls jq (from jq) and killall (from psmisc)
# anax does not use iptables directly but the github.com/coreos/go-iptables/iptables dependency needs the directory structure
# add agbotuser
# agbot_start.sh calls envsubst (from gettext)
# Create required directories
RUN mkdir -p /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust /run/user/$UID /licenses /usr/horizon/bin /var/horizon/msgKey /usr/horizon/web; \
    microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
    && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES} 1>/dev/null 2>&1; \
    id -u agbot 1>/dev/null 2>&1 || ((getent group 2001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 2001 agbot || addgroup -g 2001 -S agbot)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 2001 --gid 2001 agbot || adduser -S -u 2001 -G agbot,docker agbot)); \
    microdnf remove --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y shadow-utils 1>/dev/null 2>&1 \
    && microdnf clean all --disableplugin=subscription-manager; \
    rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.* /var/cache/dnf /var/cache/PackageKit /run/user/$UID /tmp/*; \
    mkdir -p /home/agbot/policy.d \
    && chown 2001:2001 /home/agbot/policy.d

# add license file
COPY --chmod=644 LICENSE.txt /usr/share/licenses/anax/
COPY --chmod=644 LICENSE.txt /usr/share/licenses/hzn/

# copy the horizon configurations
COPY --chmod=644 anax-in-container/config/agbot.json.tmpl anax-in-container/config/hzn.json /etc/horizon/

# copy anax and hzn binaries
# TODO: Move anax binary to /usr/local/bin from /usr/horizon/bin
# TODO: Move hzn binary to /usr/local/bin from /usr/bin
COPY --chmod=755 $TARGETPLATFORM/anax anax-in-container/script/agbot_start.sh /usr/horizon/bin/
COPY --chmod=755 $TARGETPLATFORM/hzn /usr/bin/

USER 2001:2001

# Run the application
CMD ["/usr/horizon/bin/agbot_start.sh"]
# ---------------------------------------------------------------------------------------------------------------------
# UBI 10 Requires either, the use of nftables directly (as a replacement for iptables), or firewalld over top nftables.
# TODO: Implement firewall rule sets for nftables and/or firewalld.
FROM ${BASE_IMAGE_FROM_UBI} AS agbot-ubi10

ARG BASE_IMAGE_FROM_UBI \
    DOCKER_REPO_DOMAIN="https://download.docker.com/linux/rhel/docker-ce.repo" \
    DOCKER_REPO_DOMAIN_GPG="https://download.docker.com/linux/rhel/gpg" \
    DOCKER_VER \
    OPENCONTAINERS_IMAGE_AUTHORS \
    OPENCONTAINERS_IMAGE_BASE_DIGEST \
    OPENCONTAINERS_IMAGE_BASE_NAME=${BASE_IMAGE_FROM_UBI} \
    OPENCONTAINERS_IMAGE_CREATED \
    OPENCONTAINERS_IMAGE_DESCRIPTION \
    OPENCONTAINERS_IMAGE_DOCUMENTATION_AGBOT \
    OPENCONTAINERS_IMAGE_LICENSES \
    OPENCONTAINERS_IMAGE_REVISION \
    OPENCONTAINERS_IMAGE_SOURCE \
    OPENCONTAINERS_IMAGE_TITLE_AGBOT \
    OPENCONTAINERS_IMAGE_VENDOR \
    OPENCONTAINERS_IMAGE_VERSION \
    REQUIRED_PACKAGES="ca-certificates gettext iptables jq openssl procps-ng psmisc shadow-utils" \
    SUMMARY_AGBOT \
    TARGETPLATFORM

LABEL authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT} \
      maintainer=${OPENCONTAINERS_IMAGE_AUTHORS} \
      name=${OPENCONTAINERS_IMAGE_TITLE_AGBOT} \
      summary=${SUMMARY_AGBOT} \
      vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      version=${OPENCONTAINERS_IMAGE_VERSION} \
      org.opencontainers.image.authors=${OPENCONTAINERS_IMAGE_AUTHORS} \
      org.opencontainers.image.base.digest=${OPENCONTAINERS_IMAGE_BASE_DIGEST} \
      org.opencontainers.image.base.name=${OPENCONTAINERS_IMAGE_BASE_NAME} \
      org.opencontainers.image.created=${OPENCONTAINERS_IMAGE_CREATED} \
      org.opencontainers.image.description=${OPENCONTAINERS_IMAGE_DESCRIPTION_AGBOT} \
      org.opencontainers.image.documentation=${OPENCONTAINERS_IMAGE_DOCUMENTATION} \
      org.opencontainers.image.licenses=${OPENCONTAINERS_IMAGE_LICENSES} \
      org.opencontainers.image.revision=${OPENCONTAINERS_IMAGE_REVISION} \
      org.opencontainers.image.source=${OPENCONTAINERS_IMAGE_SOURCE} \
      org.opencontainers.image.title=${OPENCONTAINERS_IMAGE_TITLE_AGBOT} \
      org.opencontainers.image.vendor=${OPENCONTAINERS_IMAGE_VENDOR} \
      org.opencontainers.image.version=${OPENCONTAINERS_IMAGE_VERSION}

# The anax binary (secrets manager code) shells out to groupadd, groupdel (from shadow-utils), pkill (from procps-ng)
# The anax.service calls jq (from jq) and killall (from psmisc)
# anax does not use iptables directly but the github.com/coreos/go-iptables/iptables dependency needs the directory structure
# add agbotuser
# agbot_start.sh calls envsubst (from gettext)
# Create required directories
RUN mkdir -p /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust /run/user/$UID /licenses /usr/horizon/bin /var/horizon/msgKey /usr/horizon/web; \
    microdnf update --disableplugin=subscription-manager --nodocs --refresh --setopt=install_weak_deps=0 -y 1>/dev/null 2>&1 \
    && microdnf install --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y ${REQUIRED_PACKAGES} 1>/dev/null 2>&1; \
    id -u agbot 1>/dev/null 2>&1 || ((getent group 2001 1>/dev/null 2>&1 || (type groupadd 1>/dev/null 2>&1 && groupadd -g 2001 agbot || addgroup -g 2001 -S agbot)) && (type useradd 1>/dev/null 2>&1 && useradd --system --create-home --uid 2001 --gid 2001 agbot || adduser -S -u 2001 -G agbot,docker agbot)); \
    microdnf remove --disableplugin=subscription-manager --nodocs --setopt=install_weak_deps=0 -y shadow-utils 1>/dev/null 2>&1 \
    && microdnf clean all --disableplugin=subscription-manager; \
    rm -rf /mnt/rootfs/var/cache/* /mnt/rootfs/var/log/dnf* /mnt/rootfs/var/log/yum.* /var/cache/dnf /var/cache/PackageKit /run/user/$UID /tmp/*; \
    mkdir -p /home/agbot/policy.d \
    && chown 2001:2001 /home/agbot/policy.d

# add license file
COPY --chmod=644 LICENSE.txt /usr/share/licenses/anax/
COPY --chmod=644 LICENSE.txt /usr/share/licenses/hzn/

# copy the horizon configurations
COPY --chmod=644 anax-in-container/config/agbot.json.tmpl anax-in-container/config/hzn.json /etc/horizon/

# copy anax and hzn binaries
# TODO: Move anax binary to /usr/local/bin from /usr/horizon/bin
# TODO: Move hzn binary to /usr/local/bin from /usr/bin
COPY --chmod=755 $TARGETPLATFORM/anax anax-in-container/script/agbot_start.sh /usr/horizon/bin/
COPY --chmod=755 $TARGETPLATFORM/hzn /usr/bin/

USER 2001:2001

# Run the application
CMD ["/usr/horizon/bin/agbot_start.sh"]
