---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Edge node agents (anax)
description: Open Horizon Edge Node Agent Documentation
lastupdated: 2026-04-08
nav_order: 5
has_children: True
has_toc: False
layout: page
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# {{site.data.keyword.edge_notm}} Agent Documentation
{: #anaxdocs}

## Installing edge clusters

Before you can install an agent, you must install and configure edge clusters first. For more information, see [Installing edge clusters](../../installing/edge_clusters.md).

## Installing an agent on an edge device

* [Installing an agent on an edge device](overview.md)

## Installing an agent on an edge cluster

Install the {{site.data.keyword.edge_notm}} agent on edge clusters.

* [Installing the agent on Red Hat OpenShift Container Platform Kubernetes edge cluster](installing_ocp_edge_cluster_agent.md)
* [Installing the agent on K3s and MicroK8s edge clusters](installing_k3s_microk8s_agent.md)
* [Deploying services to your edge cluster](deploying_services_cluster.md)
* [Configuring a storage class](configuring_storage_class.md)
* [Removing the agent from an edge cluster](removing_agent_from_cluster.md)

## Installing an agent in a container

* [Installing an agent in a container](agent_container_manual_deploy.md)

## Configuring authentication

Configure authentication methods for the anax agent to communicate with the {{site.data.keyword.edge_notm}} management hub.

* [Authenticating to the management hub](authentication_overview.md)
* [Managing secrets](secrets.md)

## Configuring policies

Configure policies for deployment, node management, and service behavior.

* [Policy based deployment](policy.md)
* [JSON fields of a deployment policy](deployment_policy.md)
* [Built-in policy properties](built_in_policy.md)
* [Policy properties and constraints](properties_and_constraints.md)
* [Node policies](node_policy.md)
* [Model objects](model_policy.md)

## Defining and deploying services

Define and deploy services to edge nodes and clusters.

* [Managing the lifecycle of services](managed_workloads.md)
* [Service Definition](service_def.md)
* [Deployment Strings](deployment_string.md)

## Upgrading agents automatically

Automatic upgrade information for the {{site.data.keyword.edge_notm}} agent.

* [Using NMPs to upgrade agents automatically](node_management_overview.md)
* [Node management policies](node_management_policy.md)
* [JSON representation of an upgrade manifest](agentfile_manifest.md)
* [JSON representation of an NMP status](node_management_status.md)

## Advanced features

Advanced features and configurations for the {{site.data.keyword.edge_notm}} agent.

* [High Availability node groups](ha_groups.md)
* [Multi-namespace for cluster agent](agent_in_multi_namespace.md)

## API Reference

API documentation for the {{site.data.keyword.edge_notm}} agent.

* [{{site.data.keyword.horizon}} APIs](api.md)
* [Attributes for {{site.data.keyword.horizon}} POST APIs](attributes.md)
* [Agreement Bot APIs](agreement_bot_api.md)
