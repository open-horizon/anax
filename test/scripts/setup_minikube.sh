#!/bin/bash

echo "Setting up minikube and helm configuration in $HOME"

isRoot=$(id -u)
cprefix="sudo"
if [ "${isRoot}" == "0" ]; then
	cprefix=""
fi

$cprefix apt-get update
$cprefix apt-get install -y apt-transport-https curl socat

curl -Lo /tmp/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.10.0/bin/linux/amd64/kubectl
chmod +x /tmp/kubectl
$cprefix mv /tmp/kubectl /usr/local/bin/

curl -Lo /tmp/minikube https://storage.googleapis.com/minikube-builds/2550/minikube-linux-amd64
chmod +x /tmp/minikube
$cprefix mv /tmp/minikube /usr/local/bin/

curl -Lo /tmp/helm-v2.9.1-linux-amd64.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-v2.9.1-linux-amd64.tar.gz
tar -xzf /tmp/helm-v2.9.1-linux-amd64.tar.gz --directory /tmp
chmod +x /tmp/linux-amd64/helm
$cprefix mv /tmp/linux-amd64/helm /usr/local/bin/

mkdir -p $HOME/.kube
touch $HOME/.kube/config

export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_WANTREPORTERRORPROMPT=false
export MINIKUBE_HOME=$HOME
export CHANGE_MINIKUBE_NONE_USER=true
export KUBECONFIG=$HOME/.kube/config

$cprefix -E minikube start --vm-driver=none

# Wait until this returns at least 4 lines of non-header output indicating that something is
# running in the kube-system namespace.
while :
do

	pods=$(kubectl get pods --all-namespaces=true 2>&1 | grep -c 'kube-system')
	if [ "${pods}" -gt "3" ]; then
		echo "Minikube is running."
		break
	else
		echo "Waiting for minikube pods to start..."
		sleep 10
	fi

done

# Assume that some part of kubernetes managed to get started, so its ok to install helm now.
helm init
