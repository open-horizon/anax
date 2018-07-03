# Horizon Agent (anax) Running in a Container

This support provides the way to build and run a container running anax (the Horizon edge agent), so that an edge node can be created by starting the container. This can be useful in several cases:
- You want to run several instances of anax on the same host, for scale testing or development.
- You want to have several instances of anax, each configured slightly differently, so you can quickly/easily start the one you want to work with.
- You want to run anax on your Mac, for development, testing, or quick experimentation, and you have docker but not a VM (or you just don't want manage a separate VM). This is a very low barrier to entry for trying out horizon (if you happen to have a mac).
- The flexibility of running anax in a container is probably useful for other situations we don't know about yet.

**Note:** This support is currently only tested for amd64.

## Build and Push the Anax Container

```
# In Makefile, modify line: DOCKER_IMAGE_VERSION ?= x.x.x, or set that variable in the environment
make docker-image
make docker-push     # push the image to docker hub
```

If you don't need to change anything about the anax image, except pick up the latest horizon deb pkgs:

```
DOCKER_MAYBE_CACHE='--no-cache' make docker-image
make docker-push
```

## Using the Anax Container for the WIoTP Production Environment

**Note: we haven't actually tested this case yet.**

```
sudo bash -c 'mkdir -p /etc/wiotp-edge /var/wiotp-edge && chmod 777 /etc/wiotp-edge /var/wiotp-edge'
mkdir -p /var/tmp/horizon/service_storage    # anax will check for this, because this will be mounted into service containers
docker pull openhorizon/amd64_anax
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -v /var/run/docker.sock:/var/run/docker.sock -v /var/tmp/horizon:/var/tmp/horizon -v /etc/wiotp-edge:/etc/wiotp-edge -v /var/wiotp-edge:/var/wiotp-edge openhorizon/amd64_anax /root/wiotp-env.sh
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
docker exec amd64_anax bash   # enter the container
wiotp_agent_setup --org $HZN_ORG_ID --deviceType $WIOTP_GW_TYPE --deviceId $WIOTP_GW_ID --deviceToken "$WIOTP_GW_TOKEN" -cn 'edge-connector'
exit   # exit the container (or you can stay in and work there)
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax; docker rm amd64_anax
```

## Using the Anax Container for the Bluehorizon/WIoTP Hyrbrid Environment

```
sudo bash -c 'mkdir -p /etc/wiotp-edge /var/wiotp-edge && chmod 777 /etc/wiotp-edge /var/wiotp-edge'
mkdir -p /var/tmp/horizon/service_storage    # anax will check for this, because this will be mounted into service containers
docker pull openhorizon/amd64_anax
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -v /var/run/docker.sock:/var/run/docker.sock -v /var/tmp/horizon:/var/tmp/horizon -v /etc/wiotp-edge:/etc/wiotp-edge -v /var/wiotp-edge:/var/wiotp-edge -v `pwd`:/outside openhorizon/amd64_anax /root/bluehorizon-env.sh "$WIOTP_GW_TOKEN"
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/wiotp/location2wiotp/horizon/userinput-service.json $HZN_ORG_ID $WIOTP_GW_TYPE
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax; docker rm amd64_anax
```

## Using a Second Anax Container on the Same Machine

**Note: you currently can't have both instances of anax running the core-iot service, because the core-iot containers collide on ports, /etc/wiotp-edge, and /var/wiotp-edge.**

```
# TODO: set Edge.MultipleAnaxInstances
docker pull openhorizon/amd64_anax
# Note the slightly different container name and port number in the next 2 cmds
docker run -d -t --name amd64_anax2 --privileged -p 127.0.0.1:8082:80 -v /var/run/docker.sock:/var/run/docker.sock -v /var/tmp/horizon:/var/tmp/horizon -v `pwd`:/outside openhorizon/amd64_anax /root/bluehorizon-env.sh
export HORIZON_URL='http://localhost:8082'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the right container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH $HZN_ORG_ID $WIOTP_GW_TYPE -f ~/examples/edge/wiotp/location2wiotp/horizon/without-core-iot/userinput.json
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax; docker rm amd64_anax
```

## Experimental: Using the Anax Container on Mac for the Bluehorizon/WIoTP Hyrbrid Environment

```
export MAC_HOST=192.168.1.12   # whatever your mac IP address is
socat TCP-LISTEN:2375,reuseaddr,fork UNIX-CONNECT:/var/run/docker.sock &   # have docker api listen on a port, in addition to a unix socket
mkdir -p /private/var/tmp/horizon/service_storage    # anax will check for this, because this will be mounted into service containers
docker pull openhorizon/amd64_anax

# Note: since a recent update to docker on mac, it won't automatically create dirs /etc/wiotp-edge /var/wiotp-edge in the VM tmpfs as a result of
#		mounting them into the container. And we can't add them to the docker preferences as dirs to allow to mount, because it automatically
#		resolves them to /private/etc/wiotp-edge and /private/var/wiotp-edge, which overlap with the existing /private mount point, which isn't allowed.
#		The docker run cmd below will work and populate /private/etc/wiotp-edge and /private/var/wiotp-edge properly, but anax can't run services that
#		use core-iot, because it will try to bind /etc/wiotp-edge and /var/wiotp-edge. (The only solution i know of is to modify the service definitions
#		of core-iot and the service that uses it to bind /private/etc/wiotp-edge to /etc/wiotp-edge and /private/var/wiotp-edge to /var/wiotp-edge.)
sudo mkdir -p -m 777 /private/etc/wiotp-edge /private/var/wiotp-edge
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -e MAC_HOST=$MAC_HOST -v /private/var/tmp/horizon:/private/var/tmp/horizon -v /private/etc/wiotp-edge:/etc/wiotp-edge -v /private/var/wiotp-edge:/var/wiotp-edge -v `pwd`:/outside openhorizon/amd64_anax /root/bluehorizon-env.sh "$WIOTP_GW_TOKEN"
#docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -e MAC_HOST=$MAC_HOST -v /private/var/tmp/horizon:/private/var/tmp/horizon -v /etc/wiotp-edge:/etc/wiotp-edge -v /var/wiotp-edge:/var/wiotp-edge -v `pwd`:/outside openhorizon/amd64_anax /root/bluehorizon-env.sh "$WIOTP_GW_TOKEN"

export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH -f ~/input/services/core-iot-input.json $HZN_ORG_ID $WIOTP_GW_TYPE
hzn agreement list
# To stop anax, use this cmd to give it time to unregister and stop the service containers:
docker stop -t 120 amd64_anax; docker rm amd64_anax
```

## Experimental: Support for 'hzn dev' on Mac

Install go and docker on your mac, clone https://github.com/open-horizon/anax and 'make cli/hzn', you can use `hzn dev` on your mac. If you are developing services that use the WIoTP core-iot service, you can get the `/etc/wiotp-edge` and `/var/wiotp-edge` directories populated using:
```
docker pull openhorizon/amd64_anax
docker run -t --rm --name amd64_anax --privileged -v /etc/wiotp-edge:/etc/wiotp-edge -v /var/wiotp-edge:/var/wiotp-edge openhorizon/amd64_anax only_certificate "$WIOTP_GW_TOKEN"
```
