---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Installing the agent on an edge cluster
description: Installing the Open Horizon agent on edge clusters
lastupdated: 2026-04-08
nav_order: 2
parent: Edge node agents (anax)
has_children: true
has_toc: false
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Installing the agent on an edge cluster
{: #installing_agent_on_edge_cluster}

After you have installed and configured your edge cluster, you can install the {{site.data.keyword.edge_notm}} agent on it. The agent enables the edge cluster to register with the management hub and receive workload deployments. You can install the {{site.data.keyword.ieam}} agent on {{site.data.keyword.open_shift_cp}}, K3s, or MicroK8s. K3s and MicroK8s are small and lightweight Kubernetes cluster solutions.

**Note**: These instructions assume that the management hub is configured to listen on an external IP using SSL transport. For more information about setting up an All-in-1 management hub, see [here](./all-in-1-setup.md).
{:shortdesc}
