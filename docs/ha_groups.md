---
copyright: Contributors to the Open Horizon project
years: 2025
title: High Availability node groups
description: High Availability node groups
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

# High availability node groups
{: #ha-node-groups}

## Overview

High availability (HA) node groups allow an administrator or node owner to group nodes together that are running the same service to ensure the service stays running on at least one of the nodes at all times. HA grouping is enforced by the agbot, which will only allow one node in a group to perform an upgrade at a time. Nodes in an HA group still complete agent and service upgrades in a coordinated manner. Nodes can only be in one HA group at a time.

## Creating HA node groups

1. To generate a template for creating HA node groups run:

   ```bash
   hzn exchange hagroup new
   ```
   {: codeblock}

2. Copy the output to a file and fill in the required fields.  In this example, `new-hagroup.json`

   ```json
   {
     "name": "",             /* Optional. The name of the HA group. */
     "description": "",      /* A description of the HA group. */
     "members": [            /* A list of node names that are members of this group. */
       "node1",
       "node2"
     ]
   }
   ```
   {: codeblock}

3. Create the group in the exchange:

   ```bash
   hzn exchange hagroup add --json-file <new-hagroup.json>
   ```
   {: codeblock}

While any user can create an HA group, only org administrators or the node's owner can add a node to a group. HA groups become effective as soon as the group is created in the exchange.

## Listing nodes in a HA group

To list the nodes in a HA group run:

```bash
hzn exchange hagroup list <group-name>
```
{: codeblock}

## Adding nodes to a HA group

After the HA group exists in the exchange, nodes can be added to it in several ways.

- At node registration, an HA group for the node can be specified with the flag `--ha-group=HA-GROUP` where `HA-GROUP` is the name of the existing HA group.

- A node (that previously exists in the exchange, need not be registered) can be added to a group with the command:

  ```bash
  hzn exchange hagroup member add <group name> --node=NODE
  ```
  {: codeblock}

  where `NODE` is the node id of the node to add.

Additionally, the HA group template used to originally create the group can be updated with the id of the node to add, then republished with:

```bash
 hzn exchange hagroup add --json-file new-hagroup.json
```
{: codeblock}

**Note**: If a node is already in an HA group, it must be removed from the group before it can be added to another.

## Removing nodes from HA groups

Remove nodes from an HA group with this command:

```bash
hzn exchange hagroup member remove <group name> --node=NODE
```
{: codeblock}

Lastly, to remove the entire HA group from the exchange:

```bash
hzn exchange hagroup remove <group name>
```
{: codeblock}

## Limitations

- This feature is only supported for device type nodes. Cluster nodes are expected to use kubernetes operator capabilities to ensure service availability.
- Services, with current agreements that are running on a node, are still upgraded, even if other nodes in its HA group are offline.
- If a node is added to an HA group while the node has already started a upgrade, the HA group membership of the node is not enforced until the ongoing service or agent upgrade has completed.
