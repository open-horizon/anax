---
copyright:
years: 2022 - 2023
lastupdated: "2023-02-05"
title: "Agent in a Container"
description: Instructions for starting an agent in a container on Linux

parent: Agent (anax)
nav_order: 2
---

## Starting an agent in a container on Linux
{: #container-agent}

These instructions assume that the agent container you want to deploy already exists and is in a container image repository.

### Usage
{: #container-usage}

Use these instructions to start the agent in a container, which provides more control over details than that allowed by the horizon-container script. For a simplified process to getting an agent running see the [agent-install instructions ](https://github.com/open-horizon/anax/tree/master/agent-install){:target="_blank"}{: .externalLink}.

### Prerequisites
{: #container-prereqs}

Docker or Podman needs to be installed on the host device. Review [instructions on installing Docker on Linux](https://docs.docker.com/engine/install/){:target="_blank"}{: .externalLink} or the [instructions on installing Podman on Linux](https://podman.io/getting-started/installation){:target="_blank"}{: .externalLink}

If the management hub you are using uses Secure Socket Layer (SSL) encryption, then you need to have the SSL certificate from the management hub.

### Starting the agent
{: #container-start}

1. Before starting the container, create a configuration file for the agent. The configuration file should include the following:

   * HZN_EXCHANGE_URL=\<address of your exchange\>
   * HZN_FSS_CSSURL=\<css address\>
   * HZN_ORG_ID=\<the org this node should be in\>
   * HZN_EXCHANGE_USER_AUTH=\<exchange username:password\>
   * HZN_AGBOT_URL=\<agbot api address\>

   Optional:

   * HZN_NODE_ID=\<a name for your node\>
   * If this parameter is not included, the node will be assigned a random alphanumeric identifier.
   * (DEPRECATED) HZN_DEVICE_ID=\<a name for your node\>
   * HZN_MGMT_HUB_CERT_PATH=\<path to the ssl certificate file on the host machine\>

2. Prior to starting the container, export the following variables:

   * HZN_AGENT_IMAGE=\<name of the agent container image in the repo\>
     * This should include the url for the repository if it is not the default one.
   * HZN_AGENT_IMAGE_TAG=\<version of the agent container to use\>
   * CONFIG_FILE=\<Location of the configuration file\>
     * The configuration file with the contents from step 1.
   * HZN_MGMT_HUB_CERT_MOUNT="-v \<ssl certificate file on host\>:\<$HZN_MGMT_HUB_CERT_PATH\>"
     * This needs to be set in the configuration file so the agent can find the certificate after it starts.
   * DOCKER_NAME=\<name for the agent container\>
   * HORIZON_AGENT_PORT=\<port number\>
     * The port to expose from the container that hzn will call the agent on. This is typically 8081.
   * DOCKER_ADD_HOSTS="--add-host=$\<host name to add to container hosts file\>"
     * This is only necessary if the exchange or css url will not be resolvable from inside the agent container.

   Example:

   ```bash
   HZN_AGENT_IMAGE=openhorizon/amd64_anax
   HZN_AGENT_IMAGE_TAG=latest
   CONFIG_FILE=/etc/default/horizon
   HZN_MGMT_HUB_CERT_PATH=/etc/default/horizon/agent.crt
   DOCKER_NAME=horizon1
   HORIZON_AGENT_PORT=8081
   ```
   {: codeblock}

3. Start the container by running this docker command:

   ```bash
   docker run $DOCKER_ADD_HOSTS -d \
   --restart always \
   --name $DOCKER_NAME \
   --privileged -p 127.0.0.1:$HORIZON_AGENT_PORT:8510 \
   -e DOCKER_NAME=$DOCKER_NAME \
   -v /var/run/docker.sock:/var/run/docker.sock \
   -v $CONFIG_FILE:/etc/default/horizon:ro $HZN_MGMT_HUB_CERT_MOUNT \
   -v $DOCKER_NAME_var:/var/horizon/ \
   -v $DOCKER_NAME_etc:/etc/horizon/ \
   -v $DOCKER_NAME_fss:/var/tmp/horizon/$DOCKER_NAME \
   $HZN_AGENT_IMAGE:$HZN_AGENT_IMAGE_TAG

   export HORIZON_URL=http://localhost:$HORIZON_AGENT_PORT
   ```
   {: codeblock}

   The docker run command uses the following arguments.

   * "--name $DOCKER_NAME"
     * Names the container something meaningful.
   * "--add-host=$HZN_EXCHANGE_HOSTS"
     * This allows you to add the exchange to the containers host list.
   * "-p 127.0.0.1:$HORIZON_AGENT_PORT:$ANAX_AGENT_PORT"
     * This exposes a port where `hzn` can reach the agent.
   * "-e DOCKER_NAME=$DOCKER_NAME"
     * Tells the agent the container name.
   * "-v /var/run/docker.sock:/var/run/docker.sock"
     * Gives the container access to the docker socket so it can control service containers.
   * "-v $CONFIG_FILE_MOUNT:/etc/default/horizon:ro"
     * The configuration file lets you set variables to tell the agent about itself and the exchange it is to work with.
   * "-v $HZN_MGMT_HUB_CERT_PATH:$HZN_MGMT_HUB_CERT_PATH"
     * Maps the ssl certificate for the exchange into the agent container. The agent will look for the certificate wherever `HZN_MGMT_HUB_CERT_PATH` is set to in the configuration file.
   * "-v $DOCKER_NAME_var:/var/horizon/"
     * This volume will store the database for the agent.
   * "-v $DOCKER_NAME_etc:/etc/horizon/"
     * This volume contains some initial configuration information for the agent.
     * This file in the container image is non-empty so mounting it to a non-empty host folder will cause the agent to fail.
   * "-v $DOCKER_NAME_fss:/var/tmp/horizon/horizon1"
     * This volume is used by the edge sync service for storing models downloaded by the service.
   * "$HZN_AGENT_IMAGE:$HZN_AGENT_IMAGE_TAG"
     * The agent container image that will be started.
   * "export HORIZON_URL=http://localhost:$HORIZON_AGENT_PORT"
     * This exported variable tells the horizon CLI how to reach the agent on the port exposed from the container.

4. To register the agent with a policy, run `hzn policy new > node_pol.json`, then edit the `node_pol.json` file to match the deployment policy for the service you want to deploy.

   ```bash
   source $CONFIG_FILE
   hzn register --policy node_pol.json
   ```
   {: codeblock}

5. To register the agent with a pattern:

   ```bash
   source $CONFIG_FILE
   hzn register -p <pattern name>
   ```
   {: codeblock}
