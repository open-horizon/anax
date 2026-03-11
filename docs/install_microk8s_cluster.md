---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Installing a microk8s cluster
description: Documentation for installing a MicroK8s edge cluster
lastupdated: 2026-01-30
nav_order: 15
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

# Installing a microk8s cluster
{: #install_microk8s_cluster}

This content provides a summary of how to install MicroK8s, a lightweight and small Kubernetes cluster, on Ubuntu 22.04.4 LTS. (For more information, see the MicroK8s documentation.)
{:shortdesc}

**Note**: This type of edge cluster is meant for development and test because a single worker node Kubernetes cluster does not provide scalability or high availability.

## Procedure

1. Install MicroK8s:

   ```bash
   sudo snap install microk8s --classic --channel=stable
   ```
   {: codeblock}

2. If you are not running as root, add your user to the MicroK8s group:

   ```bash
   sudo usermod -a -G microk8s $USER
   sudo chown -f -R $USER ~/.kube
   su - $USER
   ```
   {: codeblock}

3. Enable dns and storage modules in MicroK8s:

   ```bash
   microk8s.enable dns
   microk8s.enable hostpath-storage
   ```
   {: codeblock}

   **Note**: MicroK8s uses 8.8.8.8 and 8.8.4.4 as upstream name servers by default. If these name servers cannot resolve the management hub hostname, you must change the name servers that MicroK8s is using:

   a. Retrieve the list of upstream name servers in `/etc/resolv.conf` or `/run/system/resolve/resolv.conf`

   b. Edit coredns configmap in the kube-system namespace. Set the upstream nameservers in the forward section:

   ```bash
   microk8s.kubectl edit -n kube-system cm/coredns
   ```
   {: codeblock}

4. Check the status:

   ```bash
   microk8s.status --wait-ready
   ```
   {: codeblock}

5. The MicroK8s kubectl command is called microk8s.kubectl to prevent conflicts with an already install kubectl command. Assuming that kubectl is not installed, add this alias for microk8s.kubectl:

   ```bash
   echo 'alias kubectl=microk8s.kubectl' >> ~/.bash_aliases
   source ~/.bash_aliases
   ```
   {: codeblock}

6. Choose the image registry types: remote image registry or edge cluster local registry. Image registry is the place that will hold the agent image and agent cronjob image.

   - [Setting variables to use a remote image registry](setting_remote_image_registry.md)
   - [Setup edge cluster local image registry for MicroK8s](setup_microk8s_image_registry.md)

## What's next

* [Setup edge cluster local image registry for MicroK8s](setup_microk8s_image_registry.md)
* [Installing the agent on K3s and MicroK8s edge clusters](installing_k3s_microk8s_agent.md)