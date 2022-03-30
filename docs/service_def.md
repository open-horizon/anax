# Service Definition

Open Horizon deploys services to edge nodes, where those services are comprised of at least one container image and a configuration that conditions how the service executes.
Services are defined by a service definition, encoded in JSON. The attributes of a service definition are defined in this document.

## Structure

Service definitions come in 2 forms;
- `source`: A service definition file that is part of the source code in a service project, e.g. https://github.com/open-horizon/examples/blob/master/edge/services/helloworld/horizon/service.definition.json.
In this case, the service definition is used to create or update a service definition in the Open Horizon Exchange.
- `display`: A service definition displayed by `hzn exchange service list`.
In this case, the service definition has extra fields and some fields have a different datatype as compared to the source form version of a service definition.
The differences are noted below.

- `owner`: This is only part of the `display` form, reflecting the user that created the definition in the Exchange.
- `org`: The organization where the service is defined. Open Horizon supports multi-tenancy through the concept of an organization (org).
- `label`: A short textual label assigned to the service definition which could be displayed in a UI.
- `description`: A long textual description describing the service.
- `documentation`: A text field used to describe where to find formal documentation for a service. Usually this in the form of a URL.
- `public`: A boolean describing whether (true) or not (false) this service is available to be used by orgs other than the org where the service resides. This field should only be set to true if the service is truly reusable and the container image(s) contain publicly available information.
- `url`: The name of the service. The service does not have to be in form of a URL, but it does have to be unique. A best practice is to adhere to conventions that enable the owner of the service to provide a unique name, e.g. including your domain name, my-service.me.com.
- `version`: A 3 part, dotted decimal version string. In Open Horizon, versions have semantic meaning. Version `1.0.0` is known to be older than `1.0.1`. The last 2 decimal parts are optional. Version `1` is valid and semantically equivalent to `1.0` and `1.0.0`.
- `arch`: The hardware architecture of the service implementation in the container image. Valid values are those returned from the GOARCH constant in https://golang.org/pkg/runtime/. The anax agent can be configured to define aliases for these values, see https://github.com/open-horizon/anax/blob/master/test/docker/fs/etc/colonus/anax-combined.config.tmpl for an example. A service is deployed to edge nodes with the same hardware architecture.
- `sharable`: Can be one of 2 values; `singleton` or `multiple`. Services should be defined as multiple in most cases. The value of this field determines how many instances of the service's containers will be running on a node when the service is deployed more than once to the same node. Use `singleton` when the service is going to be used as a dependency by more than one service, AND those services all run together on a single node, AND the service implementation cannot tolerate multiple instances OR there are not enough resources to support multiple instances.
- `matchHardware`: Unused
- `requiredServices`: The list of services on which this service directly depends. A service in this list might have it's own required services. When deploying a service to a node, the full dependency tree is analyzed so that leaf services are started first, working recursively up the tree until the top level service is reached, and is started last. However, just because a service's dependencies are started first, does NOT guarantee that the dependencies are ready to process requests when the parent service is started. Parent services should be prepared to tolerate unavailable dependent services.
- `userInputs`: The list of variables that condition the behavior of the service implementation in the container image(s). These variables are typed; `string`, `int`, `float`, `boolean`, `list of strings` and MAY have a default value. If the `defaultValue` property is present, it MUST be populated with a string value, even if the `type` property is NOT a `string`.  Userinputs that DO NOT have a default value must be set in the `pattern` or `policy` that deploys the service. In some cases, userInputs need to be set on a per node basis, and therefore can be set on a node definition in the exchange `hzn exchange node update -f <userinput-settings-file>`
- `deployment`: The list of container images and container specific config for this service. See [deployment structure](./deployment_string.md) for more information on this field. In `display` form, this field is shown as stringified JSON. This field MAY be omitted if `clusterDeployment` is provided.
- `deploymentSignature`: The digital signature of the deployment field, created using an RSA key pair provided to `hzn exchange service publish`. It is a best practice to ALWAYS use the -K option when publishing a service, to ensure that the public key used to verify this signature is available for the agent to verify the signature.
- `clusterDeployment`: The Kubernetes Operator yaml for this service. See [deployment structure](./deployment_string.md) for more information on this field. In `display` form, this field is shown as stringified bytes and truncated. This field MAY be omitted if `deployment` is provided. The yaml files of a published service can be retrieved from the exchange using `hzn exchange service list -f <downloaded-yaml-file>`.
- `clusterDeploymentSignature`: The digital signature of the clusterDeployment field, created using an RSA key pair provided to `hzn exchange service publish`. It is a best practice to ALWAYS use the -K option when publishing a service, to ensure that the public key used to verify this signature is available for the agent to verify the signature.
