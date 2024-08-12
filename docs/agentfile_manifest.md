---
copyright:
years: 2022 - 2023
lastupdated: "2023-08-29"
description: Automatic Agent Upgrade manifests
title: "Automatic Agent Upgrade manifests"

parent: Agent (anax)
nav_order: 3
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Automatic agent upgrade manifests
{: #upgrade-manifests}

## Overview

The {{site.data.keyword.edge_notm}} policy based, autonomous node management capability is described in the [node management policy](./node_management_policy.md) documentation.

A manifest is used to specify files and versions that are used during the agent upgrade process.

Here is a video showing the agent auto-upgrade feature:

{% include youtubePlayer.html id="f0emDR9AIGg" %}

## Definition
{: #manifest-def}

Following are the fields in the JSON representation of a manifest:

* `softwareUpgrade`: This section is used to define the agent software packages and versions that will be used to upgrade or downgrade the agent during the automatic agent upgrade process.
  * `files`: A list of agent software files stored in the Management Hub. Use the command `hzn nodemanagement agentfiles list -t agent_software_files` to retrieve a list of available files.
  * `version`: This field specifies the version to use for all the files specified in the `files` section. Specify `latest` to get the most recent version.
* `certificateUpgrade`:
  * `files`: A list of certificates stored in the Management Hub (typically only one is specified). Use the command `hzn nodemanagement agentfiles list -t agent_cert_files` to retrieve a list of available files.
  * `version`: This field specifies the version to use for all the certificates specified in the `files` section. Specify `latest` to get the most recent version.
* `configurationUpgrade`:
  * `files`: A list of agent configuration files stored in the Management Hub (typically only one is specified). Use the command 'hzn nodemanagement agentfiles list -t agent_config_files' to retrieve a list of available files.
  * `version`: This field specifies the version to use for all the configurations specified in the `files` section. Specify `latest` to get the most recent version.

## Example
{: #manifest-example}

The following is an example of a manifest json file. This would be a common case for a user who wants to upgrade Debian-based agents. In the `softwareUpgrade` section, the files include the agent-install.sh script and the {{site.data.keyword.horizon_agent}} software packages. The agent-install.sh script is not always necessary to include unless there was a change to the file.  The {{site.data.keyword.horizon_agent}} software package includes the `horizon` and `horizon-cli` packages for an amd64 Debian-based agent. The version is set to 2.30.0, which means that only this version will be searched for in the Management Hub. If this version does not exist for both files, then the upgrade job will fail, so it is important to make sure and run the `hzn nodemanagement agentfiles` set of commands to check filenames and versioning. Similarily, the `certificateUpgrade` and `configurationUpgrade` sections define the most common case for certificate and config upgrades. Typically, an agent will want to keep up-to-date with the latest cert and config so that it can connect to the Management Hub, but there are cases where the filename or version is different than what is displayed in the json example.

```json
{
  "softwareUpgrade": {
    "files": [
      "agent-install.sh",
      "horizon-agent-linux-deb-amd64.tar.gz"
    ],
    "version": "2.30.0"
  },
  "certificateUpgrade": {
    "files": [
      "agent-install.crt"
    ],
    "version": "latest"
  },
  "configurationUpgrade": {
    "files": [
      "agent-install.cfg"
    ],
    "version": "latest"
  }
}
```
{: codeblock}

## Adding a manifest to the Management Hub
{: #manifest-add}

Adding a manifest to the Management Hub can only be performed by the **org admin** or the root user.

After the json file is obtained by running `hzn nm manifest new` and is filled out, run the following command to add the file to the hub:

```bash
hzn nm manifest add --type agent_upgrade_manifests --id <manifest-name> --json-file <path-to-manifest>
```
{: codeblock}

### Required flags

* `--type, -t`: The type of manifest that is being added. For now, this value must be `agent_upgrade_manifests` since the only supported manifests are for automatic agent upgrades.
* `--id, -i`: The name of the manifest that is being added. This will be the name that gets used in the NMP and the name that is reflected when listing manifests in the below command.
* `--json-file, -f`: The path to the that contains the manifest json.

When a manifest is added to the Management Hub, the files and versions are compared to the values stored in the Management Hub. Any attempt to add a manifest that specifies a file or version that does not exist in the Management Hub results in an error.

To obtain a list of files and versions, run: `hzn nm agentfiles list`.

## Listing manifests currently stored in the Management Hub
{: #manifest-list}

To list all the manifests that exist in the Management Hub, run the following command:

```bash
hzn nm manifest list
```
{: codeblock}

### Optional flags

* `--type, -t`: The type of manifest to list. For now, this value must be `agent_upgrade_manifests` since the only supported manifests are for automatic agent upgrades. This flag is required when using the `--long` flag described below.
* `--id, -i`: The name of a specific manifest. This flag is required when using the `--long` flag described below
* `--long, -l`: Display the contents of the manifest specified using the `--type` and `--id` flags.

## Removing a manifest from the Management Hub
{: #manifest-remove}

Removing a manifest from the Management Hub can only be performed by the **org admin** or root user.

To remove a manifest from the Management Hub, run the following command:

```bash
hzn nm man remove
```
{: codeblock}

### Required flags

* `--type, -t`: The type of manifest that is being removed. Currently, this value must be `agent_upgrade_manifests` because the only supported manifests are for automatic agent upgrades.
* `--id, -i`: The name of the manifest that is being removed.

### Optional flags

* `--force, -f`: Use this flag to skip the confirmation prompt.
