# Policy Properties

There are built-in property names that can be used in the policies.
See [Properties and Constraints](./properties_and_constraints.md) for an explanation of properties and constraints in general.
For the node properties, the agent will introspect the node for the values of those properties.
The user defined policies (deployment policy, model policy, service policy) need to add constraints on these properties if needed.

## Built-in properties

* for node policy

**Name** | **Description** | **Possible values**
----- | ----- | -----
openhorizon.cpu | The number of CPUs (will be fetched from /proc/cpuinfo file) | `int` e.g. 4
openhorizon.memory| The amount of memory in MBs (will be fetched from /proc/meminfo)| `int` e.g. 1024
openhorizon.arch| The hardware architecture of the node (will be fetched from GOARCH)| `string` e.g. amd64
openhorizon.hardwareId| The device serial number if it can be found (will be fetched from /proc/cpuinfo). A generated Id otherwise. | `string`
openhorizon.allowPrivileged| Property set to determine if privileged services may be run on this device. Can be set by user, default is false. This is the only writable node property| `boolean` 
openhorizon.kubernetesVersion| Kubernetes version of the cluster the agent is running in| `string` e.g. 1.18

**Note:Provided properties (except for allowPrivileged) are read-only, the system will ignore updating of the node policy and changing any of the built-in properties*    

* for service policy

**Name** | **Description** | **Possible values**
----- | ----- | -----
openhorizon.service.url | The unique name of the service (comes from `url` field of service definition) | `string` e.g. https://someOrg/someService
openhorizon.service.name| The unique name of the service (comes from `url` field of service definition)| `string` e.g. MyService
openhorizon.service.org| The multi-tenant org where the service is defined (comes from `org` field of service definition) | `string` e.g. MyOrg
openhorizon.service.version| The version of a service using the same semantic version syntax (comes from `version` field of service definition)| `string` e.g. 1.1.1
openhorizon.service.arch| The hardware architecture of the node this service can run on (comes from `arch` field of service definition)| `string` e.g. amd64
openhorizon.allowPrivileged| Does the service use workloads that require privileged mode or net==host to run. Can be set by user. It is an error to set it to false if service introspection indicates that the service uses privileged features. (comes from `deployment.services.someServiceName.privileged` field of service definition) | `boolean`