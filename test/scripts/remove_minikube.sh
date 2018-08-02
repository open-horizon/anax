#!/bin/bash

echo "Removing minikube and helm"

isRoot=$(id -u)
cprefix="sudo"
if [ "${isRoot}" == "0" ]; then
	cprefix=""
fi

$cprefix minikube delete

docker stop $(docker ps -aq -f "name=k8s_")

rm -fr ~/.kube ~/.minikube ~/.helm

$cprefix rm -f /usr/local/bin/localkube /usr/local/bin/minikube /usr/local/bin/kubectl /usr/local/bin/helm

$cprefix systemctl stop '*kubelet*.mount'

$cprefix rm -rf /etc/kubernetes/

# We are skipping this for now so that we dont wipe out other stuff in docker images.
#docker system prune -af --volumes

$cprefix systemctl stop localkube.service

$cprefix systemctl disable localkube.service
