---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Installing the agent on K3s and MicroK8s edge clusters
description: Documentation for installing the agent on K3s and MicroK8s edge clusters
lastupdated: 2026-01-30
nav_order: 19
parent: Agent (anax)
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

# Installing the agent on K3s and MicroK8s edge clusters
{: #installing_k3s_microk8s_agent}

This content describes how to install the {{site.data.keyword.ieam}} agent on [K3s ](https://k3s.io/){:target="_blank"}{: .externalLink} or [microk8s ](https://microk8s.io/){:target="_blank"}{: .externalLink}, lightweight and small Kubernetes clusters:
{:shortdesc}

## Prerequisites

* The architecture of the cluster must be one of the following:
  - AMD64 (x86_64)
  - ARM64 (AArch64)
  - ppc64le
  - s390x

* Operating system must be modern Linux variant with 64-bit and systemd support

**Note**: {{site.data.keyword.ieam}} cluster agent installation requires cluster admin access on the edge cluster. Additionally, the "jq" command-line JSON processor must be installed prior to running the agent install script.

## Procedure

1. Log in to your edge cluster as **root**.

2. If you don't already have your authorization information, get it now. For details about how to get the authorization information, see [Authentication overview](authentication_overview.md).

   ```bash
   export HZN_ORG_ID=<your-exchange-organization>
   export HZN_EXCHANGE_USER_AUTH=<authentication string>
   export HZN_EXCHANGE_URL= # example http://open-horizon.lfedge.iol.unh.edu:3090/v1
   export HZN_FSS_CSSURL= # example http://open-horizon.lfedge.iol.unh.edu:9443/
   export HZN_AGBOT_URL= # example http://open-horizon.lfedge.iol.unh.edu:3111
   export HZN_FDO_SVC_URL= # example http://open-horizon.lfedge.iol.unh.edu:9008/api
   ```
   {: codeblock}

3. Copy the **agent-install.sh** script to your new edge cluster.

4. The **agent-install.sh** script will store the {{site.data.keyword.ieam}} agent in the edge cluster image registry. Set the full image path (minus the tag) that should be used. For example:

   * On k3s:

     ```bash
     REGISTRY_ENDPOINT=$(kubectl get service docker-registry-service | grep docker-registry-service | awk '{print $3;}'):5000
     export IMAGE_ON_EDGE_CLUSTER_REGISTRY=$REGISTRY_ENDPOINT/openhorizon-agent/amd64_anax_k8s
     ```
     {: codeblock}

   * On microk8s:

     ```bash
     export IMAGE_ON_EDGE_CLUSTER_REGISTRY=localhost:32000/openhorizon-agent/amd64_anax_k8s
     ```
     {: codeblock}

   **Note**: The {{site.data.keyword.ieam}} agent image is stored in the local edge cluster registry because the edge cluster Kubernetes needs ongoing access to it, in case it needs to restart it or move it to another pod.

5. Instruct **agent-install.sh** to use the default storage class:

   * On k3s:

     ```bash
     export EDGE_CLUSTER_STORAGE_CLASS=local-path
     ```
     {: codeblock}

   * On microk8s:

     ```bash
     export EDGE_CLUSTER_STORAGE_CLASS=microk8s-hostpath
     ```
     {: codeblock}

6. Run **agent-install.sh** to get the necessary files from CSS (Cloud Sync Service), install and configure the {{site.data.keyword.horizon}} agent, and register your edge cluster with policy:

   ```bash
   ./agent-install.sh -D cluster -i 'css:'
   ```
   {: codeblock}

   **Notes**:
   * To see all of the available flags, run: **./agent-install.sh -h**
   * If an error occurs causing **agent-install.sh** to not complete successfully, correct the error that is displayed, and run **agent-install.sh** again. If that does not work, run **agent-uninstall.sh** (see [Removing agent from edge cluster](removing_agent_from_cluster.md)) before running **agent-install.sh** again.

7. Verify that the agent pod is running:

   ```bash
   kubectl get namespaces
   kubectl -n openhorizon-agent get pods
   ```
   {: codeblock}

8. Usually, when an edge cluster is registered for policy, but does not have any user-specified node policy, none of the deployment policies deploy edge services to it. This is expected. Proceed to [Deploying services to your edge cluster](deploying_services_cluster.md) to set node policy so that an edge service will be deployed to this edge cluster.

## What's next

* [Deploying services to your edge cluster](deploying_services_cluster.md)
* [Removing the agent from an edge cluster](removing_agent_from_cluster.md)