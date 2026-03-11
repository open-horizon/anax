---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Setup edge cluster local image registry for MicroK8s
description: Documentation for setting up local image registry for MicroK8s edge clusters
lastupdated: 2026-01-30
nav_order: 16
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

# Setup edge cluster local image registry for MicroK8s
{: #setup_microk8s_image_registry}

**Note**: Skip this section if using remote image registry. Enable the container registry and configure Docker to tolerate the insecure registry.
{:shortdesc}

## Procedure

1. Enable the container registry:

   ```bash
   microk8s.enable registry
   export REGISTRY_ENDPOINT=localhost:32000
   export REGISTRY_IP_ENDPOINT=$(kubectl get service registry -n container-registry | grep registry | awk '{print $3;}'):5000
   ```
   {: codeblock}

2. Install Docker (if not already installed, `docker --version` to check):

   ```bash
   curl -fsSL get.docker.com | sh
   ```
   {: codeblock}

3. Install jq (if not already installed):

   ```bash
   apt-get -y install jq
   ```
   {: codeblock}

4. Define this registry as insecure to Docker. Create or add to `/etc/docker/daemon.json`:

   ```bash
   echo "{
       \"insecure-registries\": [ \"$REGISTRY_ENDPOINT\", \"$REGISTRY_IP_ENDPOINT\" ]
   }" >> /etc/docker/daemon.json
   ```
   {: codeblock}

5. Restart Docker to pick up the change:

   ```bash
   systemctl restart docker
   ```
   {: codeblock}

## What's next

* [Installing the agent on K3s and MicroK8s edge clusters](installing_k3s_microk8s_agent.md)