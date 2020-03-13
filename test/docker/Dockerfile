FROM ubuntu:18.04

RUN apt-get update
RUN apt-get -y install vim iptables build-essential wget git iputils-ping net-tools curl jq kafkacat apt-transport-https socat
RUN curl -fsSL get.docker.com | sh
RUN curl https://dl.google.com/go/go1.14.linux-amd64.tar.gz | tar -xzf- -C /usr/local/

ENV GOROOT=/usr/local/go
ENV PATH="${PATH}:${GOROOT}/bin"

RUN adduser agbotuser --disabled-password --gecos "agbot user,1,2,3"

ENV HZN_ORG_ID="e2edev@somecomp.com"
ENV HZN_EXCHANGE_USER_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"

RUN mkdir -p /tmp/service_storage
WORKDIR /tmp

RUN alias dir='ls -la'
