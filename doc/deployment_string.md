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

Because Horizon uses the Docker API to start the containers, many of the fields that can be specified in the deployment string are similar to `docker run` options.

- `services`: a list of docker images that are part of this microservice or workload
  - `<container-name>`: the name docker should give the container. Equivalent to the `docker run --name` flag. Horizon will also define this as the hostname for the container on the docker network, so other containers in the same network can connect to it using this name.
    - `image`: the docker image to be downloaded from the Horizon image server. The same name:tag format as used for `docker pull`.
    - `privileged`: `{true|false}` - set to true if the container needs privileged mode. Can only be used for microservices, not workloads.
    - `cap_add`: `["SYS_ADMIN"]` - grant an individual authority to the container. See https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities for a list of capabilities that can be added.
    - `environment`: `["FOO=bar","FOO2=bar2"]` - environment variables that should be set in the container.
    - `devices`: `["/dev/bus/usb/001/001:/dev/bus/usb/001/001",...]` - device files that should be made available to the container.. Can only be used for microservices, not workloads.
    - `binds`: `["/outside/container:/inside/container",...]` - directories from the host that should be bind mounted in the container. Equivalent to the `docker run --volume` flag.. Can only be used for microservices, not workloads.
    - `specific_ports`: `[{"HostPort":"7777/udp","HostIP":"1.2.3.4"},...]` - a container port that should be mapped to the same host port number. If the protocol is not specified after the port number, it defaults to `tcp`. The `HostIP` identifies what host network interfaces this port should listen on. Use `0.0.0.0` to specify all interfaces.. Can only be used for microservices, not workloads.
    - `command`: `["--myfirstarg","argvalue",...]` - override the start CMD specified the dockerfile, or append to the ENTRYPOINT specified in the dockerfile.
    - `ports`: `[1234,...]` - publish a container port to an ephemeral host port.

## Deployment String Examples

A deployment string JSON would look like this:

```
{
  "services": {
    "gps": {
      "image": "summit.hovitos.engineering/x86/gps:2.0.3",
      "privileged": true,
      "environment": [
        "FOO=bar"
      ],
      "devices": [
        "/dev/bus/usb/001/001:/dev/bus/usb/001/001"
      ],
      "binds": [
        "/tmp/testdata:/tmp/mydata"
      ],
      "specific_ports": [
        {
          "HostPort":"6414/tcp",
          "HostIP": "0.0.0.0"
        }
      ]
    }
  }
}
```

When stringified and put in the `workloads[]` array, the above example would look like:

```
"deployment": "{\"services\":{\"gps\":{\"image\":\"summit.hovitos.engineering/x86/gps:2.0.3\",\"privileged\":true,\"environment\":[\"FOO=bar\"],\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"binds\":[\"/tmp/testdata:/tmp/mydata\"],\"specific_ports\":[{\"HostPort\":\"6414/tcp\",\"HostIP\":\"0.0.0.0\"}]}}}"
```
