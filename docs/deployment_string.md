---
copyright:
years: 2022 - 2023
lastupdated: "2023-01-24"
description: Description of deployment strings
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# {{site.data.keyword.edge_notm}} Deployment Strings
{: #deployment-strings}

When defining services in the {{site.data.keyword.horizon}} Exchange, the deployment field defines how the service will be deployed. There are two deployment fields for a service, `deployment` and `clusterDeployment`. You can define either one or both depending on the type of the service you want it to be. The `deployment` field is used by an edge node which tells Horizon how the docker images should be deployed on the node. The `clusterDeployment` field is used by an Horizon agent running in a Kubernetes cluster. It tells Horizon how the application should be deployed in the Kubernetes cluster.
This page documents what can be specified in the `deployment` and the `clusterDeployment` fields.

## Deployment String Format
{: #deployment-format}

The `deployment` and `clusterDeployment` strings are JSON which have been "stringified" (double quotes escaped, new lines removed, etc.). It should look like this:

```json
  "deployment": "{\"services\":{\"gps\":{\"image\":\"openhorizon/x86/gps:2.0.3\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"network\":\"host\"}}}",
```
{: codeblock}

```json
 "clusterDeployment": "{\"operatorYamlArchive\":\"H4sIAEu8lF4AA+1aX2/bNhDPcz4FkT4EGGZZsmxn0JuXZluxtjGcoHsMaIm2uVKiRlLO0mHffUfq..."
```
{: codeblock}

 **Note**: You do not need to stringify the deployment strings if you use the `hzn` command to create the service in the exchange. However if you use `curl` command to create the service, you must stringify it.

## Deployment string fields
{: #deployment-fields}

Because {{site.data.keyword.edge_notm}} uses the docker API to start the containers on an edge node, many of the fields that can be specified in the deployment string are similar to `docker run` options.

- `services`: a list of docker images that are part of this service
  - `<container-name>`: the name docker should give the container. Equivalent to the `docker run --name` flag. {{site.data.keyword.horizon}} will also define this as the hostname for the container on the docker network, so other containers in the same network can connect to it using this name.
    - `image`: the docker image to be downloaded from the {{site.data.keyword.horizon}} image server. The same name:tag format as used for `docker pull`.
    - `privileged`: `{true|false}` - set to true if the container needs privileged mode. When set to true, the service can only be deployed to nodes with property openhorizon.allowPrivileged set to true.
    - `cap_add`: `["SYS_ADMIN"]` - grant an individual authority to the container. See [https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities ](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities){:target="_blank"}{: .externalLink} for a list of capabilities that can be added.
    - `environment`: `["FOO=bar","FOO2=bar2"]` - (deprecated) environment variables that should be set in the container.
    - `devices`: `["/dev/bus/usb/001/001:/dev/bus/usb/001/001",...]` - device files that should be made available to the container.
    - `binds`: `["/outside/container_path:/inside/container_path1:rw","docker_volume_name:/inside/container_path2:ro"...]` - directories from the host or docker volumes that should be bind mounted in the container. Equivalent to the `docker run --volume` flag. If the first field is not in the directory format, it will be treated as a docker volume. The directory or the docker volume will be created on the host if it does not exist when the containers starts. The last field is the mount options. `ro` means readonly, `rw` means read/write (default). To bind to directories that are only available to root on the host system, the container needs to have 'privileged' set to true.
    - `tmpfs`: `{"/app":""}` - There is no source for tmpfs mounts. It creates a tmpfs mount at /app
    - `ports`: `[{"HostPort":"5555:7777/udp","HostIP":"1.2.3.4"},{"HostPort":"8888/udp","HostIP":"1.2.3.4"}...]` - container ports that should be mapped to the host. "5555" is the host port number, if omitted, the same container port number ("7777") will be used. If the protocol is not specified after the port number, it defaults to `tcp`. The `HostIP` identifies what host network interfaces this port should listen on. Use `0.0.0.0` to specify all interfaces. To use ports reserved for root, 'privileged' must be set to true.
    - `ephemeral_ports`: `[{"localhost_only":true, "port_and_protocol":"7777/udp"}, {"port_and_protocol":"8888"}...]` - publish a container port to an ephemeral host port. If `localhost_only` is set to true, the localhost ip address (`127.0.0.1`) will be used as the host network interface this port should listen on. Otherwise, all the host network interfaces on the host will be listened by this port. If the protocol is not specified after the port number for `port_and_protocol`, it defaults to `tcp`.
    - `command`: `["--myfirstarg","argvalue",...]` - override the start CMD specified the Dockerfile, or append to the ENTRYPOINT specified in the dockerfile.
    - `network`: `"host"` - start the container with host network mode. When network is set to host, the service can only be deployed to nodes with property openhorizon.allowPrivileged set to true.
    - `entrypoint`: `["executable", "param1", "param2"]` - override ENTRYPOINT specified in the Dockerfile.
    - `max_memory_mb`: `4096` - the maximum amount of memory the service container can use
    - `max_cpus`: `1.5` - how much of the available CPU resources the service container can use. For instance, if the host machine has two CPUs and you set value to 1.5, the container is guaranteed to use at most one and a half of the CPUs
    - `log_driver`: the logging driver (e.g. `json-file`) to use for container logs, instead of default one (syslog)
    - `secrets`: `{"ai_secret": {"description": "The token for cloud AI service."}, "sql_secret": {}}` - a list of secret names and the descriptions. The `description` can be omitted. A secret name is just a user defined string. A pattern or a deployment policy will associate it with the name of the secret in the secret provider. The horizon agent will mount the secrets at '/open-horizon-secrets' within the service's containers. Each secret name appears as a file in that directory, containing the details of the secret from the secret provider. Each secret file is a JSON encoded file containing the "key" and "value" set when the secret was created with the hzn secretsmanager secret add command.
    - `user`: Sets the username or UID used. root (id = 0) is the default user within a container. The image developer can create additional users. Those users are accessible by name. When passing a numeric ID, the user does not have to exist in the container.
    - `pid`: Set the PID (Process) Namespace mode for the container. `container:<name|id>` joins another container's PID namespace. `host` use the host's PID namespace inside the container. In certain cases you want your container to share the host’s process namespace, basically allowing processes within the container to see all of the processes on the system.
    - `sysctls`: Sysctl settings are exposed via Kubernetes, allowing users to modify certain kernel parameters at runtime for namespaces within a container. The parameters cover various subsystems, such as: networking (common prefix: net.), kernel (common prefix: kernel.), virtual memory (common prefix: vm.), MDADM (common prefix: dev.). To get a list of all parameters, you can run: `sudo sysctl -a`

## clusterDeployment String Fields
{: #clusterdeployment-fields}

Because {{site.data.keyword.edge_notm}} uses operators to deploy the applications in a Kubernetes cluster, the `clusterDeployment` contains the contents of the operator yaml archive files.

- `operatorYamlArchive`: The content of the operator yaml archive files. These files are compressed (tarred and gzipped). And then the compressed content is converted to a base64 string.

## Deployment String Examples
{: #deployment-examples}

A `deployment` string JSON would look like this:

```json
{
"deployment": {
    "services": {
      "gps": {
        "image": "openhorizon/x86/gps:2.0.3",
        "privileged": true,
        "devices": [
          "/dev/bus/usb/001/001:/dev/bus/usb/001/001"
        ],
        "binds": [
          "/tmp/testdata:/tmp/mydata:ro",
          "myvolume1:/tmp/mydata2"
        ],
        "tmpfs": {
          "/app":""
        },
        "ports": [
          {
            "HostPort":"5200:6414/tcp",
            "HostIP": "0.0.0.0"
          }
        ],
        "network": "host"
      }
    }
  }
}
```
{: codeblock}

When stringified, the above example would look like:

```json
"deployment": "{\"services\":{\"gps\":{\"image\":\"openhorizon/x86/gps:2.0.3\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"binds\":[\"/tmp/testdata:/tmp/mydata:ro\",\"myvolume1:/tmp/mydata2\"],\"tmpfs\":{\"/app\":\"\"},\"ports\":[{\"HostPort\":\"5200:6414/tcp\",\"HostIP\":\"0.0.0.0\"}]}}}"
```
{: codeblock}

A `clusterDeployment` string JSON would look like this when defining a service using `hzn` command:

```json
"clusterDeployment": {
  "operatorYamlArchive": "/filepath/k8s_operator_deployment_files.tar.gz"
}
```
{: codeblock}

When the content is encoded and stringified, the above would look like:

```json
"clusterDeployment": "{\"operatorYamlArchive\":\"H4sIAEu8lF4AA+1aX2/bNhDPcz4FkT4EGGZZsmxn0JuXZluxtjGcoHsMaIm2uVKiRlLO0mHffUfqjyVXkZLNcTCUvxeLR/J4vDse7yQ7w4ikjD8MT14OLuBi4ppfwP6vefb86Xji+ZOL6fjE9byRNz1BkxeUqUImFRYInQjOVde4vv7..."
```
{: codeblock}
