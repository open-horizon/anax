---
copyright:
years: 2022 - 2023
lastupdated: "2023-02-22"
title: "Agent in a Container"
description: Instructions for starting an agent in a container

parent: Agent (anax)
nav_order: 2
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Starting the {{site.data.keyword.horizon}} agent in a container on Linux or {{site.data.keyword.macOS_notm}}
{: #container-agent}

There are several techniques to start the agent in a container providing different levels of ease and flexibility.

* Option 1 - The [agent-install instructions ](https://github.com/open-horizon/anax/tree/master/agent-install){:target="_blank"}{: .externalLink} provide a `--container` option to download and launch the agent in a container.
* Option 2 - If you have already installed the {{site.data.keyword.horizon}} CLI package, it installed a `horizon-container` script onto your system that can be used to start the anax in container.
* Option 3 - Manually start the docker container with custom parameters.

## Prerequisites
{: #container-prereqs}

Docker or Podman needs to be installed on the host device. Review [instructions on installing Docker on Linux ](https://docs.docker.com/engine/install/){:target="_blank"}{: .externalLink} or the [instructions on installing Podman on Linux ](https://podman.io/getting-started/installation){:target="_blank"}{: .externalLink}. {{site.data.keyword.macOS_notm}} users may require a Docker Desktop license, if necessary, and install the most recent version of Docker on your device. For more information, see the [Docker installation for Mac ](https://docs.docker.com/docker-for-mac/install/){:target="_blank"}{: .externalLink} documentation.

If the management hub you are using uses Secure Socket Layer (SSL) encryption, then you need to have the SSL certificate from the management hub.

## Option 1 - `agent-install.sh --container`

For a simplified process to getting an agent running, see the `--container` option in [agent-install instructions ](https://github.com/open-horizon/anax/tree/master/agent-install){:target="_blank"}{: .externalLink}.

## Option 2 - `horizon-container start`

Run the `horizon-container start` command to start the agent in a container.

```text
$ horizon-container -?
Usage: /usr/bin/horizon-container {start|stop|update} [index-num] [default-file]
  start:  pull the latest horizon docker image and start it
  stop:   unregister the node and stop/remove the horizon docker container
  update: stop the horizon container (w/o unregistering), pull the latest docker image, and start it. Any running services will remain running.

Arguments:
  index-num:      an integer number identifying this instance of horizon when running multiple horizon containers on the same host. Default is 1.
  default-file:   a default file to use to set common environment variables for the horizon agent like HZN_EXCHANGE_URL, HZN_FSS_CSSURL, HZN_AGBOT_URL, HZN_DEVICE_ID, HZN_NODE_ID, HZN_AGENT_PORT and HZN_MGMT_HUB_CERT_PATH. If not specified and /etc/default/horizon exists on the host, that will be used.
```
{: codeblock}

## Option 3 - Manual `docker run` instructions (Linux only)

Use these docker run instructions to start the agent in a container, which provides more control over details than that allowed by the horizon-container script or agent-install.sh script.

### Starting the agent
{: #container-start}

1. Before starting the container, create a configuration file for the agent. The configuration file should include the following:

   * HZN_EXCHANGE_URL=`<address of your exchange>`
   * HZN_FSS_CSSURL=`<css address>`
   * HZN_ORG_ID=`<the org this node should be in>`
   * HZN_EXCHANGE_USER_AUTH=`<exchange username:password>`
   * HZN_AGBOT_URL=`<agbot api address>`

   Optional:

   * HZN_NODE_ID=`<a name for your node>`
   * If this parameter is not included, the node will be assigned a random alphanumeric identifier.
   * (DEPRECATED) HZN_DEVICE_ID=`<a name for your node>`
   * HZN_MGMT_HUB_CERT_PATH=`<path to the ssl certificate file on the host machine>`

2. Prior to starting the container, export the following variables:

   * HZN_AGENT_IMAGE=`<name of the agent container image in the repo>`
     * This should include the url for the repository if it is not the default one.
   * HZN_AGENT_IMAGE_TAG=`<version of the agent container to use>`
   * CONFIG_FILE=`<Location of the configuration file>`
     * The configuration file with the contents from step 1.
   * HZN_MGMT_HUB_CERT_MOUNT="-v `<ssl certificate file on host>`:`<$HZN_MGMT_HUB_CERT_PATH>`"
     * This needs to be set in the configuration file so the agent can find the certificate after it starts.
   * DOCKER_NAME=`<name for the agent container>`
   * HORIZON_AGENT_PORT=`<port number>`
     * The port to expose from the container that hzn will call the agent on. This is typically 8081.
   * DOCKER_ADD_HOSTS="--add-host=$`<host name to add to container hosts file>`"
     * This is only necessary if the exchange or css url will not be resolvable from inside the agent container.
     * The format would look like `--add-host=edge-openhorizon.com:169.22.10.179`

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

3. Prior to starting the container, create several shared volume directory paths:

   ```bash
   export fssBasePath=$HOME/tmp/horizon
   export fssHostSharePath=${fssBasePath}/${DOCKER_NAME}

   # create fss domain socket path, ess auth path and secret path
   mkdir -p ${fssHostSharePath}/fss-domain-socket
   mkdir -p ${fssHostSharePath}/ess-auth
   mkdir -p ${fssHostSharePath}/secrets
   mkdir -p ${fssHostSharePath}/nmp
   ```
   {: codeblock}

4. Start the container by running this docker command:

   ```bash
   docker run $DOCKER_ADD_HOSTS -d \
   --restart always \
   --name $DOCKER_NAME \
   --privileged -p 127.0.0.1:$HORIZON_AGENT_PORT:8510 \
   -e DOCKER_NAME=$DOCKER_NAME \
   -e HZN_VAR_RUN_BASE=$fssHostSharePath \
   -v /var/run/docker.sock:/var/run/docker.sock \
   -v $CONFIG_FILE:/etc/default/horizon:ro $HZN_MGMT_HUB_CERT_MOUNT \
   -v $DOCKER_NAME_var:/var/horizon/ \
   -v $DOCKER_NAME_etc:/etc/horizon/ \
   -v $fssHostSharePath:$fssHostSharePath \
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
   * "-e HZN_VAR_RUN_BASE"
     * Tells the agent where to store edge sync service files.
   * "-e DOCKER_NAME=$DOCKER_NAME"
     * Tells the agent the container name.
   * "-v /var/run/docker.sock:/var/run/docker.sock"
     * Gives the container access to the docker socket so it can control service containers.
     * If you are running podman, set up an alias from podman.sock to /var/run/docker.sock or specify /run/podman/podman.sock
   * "-v $CONFIG_FILE_MOUNT:/etc/default/horizon:ro"
     * The configuration file lets you set variables to tell the agent about itself and the exchange it is to work with.
   * "-v $HZN_MGMT_HUB_CERT_PATH:$HZN_MGMT_HUB_CERT_PATH"
     * Maps the ssl certificate for the exchange into the agent container. The agent will look for the certificate wherever `HZN_MGMT_HUB_CERT_PATH` is set to in the configuration file.
   * "-v $DOCKER_NAME_var:/var/horizon/"
     * This volume will store the database for the agent.
   * "-v $DOCKER_NAME_etc:/etc/horizon/"
     * This volume contains some initial configuration information for the agent.
     * This file in the container image is non-empty so mounting it to a non-empty host folder will cause the agent to fail.
   * "-v ${fssHostSharePath}:${fssHostSharePath}"
     * This volume is used by the edge sync service for storing models downloaded by the service.
   * "$HZN_AGENT_IMAGE:$HZN_AGENT_IMAGE_TAG"
     * The agent container image that will be started.
   * "export HORIZON_URL=http://localhost:$HORIZON_AGENT_PORT"
     * This exported variable tells the horizon CLI how to reach the agent on the port exposed from the container.

5. To register the agent with a policy, run `hzn policy new > node_pol.json`, then edit the `node_pol.json` file to match the deployment policy for the service you want to deploy.

   ```bash
   source $CONFIG_FILE
   hzn register --policy node_pol.json
   ```
   {: codeblock}

6. To register the agent with a pattern:

   ```bash
   source $CONFIG_FILE
   hzn register -p <pattern name>
   ```
   {: codeblock}
