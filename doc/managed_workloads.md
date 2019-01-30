## Horizon Managed Service Detail

Horizon manages the lifecycle, connectivity, and other features of services it launches on a device. This document is intended for service developers' consumption.

### Service Environment Variables

Horizon sets these environment variables when starting a service container:

* `HZN_DEVICE_ID`: The unique identifier for the edge node.
* `HZN_ORGANIZATION`: The organization the edge node is part of.
* `HZN_EXCHANGE_URL`: The Horizon Exchange being used by this edge node.
* `HZN_HOST_IPS`: The IP addresses configured on this edge node host.
* `HZN_ARCH`: A machine architecture designation for the host device. (This is retrieved by the golang runtime using the function `runtime.GOARCH`. Note: in the future, this may be modified to align with Ubuntu architecture designations: armel (Pi Zero), armhf (Pi 2, Odroid Xu4), arm64 (Pi 3, Odroid c2), or amd64.
* `HZN_RAM`: The quantity of RAM (in MB) that the container is restricted to use.
* `HZN_CPUS`: The quantity of CPU cores that the host device advertises. Note that the system may restrict scheduling services on a subset of the total available cores or may prioritize work on those cores.

This environment variable is only set if a pattern was used to register the edge node with. This is not set when using `hzn dev service start` to start the service (because there is no pattern at that point):

* `HZN_PATTERN`: The pattern that was deployed on this edge node, if any.

This environment variable is only set if this is a top-level service (referenced directly by the pattern):

* `HZN_AGREEMENTID`: The unique identifier for the contractual agreement that the currently-running service is a part of. The lifecycle of the service never exceeds the lifecycle of an active agreement.

These environment variables are only set if the corresponding values are set by the edge node owner in the `global` section of the registration input file:

* `HZN_LAT`: The user-provided latitude of the device.
* `HZN_LON`: The user-provided longitude of the device.
* `HZN_USER_PROVIDED_COORDS`: `false` if the system provided the coordinates and the user couldn't have provided inaccurate ones, `true` otherwise.
* `HZN_USE_GPS`: `true` if the user gives permission for the system to read corrdinates from a GPS device, `false` otherwise. Note that a `true` value does not guarantee that a GPS device will be accessible.
