FROM ubuntu:16.04

RUN apt-get update
RUN apt-get -y install vim iptables build-essential wget git iputils-ping net-tools curl jq mosquitto-clients apt-transport-https socat
RUN curl -fsSL get.docker.com | sh
RUN curl https://dl.google.com/go/go1.10.2.linux-amd64.tar.gz | tar -xzf- -C /usr/local/

ENV GOROOT=/usr/local/go
ENV PATH="${PATH}:${GOROOT}/bin"

ENV HZN_ORG_ID="e2edev"
ENV HZN_EXCHANGE_USER_AUTH="e2edev/e2edevadmin:e2edevadminpw"

# Get specific versions of kubectl, minikube and helm that are known to work and work together.
# These steps get the CLI binaries and setup the config, including environment variables that the
# CLI needs.
WORKDIR /tmp
RUN curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.10.0/bin/linux/amd64/kubectl
RUN chmod +x /tmp/kubectl
RUN mv /tmp/kubectl /usr/local/bin/
RUN curl -Lo minikube https://storage.googleapis.com/minikube-builds/2550/minikube-linux-amd64
RUN chmod +x /tmp/minikube
RUN mv /tmp/minikube /usr/local/bin/

RUN wget -q https://storage.googleapis.com/kubernetes-helm/helm-v2.9.1-linux-amd64.tar.gz
RUN tar -xzf /tmp/helm-v2.9.1-linux-amd64.tar.gz
RUN mv /tmp/linux-amd64/helm /usr/local/bin/

RUN mkdir -p /root/.kube
RUN touch /root/.kube/config

ENV MINIKUBE_WANTUPDATENOTIFICATION=false
ENV MINIKUBE_WANTREPORTERRORPROMPT=false
ENV MINIKUBE_HOME=/root
ENV CHANGE_MINIKUBE_NONE_USER=true
ENV KUBECONFIG=/root/.kube/config
# Done with Kubernetes (minikube) setup for this container.

RUN mkdir -p /tmp/service_storage
WORKDIR /tmp

RUN alias dir='ls -la'
