## Horizon APIs

This document contains the Horizon REST APIs for the Horizon agent running on an edge node. The output of the APIs is in JSON compact format. To get a better view, you can use the JSONView extension in your web browser or use the `jq` command from the command line interface. For example:

```
curl -s http://<ip>/status | jq '.'
```

### 1. Horizon Agent

#### **API:** GET  /status
---

Get the connectivity, and configuration, status on the node. The output includes the status of the agent configuration and the node's connectivity.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| configuration| json| the configuration data.  |
| configuration.exchange_api | string | the url for the exchange being used by the Horizon agent. |
| configuration.exchange_version | string | the current version of the exchange being used. |
| configuration.preferred_exchange_version | string | the preferred version for the exchange in order to use all the horizon functions. |
| configuration.required_minimum_exchange_version | string | the required minimum version for the exchange. |
| configuration.architecture | string | the hardware architecture of the node as returned from the Go language API runtime.GOARCH. |
| connectivity | json | whether or not the node has network connectivity with some remote sites. |


**Example:**
```
curl -s http://localhost/status | jq '.'
[
  {
    "configuration": {
      "exchange_api": "https://exchange.staging.bluehorizon.network/api/v1/",
      "exchange_version": "1.55.0",
      "required_minimum_exchange_version": "1.49.0",
      "preferred_exchange_version": "1.55.0",
      "architecture": "amd64",
      "horizon_version": "2.17.2"
    },
    "connectivity": {
      "firmware.bluehorizon.network": true,
      "images.bluehorizon.network": true
    }
  }
]


```

#### **API:** GET  /status/workers
---

Get the current Horizon agent worker status and the status trasition logs. 
**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| workers   | json | the current status of each worker and its subworkers. |
| worker_status_log | string array |  the history of the worker status changes. |


**Example:**
```
curl -s  http://localhost/status/workers |jq
{
  "workers": {
    "AgBot": {
      "name": "AgBot",
      "status": "initialization failed",
      "subworker_status": {}
    },
    "Agreement": {
      "name": "Agreement",
      "status": "initialized",
      "subworker_status": {
        "HeartBeat": "started"
      }
    },
    "Container": {
      "name": "Container",
      "status": "initialized",
      "subworker_status": {}
    },
    "Exchange": {
      "name": "Exchange",
      "status": "initialized",
      "subworker_status": {}
    },
    "Governance": {
      "name": "Governance",
      "status": "initialized",
      "subworker_status": {
        "BlockchainGovernor": "started",
        "ContainerGovernor": "started",
        "MicroserviceGovernor": "started"
      }
    },
    "Torrent": {
      "name": "Torrent",
      "status": "initialized",
      "subworker_status": {}
    }
  },
  "worker_status_log": [
    "2018-05-02 19:25:02 Worker Torrent: started.",
    "2018-05-02 19:25:02 Worker Torrent: initialized.",
    "2018-05-02 19:25:02 Worker AgBot: started.",
    "2018-05-02 19:25:02 Worker AgBot: initialization failed.",
    "2018-05-02 19:25:02 Worker Agreement: started.",
    "2018-05-02 19:25:02 Worker Governance: started.",
    "2018-05-02 19:25:02 Worker Exchange: started.",
    "2018-05-02 19:25:02 Worker Container: started.",
    "2018-05-02 19:25:03 Worker Container: initialized.",
    "2018-05-02 19:25:07 Worker Agreement: initialized.",
    "2018-05-02 20:17:35 Worker Agreement: subworker HeartBeat added.",
    "2018-05-02 20:17:35 Worker Agreement: subworker HeartBeat started.",
    "2018-05-02 20:17:38 Worker Exchange: initialized.",
    "2018-05-02 20:17:38 Worker Governance: subworker ContainerGovernor added.",
    "2018-05-02 20:17:38 Worker Governance: subworker MicroserviceGovernor added.",
    "2018-05-02 20:17:38 Worker Governance: initialized.",
    "2018-05-02 20:17:38 Worker Governance: subworker MicroserviceGovernor started.",
    "2018-05-02 20:17:38 Worker Governance: subworker ContainerGovernor started.",
  ]
}

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

Unconfigure the agent so that it can be re-configured. All agreements are cancelled, and workloads are stopped. The API could take minutes to send a response if invoked with block=true. This API can only be called when configstate is "configured" or "configuring". After calling this API, configstate will be changed to "unconfiguring" while the agent quiesces, and then it will become "unconfigured".

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

Change the configuration state of the agent. The valid values for the state are "configuring" and "configured". The "unconfigured" state is not settable through this API. The agent starts in the "configuring" state. You can change the state to "configured" after you have set the agent's pattern through the /node API, and have configured all the service user input variables through the /service/config API. The agent will advertise itself as available for services once it enters the "configured" state.

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

### 3. Attributes

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
        "https://bluehorizon.network/services/network"
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
        "https://bluehorizon.network/services/gps"
      ],
      "label": "gps service",
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
        "https://bluehorizon.network/services/gps"
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
        "https://bluehorizon.network/services/network"
      ],
      "label": "network service",
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

Register an attribute for a service. If the sensor_url is omitted, the attribute applies to all the services.

**Parameters:**

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to [Attribute Definitions](https://github.com/open-horizon/anax/blob/master/doc/attributes.md) for a description of all attributes.  |

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
| type| string | the attribute type. Supported attribute types are: ArchitectureAttributes, ComputeAttributes, LocationAttributes, UserInputAttributes, HAAttributes, PropertyAttributes, CounterPartyPropertyAttributes, MeteringAttributes, and AgreementProtocolAttributes. |
| sensor_urls | array | an array of sensor url. It applies to all services if it is empty. |
| publishable| bool | whether the attribute can be made public or not. |
| host_only | bool | whether or not the attribute will be passed to the service. |
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
        "https://bluehorizon.network/services/gps"
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

Modify an attribute for a service. If the sensor_url is omitted, the attribute applies to all the services.

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
        "https://bluehorizon.network/services/gps"
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

### 4. Agreement

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

### 5. Trusted Certs for Service Image Verification

#### **API:** GET  /trust[?verbose=true]
---

Get the user stored x509 certificates for service container image verification.

**Parameters:**

| name | type | description |
| -----| ---- | ---------------- |
| (query) verbose | string | (optional) parameter expands output type to include more detail about trusted certificates. Note, bare RSA PSS public keys (if trusted) are not included in detail output. |

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| pem  | json | an array of x509 certs or public keys (if the 'verbose' query param is not supplied) that are trusted by the agent. A cert can be trusted using the PUT method in an HTTP request to the trust/ path). |

**Examples:**
```
curl -s http://localhost/trust | jq  '.'
{
  "pem": [
    "Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem",
    "LULZ-1e0572c9f28c5e9a0dafa14741665c3cfd80b580-public.pem"
  ]
}
```

Verbose output:
```
curl -s 'http://localhost/trust?verbose=true' | jq  '.'
{
  "pem": [
    {
      "type": "KeyPairSimple",
      "serial_number": "1e:05:72:c9:f2:8c:5e:9a:0d:af:a1:47:41:66:5c:3c:fd:80:b5:80",
      "subject_names": {
        "commonName (CN)": "*",
        "organizationName (O)": "LULZ"
      },
      "have_private_key": false,
      "not_valid_before": "2018-01-15T13:09:00Z",
      "not_valid_after": "2022-01-16T01:08:58Z",
      "public_key": "-----BEGIN PUBLIC KEY-----\nMIICIjANBgkqhkiG9w0BAQ...",
      ...
    },
    ...
  ]
}
```

Retrieve RSA PSS public key from a particular enclosing x509 certificate suitable for shell redirection to a .pem file:
```
curl -s 'http://localhost/trust?verbose=true' | jq -r '.pem[] | select(.serial_number == "1e:05:72:c9:f2:8c:5e:9a:0d:af:a1:47:41:66:5c:3c:fd:80:b5:80")'
-----BEGIN CERTIFICATE-----
MIIE5DCCAsygAwIBAgIUHgVyyfKMXpoNr6FHQWZcPP2AtYAwDQYJKoZIhvcNAQEL
BQAwGzENMAsGA1UEChMETFVMWjEKMAgGA1UEAxMBKjAeFw0xODAxMTUxMzA5MDBa
Fw0yMjAxMTYwMTA4NThaMBsxDTALBgNVBAoTBExVTFoxCjAIBgNVBAMTASowggIi
MA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCe8yFhGf699+iDwoO+LRPSH4Ce
DICrG4SgFZ7/2/dGPduVmYUWGnu1Sb3lk1xEO+Wd4SJ3VvabzaAexgXUgup8bM2K
FbHi5BMbf3G4on+gdxdRwTriwX3UTXKj3+D47sGZS3j8tJrqTx58hhuR9qTaBzAh
rwhRtD1k7/NLIOCZ2xS5Tfb/mKgG378+xM5IQQ69P86fQN6V28qIYUIUIURRreE4
HjJAFXpkpdDYqPclt/91S+hOnc1fWJah5jXuxdrrZ2jNl/b5IDA4F2/dKbHJNBEL
3/9lg63USywzD6uP9OzB1Gkhyp6TrUHzQyBZU2kVH8Qx9Zz2eUeI5u7UmXMxEsXu
...
swRnEsXZy1Ssvmi9myJrKWc8GrkhotweVkAyAJFHJjusIR8Wm8lusZB2IFxwiuFI
NzGgG2JMPK4=
-----END CERTIFICATE-----
```

#### **API:** GET  /trust/{filename}
---

Get the content of a trusted x509 cert from a file that has been previously stored by the agent.

**Parameters:**

| name | type | description |
| -----| ---- | ---------------- |
| filename | string | the name of the x509 cert file to retrieve. |

**Response:**

code:
* 200 -- success

body:

The contents of the requested file.

**Example:**
```
curl -s http://localhost/trust/Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem > Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem
```

#### **API:** PUT  /trust/{filename}
---

Trust an x509 cert; used in service container image verification.

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| filename | string | the name of the x509 cert file to upload. |

**Response:**

code:
* 200 -- success

body:

none

**Example:**
```
curl -T ~/.rsapsstool/keypairs/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem http://localhost/trust/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem

```

#### **API:** DELETE  /trust/{filename}
---

Delete an x509 cert from the agent; this is a revocation of trust for a particular certificate and enclosing RSA PSS key.

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| filename | string | the name of the x509 cert file to remove. |

**Response:**

code:
* 204 -- success

body:

none

**Example:**
```
curl -s -X DELETE http://localhost/trust/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem

```

### 6. Event Log
#### **API:** GET  /eventlog
---

Get event logs for the Horizon agent for the current registration. It supports selection strings. The selections can be made against the attributes.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| record_id   | string | the record id in the data base. |
| timestamp | uint64 | the time when the event log was saved. |
| severity | string | the severity for this event. It can be 'info', 'warning' or 'error'. |
| message | string | a human consumable the event message.  |
| event_code | string| an event code that can be used by programs.|
| source_type | string | the source for the event. It can be 'agreement', 'service', 'exchange', 'node' etc. |
| event_source | json | a structure that holds the event source object. |

**Example:**

```
http://localhost/eventlog
[
  {
    "record_id": "270",
    "timestamp": 1536861596,
    "severity": "info",
    "message": "Start node configuration/registration for node mynode1.",
    "event_code": "start_node_configuration_registration",
    "source_type": "node",
    "event_source": {
      "node_id": "5330a6eb4b177e9203a14d6780589f539f8ec809",
      "node_org": "mycomp",
      "pattern": "comp/netspeed",
      "config_state": ""
    }
  },
  {
    "record_id": "271",
    "timestamp": 1536861598,
    "severity": "info",
    "message": "Complete node configuration/registration for node mynode1.",
    "event_code": "node_configuration_registration_complete",
    "source_type": "node",
    "event_source": {
      "node_id": "mynode1",
      "node_org": "mycomp",
      "pattern": "netspeed",
      "config_state": "configured"
    }
  },
  {
    "record_id": "272",
    "timestamp": 1536861598,
    "severity": "info",
    "message": "Start policy advertising with the exchange for service https://bluehorizon.network/services/gps.",
    "event_code": "start_policy_advertising",
    "source_type": "agreement",
    "event_source": {
       "workload_to_run": {
        "url": "",
        "org": "",
        "version": "",
        "arch": ""
      },
      "service_url": [
        "https://bluehorizon.network/services/gps"
      ],
      "agreement_id": "https://bluehorizon.network/services/gps",
      "consumer_id": "",
      "agreement_protocol": ""
    }
  }
  ....
]

```


```
http://localhost/eventlog?source_type=node&message=~Complete
[
  {
    "record_id": "271",
    "timestamp": 1536861598,
    "severity": "info",
    "message": "Complete node configuration/registration for node mynode1.",
    "event_code": "node_configuration_registration_complete",
    "source_type": "node",
    "event_source": {
      "node_id": "mynode1",
      "node_org": "mycomp",
      "pattern": "netspeed",
      "config_state": "configured"
    }
  }
]

```

#### **API:** GET  /eventlog/all
---

Get all the event logs including the previous regstrations for the Horizon agent. It supports selection strings. It supports selection strings. The selections can be made against the attributes.

**Parameters:**

none

**Response:**

code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| record_id   | string | the record id in the data base. |
| timestamp | uint64 | the time when the event log was saved. |
| severity | string | the severity for this event. It can be 'info', 'warning' or 'error'. |
| message | string | a human consumable the event message.  |
| event_code | string| an event code that can be used by programs.|
| source_type | string | the source for the event. It can be 'agreement', 'service', 'exchange', 'node' etc. |
| event_source | json | a structure that holds the event source object. |

**Example:**

```
http://localhost/eventlog/all
[
  {
    "record_id": "1",
    "timestamp": 1336861590,
    "severity": "info",
    "message": "Start node configuration/registration for node mynode1.",
    "event_code": "start_node_configuration_registration",
    "source_type": "node",
    "event_source": {
      "node_id": "mynode1",
      "node_org": "mycomp",
      "pattern": "mycomp/netspeed",
      "config_state": ""
    }
  },
  {
    "record_id": "2",
    "timestamp": 1336861600,
    "severity": "info",
    "message": "Complete node configuration/registration for node mynode1.",
    "event_code": "node_configuration_registration_complete",
    "source_type": "node",
    "event_source": {
      "node_id": "mynode1",
      "node_org": "mycomp",
      "pattern": "netspeed",
      "config_state": "configured"
    }
  },

  ....

```

