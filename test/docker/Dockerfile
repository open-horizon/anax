FROM ubuntu:18.04

RUN apt-get update
RUN apt-get -y install vim iptables build-essential wget git iputils-ping net-tools curl jq kafkacat
RUN curl -fsSL get.docker.com | sh
RUN curl https://dl.google.com/go/go1.10.2.linux-amd64.tar.gz | tar -xzf- -C /usr/local/

ENV GOROOT=/usr/local/go
ENV PATH="${PATH}:${GOROOT}/bin"

ENV HZN_ORG_ID="e2edev"
ENV HZN_EXCHANGE_USER_AUTH="e2edev/e2edevadmin:e2edevadminpw"

RUN mkdir -p /tmp/service_storage
WORKDIR /tmp

RUN alias dir='ls -la'
