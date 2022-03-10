# Horizon Agent (anax) Running in a Container

This page provides guidance for building and running a containerized version of the Horizon edge agent, so that an edge node can be created by starting the container. This can be useful in several cases:

- You want to run several instances of the Horizon agent on the same host, for scale testing or development.
- You want to have several instances of the Horizon agent, each configured slightly differently, so you can quickly/easily start the one you want to work with.
- You want to run the Horizon agent on your Mac, for development, testing, or quick experimentation, and you have docker but not a VM (or you just don't want manage a separate VM). This is a very low barrier to entry for trying out horizon.
- The flexibility of running the Horizon agent in a container is probably useful for other situations we don't know about yet.

> **Note:** Currently supported architectures for running Horizon Agent (anax) in container:
>
> - amd64
> - arm64
> - ppc64el
> - s390x

## Build and Push the Horizon Agent Container

Building multi-architecture Docker images can be done by exporting the target host platform variables and a variable to enable
[Docker Buildx](https://docs.docker.com/buildx/working-with-buildx/). This requires Docker >= 19.03 and certain host packages to
be installed and running (`qemu-user-static` and `binmt-support`). Alternatively, instead of installing the needed packages, you can
also run the `multiarch/qemu-user-static` Docker image which will setup the QEMU simulator for use with Buildx. Note that you must
continually run this Docker image in order to use Buildx to build muli-arch images.

```bash
# List of possible values:
#   `arch`: arm64, amd64, ppc64el, s390x
#   `opsys`: Linux, Darwin
export arch=ppl64el
#export arch=arm64
export opsys=Linux # (output of 'uname -s' command on target)
export USE_DOCKER_BUILDX=true
# setup the QEMU simulator. You can then run `docker buildx ls` to see which platforms are available.
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
# In Makefile, modify line: DOCKER_IMAGE_VERSION ?= x.x.x, or set that variable in the environment
make anax-image
# test container locally with:
HC_DONT_PULL=1 horizon-container start
make docker-push-only     # push the image to docker hub
```

## Using the Horizon Agent Container on **Mac**

One of the most convenient uses of the Horizon agent container is to run the agent on a Mac, since the full Horizon agent install package is not available for Mac. This enables you to use your mac as a quick edge node for experimenting, or for edge service development.

You will need the following prerequisites:


- Docker for Mac OS X: https://docs.docker.com/docker-for-mac/install/
- Socat, install using **one** of these methods:
    - Homebrew: http://macappstore.org/socat/
    - MacPorts: https://www.macports.org/ then 'sudo port install socat'"

The horizon-cli package **is** available for Mac. The easiest way to install is to use the `agent-install.sh` script. This following command will
- download the latest agent package from github
- install the horizon cli and horizon-container script
- start the agent container
- register the node to run a helloworld service
- wait until an agreement is formed and the service begins execution

For information about the configuration required for the agent, and additional options supported the agent install script see [agent install directions](https://github.com/open-horizon/anax/blob/master/agent-install/README.md).

```bash
sudo -s -E ./agent-install.sh -i 'https://github.com/open-horizon/anax/releases' -p IBM/pattern-ibm.helloworld -w '*' -T 120
```

Once an agreement is formed, you can use `docker ps` to see the edge service containers.

If you no longer want to run edge services on your mac, you can unregister your node and stop the containers with:

```bash
horizon-container stop
```

Note: since you have the `hzn` command installed, you can also run edge services using `hzn dev` while developing them.

## Using the Horizon agent Container on **Linux**

The Horizon agent container can be used on a linux host using the `agent-install.sh` script by following the proceeding steps:

- Set the required configuration options using the [agent install directions](https://github.com/open-horizon/anax/blob/master/agent-install/README.md).
- Install the Horizon CLI by running the following:
```bash
sudo -s -E ./agent-install.sh -i 'https://github.com/open-horizon/anax/releases' -C
```
- Start the horizon agent with `horizon-container start`.
- On your linux host run `export HORIZON_URL=http://localhost:8081` to direct the `hzn` command to the container.
- Run `hzn node list` to verify that the agent is running.

## Running More Horizon agent Containers on the Same Machine

You can easily start additional Horizon agent containers on the same machine by passing an integer number to horizon-container:

```bash
horizon-container start 2
```

This can be useful for scaling testing, or switching between patterns of services quickly.

## Pointing the Horizon Agent to a Different Exchange

The default agent configuration file is `/etc/default/horizon` on the host. In that file set the following variables for the agent to use:

```bash
HZN_EXCHANGE_URL=
HZN_EXCHANGE_USER_AUTH=
HZN_FSS_CSSURL=
HZN_AGBOT_URL=
HZN_ORG_ID=
HZN_NODE_ID=<optional name for your node>
HZN_MGMT_HUB_CERT_PATH=
```

If changes are made to the its configuration, the agent needs to be restarted with `horizon-container restart` before the changes will be picked up.

You can also use a different file on the host by specifying it on the command line. Note that the script expects an integer to number the container before the filepath:

```bash
horizon-container start 1 /etc/default/horizon.stg
```

## Manually Starting the Horizon agent Container

The horizon-container script handles all of the details of invoking the Horizon agent container, but in case you need to do something out of the ordinary, here are the main commands to run it manually on a **linux** machine (for `amd64` arch):

```bash
docker pull openhorizon/amd64_anax
# Note the slightly different container name and port number in the next 2 cmds:
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:8510 -v /var/run/docker.sock:/var/run/docker.sock -v /var/tmp/horizon:/var/tmp/horizon openhorizon/amd64_anax
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you are talking to the right container, and it is talking to the right exchange
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/msghub/cpu2msghub/horizon/userinput.json $HZN_ORG_ID $HZN_PATTERN
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax2; docker rm amd64_anax2
```
