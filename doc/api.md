## Horizon APIs

This document contains the Horizon REST APIs for the Horizon agent running on an edge node. The output of the APIs is in JSON compact format. To get a better view, you can use the JSONView extension in your web browser or use the `jq` command from the command line interface. For example:

```
curl -s http://<ip>/status | jq '.'
```

### 1. Horizon Agent

#### **API:** GET  /status
---

Get the connectivity, configuration, and blockchain status on the node. The output includes the status of all blockchain containers, agent configuration and the node's connectivity. The only blockchain currently supported by Horizon is ethereum.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| geth   | json | the information about the ethereum client. |
| geth.net_peer_count | int64 |  the number of peers that the ethereum client is actively contacting. |
| geth.eth_syncing | boolean |  whether the ethereum client is syncing with the blockchain or not. |
| geth.eth_block_number | int64 | the latest block number that the client has imported. |
| geth.eth_balance| string | the current ether balance of the ethereum account. |
| geth.eth_accounts | array | an array of ethereum account numbers for this agent. |
| configuration| json| the configuration data.  |
| configuration.exchange_api | string | the url for the exchange being used by the Horizon agent. |
| configuration.architecture | string | the hardware architecture of the node as returned from the Go language API runtime.GOARCH. |
| connectivity | json | whether or not the node has network connectivity with some remote sites. |


**Example:**
```
curl -s http://localhost/status | jq '.'
[
  {
    "geth": {
      "net_peer_count": 6,
      "eth_syncing": false,
      "eth_block_number": 1684156,
      "eth_accounts": [
        "0x428ce7bcdc0459dd818c353ffc8a043f87ab3800"
      ],
      "eth_balance": "0x1bc16d674ec80000"
    },
    "configuration": {
      "exchange_api": "https://exchange.staging.bluehorizon.network/api/v1/",
      "architecture": "amd64"
    },
    "connectivity": {
      "firmware.bluehorizon.network": true,
      "images.bluehorizon.network": true
    }
  }
]


```

### 2. Node
#### **API:** GET  /node
---

Get the Horizon platform configuration of the Horizon agent. The configuration includes the agent's exchange id, organization, configuration state, and whether or not the agent is using a pattern configuration.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the agent's unique exchange id. |
| organization | string | the agent's organization. |
| pattern | string | the pattern that will be deployed on the node. |
| name | string | the user readable name for the agent.  |
| token_valid | bool| whether the agent's exchange token is valid or not. |
| token_last_valid_time | uint64 | the time stamp when the agent's token was last valid. |
| ha | bool | whether the node is part of an HA group or not. |
| configstate | json | the current configuration state of the agent. It contains the state and the last_update_time. The valid values for the state are "configuring", "configured", "unconfiguring", and "unconfigured". |

**Example:**
```
curl -s http://localhost/node | jq '.'
{
  "id": "myvs1",
  "organization": "mycompany",
  "pattern": "netspeed-amd64",
  "name": "mydevice",
  "token_last_valid_time": 1508174346,
  "token_valid": true,
  "ha": false,
  "configstate": {
    "state": "configured",
    "last_update_time": 1508174348
  }
}

```


#### **API:** POST  /node
---

Configure the Horizon agent. This API assumes that the agent's node has already been registered in the exchange. The configstate of the agent is changed to "configuring" after calling this API.

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the agent's unique exchange id. |
| token | string | the agent's authentication token for the exchange. |
| organization | string | the agent's organization. |
| pattern | string | the pattern that will be deployed on the node. |
| name | string | the user readable name for the agent.  |
| ha | bool | whether the node is part of an HA group or not. |

**Response:**

code:

* 201 -- success

**Example:**
```
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '{
      "id": "mydevice",
      "organization": "mycompany",
      "pattern": "pat3",
      "name": "mydevice",
      "token": "dfjskjdsfkj"
    }'  http://localhost/node

```


#### **API:** PATCH  /node
---

Update the agent's exchange token. This API can only be called when configstate is "configuring".

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the agent's unique exchange id. |
| token | string | the agent's authentication token for the exchange. |

**Response:**

code:

* 200 -- success

**Example:**
```
curl -s -w "%{http_code}" -X PATCH -H 'Content-Type: application/json'  -d '{
      "id": "mydevice",
      "token": "kj123idifdfjsklj"
    }'  http://localhost/node

```

#### **API:** DELETE  /node
---

Unconfigure the agent so that it can be re-configured. All agreements are cancelled, workloads, microservices and blockchain containers are stopped. The API could take minutes to send a respone if invoked with block=true. This API can only be called when configstate is "configured" or "configuring". After calling this API, configstate will be changed to "unconfiguring" while the agent quiesces, and then it will become "unconfigured".

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| block | bool | If true (the default), the API blocks until the agent is quiesced. If false, the caller will get control back quickly while the quiesce happens in the background. While this is occurring, the caller should invoke GET /node until they receive an HTTP status 404. |
| removeNode | bool | If true, the node’s entry in the exchange is also deleted, instead of just being cleared. The default is false. |

**Response:**

code:

* 204 -- success

body:

none

**Example:**
```
curl -s -w "%{http_code}" -X DELETE "http://localhost/node?block=true&removeNode=false"
```


#### **API:** GET  /node/configstate
---

Get the current configuration state of the agent.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| state   | string | Current configuration state of the agent. Valid values are "configuring", "configured", "unconfiguring", and "unconfigured". |
| last_update_time | uint64 | timestamp when the state was last updated. |

**Example:**

```
curl -s http://localhost/node/configstate |jq '.'
{
  "state": "configured",
  "last_update_time": 1510174292
}
```


#### **API:** PUT  /node/configstate
---

Change the configuration state of the agent. The valid values for the state are "configuring" and "configured". The "unconfigured" state is not settable through this API. The agent starts in the "configuring" state. You can change the state to "configured" after you have set the agent's pattern through the /node API, and have configured all the microservice and workload user input variables through the /microservice/config and /workload/config APIs. The agent will advertise itself as available for microservices and workloads once it enters the "configured" state.

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| state  | string | the agent configuration state. The valid values are "configuring" and "configured".|


**Response:**

code:

* 201 -- success

body:

none

**Example:**
```
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json'  -d '{
       "state": "configured"
    }'  http://localhost/node/configstate

```


### 3. Microservice

A *microservice* is a containerized service running on the node that provides an API to access a sensor on the node, or to provide other capability that a workload can use.

#### **API:** GET  /microservice
---

Get the configuration of each microservice, and the running and archived instances of microservices.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| config   | json | a list of microservices. |
| config[].attributes | a list of user defined attributes that have been configured on the microservice. |
| instances.active | json | a list of all actively running microservices. |
| instances.archived | json | a list of all archived microservices. |
| definitions.active | json | a list of microservice definitions that represent actively running microservices. |
| definitions.archived | json | a list of microservice definitions that represent archived microservices. |

microservice attribute

| name | type | description |
| ---- | ---- | ---------------- |
| meta | json |  the metadata that describes the attribute. It includes the id, the sensor_url, the label, the data type and whether or not the attribute is publishable or not. If the sensor_url is empty, the attribute is applied to all microservices. |
| {name, value} | json | The names and values of the attributes. Each type has a different set of name and value pairs. Supported attribute types are: architecture (architecture), compute (cpu, ram), location (lat, lon, location_accuracy_km, use_gps), UserInput (mappings). ha (partners), property (mappings), counterpartyproperty (expression), agreementprotocol (protocols), etc. |

A microservice instance has the following fields:

| name | type | description |
| ---- | ---- | ---------------- |
| ref_url | string | the specification reference url |
| version | string | the implementation version.of the microservice. |
| arch | string | the machine architecture this microservice implementation can run at. |
| instance_id | string | the record id. |
| archived | bool | archived or not |
| instance_creation_time | uint64 | the creation time for this microservice instance. |
| execution_start_time | uint64 | the time when the microservice containers are up and running. |
| execution_failure_code | uint64 | microservice instance execution failure code. |
| execution_failure_desc | string | the description for the microservice instance execution failure|
| cleanup_start_time | uint64 | the time when the microservice instance started the cleanup process before getting archived. |
| associated_agreements | array | an array of agreement ids that are currently associated with this microservice instance. |
| microservicedef_id | string | the record id of the microservice definition that this instance is for. |

A microservice definition has the following fields:

| name | type | description |
| ---- | ---- | ---------------- |
| record_id | string | a record id in the db |
| owner | string | the owner of this microservice. The format is company/user. |
| label | string | a simple label for this microservice. |
| description | string | the description. |
| specRef | string | the microservice specification reference url. |
| version | string | the implementation version. |
| arch | string | the machine architecture this microservice implementation can run at. |
| organization | string | the company what owns this microservice. |
| sharable | string | the sharing mode for the microservice containers. The valid values are: exclusive, single and multiple, where single means 1 instance shared by x workloads, multiple means x instances each supporting a workload. |
| downloadUrl | string | not used yet. |
| matchHardware | json | match hardware. |
| userInput | array | an array of user input definitions, each containing name, lable, date type and default value. |
| workloads | array | an array of container deployment information, each containing deployment images, deployment signature etc. |
| lastUpdated | string | the last time the record was updated in the exchange. |
| archived | bool | whether it is archived or not. |
| name | string | a short name used for policy file name. it is from the user request where the user registers a microservice through the /service api. |
| upgrade_version_range | string | a version range. it is from the user request where the user registers a microservice through the /service api. |
| auto_upgrade | bool | if the microservice should be automatically upgraded when new versions become available. it is from the user request where the user registers a microservice through the /service api.|
| active_upgrade | bool | horizon to actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade. it is from the user request where the user registers a microservice through the /service api. |
| upgrade_start_time | uint64 | microservice upgrading start time |
| upgrade_ms_unregistered_time | uint64 | the time when microservice was temporally unregistered during the upgrade process.  |
| upgrade_agreements_cleared_time | uint64 | the time when all the associated agreements were deleted during the upgrade process. |
| upgrade_execution_start_time | uint64 | the time when microservice containers are up and running during the upgrade process. |
| upgrade_ms_reregistered_time | uint64 | the time when the microservice was reregisterd in the exchange during the upgrade process.  |
| upgrade_failed_time | uint64 | upgrade failed time. |
| upgrade_failure_reason | uint64 | upgrade failure reason code. |
| upgrade_failure_description | string | detailed description for the upgrade failure. |
| upgrade_new_ms_id | string | the record id for the new microservice definition that this one will upgrade to. |
| metadata_hash | string | a hash of the record in the exchange for this microservice definition. |

**Example:**
```
curl -s -w %{http_code}  http://localhost/microservice | jq  '.'

{
  "config": [
    {
      "sensor_url": "https://bluehorizon.network/microservices/gps",
      "sensor_version": "2.0.4",
      "sensor_org": "e2edev",
      "active_upgrade": true,
      "auto_upgrade": false,
      "attributes": [
        {
          "ram": 1024,
          "cpus": 1,
          "meta": {
            "publishable": true,
            "host_only": false,
            "label": "Compute Resources",
            "sensor_urls": [
              "https://bluehorizon.network/microservices/gps"
            ],
            "type": "ComputeAttributes",
            "id": "ce72bbb7-725d-49ea-b1f7-72aa0f6c6c82"
          }
        },
        {
          "architecture": "amd64",
          "meta": {
            "publishable": true,
            "host_only": false,
            "label": "Architecture",
            "sensor_urls": [
              "https://bluehorizon.network/microservices/gps"
            ],
            "type": "ArchitectureAttributes",
            "id": "edbca886-4f7f-444b-931c-c0a92a5d884f"
          }
        }
      ]
    }
  ],
  "definitions": {
    "archived": [],
    "active": [
      {
        "metadata_hash": "koBj1yLaigwUIAg7DdmdKOpRKLkgR242yzWrRncyJSc=",
        "upgrade_new_ms_id": "",
        "upgrade_failure_description": "",
        "upgrade_failure_reason": 0,
        "upgrade_failed_time": 0,
        "upgrade_ms_reregistered_time": 0,
        "upgrade_execution_start_time": 0,
        "upgrade_agreements_cleared_time": 0,
        "upgrade_ms_unregistered_time": 0,
        "upgrade_start_time": 0,
        "active_upgrade": true,
        "auto_upgrade": false,
        "upgrade_version_range": "2.0.4",
        "arch": "amd64",
        "version": "2.0.4",
        "organization": "e2edev",
        "specRef": "https://bluehorizon.network/microservices/gps",
        "description": "GPS microservice",
        "label": "GPS for x86_64",
        "owner": "e2edev/agbot1",
        "record_id": "5",
        "sharable": "exclusive",
        "downloadUrl": "",
        "matchHardware": {
          "devFiles": "/dev/ttyUSB*,/dev/ttyACM*",
          "usbDeviceIds": "1546:01a7"
        },
        "userInput": [],
        "workloads": [...],
        "lastUpdated": "2017-11-17T12:11:28.811Z[UTC]",
        "archived": false,
        "name": "bluehorizon.network-microservices-gps_e2edev_2.0.4"
      }
    ]
  },
  "instances": {
    "archived": [],
    "active": [
      {
        "containers": [
          {
            "Mounts": [
              {
                "Mode": "ro",
                "Destination": "/workload_config",
                "Source": "/tmp/anaxRO/bluehorizon.network-microservices-gps_2.0.4_4b725595-e8b8-4554-9468-311bf94f5287"
              }
            ],
            "NetworkSettings": {
              "Networks": {
                "bluehorizon.network-microservices-gps_2.0.4_4b725595-e8b8-4554-9468-311bf94f5287": {...}
              }
            },
            "Labels": {...},
            "Id": "8ed1893fa389373f499ce39089f0314a55da68c2518726250018417872580c47",
            "Image": "summit.hovitos.engineering/x86/gps:2.0.3",
            "Command": "/bin/sh -c /gps",
            "Created": 1510920755,
            "State": "running",
            "Status": "Up 3 hours",
            "Ports": [
              {
                "Type": "tcp",
                "PrivatePort": 31779
              }
            ],
            "Names": [
              "/bluehorizon.network-microservices-gps_2.0.4_4b725595-e8b8-4554-9468-311bf94f5287-gps"
            ]
          }
        ],
        "microservicedef_id": "5",
        "associated_agreements": [
          "38220bf2b45f844d57ad607a2a5bfccafc02991d354a886320194b497ed46be5"
        ],
        "cleanup_start_time": 0,
        "execution_failure_desc": "",
        "ref_url": "https://bluehorizon.network/microservices/gps",
        "version": "2.0.4",
        "arch": "amd64",
        "instance_id": "4b725595-e8b8-4554-9468-311bf94f5287",
        "archived": false,
        "instance_creation_time": 1510920729,
        "execution_start_time": 1510920755,
        "execution_failure_code": 0
      }
    ]
  }
}
```

#### **API:** GET  /microservice/config
---

Get the current list of registered microservices.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| config   | json | a list of microservices. |
| config[].attributes | a list of user defined attributes that have been configured on the microservice. |

**Example:**
```
{
  "config": [
    {
      "sensor_url": "https://bluehorizon.network/microservices/gps",
      "sensor_version": "2.0.4",
      "sensor_org": "e2edev",
      "active_upgrade": true,
      "auto_upgrade": false,
      "attributes": [
        {
          "ram": 1024,
          "cpus": 1,
          "meta": {
            "publishable": true,
            "host_only": false,
            "label": "Compute Resources",
            "sensor_urls": [
              "https://bluehorizon.network/microservices/gps"
            ],
            "type": "ComputeAttributes",
            "id": "ce72bbb7-725d-49ea-b1f7-72aa0f6c6c82"
          }
        },
        {
          "architecture": "amd64",
          "meta": {
            "publishable": true,
            "host_only": false,
            "label": "Architecture",
            "sensor_urls": [
              "https://bluehorizon.network/microservices/gps"
            ],
            "type": "ArchitectureAttributes",
            "id": "edbca886-4f7f-444b-931c-c0a92a5d884f"
          }
        }
      ]
    }
  ]
}
```

#### **API:** GET  /microservice/policy
---

Get the current list of policies for each registered microservice.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| {policy name} | json | the name of a policy generated for a microservice. |
| {policy name}.policy | json | the policy document. |

policy

| name | type | description |
| ---- | ---- | ---------------- |
| header | json|  the header of the policy. It includes the name and the version of the policy. |
| apiSpec | array | an array of api specifications. Each one includes a URL pointing to the definition of the API spec, the version of the API spec in OSGI version format, the organization that implements the API spec, whether or not exclusive access to this API spec is required and the hardware architecture of the API spec implementation. |
| agreementProtocols | array | an array of agreement protocols. Each one includes the name of the agreement protocol.|
| maxAgreements| int | the maximum number of agreements allowed to make. |
| properties | array | an array of name value pairs that the current party have. |
| counterPartyProperties | json | an array of (name, value, op)s that the counter party is required to have. |
| requiredWorkload | string | the name of the workload that is required. |
| ha_group | json | a list of ha partners. |
| blockchains| array | an array of blockchain specifications including bockchain type, boot nodes, network ids etc. |

Note: The policy also contains other fields that are unused and therefore not documented.

**Example:**
```
{
  "Policy for locgps": {
    "nodeHealth": {},
    "ha_group": {},
    "header": {
      "version": "2.0",
      "name": "Policy for locgps"
    },
    "apiSpec": [
      {
        "arch": "amd64",
        "exclusiveAccess": true,
        "version": "2.0.4",
        "organization": "e2edev",
        "specRef": "https://bluehorizon.network/microservices/locgps"
      }
    ],
    "valueExchange": {},
    "resourceLimits": {},
    "dataVerification": {
      "metering": {}
    },
    "proposalRejection": {},
    "maxAgreements": 1,
    "properties": [
      {
        "value": "1",
        "name": "cpus"
      },
      {
        "value": "1024",
        "name": "ram"
      }
    ]
  }
}
```

#### **API:** POST  /microservice/config
---

Register a microservice.

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| sensor_url  | string | the url for the microservice specification. |
| sensor_name  | string | the name of the microservice. |
| sensor_version  | string | the version range of the microservice. It should comply with the OSGI version specification. |
| sensor_org  | string | the organization that holds the microservice definition. |
| auto_upgrade  | bool | If the microservice should be automatically upgraded when new versions become available. |
| active_upgrade  | bool | If horizon should actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade. |
| attributes | array | an array of attributes.  Please refer to the response body for the GET /attribute api for the fields of an attribute.  |

**Response:**

code:

* 201 -- success

body:

none

**Example:**
```
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '{
  "sensor_url": "https://bluehorizon.network/microservices/network",
  "sensor_name": "network",
  "sensor_org": "mycompany",
  "sensor_version": "1.0.0",
  "auto_upgrade": true,
  "active_upgrade": true,
  "attributes": [
    {
      "type": "ComputeAttributes",
      "label": "network microservice",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "ram": 128,
        "cpus": 1
      }
    }
  ]
}'  http://localhost/microservice/config
```

### 4. Attributes

#### **API:** GET  /attribute
---

Get all the attributes for the registered services.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attributes | array | an array of all the attributes for all the services. The fields of an attribute are defined in the following. |

attribute

| name | type| description |
| ---- | ---- | ---------------- |
| id | string| the id of the attribute. |
| label | string | the user readable name of the attribute |
| type| string | the attribute type. Supported attribute types are: ArchitectureAttributes, ComputeAttributes, LocationAttributes, MappedAttributes, HAAttributes, PropertyAttributes, CounterPartyPropertyAttributes, MeteringAttributes, AgreementProtocolAttributes and ArchitectureAttributes. |
| sensor_urls | array | an array of sensor url. It applies to all services if it is empty. |
| publishable| bool | whether the attribute can be made public or not. |
| host_only | bool | whether or not the attribute will be passed to the workload. |
| mappings | map | a list of key value pairs. |


**Example:**
```
curl -s http://localhost/attribute | jq '.'
{
  "attributes": [
    {
      "id": "38596bf5-6713-422a-ad87-5892b819ba7a",
      "type": "LocationAttributes",
      "sensor_urls": [],
      "label": "Registered Location Facts",
      "publishable": false,
      "mappings": {
        "lat": 40.4273,
        "lon": -111.898,
        "use_gps": false,
        "location_accuracy_km": 0.5
      }
    },
    {
      "id": "55fc2373-d47c-48bc-9934-22f3e2e4de78",
      "type": "ArchitectureAttributes",
      "sensor_urls": [
        "https://bluehorizon.network/microservices/network"
      ],
      "label": "Architecture",
      "publishable": true,
      "mappings": {
        "architecture": "amd64"
      }
    },
    {
      "id": "692d375b-031c-4c90-95ef-c1a038c0b551",
      "type": "ComputeAttributes",
      "sensor_urls": [
        "https://bluehorizon.network/microservices/gps"
      ],
      "label": "gps microservice",
      "publishable": true,
      "mappings": {
        "cpus": 1,
        "ram": 128
      }
    },
    {
      "id": "9aeea17c-73cf-48b7-ba65-a678c60c2637",
      "type": "ArchitectureAttributes",
      "sensor_urls": [
        "https://bluehorizon.network/microservices/gps"
      ],
      "label": "Architecture",
      "publishable": true,
      "mappings": {
        "architecture": "amd64"
      }
    },
    {
      "id": "cce6681d-cc50-480d-ab3c-36cf2f1b10a5",
      "type": "AgreementProtocolAttributes",
      "sensor_urls": [],
      "label": "Agreement Protocol",
      "publishable": true,
      "mappings": {
        "protocols": [
          {
            "name": "Basic",
            "protocolVersion": 1
          }
        ]
      }
    },
    {
      "id": "ed356f2c-7c4d-432e-80e8-82064b5703bd",
      "type": "ComputeAttributes",
      "sensor_urls": [
        "https://bluehorizon.network/microservices/network"
      ],
      "label": "network microservice",
      "publishable": true,
      "mappings": {
        "cpus": 1,
        "ram": 128
      }
    }
  ]
}
```

#### **API:** POST  /attribute
---

Register an attribute for a microservice. If the sensor_url is omitted, the attribute applies to all the microservices.

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute api for the fields of an attribute.  |

**Response:**

code:

* 201 -- success

body:

none

**Example:**
```
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json' -d '{
    "type": "ComputeAttributes",
    "label": "Compute Resources",
    "publishable": true,
    "host_only": false,
    "mappings": {
      "ram": 256,
      "cpus": 1
    }
  }'  http://localhost/attribute

```


#### **API:** GET  /attribute/{id}
---

Get the attribute with the given id

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type| description |
| ---- | ---- | ---------------- |
| id | string| the id of the attribute. |
| label | string | the user readable name of the attribute |
| type| string | the attribute type. Supported attribute types are: ArchitectureAttributes, ComputeAttributes, LocationAttributes, UserInputAttributes, HAAttributes, PropertyAttributes, CounterPartyPropertyAttributes, MeteringAttributes, AgreementProtocolAttributes and ArchitectureAttributes. |
| sensor_urls | array | an array of sensor url. It applies to all microservices if it is empty. |
| publishable| bool | whether the attribute can be made public or not. |
| host_only | bool | whether or not the attribute will be passed to the microservice. |
| mappings | map | a list of key value pairs. |


**Example:**
```
curl -s -w "%{http_code}" http://localhost/attribute/0d5762bf-67a6-49ff-8ff9-c0fd32a8699f | jq '.'
{
  "attributes": [
    {
      "id": "0d5762bf-67a6-49ff-8ff9-c0fd32a8699f",
      "type": "UserInputAttributes",
      "sensor_urls": [
        "https://bluehorizon.network/microservices/gps"
      ],
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "foo": "hello"
      }
    }
  ]
}
200

```


#### **API:** PUT, PATCH  /attribute/{id}
---

Modify an attribute for a microservice. If the sensor_url is omitted, the attribute applies to all the microservices.

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute/{id} api for the fields of an attribute.  |

**Response:**

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute/{id} api for the fields of an attribute.  |


**Example:**
```
curl -s -w "%{http_code}" -X PUT -d '{
      "id": "0d5762bf-67a6-49ff-8ff9-c0fd32a8699f",
      "type": "UserInputAttributes",
      "sensor_urls": [
        "https://bluehorizon.network/microservices/gps"
      ],
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "foo": "bar"
      }
    }' http://localhost/attribute/0d5762bf-67a6-49ff-8ff9-c0fd32a8699f | jq '.'

```

#### **API:** DELETE /attribute/{id}
---

Modify an attribute for a service. If the sensor_url is omitted, the attribute applies to all the services.

**Parameters:**

none

**Response:**

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute/{id} api for the fields of an attribute.  |


**Example:**
```
curl -s -w "%{http_code}" -X DELETE http://localhost/attribute/0d5762bf-67a6-49ff-8ff9-c0fd32a8699f | jq '.'

```

### 5. Agreement

#### **API:** GET  /agreement
---

Get all the active and archived agreements ever made by the agent. The agreements that are being terminated but not yet archived are treated as archived in this api.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| agreements  | json | contains active and archived agreements |
| active | array | an array of current agreements. The attributes of each agreement are defined in the following rows. |
| archived | array | an array of canceled agreements. The attributes of each agreement are defined in the following rows. |
| name | string |  the name of the policies used to make the agreement.  |
| current_agreement_id | string | the id of the agreement. |
| counterparty_address | string |  the ethereum account of the agbot. |
| consumer_id| string |  the id of the agbot that proposed the agreement. |
| agreement_creation_time | uint64 | the time when the agent received the agreement proposal from the agbot. The negotiation process starts. |
| agreement_accepted_time | uint64 | the time when the agbot and the agent have come to agreement on the terms. Workload downloading starts. |
| agreement_execution_start_time | uint64 | the time when the agent starts running the workloads. |
| agreement_finalized_time | uint64 | the time when the agbot and the agent have finalized the agreement. Workloads are running and data is verified by the agbot. |
| agreement_data_received_time | uint64 | the time when the agbot has verified that data was received from the workload. |
| agreement_terminated_time| uint64 | the time when the agreement is terminated. |
| terminated_reason| uint64 | the reason code for the agreement termination. |
| terminated_description | string | the description of the agreement termination. |
| agreement_protocol_terminated_time | uint64 | the time when the agreement protocol terminated. |
| workload_terminated_time | uint64 | the time when the workload for an agreement terminated. |
| proposal| string | the proposal currently in effect. |
| proposal_sig| string | the proposal signature. |
| agreement_protocol | string | the name of the agreement protocol being used. |
| protocol_version | int | the version of the agreement protocol being used. |
| current_deployment | json | contains the deployment configuration for the workload. The key is the name of the workload and the value is the result of the [/containers/<id> docker remote API call](https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/inspect-a-container) for the workload container. Please refer to the link for details. |
| archived | bool |  if the agreement is archived or not.  |
| metering_notification | json |  the most recent metering notification received. It includes the amount, metering start time, data missed time, consumer address, consumer signature etc. |


**Example:**
```
curl -s http://localhost/agreement | jq '.'
{
  "agreements": {
    "active": [
      {
        "name": "Policy for netspeed merged with netspeed arm",
        "sensor_url": ["https://bluehorizon.network/documentation/netspeed-device-api"],
        "archived": false,
        "current_agreement_id": "7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f",
        "consumer_id": "stg-agbot-lon02-01.horizon.hovitos.engineering",
        "counterparty_address": "76224dc349ce6a51728f32d0be3a33349bdd0680",
        "agreement_creation_time": 1481235346,
        "agreement_accepted_time": 1481235346,
        "agreement_finalized_time": 1481235492,
        "agreement_terminated_time": 0,
        "agreement_data_received_time": 1481235526,
        "agreement_execution_start_time": 1481235515,
        "current_deployment": {
          "netspeed5": {
            "config": {
              "Cpuset": "1-3",
              "Env": [
                "DEVICE_ID=00000000217597c7",
    ...
           ],
              "Cmd": null,
              "Image": "summit.hovitos.engineering/armhf/netspeed5:v1.8",
              "Volumes": {
                "/var/snap/bluehorizon/common/workload_ro/7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f": {}
              },
              "Entrypoint": null,
              "Labels": {
                "network.bluehorizon.colonus.agreement_id": "7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f",
                "network.bluehorizon.colonus.deployment_description_hash": "mE-GfPU0RkH6lSWPU0F1WPmpooU=",
                "network.bluehorizon.colonus.service_name": "netspeed5",
                "network.bluehorizon.colonus.variation": ""
              }
            },
            "host_config": {
              "Binds": [
                "/var/snap/bluehorizon/common/workload_ro/7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f:/workload_config:ro"
              ],
              "RestartPolicy": {
                "Name": "always"
              },
              "LogConfig": {
                "Type": "syslog",
                "Config": {
                  "tag": "workload-7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f_netspeed5"
                }
              }
            }
          }
        },
        "proposal"...",
        "proposal_sig": "174f67d343fcb0241e9bd8c01ef93f9cb17e1eb434f0234af6e5a3afcd227d93229bb97d26a2b9fae706aa3e5f2521a8dc48503405d682f319cc8925dcc3c34c01",
        "agreement_protocol": "Citizen Scientist",
        "terminated_reason": 0,
        "terminated_description": ""
        "agreement_protocol_terminated_time": 0,
        "workload_terminated_time": 0,
        "metering_notification": {
            "amount": 42,
            "start_time": 1496257354,
            "current_time": 1496259896,
            "missed_time": 10,
            "consumer_meter_signature": "6955047f484573055a9a27b80c5f86daa9d6ae48ac...",
            "agreement_hash": "91a9099372ade4a41db26b0f27819d60080f7774d51e71dc026269c3ae86d727",
            "consumer_agreement_signature": "cf08e366c59b4e130a4f05d1921f3f56d899f...",
            "consumer_address": "0xc299ff03ff89b48c6e56c25bd6b53932b100f9d6",
            "producer_agreement_signature": "0ee94042d5c50e8773e23923e32826e4f01df...",
            "blockchain_type": "ethereum"
        }
      }
    ],
    "archived": []
  }
}


```

#### **API:** DELETE  /agreement/{id}
---

Delete an agreement. The agbot will start a new agreement negotiation with the agent after the agreement is deleted.

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the id of the agreement to be deleted. |

**Response:**

code:

* 200 -- success
* 404 -- the agreement does not exist.


body:

none

**Example:**
```
curl -X DELETE -s http://localhost/agreement/a70042dd17d2c18fa0c9f354bf1b560061d024895cadd2162a0768687ed55533

```

### 6. Workload

#### **API:** GET  /workload
---

Get the detailed information for all the workloads currently running on the device.

**Parameters:**

none

**Response:**

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| config | array | a list of workload configurations for workloads that can run on the node. See the GET /workload/config API for a description of the config section. |
| containers | array | an array of workload containers current running on the node. Each array element is the result of /containers docker remote API call. Please refer to [/containers docker API] (https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/list-containers) for details. |

**Example:**
```
{
  "containers": [
    {
      "Mounts": [...],
      "NetworkSettings": {'...'},
      "Id": "4669f1f36663d724300bcedde0fe72547b19113049365a6c90049f86c07c2048",
      "Image": "summit.hovitos.engineering/x86/eaweather:v1.8",
      "Command": "/bin/sh -c /start.sh",
      "Created": 1510934587,
      "State": "running",
      "Status": "Up About an hour",
      "Names": [
        "/32b1559e27860eef8828cc11cfa3cb7e1a4b4d243235fa20844ab6a97099ee76-eaweather"
      ],
      "Labels": {
        "network.bluehorizon.colonus.variation": "",
        "network.bluehorizon.colonus.service_name": "eaweather",
        "network.bluehorizon.colonus.deployment_description_hash": "gkzGNGxrxFERkg_JuDCtvWc8xeI=",
        "network.bluehorizon.colonus.agreement_id": "32b1559e27860eef8828cc11cfa3cb7e1a4b4d243235fa20844ab6a97099ee76"
      }
    }
  ],
  "config": [
    {
      "workload_url": "https://bluehorizon.network/workloads/weather"
      "workload_version": "[1.0.0,INFINITY)",
      "organization": "e2edev",
      "attributes": [
        {
          "mappings": {
            "MTN_PWS_ST_TYPE": "WS23xx",
            "MTN_PWS_MODEL": "LaCrosse WS2317",
            "HZN_WUGNAME": "e2edev mocked pws",
            "HZN_PWS_ST_TYPE": "WS23xx",
            "HZN_PWS_MODEL": "LaCrosse WS2317"
          },
          "meta": {
            "publishable": false,
            "host_only": false,
            "label": "User input variables",
            "sensor_urls": [],
            "type": "UserInputAttributes",
            "id": ""
          }
        }
      ]
    }
  ]
}
```

#### **API:** GET  /workload/config
---

Get the detailed configuration for all the workloads currently running on the node.

**Parameters:**

none

**Response:**

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| config | array | a list of workload configurations for workloads that can run on the node. |

configuration:

| name | type| description |
| ---- | ---- | ---------------- |
| workload_url | string | the specification url for the workload. |
| workload_version| string | the version range for the workload. |
| organization | string | the organization the workload belongs to. |
| attributes | map | a list of attributes containing configuration for the workload. Currently the only supported attribute is UserInputAttributes which is used for configuring workload variables. See the GET /attributes API for a description of the fields of an attribute. |

**Example:**
```
{
  "config": [
    {
      "workload_url": "https://bluehorizon.network/workloads/weather"
      "workload_version": "[1.0.0,INFINITY)",
      "organization": "e2edev",
      "attributes": [
        {
          "meta": {
            "type": "UserInputAttributes",
            "publishable": false,
            "host_only": false,
            "label": "User input variables",
            "sensor_urls": [],
            "id": ""
          },
          "mappings": {
            "MTN_PWS_ST_TYPE": "WS23xx",
            "MTN_PWS_MODEL": "LaCrosse WS2317",
            "HZN_WUGNAME": "e2edev mocked pws",
            "HZN_PWS_ST_TYPE": "WS23xx",
            "HZN_PWS_MODEL": "LaCrosse WS2317"
          },
        }
      ]
    }
  ]
}
```

#### **API:** POST  /workload/config
---

Configure the user input variables of a workload that might run on the node.

**Parameters:**

body:

| name | type| description |
| ---- | ---- | ---------------- |
| workload_url | string | the url that identifies the workload. |
| workload_version| string | the version range of the workload. |
| organization | string | the organization that owns the workload. |
| attributes | map | a list of attributes containing configuration for the workload. Currently the only supported attribute is UserInputAttributes which is used for configuring workload variables. See the GET /attributes API for a description of the fields of an attribute. |

**Response:**

code:

* 201 -- success

**Example:**
```
{
  "workload_url": "https://bluehorizon.network/workloads/weather"
  "workload_version": "[1.0.0,INFINITY)",
  "organization": "e2edev",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "publishable": false,
      "host_only": false,
      "label": "User input variables",
      "mappings": {
        "foo": "bar"
      },
    }
  ]
}
```

#### **API:** DELETE  /workload/config
---

Delete the workload configuration.

**Parameters:**

body:

| name | type| description |
| ---- | ---- | ---------------- |
| workload_url | string | the url that identifies the workload. |
| workload_version| string | the version range of the workload. |
| organization | string | the organization that owns the workload. |

**Response:**

code:

* 204 -- success

**Example:**
```
curl -s -w "%{http_code}" -X DELETE -H 'Content-Type: application/json' -d '{
  "workload_url": "https://bluehorizon.network/workloads/pws",
  "organization": "IBM",
  "workload_version": "1.8"
}
'  http://localhost/workload/config

```


### 7. Public Keys for Workload Image Verification

#### **API:** GET  /publickey
---

Get the user stored public keys for container image verification.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| pem  | json | an array of public key files or x509 certs that have previously been PUT to the agent. |

**Example:**
```
curl -s http://localhost/publickey | jq  '.'
{
  "pem": ["akeyfile.pem"]
}

```

#### **API:** GET  /publickey/{filename}
---

Get the content of the user public key or x509 cert from a file that has been previously stored to the agent.

**Parameters:**

| name | type | description |
| -----| ---- | ---------------- |
| filename | string | the name of the public key file to retrieve. |

**Response:**

code:
* 200 -- success

body:

The contents of the requested file.

**Example:**
```
curl -s http://localhost/publickey/akeyfile.pem > akeyfile.pem

```

#### **API:** PUT  /publickey/{filename}
---

Put a user public key or cert to the agent for container image verification.

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| filename | string | the name of the public key file to upload. |

**Response:**

code:
* 200 -- success

body:

none

**Example:**
```
curl -T ~/.rsapsstool/keypairs/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem http://localhost/publickey/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem

```

#### **API:** DELETE  /publickey/{filename}
---

Delete a user public key or x509 cert from the agent.

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| filename | string | the name of the public key file to remove. |

**Response:**

code:
* 204 -- success

body:

none

**Example:**
```
curl -s -X DELETE http://localhost/publickey/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem

```
