FROM alpine:3.6

ENV ANAX_LOG_LEVEL 3
ENV DEPLOYMENT_TYPE cluster
ARG UID=1000590000
ARG GID=0

RUN apk update \    
    && apk add --no-cache ca-certificates \ 
    && update-ca-certificates \
    && apk --no-cache add bash openssl curl vim jq wget gettext psmisc gnupg dpkg coreutils shadow iptables \
    && useradd -u ${UID} -g ${GID} anaxuser \
    && apk --no-cache add ca-certificates

RUN mkdir -p /usr/horizon/bin /usr/horizon/web \
    && mkdir -p /etc/horizon/agbot/policy.d /etc/horizon/policy.d /etc/horizon/trust

WORKDIR /home/anaxuser

COPY script/* /home/anaxuser/
COPY config/* /etc/horizon/

ADD anax /usr/horizon/bin/
ADD hzn /usr/bin/

RUN chown -R ${UID}:${GID} /var /etc /usr /home/anaxuser 

USER anaxuser

ENTRYPOINT ["/home/anaxuser/anax.service", "start"]

