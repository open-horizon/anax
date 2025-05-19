---
copyright: Contributors to the Open Horizon project
years: 2022 - 2025
title: Horizon Edge Service Detail
description: Horizon Edge Services details
lastupdated: 2025-05-03
nav_order: 10
parent: Agent (anax)
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# {{site.data.keyword.horizon}} Edge service details
{: #edge-services}

{{site.data.keyword.edge_notm}} manages the lifecycle, connectivity, and other features of services it launches on a device. This section is intended for developers creating {{site.data.keyword.horizon}} service container workload definitions.

## Service environment variables
{: #edge-service-vars}

{{site.data.keyword.horizon}} sets the following environment variables when starting a service container:

* `HZN_NODE_ID`: The unique identifier for the edge node.
* `HZN_DEVICE_ID` (DEPRECATED): The unique identifier for the edge node.
* `HZN_ORGANIZATION`: The organization the edge node is part of.
* `HZN_EXCHANGE_URL`: The {{site.data.keyword.horizon}} Exchange being used by this edge node.
* `HZN_HOST_IPS`: The IP addresses configured on this edge node host.
* `HZN_ARCH`: A machine architecture designation for the host device. (This is retrieved by the golang runtime using the function `runtime.GOARCH`).
* `HZN_RAM`: The quantity of RAM (in MB) that the container is restricted to use.
* `HZN_CPUS`: The quantity of CPU cores that the host device advertises. Note that the system might restrict scheduling services on a subset of the total available cores or may prioritize work on those cores.
* `HZN_HARDWAREID`: The device serial number (if it can be located); otherwise, a generated ID.
* `HZN_PRIVILEGED`: The running mode for a service docker container. If assigned to `true`, the service will have all of the root capabilities of a host machine.

This environment variable is only set if a pattern was used to register the edge node with. This is not set when using `hzn dev service start` to start the service (because there is no pattern at that point):

* `HZN_PATTERN`: The pattern that was deployed on this edge node, if any.

This environment variable is only set if this is a top-level service (referenced directly by the pattern):

* `HZN_AGREEMENTID`: The unique identifier for the contractual agreement that the currently-running service is a part of. The lifecycle of the service never exceeds the lifecycle of an active agreement.

These environment variables are for Model Management System (MMS), which is implemented by the embedded Edge Sync Service (ESS). If these variables are absent, that indicates that MMS is not available to the service.

* `HZN_ESS_API_PROTOCOL`: The transport protocol to use when accessing the ESS API. The only supported value is secure-unix. The secure-unix protocol means that HTTPS should be used and the network transport will be through a unix domain socket.
* `HZN_ESS_API_ADDRESS`: The network address on which the ESS is listening. When HZN_ESS_API_PROTOCOL is secure-unix, this field contains the unix domain socket file to be used as the network transport. In this case, the hostname of the ESS API URL should be localhost. No port should be specified in the URL.
* `HZN_ESS_API_PORT`: The port on which the ESS listens. This is ignored when HZN_ESS_API_PROTOCOL is secure-unix.
* `HZN_ESS_AUTH`: The path to a JSON file containing the service's userid and token which should be passed to all ESS APIs as basic auth credentials in the HTTP header. Within the JSON file, the field "id" contains the userid and the field "token" contains the authentication token. Each service gets its own id and token, and should not be shared with any other service.
* `HZN_ESS_CERT`: The path to a TLS (SSL) certificate used to encrypt the call to all ESS APIs.
