---
copyright: Contributors to the Open Horizon project
years: 2022 - 2026
title: Deploying services to your edge cluster
description: Documentation for Deploying services to an edge cluster
lastupdated: 2026-01-30
nav_order: 20
parent: Agent (anax)
---

# Deploying services to your edge cluster

## How to deploy services to an edge cluster
{: #deploying_services}

Setting node policy on this edge cluster can cause deployment policies to deploy edge services here. This content shows an example of doing that.

1. Set some aliases to make it more convenient to run the `hzn` command. (The `hzn` command is inside the agent container, but these aliases make it possible to run `hzn` from this host.)

   ```bash
   cat << 'END_ALIASES' >> ~/.bash_aliases
   alias getagentpod='kubectl -n openhorizon-agent get pods --selector=app=agent -o jsonpath={.items[].metadata.name}'
   alias hzn='kubectl -n openhorizon-agent exec -i $(getagentpod) -- hzn'
   END_ALIASES
   source ~/.bash_aliases
   ```
   {: codeblock}

2. Verify that your edge node is configured (registered with the {{site.data.keyword.ieam}} management hub):

   ```bash
   hzn node list
   ```
   {: codeblock}

3. To test your edge cluster agent, set your node policy with a property that deploys the example helloworld operator and service to this edge node:

   ```bash
   cat << 'EOF' > operator-example-node.policy.json
   {
     "properties": [
       { "name": "openhorizon.example", "value": "nginx-operator" }
     ]
   }
   EOF

   cat operator-example-node.policy.json | hzn policy update -f-
   hzn policy list
   ```
   {: codeblock}

   **Note**:
   * Because the real **hzn** command is running inside the agent container, for any `hzn` commands that require an input file, you need to pipe the file into the command so its content will be transferred into the container.

4. After a minute, check for an agreement and the running edge operator and service containers:

   ```bash
   hzn agreement list
   kubectl -n openhorizon-agent get pods
   ```
   {: codeblock}

5. Using the pod IDs from the previous command, view the log of edge operator and service:

   ```bash
   kubectl -n openhorizon-agent logs -f <operator-pod-id>
   # control-c to get out
   kubectl -n openhorizon-agent logs -f <service-pod-id>
   # control-c to get out
   ```
   {: codeblock}

6. You can also view the environment variables that the agent passes to the edge service:

   ```bash
   kubectl -n openhorizon-agent exec -i <service-pod-id> -- env | grep HZN_
   ```
   {: codeblock}

### Changing what services are deployed to your edge cluster
{: #changing_services}

* To change what services are deployed to your edge cluster, change the node policy:

  ```bash
  cat <new-node-policy>.json | hzn policy update -f-
  hzn policy list
  ```
  {: codeblock}

   After a minute or two the new services will be deployed to this edge cluster.

* **Note**: On some VMs with microk8s, the service pods that are being stopped (replaced) might stall in the **Terminating** state. If that happens, run:

  ```bash
  kubectl delete pod <pod-id> -n openhorizon-agent --force --grace-period=0
  pkill -fe <service-process>
  ```
  {: codeblock}

* If you want to use a pattern instead of a policy to run services on your edge cluster:

  ```bash
  hzn unregister -f
  hzn register -n $HZN_EXCHANGE_NODE_AUTH -p <pattern-name>
  ```
  {: codeblock}