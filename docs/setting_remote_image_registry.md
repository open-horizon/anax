---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Setting variables to use a remote image registry
description: Documentation for configuring remote image registry for edge clusters
lastupdated: 2026-01-30
nav_order: 17
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

# Setting variables to use a remote image registry
{: #setting_remote_image_registry}

Set `USE_EDGE_CLUSTER_REGISTRY` environment variable to `false` to instruct `agent-install.sh` script to use remote image registry. The following environment variables need to be set if use remote image registry:
{:shortdesc}

```bash
export USE_EDGE_CLUSTER_REGISTRY=false
export EDGE_CLUSTER_REGISTRY_USERNAME=<remote-image-registry-username>
export EDGE_CLUSTER_REGISTRY_TOKEN=<remote-image-registry-password>
export IMAGE_ON_EDGE_CLUSTER_REGISTRY=<remote-image-registry-host>/<repository-name>/amd64_anax_k8s
```
{: codeblock}

Or for s390x architecture:

```bash
export IMAGE_ON_EDGE_CLUSTER_REGISTRY=<remote-image-registry-host>/<repository-name>/s390x_anax_k8s
```
{: codeblock}

## What's next

* [Installing the agent on Red Hat OpenShift Container Platform Kubernetes edge cluster](installing_ocp_edge_cluster_agent.md)
* [Installing the agent on K3s and MicroK8s edge clusters](installing_k3s_microk8s_agent.md)