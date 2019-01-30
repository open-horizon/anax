# Horizon Deployment Strings

When defining services in the Horizon Exchange, one of the fields that needs to be specified is `deployment`, which tells Horizon how the docker images should be deployed on the edge node. This page documents what can be specified in that string.

## Deployment String Format

The deployment string is JSON which has been "stringified" (double quotes escaped, new lines removed, etc.). It should look like this:

```
  "deployment": "{\"services\":{\"gps\":{\"image\":\"openhorizon/x86/gps:2.0.3\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"]}}}",
 ```

 **Note**: You do not need to stringify the `deployent` string if you use `hzn` command to create the service in the exchange. However if you use `curl` command to create the service, you must stringify it.

## Deployment String Fields

Because Horizon uses the Docker API to start the containers, many of the fields that can be specified in the deployment string are similar to `docker run` options.

- `services`: a list of docker images that are part of this service
  - `<container-name>`: the name docker should give the container. Equivalent to the `docker run --name` flag. Horizon will also define this as the hostname for the container on the docker network, so other containers in the same network can connect to it using this name.
    - `image`: the docker image to be downloaded from the Horizon image server. The same name:tag format as used for `docker pull`.
    - `privileged`: `{true|false}` - set to true if the container needs privileged mode.
    - `cap_add`: `["SYS_ADMIN"]` - grant an individual authority to the container. See https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities for a list of capabilities that can be added.
    - `environment`: `["FOO=bar","FOO2=bar2"]` - environment variables that should be set in the container.
    - `devices`: `["/dev/bus/usb/001/001:/dev/bus/usb/001/001",...]` - device files that should be made available to the container.
    - `binds`: `["/outside/container_path:/inside/container_path1:rw","docker_volume_name:/inside/container_path2:ro"...]` - directories from the host or docker volumes that should be bind mounted in the container. Equivalent to the `docker run --volume` flag. If the first field is not in the directory format, it will be treated as a docker volume. The directory or the docker volume will be created on the host if it does not exist when the containers starts. The last field is the mount options. `ro` means readonly, `rw` means read/write (default).
    - `ports`: `[{"HostPort":"5555:7777/udp","HostIP":"1.2.3.4"},{"HostPort":"8888/udp","HostIP":"1.2.3.4"}...]` -  container ports that should be mapped to the host. "5555" is the host port number, if omitted, the same container port number ("7777") will be used. If the protocol is not specified after the port number, it defaults to `tcp`. The `HostIP` identifies what host network interfaces this port should listen on. Use `0.0.0.0` to specify all interfaces.
    - `ephemeral_ports`: `[{"localhost_only":true, "port_and_protocol":"7777/udp"}, {"port_and_protocol":"8888"}...]` - publish a container port to an ephemeral host port. If `localhost_only` is set to true, the localhost ip address (`127.0.0.1`) will be used as the host network interface this port should listen on. Otherwise, all the host network interfaces on the host will be listened by this port. If the protocol is not specified after the port number for `port_and_protocol`, it defaults to `tcp`.
    - `command`: `["--myfirstarg","argvalue",...]` - override the start CMD specified the dockerfile, or append to the ENTRYPOINT specified in the dockerfile.

## Deployment String Examples

A deployment string JSON would look like this:

```
{
  "services": {
    "gps": {
      "image": "openhorizon/x86/gps:2.0.3",
      "privileged": true,
      "environment": [
        "FOO=bar"
      ],
      "devices": [
        "/dev/bus/usb/001/001:/dev/bus/usb/001/001"
      ],
      "binds": [
        "/tmp/testdata:/tmp/mydata:ro",
        "myvolume1:/tmp/mydata2"       
      ],
      "ports": [
        {
          "HostPort":"5200:6414/tcp",
          "HostIP": "0.0.0.0"
        }
      ]
    }
  }
}
```

When stringified, the above example would look like:

```
"deployment": "{\"services\":{\"gps\":{\"image\":\"openhorizon/x86/gps:2.0.3\",\"privileged\":true,\"environment\":[\"FOO=bar\"],\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"binds\":[\"/tmp/testdata:/tmp/mydata:ro\",\"myvolume1:/tmp/mydata2\"],\"ports\":[{\"HostPort\":\"5200:6414/tcp\",\"HostIP\":\"0.0.0.0\"}]}}}"
```
