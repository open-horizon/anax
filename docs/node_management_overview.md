# Overview of Node Management

## Automatic Agent Upgrade

The Automatic Agent Upgrade is a policy-based node management feature that allows an org admin to create node management policies that deploy upgrade jobs to nodes and manages them autonomously. This allows the admin to ensure that all the nodes in the system are on their intended versions.

### How to set-up an Automatic Agent Upgrade policy

1. Create a manifest
    - The first step is to create a manifest using the command below and saving it to a file. In this example, the file is named `manifest.json`
    ```
    hzn nodemanagement manifest new > manifest.json
    ```
    **Note**: For more detailed information about manifests, see [here](./agentfile_manifest.md)

2. Edit the manifest file
    - Using a text editor, edit the manifest file to include all the files and versions needed for the upgrade job. The file should look similar to the example below.
    ```
    {
        "softwareUpgrade": {
            "files": [
                "agent-install.sh",
                "horizon-agent-linux-deb-amd64.tar.gz"
            ],
            "version": "latest"
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
    **Note**: The files and versions must correspond to objects stored on the Management Hub. Use the `hzn nodemanagement agentfiles list` command to get a list of available files and versions.

3. Add the manifest file to the Management Hub

    Once the manifest is populated with all the necessary files and versions, the next step is to add the manifest to the Management Hub using the command below. In this example, the name of the manifest in the Hub will be `sample_manifest` and the file that contains the manifest is called `manifest.json`  
    ```
    hzn nodemanagement manifest add -t agent_upgrade_manifests -i sample_manifest -f manifest.json
    ```
  

4. Create a Node Management Policy
    - Use the following command to create a node management policy (NMP) and save it to a file. An NMP is responsible for determining which nodes the upgrade will be performed on. In this example, the file is named `nmp.json`
    ```
    hzn exchange nmp new > nmp.json
    ```
    **Note**: For more detailed information about NMP's, see [here](./node_management_policy.md)

5. Edit the NMP file
    - Using a text editor, edit the NMP file to include all the files and versions needed for the upgrade job. The file should look similar to the example below.
    ```
    {
        "label": "Sample NMP",
        "description": "A sample description of the NMP",
        "constraints": [
            "myproperty == myvalue"
        ],
        "properties": [
            {
            "name": "myproperty",
            "value": "myvalue""
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

6. Add the NMP to the Exchange
    - Once the NMP is populated with all the necessary information, the next step is to add the NMP to the Exchange using the command below. In this example, the name of the NMP in the Hub will be `sample_nmp` and the file that contains the NMP is called `nmp.json` 
    ```
    hzn exchange nmp add sample_nmp -f nmp.json
    ```
    **Note**: The following command can be used to check which nodes the NMP will deploy to before publishing the NMP to the Exchange. This is useful to ensure the NMP does not deploy to any unintended nodes.
    ```
    hzn exchange nmp add sample_nmp -f nmp.json --dry-run --applies-to
    ```

7. Observe the status of the upgrade job (optional)
    - Now that the NMP has been published, it will soon get picked up by the worker on the agent to perform the upgrade. The status of the NMP can then be observed using the following command.
    ```
    hzn exchange nmp status sample-nmp
    ```
    OR
    ```
    hzn exchange node management status {node-name}
    ```
