---
copyright:
years: 2022 - 2023
lastupdated: "2023-01-24"
title: "Node management policy"
description: Automatic agent upgrade using policy based node management
parent: Agent (anax)
nav_order: 13
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Node management policy
{: #nmp-policy}

## Overview
{: #nmp-overview}

The {{site.data.keyword.edge_notm}} policy based, autonomous node management capability is described [here](./node_management_overview.md).

A Node Management Policy (NMP) is used to define management jobs that run autonomously on the Agent. Once a NMP is added to the Exchange, all of the nodes registered with the Exchange will check to see if they are compatible, and if so, carry out the job(s) defined in the NMP. A NMP operates very similarly to a deployment policy, defined [here](./deployment_policy.md), with the main difference being that a NMP is used to carry out jobs that manage the node in some way. For example, a NMP can be used to automatically upgrade the agent software. Another key difference between a deployment policy and a NMP is that the NMP can specify pattern(s) instead of only properties and constraints. However, patterns are mutually exclusive with properties and constraints.

Use the `hzn exchange nmp new` command to generate an empty NMP.

## Definition
{: #nmp-definition}

Following are the fields in the JSON representation of a NMP:

* `label`: A short description of the node management policy suitable to be displayed in a UI.
* `description`: A longer description of the node management policy.
* `constraints`: Policy constraints as described [here](./properties_and_constraints.md) which refer to node policy properties.
* `properties`: Policy properties as described [here](./properties_and_constraints.md) which a node policy constraint can refer to.
* `patterns`: A list of patterns that this policy is compatible with. This field is mutually exclusive with the properties and constraints fields.
* `enabled`: A boolean to indicate whether this NMP can be deployed to nodes or not.
* `start`: A RFC3339 formatted timestamp for when the NMP should be executed on compatible nodes. The value "now" can be specified to indicate that the NMP should be executed immediately.
* `startWindow`: A value (in seconds) used to randomize the NMP start time in order to reduce the number of nodes running the same NMP simultaneously. The NMP will start within `start` + `startWindow` seconds. For 1000's of agents, it is recommended a startWindow of an hour or more (3600 seconds) be used.
* `agentUpgradePolicy`: A JSON structure to define an automatic agent upgrade job.
  * `manifest`: The name of a manifest that exists in the Management Hub that describes the packages and versions that will be installed. Manifests are described in more detail [here](./agentfile_manifest.md)
  * `allowDowngrade`: A boolean to indicate whether this upgrade job can perform a downgrade to a previous version.

## Example
{: nmp-example}

The following is an example of a NMP json file. In this example, the properties and constraints are being used to specify deployment, so the patterns field is omitted. This NMP will be executed on the next node heartbeat since the start field is set to "now" and it has been enabled. The job being performed in this NMP is an automatic agent upgrade job, so a manifest has been specified in the `agentUpgradePolicy` field. This manifest should contain the software, certificate, and / or config files and versions that the agent will be upgraded to. In this case, if any of the versions specified in the manifest are lower than currently installed, they will be skipped since the `allowDowngrade` field is set to false.

```json
{
  "label": "Sample NMP",
  "description": "A sample description of the NMP",
  "constraints": [
    "myproperty == myvalue"
  ],
  "properties": [
    {
      "name": "myproperty",
      "value": "myvalue"
    }
  ],
  "enabled": true,
  "start": "now",
  "startWindow": 300,
  "agentUpgradePolicy": {
    "manifest": "sample_manifest",
    "allowDowngrade": false
  }
}
```
{: codeblock}

## Adding a NMP to the Exchange
{: nmp-mgmthub}

Adding a NMP to the Management Hub can only be performed by the **org admin** or root user.

Once the json file obtained by running `hzn exchange nmp new` is filled out, adding the file to the Exchange can be performed by running the following command:

```bash
hzn exchange nmp add <nmp-name> --json-file <path-to-nmp>
```
{: codeblock}

### Required flags

* `--json-file, -f`: The path that contains the NMP json.

### Optional Flags

* `--no-constraints`: This flag must be specified if the patterns field is omitted and the constraints field is omitted. By specifying this flag, the user is verifying that this NMP should apply to all nodes that also omitted constraints in their node policy, as well as the nodes who have constraints that match this NMP's properties.

* `--applies-to`: This flag will output a list of nodes that are compatible with this NMP. If the `--dry-run` flag is also specified, the NMP will not be added to the Exchange - this is useful when checking the compatiility of a NMP without the risk of deploying to unintended nodes.

## Listing NMPs currently stored in the Exchange
{: npm-list}

To list all the NMPs that exist in the Exchange, use the following command:

```bash
hzn exchange nmp list [<nmp-name>]
```
{: codeblock}

The `nmp-name` argument is optional and lets the user list a single NMP. This is useful when used with the `--long` flag.

### Optional flags

* `--long, -l`: List the contents of the NMP specified with the `nmp-name` argument, or all the NMPs, if `nmp-name` is omitted.
* `--nodes`: Return a list of compatible nodes for the NMP specified with the `nmp-name` argument, or a map of all NMPs stored in the Exchange with their corresponding compatible node lists, if `nmp-name` is omitted.

## Removing a NMP from the Exchange
{: npm-remove}

Removing a NMP from the Exchange can only be performed by the **org admin** or root user.

To remove a NMP from the Exchange, use the following command:

```bash
hzn exchange nmp remove <nmp-name>
```
{: codeblock}

### Optional flags

* `--force, -f`: Use this flag to skip the 'Are you sure prompt?'

## Enabling and Disabling NMP's currently stored in the Exchange
{: nmp-enable}

Enabling and disabling a NMP stored in the Exchange can only be performed by the **org admin** or root user.

To enable a NMP that exist in the Exchange, use the following command:

```bash
hzn exchange nmp enable <nmp_name>
```
{: codeblock}

### Optional flags

* `--start-time, -s`: Set a new start time for the enabled NMP.
* `--start-window, -w`: Set a new start window for the enabled NMP.

To disable a NMP that exists in the Exchange, use the following command:

```bash
hzn exchange nmp disable <nmp_name>
```
{: codeblock}
