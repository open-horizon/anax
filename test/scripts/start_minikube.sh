#!/bin/bash

# A script to automate starting a minikube instance on the local machine.

isRoot=$(id -u)
if [ "${isRoot}" == "0" ]; then
	minikube start --vm-driver=none
else
	sudo -E minikube start --vm-driver=none
fi

echo "Delaying while minikube starts..."
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
