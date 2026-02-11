---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Setup edge cluster local image registry for K3s
description: Documentation for setting up local image registry for K3s edge clusters
lastupdated: 2026-01-30
nav_order: 14
parent: Planning to install an edge cluster agent
grand_parent: Agent (anax)
has_children: false
has_toc: false
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Setup edge cluster local image registry for K3s
{: #setup_k3s_image_registry}

**Note**: Skip this section if using remote image registry.
{:shortdesc}

## Procedure

1. Create the K3s image registry service:

   a. Set `USE_EDGE_CLUSTER_REGISTRY` environment variable to `true`. This env indicates `agent-install.sh` script to use local image registry:

   ```bash
   export USE_EDGE_CLUSTER_REGISTRY=true
   ```
   {: codeblock}

   b. Create a file called **k3s-persistent-claim.yaml** with this content:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: docker-registry-pvc
   spec:
     storageClassName: "local-path"
     accessModes:
       - ReadWriteOnce
     resources:
       requests:
         storage: 10Gi
   ```
   {: codeblock}

   Or download it from the server:

   ```bash
   curl -sSLO https://raw.githubusercontent.com/open-horizon/open-horizon.github.io/master/docs/installing/k3s-persistent-claim.yaml
   ```
   {: codeblock}

   c. Create the persistent volume claim:

   ```bash
   kubectl apply -f k3s-persistent-claim.yaml
   ```
   {: codeblock}

   d. Verify that the persistent volume claim was created and it is in "Pending" status:

   ```bash
   kubectl get pvc
   ```
   {: codeblock}

   e. Create a file called **k3s-registry-deployment.yaml** with this content:

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: docker-registry
     labels:
       app: docker-registry
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: docker-registry
     template:
       metadata:
         labels:
           app: docker-registry
       spec:
         volumes:
           - name: registry-pvc-storage
             persistentVolumeClaim:
               claimName: docker-registry-pvc
         containers:
           - name: docker-registry
             image: registry
             ports:
               - containerPort: 5000
             volumeMounts:
               - name: registry-pvc-storage
                 mountPath: /var/lib/registry
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: docker-registry-service
   spec:
     selector:
       app: docker-registry
     type: NodePort
     ports:
       - protocol: TCP
         port: 5000
   ```
   {: codeblock}

   Or download it from the server:

   ```bash
   curl -sSLO https://raw.githubusercontent.com/open-horizon/open-horizon.github.io/master/docs/installing/k3s-registry-deployment.yaml
   ```
   {: codeblock}

   f. Create the registry deployment and service:

   ```bash
   kubectl apply -f k3s-registry-deployment.yaml
   ```
   {: codeblock}

   g. Verify that the service was created:

   ```bash
   kubectl get deployment
   kubectl get service
   ```
   {: codeblock}

   h. Define the registry endpoint:

   ```bash
   export REGISTRY_ENDPOINT=$(kubectl get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
   cat << EOF >> /etc/rancher/k3s/registries.yaml
   mirrors:
     "$REGISTRY_ENDPOINT":
       endpoint:
         - "http://$REGISTRY_ENDPOINT"
   EOF
   ```
   {: codeblock}

   i. Restart K3s to pick up the change to **/etc/rancher/k3s/registries.yaml**:

   ```bash
   systemctl restart k3s
   ```
   {: codeblock}

2. Define this registry to Docker as an insecure registry:

   a. Install Docker (if not already installed, `docker --version` to check):

   ```bash
   curl -fsSL get.docker.com | sh
   ```
   {: codeblock}

   b. Create or add to **/etc/docker/daemon.json** (replacing `<registry-endpoint>` with the value of the `$REGISTRY_ENDPOINT` environment variable you obtained in a previous step):

   ```json
   {
     "insecure-registries": [ "<registry-endpoint>" ]
   }
   ```
   {: codeblock}

   c. Restart Docker to pick up the change:

   ```bash
   systemctl restart docker
   ```
   {: codeblock}

   d. Install `jq`:

   ```bash
   apt-get -y install jq
   ```
   {: codeblock}

## What's next

* [Installing the agent on K3s and MicroK8s edge clusters](installing_k3s_microk8s_agent.md)