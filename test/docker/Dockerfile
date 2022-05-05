FROM ubuntu:18.04

ARG ARCH

RUN apt-get update \
    && apt-get -y install vim iptables build-essential wget git iputils-ping net-tools curl jq kafkacat apt-transport-https socat software-properties-common lsb-release gettext-base

ARG DOCKER_VER=19.03.8

# install docker cli
RUN curl -4fsSLO https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VER}.tgz \
    && tar xzvf docker-${DOCKER_VER}.tgz --strip 1 -C /usr/bin docker/docker \
    && rm docker-${DOCKER_VER}.tgz

RUN curl https://dl.google.com/go/go1.17.linux-amd64.tar.gz | tar -xzf- -C /usr/local/

RUN curl -4fsSL https://apt.releases.hashicorp.com/gpg | apt-key add - && \
    apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main" && \
    apt-get update && apt-get -y install vault ;

ENV GOROOT=/usr/local/go
ENV PATH="${PATH}:${GOROOT}/bin"

RUN adduser agbotuser --disabled-password --gecos "agbot user,1,2,3"

ENV HZN_ORG_ID="e2edev@somecomp.com"
ENV HZN_EXCHANGE_USER_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"

RUN mkdir -p /tmp/service_storage
WORKDIR /tmp

RUN alias dir='ls -la'
