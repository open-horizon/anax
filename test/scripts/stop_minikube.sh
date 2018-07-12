#!/bin/bash

# A script to automate stopping a minikube instance on the local machine.

isRoot=$(id -u)
if [ "${isRoot}" == "0" ]; then
	minikube stop
else
	sudo minikube stop
fi

kubedown=$(kubectl get pods 2>&1 | grep -c 'refused')
if [ "${kubedown}" -gt "0" ]; then
	echo "Minikube is stopped, cleaning up stopped containers..."
	runc=$(docker ps -aq)
	if [ "${runc}" != "" ]; then
		docker rm ${runc}
	fi
else
	echo "Minikube might not be stopped."
	kubectl get pods --all-namespaces=true
fi
