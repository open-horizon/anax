---
copyright:
years: 2022 - 2023
lastupdated: "2023-01-30"
layout: page
title: "Agent (anax)"
description: "Open Horizon Anax Documentation"

nav_order: 1
has_children: true
has_toc: false
---

# Open {{site.data.keyword.horizon}} Anax Documentation
{: #anaxdocs}

## [Automatic agent upgrade using policy based node management](node_management_overview.md)

## [Instructions for starting an agent in a container on Linux](agent_container_manual_deploy.md)

These instructions are for the user who wants to start the agent in a container with finer control over the details than that allowed by the {{site.data.keyword.horizon}}-container script.

## [{{site.data.keyword.horizon}} Agreement Bot APIs](agreement_bot_api.md)

This document contains the {{site.data.keyword.horizon}} JSON APIs for the {{site.data.keyword.horizon}} system running an Agreement Bot.

## [{{site.data.keyword.horizon}} APIs](api.md)

This document contains the {{site.data.keyword.horizon}} REST APIs for the {{site.data.keyword.horizon}} agent running on an edge node.

## [{{site.data.keyword.horizon}} Attributes](attributes.md)

This document contains the definition for each attribute that can be set on the [POST /attribute](./api.md#api-post--attribute) API or the [POST /service/config](./api.md#api-post--serviceconfig) API.

## [Policy Properties](built_in_policy.md)

There are built-in property names that can be used in the policies.

## [Deployment Policy](deployment_policy.md)

A deployment policy is just one aspect of the deployment capability, and is described here in detail.

## [{{site.data.keyword.horizon}} Deployment Strings](deployment_string.md)

When defining services in the {{site.data.keyword.horizon}} Exchange, the deployment field defines how the service will be deployed.

## [{{site.data.keyword.horizon}} Edge Service Detail](managed_workloads.md)

{{site.data.keyword.horizon}} manages the lifecycle, connectivity, and other features of services it launches on a device. This document is intended for service developers' consumption.

## [Model Object](model_policy.md)

Model objects in Open {{site.data.keyword.horizon}} are the metadata representation of application metadata objects.

## [Policy based deployment](policy.md)

The policy based deployment support in Open {{site.data.keyword.horizon}} enables containerized workloads (aka services) to be deployed to edge nodes that are running the Open {{site.data.keyword.horizon}} agent and which are registered to an Open {{site.data.keyword.horizon}} Management Hub.

## [Policy Properties and Constraints](properties_and_constraints.md)

Properties and constraints are the foundation of the policy expressions used to direct Open {{site.data.keyword.horizon}}'s workload deployment engine.

## [Service Definition](service_def.md)

Open {{site.data.keyword.horizon}} deploys services to edge nodes, where those services are comprised of at least one container image and a configuration that conditions how the service executes.
