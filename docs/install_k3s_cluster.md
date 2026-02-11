---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Installing a K3s cluster
description: Documentation for installing a K3s edge cluster
lastupdated: 2026-01-30
nav_order: 13
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

# Installing a K3s cluster
{: #install_k3s_cluster}

This content provides a summary of how to install K3s, a lightweight and small Kubernetes cluster, on [Ubuntu 22.04.4 LTS ](https://ubuntu.com/download/server){:target="_blank"}{: .externalLink}. For more information, see [the K3s documentation ](https://docs.k3s.io/){:target="_blank"}{: .externalLink}.
{:shortdesc}

**Note**: If installed, uninstall kubectl before completing the following steps.

## Procedure

1. Either login as root or elevate to root with `sudo -i`

2. The full hostname of your machine must contain at least two dots. Check the full hostname:

   ```bash
   hostname
   ```
   {: codeblock}

   If you need to update it (for example, from `k3s` to `k.3.s`), use the following pattern:

   ```bash
   hostnamectl hostname k.3.s
   ```
   {: codeblock}

3. Install K3s:

   ```bash
   curl -sfL https://get.k3s.io | sh -
   ```
   {: codeblock}

4. Choose the image registry types: remote image registry or edge cluster local registry. Image registry is the place that will hold the agent image and agent cronjob image.

   - [Setting variables to use a remote image registry](setting_remote_image_registry.md)
   - [Setup edge cluster local image registry for K3s](setup_k3s_image_registry.md)

## What's next

* [Setup edge cluster local image registry for K3s](setup_k3s_image_registry.md)
* [Installing the agent on K3s and MicroK8s edge clusters](installing_k3s_microk8s_agent.md)