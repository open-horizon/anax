---
copyright:
years: 2022 - 2023
lastupdated: "2023-02-05"
title: "Policy Properties"
description: Built in Policy Properties

parent: Agent (anax)
nav_order: 7
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Policy properties
{: #builtin}

Built-in property names can be used in policies.
See [Properties and Constraints](./properties_and_constraints.md) for an explanation of properties and constraints in general.
For the node properties, the agent will introspect the node for the values of those properties.
The user-defined policies (deployment policy, model policy, service policy) need to add constraints on these properties if needed.

## Built-in properties
{: #builtin-props}

### Built-in node policy properties

| **Name** | **Description** | **Possible values** |
| ----- | ----- | ----- |
| openhorizon.cpu | the number of CPUs (from /proc/cpuinfo file) | `int` for example 4 |
| openhorizon.memory| the amount of memory in MBs (from /proc/meminfo) | `int` for example 1024 |
| openhorizon.arch| the hardware architecture of the node (from GOARCH) | `string` for example amd64 |
| openhorizon.hardwareId| the device serial number if it can be found (from /proc/cpuinfo). A generated Id otherwise. | `string` |
| openhorizon.allowPrivileged| a property set to determine if privileged services may be run on this device. Can be set by user, default is false. This is the only writable node property | `boolean` |
| openhorizon.kubernetesVersion| Kubernetes version of the cluster the agent is running in | `string` for example 1.18 |
| openhorizon.operatingSystem | the operating system the agent is running on. If the agent is containerized, this will be the host os | `string` for example ubuntu |
| openhorizon.containerized | this indicates if the agent is running in a container or natively | `boolean` |
{: caption="Table 1. {{site.data.keyword.edge_notm}} built-in node properties" caption-side="top"}

**Note: Provided properties (except for allowPrivileged) are read-only; the system ignores node policy updates and built-in properties changes.

### Built-in service policy properties

| **Name** | **Description** | **Possible values** |
| ----- | ----- | ----- |
| openhorizon.service.url | the unique name of the service (comes from `url` field of service definition) | `string` for example `https://someOrg/someService` |
| openhorizon.service.name | the unique name of the service (comes from `url` field of service definition) | `string` for example MyService |
| openhorizon.service.org | the multi-tenant org where the service is defined (comes from `org` field of service definition) | `string` for example MyOrg |
| openhorizon.service.version | the version of a service using the same semantic version syntax (comes from `version` field of service definition) | `string` for example 1.1.1 |
| openhorizon.service.arch | the hardware architecture of the node this service can run on (comes from `arch` field of service definition) | `string` for example amd64 |
| openhorizon.allowPrivileged | does the service use workloads that require privileged mode or net==host to run. Can be set by user. It is an error to set it to false if service introspection indicates that the service uses privileged features. (comes from `deployment.services.someServiceName.privileged` field of service definition) | `boolean` |
{: caption="Table 2. {{site.data.keyword.edge_notm}} built-in service properties" caption-side="top"}
