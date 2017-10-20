# Horizon Deployment Strings

When defining microservices and workloads in the Horizon Exchange, one of the fields that needs to be specified is `workloads[].deployment`, which tells Horizon how the docker images should be deployed on the edge node. This page documents what can be specified in that string.

## Deployment String Format

The deployment string is JSON which has been "stringified" (double quotes escaped, new lines removed, etc.). It should look like this:

```
  "workloads": [
    {
      "deployment": "{\"services\":{\"gps\":{\"image\":\"summit.hovitos.engineering/x86/gps:2.0.3\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"]}}}",
      ...
    }
  ]
```

## Deployment String Fields

- `services`: a list of docker images that are part of this microservice or workload
    - `<container-name>`: the name docker should give the container. Equivalent to the `docker run --name` flag. Horizon will also define this as the hostname for the container on the docker network, so other containers in the same network can connect to it using this name.
