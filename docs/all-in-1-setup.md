---
copyright:
years: 2022 - 2023
lastupdated: "2023-04-02"
title: "How to set-up the Open Horizon All-in-1 Horizon Management Hub for edge clusters"

parent: Agent (anax)
nav_order: 19
---
# How to set-up the Open Horizon All-in-1 Horizon Management Hub for edge clusters

## Deploy the Open Horizon All-in-1 Horizon Management Hub

1. Install the Management Hub without the native agent:

   a. Set HZN_LISTEN_IP to an external IP that can be reached outside of the local network. The Management Hub needs to be listening on this IP address and it should be reachable by the edge cluster devices.

   ```bash
   export HZN_LISTEN_IP=<your-external-ip>
   ```
   {: codeblock}

   b. Enable SSL connections by setting HZN_TRANSPORT to https. This step is optional but recommended to ensure the traffic between the cluster agent and the Hub is secure.

   ```bash
   export HZN_TRANSPORT=https
   ```
   {: codeblock}

   c. Install the management hub

   ```bash
   curl -sSL https://raw.githubusercontent.com/open-horizon/devops/master/mgmt-hub/deploy-mgmt-hub.sh | bash -s -- -A
   ```
   {: codeblock}

2. The end of the command output will include a summary of steps performed. In step 2, you will find a list of passwords and tokens that were automatically generated. These include the exchange root password and Hub admin password, so it is important you write these down somewhere safe.

   The final two lines of output will list the HZN_ORG_ID and HZN_EXCHANGE_USER_AUTH environment variables and prompt you to export them. Exporting these will allow us to continue the tutorial without the need to specify them later.

   If you would like to use different credentials to connect your agent, use the hzn exchange org create and hzn exchange user create commands to add a new org and user, respectively. Export these variables and/or take note of them.

   **Notes**:
   - For more information about the all-in-1 hub, visit the [all-in-one deployment page](/docs/mgmt-hub/docs/).
   - The following section on installing and configuring a cluster currently has separate directions for two solutions - k3s and microk8s.  Please choose one of those two supported solutions or use your own and translate our directions accordingly.

3. Now that the Management Hub is up and running, an edge cluster agent can be installed following the instructions [on the Cluster Agent installations instructions page](/docs/anax/docs/cluster_install/).
