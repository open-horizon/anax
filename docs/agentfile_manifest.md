# Automatic Agent Upgrade Manifests

## Overview
The Open Horizon policy based, autonomous node management capability is described [here](./node_management.md). 

A manifest is used to specify files and versions that are used during the autonomous agent upgrade process. 

Use the `hzn nodemanagement manifest new` command to generate an empty manifest. 

## Definition

Following are the fields in the JSON representation of a manifest:

* `softwareUpgrade`: This section is used to define the agent software packages and versions that will be used to upgrade or downgrade the agent during the automatic agent upgrade process.
    * `files`: A list of agent software files stored in the Management Hub. Use the command 'hzn nodemanagement agentfiles list -t agent_software_files' to get a list of available files.
    * `version`: This field specifies the version to use for all the files specified in the `files` section. Specify "latest" to get the most recent version.
* `certificateUpgrade`:
    * `files`: A list of certificates stored in the Management Hub (typically only 1 is specified). Use the command 'hzn nodemanagement agentfiles list -t agent_cert_files' to get a list of available files.
    * `version`: This field specifies the version to use for all the certs specified in the `files` section. Specify "latest" to get the most recent version.
* `configurationUpgrade`:
    * `files`: A list of agent configuration files stored in the Management Hub (typically only 1 is specified). Use the command 'hzn nodemanagement agentfiles list -t agent_config_files' to get a list of available files.
    * `version`: This field specifies the version to use for all the configs specified in the `files` section. Specify "latest" to get the most recent version.

## Example

The following is an example of a manifest json file. This would be a very common case for a user who wants to upgrade a debian-based agent. In the `softwareUpgrade` section, the files include the agent-install.sh script, which is not always necessary to include unless there was a change to the file, and the horizon agent software package which includes the `horizon` and `horizon-cli` packages for an amd64 debian-based agent. The version is set to 2.30.0 which means that only this version will be searched for in the Management Hub. If this version does not exist for **both** files, then the upgrade job will fail, so it is important to make sure and run the `hzn nodemanagement agentfiles` set of commands to check filenames and versioning. Similarily, the `certificateUpgrade` and `configurationUpgrade` sections define the most common case for certificate and config upgrades. Typically, an agent will want to keep up-to-date with the latest cert and config so that it can connect to the Management Hub, but there are cases where the filename or version is different than what is displayed below.

```
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

## Adding a manifest to the Management Hub
Adding a manifest to the Management Hub can only be performed by the admins of the system - both the **hub admin** and the **org admin** (as well as root).

Once the json file obtained by running `hzn nm manifest new` is filled out, adding the file to the hub can be performed by running the following command:

```
hzn nm manifest add --type agent_upgrade_manifests --id <manifest-name> --json-file <path-to-manifest>
```

**Required Flags**:  
 * `--type, -t`: The type of manifest that is being added. For now, the only valid value to put here is `agent_upgrade_manifests` since the only supported manifests are for automatic agent upgrades.

 * `--id, -i`: The name of the manifest that is being added. This will be the name that gets used in the NMP's and the name that is reflected when listing manifests in the below command.

 * `--json-file, -f`: The path to the that contains the manifest json.

 When adding a manifest to the Management Hub, the files and versions will be compared to the values stored in the Management Hub. Attempting to add a manifest that specifies a file and/or version that does not exist in the Management Hub will result in an error.

 To obtain a list of files and versions, run: `hzn nm agentfiles list`.

## Listing manifests currently stored in the Management Hub
To list all the manifests that exist in the Management Hub, use the following command:
```
hzn nm manifest list 
```

**Optional Flags**:  
 * `--type, -t`: The type of manifest to list. For now, the only valid value to put here is `agent_upgrade_manifests` since the only supported manifests are for automatic agent upgrades. This flag is required when using the `--long` flag described below

 * `--id, -i`: The name of a specific manifest. This flag is required when using the `--long` flag described below

 * `--long, -l`: Display the contents of the manifest specified using the `--type` and `--id` flags.

## Removing a manifest from the Management Hub
Removing a manifest from the Management Hub can only be performed by the admins of the system - both the **hub admin** and the **org admin** (as well as root).

To remove a manifest from the Management Hub, use the following command:
```
hzn nm man remove
```

**Required Flags**:  
 * `--type, -t`: The type of manifest that is being removed. For now, the only valid value to put here is `agent_upgrade_manifests` since the only supported manifests are for automatic agent upgrades.

 * `--id, -i`: The name of the manifest that is being removed.
 
**Optional Flags**:  
 * `--force, -f`: Use this flag to skip the 'Are you sure prompt?'