---
copyright: Contributors to the Open Horizon project
years: 2020 - 2026
title: Installing an OCP cluster
description: Documentation for Installing an OCP edge cluster
lastupdated: 2026-01-30
nav_order: 11
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

# Installing an OCP cluster
{: #install_ocp_edge_cluster}

1. Install OCP by following the installation instructions in the [{{site.data.keyword.open_shift_cp}} Documentation ](https://docs.openshift.com/container-platform/4.6/welcome/index.html){:target="_blank"}{: .externalLink}. ({{site.data.keyword.ieam}} only supports OCP on x86_64 platforms.)

2. Install the Kubernetes CLI (**kubectl**), Openshift client CLI (**oc**) and Docker on the admin host where you administer your OCP edge cluster. This is the same host where you run the agent installation script. For more information, see [Installing cloudctl, kubectl, and oc](../../cli/cloudctl_oc_cli.md).

## What's next

* [Setting up a local image registry for a Red Hat OpenShift Container Platform edge cluster](setting_up_ocp_image_registry.md)
* [Installing the agent on Red Hat OpenShift Container Platform Kubernetes edge cluster](installing_ocp_edge_cluster_agent.md)