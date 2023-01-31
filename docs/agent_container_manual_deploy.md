---
copyright:
years: 2022 - 2023
lastupdated: "2023-01-30"
description: Instructions for starting an agent in a container on Linux

parent: Agent (anax)
nav_order: 2
---

## Instructions for starting an agent in a container on Linux
{: #container-agent}

These instructions assume that the agent container you wish to deploy already exists and is in a container image repository somewhere.

### Usage
{: #container-usage}

These instructions are for the user who wants to start the agent in a container with finer control over the details than that allowed by the horizon-container script. For a simplified process to getting an agent running see the [agent-install instructions ](https://github.com/open-horizon/anax/tree/master/agent-install){:target="_blank"}{: .externalLink}.

### Prerequisites
{: #container-prereqs}

Docker needs to be installed on the host device. For instructions on installing Docker on Linux see [here ](https://docs.docker.com/engine/install/){:target="_blank"}{: .externalLink}.

If the management hub you are using uses SSL, then you need to have the SSL certificate from the management hub.

### Starting the agent
{: #container-start}

Before starting the container, create a config file for the agent. This should include the following:

* HZN_EXCHANGE_URL=\<address of your exchange\>
* HZN_FSS_CSSURL=\<css address\>
* HZN_ORG_ID=\<the org this node should be in\>
* HZN_EXCHANGE_USER_AUTH=\<exchange username:password\>
* HZN_AGBOT_URL=\<agbot api address\>

Optional:

* HZN_NODE_ID=\<a name for your node\>
* If this isn't included, the node will be assigned a random alphanumeric identifier.
* (DEPRECATED) HZN_DEVICE_ID=\<a name for your node\>
* HZN_MGMT_HUB_CERT_PATH=\<path to the ssl cert file on the host machine\>

Prior to starting the container, the following variables need to be set.

* HZN_AGENT_IMAGE=\<name of the agent container image in the repo\>
  * This should include the url for the repository if it's not the default one.
* HZN_AGENT_IMAGE_TAG=\<version of the agent container to use\>
* CONFIG_FILE=\<Location of the config file\>
  * The config file with the contents from above.
* HZN_MGMT_HUB_CERT_MOUNT="-v \<ssl cert file on host\>:\<$HZN_MGMT_HUB_CERT_PATH\>"
  * This needs to be set in the config file so the agent can find the cert after it starts.
* DOCKER_NAME=\<name for the agent container\>
* HORIZON_AGENT_PORT=\<port number\>
  * The port to expose from the container that hzn will call the agent on. Typically 8081.
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

```bash
docker run $DOCKER_ADD_HOSTS -d --restart always --name $DOCKER_NAME --privileged -p 127.0.0.1:$HORIZON_AGENT_PORT:8510 -e DOCKER_NAME=$DOCKER_NAME -v /var/run/docker.sock:/var/run/docker.sock -v $CONFIG_FILE:/etc/default/horizon:ro $HZN_MGMT_HUB_CERT_MOUNT -v $DOCKER_NAME_var:/var/horizon/ -v $DOCKER_NAME_etc:/etc/horizon/ -v $DOCKER_NAME_fss:/var/tmp/horizon/$DOCKER_NAME $HZN_AGENT_IMAGE:$HZN_AGENT_IMAGE_TAG

export HORIZON_URL=http://localhost:$HORIZON_AGENT_PORT
```
{: codeblock}

The docker run command uses the following arguments. Here is what they are for:

* "--name $DOCKER_NAME"
  * Names the container something meaningful to the user for convenience.
* "--add-host=$HZN_EXCHANGE_HOSTS"
  * This allows the user to add the exchange to the containers host list.
* "-p 127.0.0.1:$HORIZON_AGENT_PORT:$ANAX_AGENT_PORT"
  * This exposes a port that `hzn` can reach the agent on.
* "-e DOCKER_NAME=$DOCKER_NAME"
  * Lets the agent container know what its name is.
* "-v /var/run/docker.sock:/var/run/docker.sock"
  * Gives the container access to the docker socket so it can control service containers.
* "-v $CONFIG_FILE_MOUNT:/etc/default/horizon:ro"
  * The config file lets the user set variables to tell the agent about itself and the exchange it is to work with.
* "-v $HZN_MGMT_HUB_CERT_PATH:$HZN_MGMT_HUB_CERT_PATH"
  * Maps the ssl certificate for the exchange into the agent container. The agent will look for the cert wherever `HZN_MGMT_HUB_CERT_PATH` is set to in the config file.
* "-v $DOCKER_NAME_var:/var/horizon/"
  * This volume will hold the database for the agent.
* "-v $DOCKER_NAME_etc:/etc/horizon/"
* This contains some initial configuration information for the agent.
* This file in the container image is non-empty so mounting it to a non-empty host folder will cause the agent to fail.
* "-v $DOCKER_NAME_fss:/var/tmp/horizon/horizon1"
  * This volume is used by the edge sync service for storing models downloaded by the service.
* "$HZN_AGENT_IMAGE:$HZN_AGENT_IMAGE_TAG"
* The agent container image that will be started.
* "export HORIZON_URL=http://localhost:$HORIZON_AGENT_PORT"
* This tells the horizon CLI how to reach the agent on the port exposed from the container.

To register the agent with a policy, run `hzn policy new > node_pol.json`, then edit this to match the deployment policy for the service you want to deploy.

```bash
source $CONFIG_FILE
hzn register --policy node_pol.json
```
{: codeblock}

To register the agent with a pattern

```bash
source $CONFIG_FILE
hzn register -p <pattern name>
```
{: codeblock}
