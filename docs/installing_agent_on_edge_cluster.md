---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Installing the agent on an edge cluster
description: Documentation for Installing the edge cluster agent
lastupdated: 2026-03-12
nav_order: 11
parent: Agent (anax)
has_children: true
has_toc: true
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

This content describes how to install the {{site.data.keyword.ieam}} agent on OCP, K3s, or MicroK8s. K3s and MicroK8s are small and lightweight Kubernetes cluster solutions.

Note: These instructions assume that the Hub was configured to listen on an external IP using SSL transport. For more information about setting up an All-in-1 Management Hub, see [here](./all-in-1-setup.md).