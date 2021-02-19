# Deployment Policy

The OpenHorizon policy based, autonomous deployment capability is described [here](./policy.md).
A deployment policy is just one aspect of the deployment capability, and is described here in detail.

Use the `hzn exchange deployment new` command to generate an empty deployment policy.
The `hzn dev service new` command will also generate a deployment policy along with a new service.

Use the `hzn deploycheck` command to evaluate the compatibility of your deployment policy with the node where you want the service to be dpeloyed.

Following are the fields in the JSON representation of a deployment policy:
- `label`: A short description of the deployment policy suitable to be displayed in a UI. This field is not required.
- `description`: A longer description of the deployment policy. This field is not required.
- `services`: A list of service to be deployed. There MUST be at least one service in the list.
  - `name`: The name (URL) of a service to be deployed. This is the same value as found in the `url` field [here](./service_def.md).
  - `org`: The organization in which the service in `name` is defined.
  - `arch`: The hardware architecture of the service in `name`, or `*` to indicate any compatible architecture. This is the same value as found in the `arch` field [here](./service_def.md).
  - `serviceVersions`: A list of versions of the service. At least 1 version MUST be specified.
    - `version`: One of the versions of the service in `name`. This is the same value as found in the `version` field [here](./service_def.md).
    - `priority`: The relative priority of deploying this version over another version in the list of service versions.
      - `priority_value`: The priority value assigned to this version. Priority is expressed in human terms, where a lower ordinal value means higher priority. Priority values within the list are not required to be sequential, just unique within the list. When deploying a service, OpenHorizon will attempt to deploy the higest priority version first. If the service is not successfully started, the next highest version will be attempted.
      - `retries`: The number of times to retry starting a failed service.
      - `retry_durations`: The number of seconds (i.e. elapsed time) in which the indicated number of `retries` must occur before giving up and moving on to the next highest priority service version.
  - `nodeHealth`: For nodes that are expected to remain network connected to the management, these setting indicate how agressive the Agbot should be in determining if a node is out of policy.
    - `missing_heartbeat_interval`: The number of seconds a heartbeat can be missed (from the perspective of the management hub) until the node is considered missing. When a node is detected as missing, its agreements are cancelled by the Agbot.
    - `check_agreement_status`: The number of seconds between checks (by the management hub) to verify that the node still has an agreement for this service.
- `properties`: Policy properties as described [here](./properties_and_constraints.md) which a node policy constraint can refer to.
- `constraints`: Policy constraints as described [here](./properties_and_constraints.md) which refer to node policy properties.
- `userInput`: This section is used to set service variables for any service (including this service) that is deployed as a result of deploying this service.
  - `serviceUrl`: The name of the service to be configured. This is the same value as found in the `url` field [here](./service_def.md).
  - `serviceOrgid`: The organization in which the service in `serviceUrl` is defined.
  - `serviceArch`: The hardware architecture of the service in `serviceUrl`, or `*` to indicate any compatible architecture. This is the same value as found in the `arch` field [here](./service_def.md).
  - `serviceVersionRange`: A version range indicating the set of service versions to which this variable setting should be applied.
  - `inputs`: A list of service variables to set.
    - `name`: The name of the variable. This is the same as a variable name found in `userInputs` as defined [here](./service_def.md).
    - `value`: The value to be assigned to the variable. Service variables are typed as described in `userInputs` defined [here](./service_def.md).

The following is an example of a deployment policy that deploys a service called `my.company.com.service.this-service`.
The service is defined within organization `yourOrg`.
This policy will deploy the service to any node which matches one of the architectures for which the service is defined, and is also compatible with nodes that have the property `aNodeProperty` set to `someValue`.
Two versions of the service are mentioned, with version `2.3.1` having a higher priority for dpeloyment than version `2.3.0`.
The deployed service is dependent on service `my.company.com.service.other` which has a variable `var1` that needs to be set in order for it to deploy correctly.
```
{
  "label": "something short for a UI to display",
  "description": "a longer explanation of what this policy does",
  "service": {
    "name": "my.company.com.service.this-service",
    "org": "yourOrg",
    "arch": "*",
    "serviceVersions": [
      {
        "version": "2.3.0",
        "priority":{
          "priority_value": 3,
          "retries": 1,
          "retry_durations": 1800
        }
      },
      {
        "version": "2.3.1",
        "priority":{
          "priority_value": 2,
          "retries": 1,
          "retry_durations": 3600
        }
      }
    ],
    "nodeHealth": {
      "missing_heartbeat_interval": 60,
      "check_agreement_status": 60
    }
  },
  "properties": [
      {
          "name": "prop1",
          "type": int,
          "value": 11
      }
  ],
  "constraints": [
    "aNodeProperty==someValue"
  ],
  "userInput": [
    {
      "serviceOrgid": "serviceOrg",
      "serviceUrl": "my.company.com.service.other",
      "serviceArch": "*",
      "serviceVersionRange": "1.10.2",
      "inputs": [
        {
          "name": "var1",
          "value": "astring"
        }
      ]
    }
  ]
}
```