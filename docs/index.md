# Open Horizon Anax Documentation

## [Instructions for starting an agent in a container on Linux](agent_container_manual_deploy.md)

These instructions are for the user who wants to start the agent in a container with finer control over the details than that allowed by the horizon-container script.

## [Horizon Agreement Bot APIs](agreement_bot_api.md)

This document contains the Horizon JSON APIs for the horizon system running an Agreement Bot.

## [Horizon APIs](api.md)

This document contains the Horizon REST APIs for the Horizon agent running on an edge node.

## [Horizon Attributes](attributes.md)

This document contains the definition for each attribute that can be set on the [POST /attribute](./api.md#api-post--attribute) API or the [POST /service/config](./api.md#api-post--serviceconfig) API.

## [Policy Properties](built_in_policy.md)

There are built-in property names that can be used in the policies.

## [Deployment Policy](deployment_policy.md)

A deployment policy is just one aspect of the deployment capability, and is described here in detail.

## [Horizon Deployment Strings](deployment_string.md)

When defining services in the Horizon Exchange, the deployment field defines how the service will be deployed.

## [Horizon Edge Service Detail](managed_workloads.md)

Horizon manages the lifecycle, connectivity, and other features of services it launches on a device. This document is intended for service developers' consumption.

## [Model Object](model_policy.md)

Model objects in Open Horizon are the metadata representation of application metadata objects.

## [Policy based deployment](policy.md)

The policy based deployment support in Open Horizon enables containerized workloads (aka services) to be deployed to edge nodes that are running the Open Horizon agent and which are registered to an Open Horizon Management Hub.

## [Policy Properties and Constraints](properties_and_constraints.md)

Properties and constraints are the foundation of the policy expressions used to direct Open Horizon's workload deployment engine.

## [Service Definition](service_def.md)

Open Horizon deploys services to edge nodes, where those services are comprised of at least one container image and a configuration that conditions how the service executes.
