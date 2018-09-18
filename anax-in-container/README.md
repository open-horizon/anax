# Horizon Agent (anax) Running in a Container

This support provides the way to build and run a container running anax (the Horizon edge agent), so that an edge node can be created by starting the container. This can be useful in several cases:
- You want to run several instances of the Horizon agent on the same host, for scale testing or development.
- You want to have several instances of the Horizon agent, each configured slightly differently, so you can quickly/easily start the one you want to work with.
- You want to run the Horizon agent on your Mac, for development, testing, or quick experimentation, and you have docker but not a VM (or you just don't want manage a separate VM). This is a very low barrier to entry for trying out horizon (if you happen to have a mac).
- The flexibility of running the Horizon agent in a container is probably useful for other situations we don't know about yet.

**Note:** This support is currently only tested for amd64.

## Build and Push the Horizon agent Container

```
# In Makefile, modify line: DOCKER_IMAGE_VERSION ?= x.x.x, or set that variable in the environment
make docker-image
make docker-push     # push the image to docker hub
```

If the anax files have not changed, but you need to force a rebuild to pick up the latest horizon deb pkgs:

```
DOCKER_MAYBE_CACHE='--no-cache' make docker-image
make docker-push
```

## Using the Horizon agent Container on **Mac** for the Bluehorizon Environment

One of the most convenient uses of the Horizon agent container is to run it on a Mac, since the full Horizon agent install package is not available for Mac. This enables you to use your mac as a quick edge node for experimenting, or edge service development.

The horizon-cli package **is** available for Mac, so first download and install the latest version of horizon-cli-x.x.x.pkg.zip from https://github.com/open-horizon/anax/releases

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


## Using the Anax Container for the Bluehorizon Environment

```
mkdir -p /var/tmp/horizon/service_storage    # anax will check for this, because this will be mounted into service containers
docker pull openhorizon/amd64_anax
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -v /var/run/docker.sock:/var/run/docker.sock -v /var/tmp/horizon:/var/tmp/horizon -v `pwd`:/outside openhorizon/amd64_anax /root/bluehorizon-env.sh
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/msghub/cpu2msghub/horizon/userinput.json $HZN_ORG_ID $HZN_PATTERN
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax; docker rm amd64_anax
```

## Using a Second Anax Container on the Same Machine

```
docker pull openhorizon/amd64_anax
# Note the slightly different container name and port number in the next 2 cmds:
docker run -d -t --name amd64_anax2 --privileged -p 127.0.0.1:8082:80 -v /var/run/docker.sock:/var/run/docker.sock -v /var/tmp/horizon:/var/tmp/horizon -v `pwd`:/outside openhorizon/amd64_anax /root/bluehorizon-env.sh
export HORIZON_URL='http://localhost:8082'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the right container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/msghub/cpu2msghub/horizon/userinput.json $HZN_ORG_ID $HZN_PATTERN
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax2; docker rm amd64_anax2
```

## Using the Anax Container on **Mac** for the Bluehorizon Environment

```
socat TCP-LISTEN:2375,reuseaddr,fork UNIX-CONNECT:/var/run/docker.sock &   # have docker api listen on a port, in addition to a unix socket
mkdir -p /private/var/tmp/horizon/service_storage    # anax will check for this, because this will be mounted into service containers
docker pull openhorizon/amd64_anax
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -e ANAX_DOCKER_ENDPOINT=tcp://host.docker.internal:2375 -v /private/var/tmp/horizon:/private/var/tmp/horizon -v `pwd`:/outside openhorizon/amd64_anax /root/ibmcloud-env.sh
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/msghub/cpu2msghub/horizon/userinput.json $HZN_ORG_ID $HZN_PATTERN
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax; docker rm amd64_anax
```

## Support for 'hzn dev' on Mac

Install go and docker on your mac, clone https://github.com/open-horizon/anax and 'make cli/hzn'. Then you can use `hzn dev` on your mac.
