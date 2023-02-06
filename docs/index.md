---
copyright:
years: 2022 - 2023
lastupdated: "2023-02-05"
layout: page
title: "Agent (anax)"
description: "Open Horizon Anax Documentation"

nav_order: 5
has_children: true
has_toc: false
---

# {{site.data.keyword.edge_notm}} Agent Documentation
{: #anaxdocs}

## [Automatic agent upgrade using policy based node management](node_management_overview.md)

Automatic agent upgrade is a policy-based node management feature that allows an org admin to create node management policies that deploy upgrade jobs to nodes and manages them autonomously. This allows the administrator to ensure that all the nodes in the system are using the intended versions.

## [Instructions for starting an agent in a container on Linux](agent_container_manual_deploy.md)

Use these instructions to start the agent in a container and have more control over the details than that allowed by the `horzion-container` script.

## [{{site.data.keyword.edge_notm}} Agreement Bot APIs](agreement_bot_api.md)

This section contains the {{site.data.keyword.edge_notm}} JSON APIs for the {{site.data.keyword.edge_notm}} system running an Agreement Bot.

## [{{site.data.keyword.edge_notm}} APIs](api.md)

This section contains the {{site.data.keyword.edge_notm}} REST APIs for the {{site.data.keyword.edge_notm}} agent running on an edge node.

## [{{site.data.keyword.edge_notm}} Attributes](attributes.md)

This section contains the definition for each attribute that can be set on the [POST /attribute](./api.md#api-post--attribute) API or the [POST /service/config](./api.md#api-post--serviceconfig) API.

## [Policy Properties](built_in_policy.md)

There are built-in property names that can be used in the policies.

## [Deployment Policy](deployment_policy.md)

A deployment policy is just one aspect of the deployment capability, and is described here in detail.

## [{{site.data.keyword.edge_notm}} Deployment Strings](deployment_string.md)

When defining services in the {{site.data.keyword.edge_notm}} Exchange, the deployment field defines how the service will be deployed.

## [{{site.data.keyword.edge_notm}} Edge Service Detail](managed_workloads.md)

{{site.data.keyword.edge_notm}} manages the lifecycle, connectivity, and other features of services it launches on a device. This section is intended for developers creating {{site.data.keyword.edge_notm}} service container workload definitions.

## [Model Object](model_policy.md)

Model objects in {{site.data.keyword.edge_notm}} are the metadata representation of application metadata objects.

## [Policy based deployment](policy.md)

The policy based deployment support in {{site.data.keyword.edge_notm}} enables containerized workloads (services) to be deployed to edge nodes that are running the {{site.data.keyword.edge_notm}} agent and which are registered to an {{site.data.keyword.edge_notm}} Management Hub.

## [Policy Properties and Constraints](properties_and_constraints.md)

Properties and constraints are the foundation of the policy expressions used to direct {{site.data.keyword.edge_notm}}'s workload deployment engine.

## [Service Definition](service_def.md)

{{site.data.keyword.edge_notm}} deploys services to edge nodes, where those services are comprised of at least one container image and a configuration that conditions how the service executes.
