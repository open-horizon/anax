# Horizon Agent (anax) Running in a Container

This support provides the way to build and run a container running the Horizon edge agent, so that an edge node can be created by starting the container. This can be useful in several cases:
- You want to run several instances of the Horizon agent on the same host, for scale testing or development.
- You want to have several instances of the Horizon agent, each configured slightly differently, so you can quickly/easily start the one you want to work with.
- You want to run the Horizon agent on your Mac, for development, testing, or quick experimentation, and you have docker but not a VM (or you just don't want manage a separate VM). This is a very low barrier to entry for trying out horizon (if you happen to have a mac).
- The flexibility of running the Horizon agent in a container is probably useful for other situations we don't know about yet.

> **Note:** Currently supported architectures for running Horizon Agent (anax) in container:
>
> - amd64
> - ppc64le

## Build and Push the Horizon agent Container

```
# In Makefile, modify line: DOCKER_IMAGE_VERSION ?= x.x.x, or set that variable in the environment
make docker-image
# test container locally with:
HC_DONT_PULL=1 horizon-container start
make docker-push-only     # push the image to docker hub
```

## Using the Horizon agent Container on **Mac**

One of the most convenient uses of the Horizon agent container is to run it on a Mac, since the full Horizon agent install package is not available for Mac. This enables you to use your mac as a quick edge node for experimenting, or edge service development.

You need the following prerequisites:
- The horizon-cli package **is** available for Mac. Download and install the latest version of `horizon-cli-x.x.x.pkg.pkg` from http://pkg.bluehorizon.network/macos/, install using `sudo installer -pkg "<path>/horizon-cli-x.x.x.pkg" -target /` command.
- Docker for Mac OS X: https://docs.docker.com/docker-for-mac/install/
- Socat, install using **one** of these methods:
    - Homebrew: http://macappstore.org/socat/
    - MacPorts: https://www.macports.org/ then 'sudo port install socat'"


If you don't already have `/usr/local/bin` in your command line PATH, add that, or fully qualify the commands below. Then:

```
horizon-container start
hzn node list   # ensure you talking to the container successfully and it is using the exchange you want
# start the sample hello world edge service
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/msghub/cpu2msghub/horizon/userinput.json $HZN_ORG_ID $HZN_PATTERN
hzn agreement list    # run repeatedly until an agreement is finalized
```

Once an agreement is formed, you can use `docker ps` to see the edge service containers, and you can subscribe to your message hub to see the data from your edge service.

If you no longer want to run edge services on your mac, you can unregister your node and stop the containers with:
```
horizon-container stop
```

Note: since you have the `hzn` command installed, you can also run edge services using `hzn dev` while developing them.

## Using the Horizon agent Container on **Linux**

The Horizon agent container can be used on a linux host in a very similar way to the instructions above, with these differences:
- Install the Horizon CLI using the horizon-cli debian package
- On your linux host run `export HORIZON_URL=http://localhost:8081` to direct the `hzn` command to the container

## Running More Horizon agent Containers on the Same Machine

You can easily start additional Horizon agent containers on the same machine by passing an integer number to horizon-container:
```
horizon-container start 2
```

This can be useful for scaling testing, or switching between patterns of services quickly.

## Pointing the Horizon Agent to a Different Exchange

By default, the Horizon agent container uses the production exchange, but you can tell it to use a different exchange using the standard `/etc/default/horizon` on the host. In that file set, for example:
```
HZN_EXCHANGE_URL=https://stg.edge-fabric.com/v1
```

You can also use a different file on the host by specifying it on the command line:
```
horizon-container start 1 /etc/default/horizon.stg
```

## Manually Starting the Horizon agent Container

The horizon-container script handles all of the details of invoking the Horizon agent container, but in case you need to do something out of the ordinary, here are the main commands to run it manually on a **linux** machine (for `amd64` arch):


```
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
