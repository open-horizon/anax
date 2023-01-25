---
copyright:
years: 2022 - 2023
lastupdated: "2023-01-24"
description: Every registered node has a node policy.
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Node policy
{: #node-policy}

Every registered node has a node policy. It consists of [built-in node properties](./built_in_policy.md) which are read-only as well as properties and constraints which can be set by the node owner. Node policies have several sections with repeated fields. This is due to the dual function served by node policies. Firstly, the deployment and top-level sections of node policies are used by agbots to determine service deployment on the nodes. For more on this use of node policy see [here](./policy.md). Secondly, the management and top-level node policy sections are used by the nodes to determine what node management policies they should execute. For more on that application see [here](./node_management.md). The command `hzn policy new` will return an empty node policy with comments on how to fill it out.

The node policies have three pairs of properties and constraints. If they are not empty, the deployment and management properties and constraints will used in determining compatibility. The deployment properties or management properties are merged with the top-level properties to be used for determining eligibility for deployment or management policies respectively. If a property in the deployment or management section shares the name with a property in the top-level section, the value in deployment or management property will be used. The constraints from the deployment or management section will be used for determining eligibility for deployment or management policies respectively. If the relevant section is empty, the top-level constraints will be used. Node policies in the older format that had only one pair of properties and constraints will be interpreted as equivalent to the deployment section of the new policy format.

While top level properties can be used to match deployment policy constraints and management policy constraints, it is recommended that intents for service deployments be placed in the deployment properties and intents for management controls be placed in the management properties.

The following is an example of a node policy.

```json
{
  "properties": [
    {
       "name": "openhorizon.arch",
       "value": "amd64"
    }
  ],
  "constraints": [
       "station > 15"
  ],
  "deployment": {
      "properties": [
        {
           "name": "application-version",
           "value": "1.2.3"
        }
      ],
      "constraints": [
         "equipment == camera"
      ]
  },
  "management": {
      "properties": [
        {
           "name": "latest-version",
           "value": true
        }
      ],
      "constraints": [
         "node1 == true"
      ]
  }
}
```
{: codeblock}
