---
copyright: Contributors to the Open Horizon project
years: 2022 - 2025
title: Policy based deployment
description: Policy based deployment enables containerized workloads (services) to be deployed to edge nodes
lastupdated: 2025-05-03
nav_order: 16
parent: Agent (anax)
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Policy based deployment
{: #policy-deployment}

The policy based deployment support in {{site.data.keyword.edge_notm}} enables containerized workloads (services) to be deployed to edge nodes that are running the {{site.data.keyword.edge_notm}} agent and which are registered to an {{site.data.keyword.edge_notm}} Management Hub.
The deployment engine (implemented inside the {{site.data.keyword.edge_notm}} Agbot) uses policy to autonomously determine where services should be deployed, undeployed or re-deployed.
As nodes, models, services and deployment policies are added, updated or removed from the management hub, the deployment engine will automatically react to the change.
An adminstrator never has to interact directly with the deployment engine, it just works quietly in the background ensuring that the edge computing environment it is managing has services deployed where they should be.

To accomplish this there are four kinds of policy in {{site.data.keyword.edge_notm}}; node policy, deployment policy, service policy, and model policy.
Each kind of policy is composed of properties and constraints which are described in more detail in the [Properties and Constraints]](./properties_and_constraints.md) documentation.

## Node policy
{: #node-policy}

Policy can be attached to a node.
The node owner can provide this at registration time, and it can be changed at any time directly on the node or centrally by a management hub administrator.
When node policy is changed centrally, it is reflected to the node the next time the node heartbeats to the management hub.
When node policy is changed directly on the node, the changes are reflected immediately to the management hub so that service and model deployment can be reevaluated immediately.
By default, a node has some [built-in properties](./built_in_policy.md) that reflect memory, architecture, and number of CPUs.
It can optionally contain any arbitrary properties; for example, the product model, attached devices, software configuration, or anything else deemed relevant by the node owner.
Node policy constraints can be used to restrict which services are permitted to run on this node and which node management policies apply to the node.
Each node has only one policy that contains all the properties and constraints that are assigned to that node.

The JSON representation of node policy and details of how it is used are elaborated [here](./node_policy.md):

## Service policy
{: #service-policy}

Service policy is an optional feature.

Like nodes, services can express policy and have some [built-in properties](built_in_policy.md) as well.
This policy is created by the service developer and published to the exchange when the service is published.
Service policy properties could state characteristics of the service code that node policy authors might find relevant.
Service policy constraints can be used to restrict where, and on what type of devices, this service can run.
For example, the service developer can assert that this service requires a particular hardware setup such as CPU / GPU constraints, memory constraints, specific sensors, actuators, or other peripheral devices.
Typically, properties and constraints remain static for the life of the service because they describe aspects of the service implementation.
In expected usage scenarios, a change to one of these is usually coincident with code changes that necessitate a new service version.
Deployment policies are used to capture the more dynamic aspects of service deployment that arise from business requirements.

The JSON representation of service policy contains properties and constraints as defined [here](./properties_and_constraints.md):

```json
{
 "properties": [],
 "constraints": []
}
```
{: codeblock}

## Deployment policy
{: #deploy-policy}

Deployment policy drives service deployment.
Like the other policy types, it contains a set of properties and constraints, but it also contains other things.
For example, it explicitly identifies a service to be deployed, and it can optionally contain configuration variable values, service rollback versions, and node health configuration information.
The Deployment policy approach for configuration values is powerful because this operation can be performed centrally, with no need to connect directly to the edge node.

Administrators create deployment policies, and the {{site.data.keyword.edge_notm}} deployment engine uses that policy to locate all of the nodes that match the defined constraints and deploys the specified service to those nodes.
Service rollback versions instruct the deployment engine which service versions should be deployed if a higher version of the service fails to deploy.
The node health configuration indicates how the deployment engine should gauge the health (heartbeats and management hub communication) of a node before determining if the node is out of policy.

Because deployment policies capture the more dynamic, business-like service properties and constraints, they are expected to change more often than service policy. Their lifecycle is independent from the service they refer to, which gives the policy administrator the ability to state a specific service version or a version range.
The deployment engine merges service policy and deployment policy (by performing a logical AND of the two policies), and then attempts to find nodes whose policy is compatible with that merged policy.

The JSON representation of deployment policy is more complex than node and service policy, as shown [here](./deployment_policy.md).

## Model policy
{: #model-policy}

Machine learning (ML)-based services require specific trained models to operate correctly.
{{site.data.keyword.edge_notm}} provides the ability to deploy models independently from services.
In ML use cases, the model tends to be large and change more more often than the algorithmic code which uses the model to analyze data.
Model policy enables the administrator to deploy specific models on the same, or a subset of, nodes where the services that use the mode have been placed.
The purpose of model policy is to further narrow the set of nodes where a given service is deployed, which enables a subset of those nodes to receive a specific model object.
This is useful when you want to test a new model on a subset of nodes where the algorithmic service is deployed.

The JSON representation of model policy is more complex than node and service policy, as shown [here](./model_policy.md).
