---
copyright: Contributors to the Open Horizon project
years: 2022 - 2025
title: Secrets
description: Secrets Management
lastupdated: 2025-05-03
nav_order: 9
parent: Agent (anax)
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# {{site.data.keyword.edge_notm}} Secrets Management with open-horizon
{: #secrets}
The open-horizon secrets management mechanism allows agents to securely recieve the necessary information required to run needed services. Secrets provide a similar capability to user inputs, however the mechanism is designed for keeping the data private. To achieve this, secret data is stored in an additional component, the secret manager which is only directly accessed by the agbot. Currently the secret manager capability is provided using a Vault instance. 
Services that require secrets specify the name of the expected secret in the deployment section of the service description. A description of what the secret is for is optional to include here also. For more about this, see [here](./deployment_string.md).
```
"secrets": {
  "service_token": { "description": "the authentication token for this service" }, 
  "ai_secret": {}
}
```
Then in the deployment policy or pattern, the service secret is bound to a Vault secret name. This allows for nodes running the same service to use different secret values.
```
"secretBinding": [
  {
    "serviceOrgid": "yourOrg",
    "serviceUrl": "my.company.com.service.this-service",
    "serviceArch": "*",
    "serviceVersionRange": "2.3.0",
    "enableNodeLevelSecrets": true,
    "secrets": [
      {"ai_secret": "cloud_ai_secret_name"},
      {"service_token": "user/myUser/service_token"}
    ]
  }
]
```
Vault secrets can be created at either the org or user level. Only admins can create org-level secrets and any node in the org can use the org-level secret in their services. User-level secrets can be created by any user and are only availible to nodes owned by the user who created the secret. Org-level secrets are identified with "secret-name" and user-level secrets are identified by "user/username/secret-name".

## Creating Secrets
{: #creating-secrets}
Secrets can be created and managed with the hzn cli. `hzn secretsmanager secret add` will call the agbot secure-api with the provided secret information and the agbot will create the secret in the secret manager. 

## Secrets on a Node
{: #secrets-on-a-node}
When the agbot needs to send a proposal to an agent which includes service which requires secrets, the secret values to be used are included in the proposal. Once the proposal is accepted, the secrets are copied from the agreement to a file accessible only to the target service. 
The service can find the secrets in a file called `/secretName` where secretName is the name from the service description.

## Node-level secrets
{: #node-level-secrets}
This capability allows for nodes running the same service under the same deployment policy or pattern to use different values for their service secrets. Node specific secrets can also be org or user-level. To use node specific secrets, a service and deployment policy or pattern are published just as with regular org or user level secrets except "allowNodeLevelSecrets" must be set to true in the deployment policy or pattern. 
```
"secretBinding": [
  {
    "serviceOrgid": "yourOrg",
    "serviceUrl": "my.company.com.service.this-service",
    "serviceArch": "*",
    "serviceVersionRange": "2.3.0",
    "enableNodeLevelSecrets": true,
    "secrets": [
      {
        "ai_secret": "cloud_ai_secret_name"
      }
    ]
  }
]
```
Then create the secret `node/<nodeName>/cloud_ai_secret_name`. Alternatively, the node name can be specified with the `-n` flag in the command `hzn secretmanager secret add`. Once this secret exists in vault, the node with the given name will recieve the secret value to use that is specific to this node.

## Updating Secrets
{: #updating-secrets}
Once a secret is deployed to an agent, the secret can be updated in the secret manager and the agbot will send out an agreement update message to get the new secret value to any agents using that secret. If a node is using a regular org or user-level secret and a node-specific secret is added for that node then the agreement update will swtich the node to its node-specific secret. If the node-specific secret is later deleted, the node will get an agreement update to switch back to the regular org or user-level secret. 

## Deleteing Secrets
{: #deleting-secrets}
When a secret is no longer needed by any service, it can be removed from the secret manager with the command `hzn secretmanager secret remove <secretname>`.
