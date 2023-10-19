---
copyright:
years: 2022 - 2023
lastupdated: "2023-10-19"
title: "Deployment Policy"
description: Description of Deployment policy json fields

parent: Agent (anax)
nav_order: 8
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Deployment Policy
{: #deployment-policy}

The {{site.data.keyword.edge_notm}} policy based, autonomous deployment capability is described [here](./policy.md).
A deployment policy is just one aspect of the deployment capability, and is described here in detail.

Use the `hzn exchange deployment new` command to generate an empty deployment policy.
The `hzn dev service new` command will also generate a deployment policy along with a new service.

Use the `hzn deploycheck` command to evaluate the compatibility of your deployment policy with the node where you want the service to be deployed.

Following are the fields in the JSON representation of a deployment policy:

- `label`: A short description of the deployment policy suitable to be displayed in a UI. This field is not required.
- `description`: A longer description of the deployment policy. This field is not required.
- `services`: A list of services to be deployed. There must be at least one service in the list.
  - `name`: The name (URL) of a service to be deployed. This is the same value as found in the `url` field [here](./service_def.md).
  - `org`: The organization in which the service in `name` is defined.
  - `arch`: The hardware architecture of the service in `name`, or `*` to indicate any compatible architecture. This is the same value as found in the `arch` field [here](./service_def.md).
  - `clusterNamespace`: Namespace that this service will be deployed to. Only apply to cluster service.
  - `serviceVersions`: A list of versions of the service. At least one version must be specified.
    - `version`: One of the versions of the service in `name`. This is the same value as found in the `version` field [here](./service_def.md).
    - `priority`: The relative priority of deploying this version over another version in the list of service versions.
      - `priority_value`: The priority value assigned to this version. Priority is expressed in human terms, where a lower ordinal value means higher priority. Priority values within the list are not required to be sequential, just unique within the list. When deploying a service, {{site.data.keyword.edge_notm}} will attempt to deploy the highest priority version first. If the service is not successfully started, the next highest version will be attempted.
      - `retries`: The number of times to retry starting a failed service.
      - `retry_durations`: The number of seconds (elapsed time) in which the indicated number of `retries` must occur before giving up and moving on to the next highest priority service version.
  - `nodeHealth`: For nodes that are expected to remain network connected to the management, these settings indicate how aggressive the Agbot should be in determining if a node is out of policy.
    - `missing_heartbeat_interval`: The number of seconds a heartbeat can be missed (from the perspective of the management hub) until the node is considered missing. When a node is detected as missing, its agreements are cancelled by the Agbot.
    - `check_agreement_status`: The number of seconds between checks (by the management hub) to verify that the node still has an agreement for this service.
- `properties`: Policy properties as described [here](./properties_and_constraints.md) which a node policy constraint can refer to.
- `constraints`: Policy constraints as described [here](./properties_and_constraints.md) which refer to node policy properties.
- `userInput`: This section is used to set service variables for any service (including this service) that is deployed as a result of deploying this service.
  - `serviceUrl`: The name of the service to be configured. This is the same value as found in the `url` field [here](./service_def.md).
  - `serviceOrgid`: The organization in which the service in `serviceUrl` is defined.
  - `serviceArch`: The hardware architecture of the service in `serviceUrl`, or `*` to indicate any compatible architecture. This is the same value as found in the `arch` field [here](./service_def.md).
  - `serviceVersionRange`: A version range indicating the set of service versions to which this variable setting should be applied.
  - `inputs`: A list of service variables to set.
    - `name`: The name of the variable. This is the same as a variable name found in `userInputs` as defined [here](./service_def.md).
    - `value`: The value to be assigned to the variable. Service variables are typed as described in `userInputs` defined [here](./service_def.md).
- `secretBinding`: This section is used to bind secret names defined in the service with the secret names in the secret provider. The secret value will be retrived from the secret provider and passed to the service container at the deployment time. The secret value is used by the service container to access other applications.
  - `serviceUrl`: The name of the service. It can be the top level services defined in the `services` attribute or one of its dependency services. This is the same value as found in the `url` field [here](./service_def.md).
  - `serviceOrgid`: The organization in which the service in `serviceUrl` is defined.
  - `serviceArch`: The hardware architecture of the service in `serviceUrl`, or `*` to indicate any compatible architecture. This is the same value as found in the `arch` field [here](./service_def.md).
  - `serviceVersionRange`: A version range indicating the set of service versions to which this secret binding should be applied.
  - `secrets`: A list of secret bindings. Each elelment is a map of string keyed by the name of the secret in the service. The value is the name of the secret in the secret provider. The valid formats for the secret provider secret names are: `<secretname>` for the organization level secret; `user/<username>/<secretname>` for the user level secret.

The following is an example of a deployment policy that deploys a service called `my.company.com.service.this-service`.
The service is defined within organization `yourOrg`.
This policy will deploy the service to any node which matches one of the architectures for which the service is defined, and is also compatible with nodes that have the property `aNodeProperty` set to `someValue`.
Two versions of the service are mentioned, with version `2.3.1` having a higher priority for deployment than version `2.3.0`.
The deployed service is dependent on service `my.company.com.service.other` which has a variable `var1` that needs to be set in order for it to deploy correctly.
Both `2.3.0` and `2.3.1` versions of the services have a secret `ai_secret` defined that the service container will use to access an AI service on the cloud once the secret provider secret name is bound to it. The policy binds it to a secret provider secret named `cloud_ai_secret_name`.

```json
{
  "label": "something short for a UI to display",
  "description": "a longer explanation of what this policy does",
  "service": {
    "name": "my.company.com.service.this-service",
    "org": "yourOrg",
    "arch": "*",
    "serviceVersions": [
      {
        "version": "2.3.0",
        "priority":{
          "priority_value": 3,
          "retries": 1,
          "retry_durations": 1800
        }
      },
      {
        "version": "2.3.1",
        "priority":{
          "priority_value": 2,
          "retries": 1,
          "retry_durations": 3600
        }
      }
    ],
    "nodeHealth": {
      "missing_heartbeat_interval": 60,
      "check_agreement_status": 60
    }
  },
  "properties": [
      {
          "name": "prop1",
          "type": int,
          "value": 11
      }
  ],
  "constraints": [
    "aNodeProperty==someValue"
  ],
  "userInput": [
    {
      "serviceOrgid": "serviceOrg",
      "serviceUrl": "my.company.com.service.other",
      "serviceArch": "*",
      "serviceVersionRange": "1.10.2",
      "inputs": [
        {
          "name": "var1",
          "value": "astring"
        }
      ]
    }
  ],
  "secretBinding": [
    {
      "serviceOrgid": "yourOrg",
      "serviceUrl": "my.company.com.service.this-service",
      "serviceArch": "*",
      "serviceVersionRange": "2.3.0",
      "secrets": [
        {
          "ai_secret": "cloud_ai_secret_name"
        }
      ]
    }
  ]
}
```
{: codeblock}
