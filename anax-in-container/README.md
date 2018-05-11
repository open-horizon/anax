# Horizon Agent (anax) Running in a Container

This support provides the way to build and run a container running anax (the Horizon edge agent), so that an edge node can be created by starting the container. This can be useful in several cases:
- You want to run several instances of anax on the same host, for scale testing or development.
- You want to have several instances of anax, each configured slightly differently, so you can quickly/easily start the one you want to work with.
- You want to run anax on your Mac, for development, testing, or quick experimentation, and you have docker but not a VM (or you just don't want manage a separate VM). This is a very low barrier to entry for trying out horizon (if you happen to have a mac).
- The flexibility of running anax in a container is probably useful for other situations we don't know about yet.

**Note:** This support is currently only tested for amd64.

## Build and Push the Anax Container

```
export DOCKER_IMAGE_VERSION=0.5.0
make docker-image
make docker-push     # push the image to docker hub
```

## Using the Anax Container for the WIoTP Production Environment

```
docker pull openhorizon/amd64_anax:0.5.0
mkdir -p /var/tmp/horizon/workload_ro    # anax will check for this, because this will be mounted into service containers
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -v /var/run/docker.sock:/var/run/docker.sock -v /etc/wiotp-edge:/etc/wiotp-edge -v /var/wiotp-edge:/var/wiotp-edge openhorizon/amd64_anax:0.5.0
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
docker exec amd64_anax bash   # enter the container
wiotp_agent_setup --org $HZN_ORG_ID --deviceType $WIOTP_GW_TYPE --deviceId $WIOTP_GW_ID --deviceToken "$WIOTP_GW_TOKEN" -cn 'edge-connector'
exit   # exit the container (or you can stay in and work there)
hzn agreement list
```

## Using the Anax Container for the Bluehorizon/WIoTP Hyrbrid Environment

```
docker pull openhorizon/amd64_anax:0.5.0
mkdir -p /var/tmp/horizon/workload_ro    # anax will check for this, because this will be mounted into service containers
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -v /var/run/docker.sock:/var/run/docker.sock -v /etc/wiotp-edge:/etc/wiotp-edge -v /var/wiotp-edge:/var/wiotp-edge -v `pwd`:/outside openhorizon/amd64_anax:0.5.0 /root/bluehorizon-env.sh
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the container, and the bluehorizon-env.sh config script ran
docker exec amd64_anax wiotp_create_certificate -p $WIOTP_GW_TOKEN
hzn register -n $EXCHANGE_NODEAUTH -f ~/examples/edge/wiotp/location2wiotp/horizon/userinput-service.json $HZN_ORG_ID $WIOTP_GW_TYPE
hzn agreement list
```

## Using a Second Anax Container on the Same Machine

**Note: you currently can't have both instances of anax running the core-iot service, because the core-iot containers collide on ports, /etc/wiotp-edge, and /var/wiotp-edge.**

```
docker pull openhorizon/amd64_anax:0.5.0
# Note the slightly different container name and port number in the next 2 cmds
docker run -d -t --name amd64_anax2 --privileged -p 127.0.0.1:8082:80 -v /var/run/docker.sock:/var/run/docker.sock -v `pwd`:/outside openhorizon/amd64_anax:0.5.0 /root/bluehorizon-env.sh
export HORIZON_URL='http://localhost:8082'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the right container, and the bluehorizon-env.sh config script ran
hzn register -n $EXCHANGE_NODEAUTH $HZN_ORG_ID $WIOTP_GW_TYPE -f ~/examples/edge/wiotp/location2wiotp/horizon/without-core-iot/userinput.json
hzn agreement list
```

## Experimental: Using the Anax Container on Mac for the Bluehorizon/WIoTP Hyrbrid Environment

**Note: this doesn't quite work yet, because docker can't configure the unix syslog driver. See: https://github.com/open-horizon/anax/issues/628**

```
socat TCP-LISTEN:2375,reuseaddr,fork UNIX-CONNECT:/var/run/docker.sock &   # have docker api listen on a port, in addition to a unix socket
mkdir -p /private/var/tmp/horizon/workload_ro    # anax will check for this, because this will be mounted into service containers
docker pull openhorizon/amd64_anax:0.5.0
docker run -d -t --name amd64_anax --privileged -p 127.0.0.1:8081:80 -e MAC_HOST=192.168.1.10 -v /private/var/tmp/horizon:/private/var/tmp/horizon -v /private/etc/wiotp-edge:/etc/wiotp-edge -v /private/var/wiotp-edge:/var/wiotp-edge -v `pwd`:/outside openhorizon/amd64_anax:0.5.0 /root/bluehorizon-env.sh
export HORIZON_URL='http://localhost:8081'    # to point the hzn cmd to the container
hzn node list   # ensure you talking to the container, and the bluehorizon-env.sh config script ran
docker exec amd64_anax wiotp_create_certificate -p $WIOTP_GW_TOKEN
hzn register -n $EXCHANGE_NODEAUTH -f ~/input/services/core-iot-input.json $HZN_ORG_ID $WIOTP_GW_TYPE
hzn agreement list
```
