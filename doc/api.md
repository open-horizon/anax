## Horizon APIs

This document contains the Horizon JSON APIs for the horizon system running on an IOT device. The output of the APIs is in JSON compact format. To get a better view, you can use JSONView extension in your web browser or use `jq` command from the command line interface. For example:

```
curl -s http://<ip>/status | jq '.'
```

### 1. Horizon system

#### **API:** GET  /status
---

**Parameters:**
none

**Response:**

code: 
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| geth   | json | the information about the ethereum client. |
| geth.net_peer_count | int64 |  the number of peers that the ethereum client is contacting with |
| geth.eth_syncing | boolean |  whether the ethereum client is syncing with the blockchain or not.   |
| geth.eth_block_number | int64 | the latest block number that the client has imported. |
| geth.eth_balance| string | the current balance of the ethereum account. |
| geth.eth_accounts | array | an array of ethereum account numbers for this device. |
| configuration| json| the configuration data.  |
| configuration.exchange_api | string | the url for the exchange api.  |
| connectivity | json | device has connections with some sites or not.  |


**Example:**
```
curl -s http://localhost/status |jq '.'
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
    "exchange_api": "https://exchange.staging.bluehorizon.network/api/v1/"
  },
  "connectivity": {
    "firmware.bluehorizon.network": true,
    "images.bluehorizon.network": true
  }
}


```

### 2. Device
#### **API:** GET  /horizondevice
---

**Parameters:**
none

**Response:**

code: 
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the unique id of the device. |
| name | string | the user readable name for the device  |
| token_valid | bool| whether the device token is valid or not |
| token_last_valid_time | uint64 | the time stamp when the device token was last valid. |
| ha_device | bool | whether the device is part of an HA group or not |
| account | json | the account information for the user who owns this device. |
| account.id | string |  the user id on the account.   |
| account.email | string | the user email on the account. |

**Example:**
```
curl -s http://localhost/horizondevice |jq '.'
{
  "id": "000000002175f7a9",
  "account": {
    "id": "myname",
    "email": "myname@mycomany.com"
  },
  "name": "mydevice1",
  "token_last_valid_time": 1481310188,
  "token_valid": true
}

```


#### **API:** POST  /horizondevice
---

**Parameters:**

Please refer to the response body of GET /horizondevice api.

**Response:**

code: 

* 201 -- success

body: 

none

**Example:**
```
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '{
      "account": {
        "id": "user1",
        "email": "'$EMAIL'"
      },
      "id": "'$DEVICE_ID'",
      "name": "'$DEVICE_NAME'",
      "token": "'$DEVICE_TOKEN'"
    }'  http://localhost/horizondevice

```

#### **API:** PATCH  /horizondevice
---

**Parameters:**

Please refer to the response body of GET /horizondevice api.

**Response:**

code: 

* 200 -- success

body: 

none

**Example:**
```
curl -s -w "%{http_code}" -X PATCH-H 'Content-Type: application/json'  -d '{
      "account": {
      "id": "user1"
      },
      "name": "'$DEVICE_NAME'",
      "token": "'$DEVICE_TOKEN'"
    }'  http://localhost/horizondevice

```

### 3. Service

#### **API:** GET  /service
---

**Parameters:**
none

**Response:**

code: 
* 200 -- success

body:


| name | type | description |
| ---- | ---- | ---------------- |
| services   | json | a list of services with each service containing policy and attributes. |
| services.{name}.policy | json | the policy for the service. This policy will be used to negotiate an agreement with the agbot. |
| services.{name}.attributes | array | an array of the user defined service attributes for the service. The attributes will be passed into the workloads environmental variables after the agreement is reached. |

policy

| name | type | description |
| ---- | ---- | ---------------- |
| header | json|  the header of the policy. It includes the name and the version of the policy. |
| apiSpec | array | an array of api specifications. Each one includes a URL pointing to the definition of the API spec, the version of the API spec in OSGI version format, whether or not exclusive access to this API spec is required and the hardware architecture of the API spec implementation. | 
| agreementProtocols | array | an array of agreement protocols. Each one includes the name of the agreement protocol.|
| maxAgreements| int | the maximum number of agreements allowed to make. |
| properties | array | an array of name value pairs that will be matched with the counter party during agreement negotiation process. |
| blockchains| array | an array of blockchain specifications including bockchain type, boot nodes, network ids etc. |
Note: The policy also contains other fields that are unused and therefore not documented.

service attribute

| name | type | description |
| ---- | ---- | ---------------- |
| meta | json |  the metadata that describes the attribute. It includes the id, the sensor_url, the label, the data type and weather or not the attribute is publishable or not. If the sensor_url is empty, the attribute is applied to all services. |
| {name, value} |  | The names and values. Each type has a different set of name and value pairs.     Currently there are 4 supported types: ArchitectureAttributes (architecture), ComputeAttributes (cpu, ram), LocationAttributes (lat, lon, user_provided_coords, use_gps) and MappedAttributes (mappings) |


**Example:**
```
curl -s -w %{http_code}  http://localhost/service | jq  '.'
{
  "services": {
    "Policy for netspeed": {
      "policy": {
        "header": {
          "name": "Policy for netspeed",
          "version": "2.0"
        },
        "apiSpec": [
          {
            "specRef": "https://bluehorizon.network/documentation/netspeed-device-api",
            "version": "1.0.0",
            "exclusiveAccess": true,
            "arch": "arm"
          }
        ],
        "agreementProtocols": [
          {
            "name": "Citizen Scientist"
          }
        ],
        "valueExchange": {
          "type": "",
          "value": "",
          "paymentRate": 0,
          "token": ""
        },
        "resourceLimits": {
          "networkUpload": 0,
          "networkDownload": 0,
          "memory": 0,
          "cpus": 0
        },
        "dataVerification": {
          "enabled": false,
          "URL": "",
          "URLUser": "",
          "URLPassword": "",
          "interval": 0
        },
        "proposalRejection": {
          "number": 0,
          "duration": 0
        },
        "maxAgreements": 1,
        "properties": [
          {
            "name": "cpus",
            "value": "1"
          },
          {
            "name": "ram",
            "value": "0"
          }
        ],
        "blockchains": [
          {
            "type": "ethereum",
            "details": {
              "bootnodes": [
                "https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/peers",
                "https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/peers",
                "https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/peers"
              ],
              "directory": [
                "https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/directory.address",
                "https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/directory.address",
                "https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/directory.address"
              ],
              "genesis": [
                "https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json",
                "https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json",
                "https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/genesis.json"
              ],
              "networkid": [
                "https://dal05.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/networkid",
                "https://tok02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/networkid",
                "https://lon02.objectstorage.softlayer.net/v1/AUTH_773b8ed6-b3c8-4683-9d7a-dbe2ee11095e/volcano/networkid"
              ]
            }
          }
        ]
      },
      "attributes": [
        {
          "meta": {
            "id": "app",
            "sensor_urls": [
              "https://bluehorizon.network/documentation/netspeed-device-api"
            ],
            "label": "app",
            "publishable": true,
            "type": "persistence.MappedAttributes"
          },
          "mappings": {
            "MTN_IS_BANDWIDTH_TEST_ENABLED": "true",
            "MTN_TARGET_SERVER": "closest"
          }
        },
        {
          "meta": {
            "id": "architecture",
            "sensor_urls": [
              "https://bluehorizon.network/documentation/netspeed-device-api"
            ],
            "label": "Architecture",
            "publishable": true,
            "type": "persistence.ArchitectureAttributes"
          },
          "architecture": "arm"
        },
        {
          "meta": {
            "id": "compute",
            "sensor_urls": [
              "https://bluehorizon.network/documentation/netspeed-device-api"
            ],
            "label": "Compute Resources",
            "publishable": true,
            "type": "persistence.ComputeAttributes"
          },
          "cpus": 1,
          "ram": 0
        },
        {
          "meta": {
            "id": "location",
            "sensor_urls": [],
            "label": "Registered Location Facts",
            "publishable": false,
            "type": "persistence.LocationAttributes"
          },
          "lat": "41.5861",
          "lon": "-73.883",
          "user_provided_coords": true,
          "use_gps": false
        }
      ]
    }
  }
}
```


#### **API:** POST  /service
---

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| sensor_url  | string | the url for the service. |
| sensor_name  | string | the name of the service. |
| attributes | array | an array of attributes.  Please refer to the response body for the GET /service/attribute api for the fields of an attribute.  |

**Response:**

code: 

* 201 -- success

body: 

none

**Example:**
```
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '{
  "sensor_url": "https://bluehorizon.network/documentation/netspeed-device-api",
  "sensor_name": "netspeed",
  "attributes": [
    {
      "id": "app",
      "short_type": "mapped",
      "label": "app",
      "publishable": true,
      "mappings": {
        "MTN_IS_BANDWIDTH_TEST_ENABLED": "true",
        "MTN_TARGET_SERVER": "closest"
      }
    }
  ]
}'  http://localhost/service

```

#### **API:** GET  /service/attribute
---

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

| id | string| the id of the attribute. |
| ---- | ---- | ---------------- |
| label | string | the user readable name of the attribute |
| short_type| string | the short type name of the service. This filed is omitted if it is empty. Supported types are: compute, location, architecture, ha, and mapped. |
| sensor_urls | array | an array of sensor url. It applies to all services if it is empty. |
| publishable| bool | whether the attribute can be made public or not. |
| mappings | map | a list of key value pairs. |


**Example:**
```
 curl -s -w %{http_code}  http://localhost/service/attribute | jq  '.'
{
  "attributes": [
    {
      "id": "app",
      "sensor_urls": [
        "https://bluehorizon.network/documentation/netspeed-device-api"
      ],
      "label": "app",
      "publishable": true,
      "mappings": {
        "MTN_IS_BANDWIDTH_TEST_ENABLED": "true",
        "MTN_TARGET_SERVER": "closest"
      }
    },
    {
      "id": "architecture",
      "sensor_urls": [
        "https://bluehorizon.network/documentation/netspeed-device-api"
      ],
      "label": "Architecture",
      "publishable": true,
      "mappings": {
        "architecture": "arm"
      }
    },
    {
      "id": "compute",
      "sensor_urls": [
        "https://bluehorizon.network/documentation/netspeed-device-api"
      ],
      "label": "Compute Resources",
      "publishable": true,
      "mappings": {
        "cpus": 1,
        "ram": 0
      }
    },
    {
      "id": "location",
      "sensor_urls": [],
      "label": "Registered Location Facts",
      "publishable": false,
      "mappings": {
        "lat": "41.5861",
        "lon": "-73.883",
        "use_gps": false,
        "user_provided_coords": true
      }
    }
  ]
}

```

#### **API:** POST  /service/attribute
---

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /service/attribute api for the fields of an attribute.  |

**Response:**

code: 

* 201 -- success

body: 

none

**Example:**
```
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json' -d '{
    "id": "compute",
    "short_type": "compute",
    "label": "Compute Resources",
    "publishable": true,
    "mappings": {
      "ram": 256,
      "cpus": 1
    }
  }'  http://localhost/service/attribute

```

### 4. Agreement

#### **API:** GET  /agreement
---

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
| name | string |  the name of the agreement.  |
| current_agreement_id | string | the id of the agreement. |
| counterparty_address | string |  the ethereum account of the agbot. |
| consumer_id| string |  the id of the agbot that device is forming the agreement with. |
| agreement_creation_time | uint64 | the time when the devices receives the agreement proposal from the agbot. the negotiation process starts. |
| agreement_accepted_time | uint64 | the time when the agbot and the device have come to agreement on the terms. workload downloading starts. |
| agreement_execution_start_time | uint64 | the time when the device starts running the workloads. |
| agreement_finalized_time | uint64 | the time when the agbot and the device have finalized the agreements. workloads are running and data is verified by the agbot.  |
| agreement_data_received_time | uint64 | the time when the agbot has verified that the data was received by the data consumer.  |
| agreement_terminated_time| uint64 | the time when the agreement is terminated. |
| terminated_reason| uint64 | the reason code for the agreement termination. |
| terminated_description | string | the description of the agreement termination. |
| proposal| string | the proposal currently in effect. |
| proposal_sig| string | the proposal signature. |
| agreement_protocol | string | the name of the agreement protocol being used. |
| protocol_version | int | the version of the agreement protocol being used. |
| current_deployment | json | contains the information of the workloads. The key is the name of the workload and the value is the result of [/containers/<id> docker remote API call](https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/inspect-a-container) for the  workload container. Please refer to the link for details. |
| archived | bool |  if the agreement is archived or not.  |
Note: The agreements that are being terminated but not yet archived are treated as archived in this api.


**Example:**
```
curl -s http://localhost/agreement |jq '.'
{
  "agreements": {
    "active": [
      {
        "name": "Policy for netspeed merged with netspeed arm",
        "sensor_url": "https://bluehorizon.network/documentation/netspeed-device-api",
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
          },
          "pvp": {
            "config": {
              "Cpuset": "1-3",
              "Env": [
                "HZN_ARCH=arm",
                "HZN_USE_GPS=false",
    ...
             ],
              "Cmd": null,
              "Image": "summit.hovitos.engineering/armhf/pvp:v1.0.2",
              "Volumes": {
                "/var/snap/bluehorizon/common/workload_ro/7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f": {}
              },
              "Entrypoint": null,
              "Labels": {
                "network.bluehorizon.colonus.agreement_id": "7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f",
                "network.bluehorizon.colonus.deployment_description_hash": "lstukhZqPgmMcGZNSa3OdY-VvoI=",
                "network.bluehorizon.colonus.service_name": "pvp",
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
                  "tag": "workload-7539aad7bf9269c97bf6285b173b50f016dc13dbe722a1e7cedcfec8f23c528f_pvp"
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
      }
    ],
    "archived": []
  }
}


```

#### **API:** DELETE  /agreement/{id}
---

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
### 5. Workload

#### **API:** GET  /workload
---

**Parameters:**
none

**Response:**

| name | type | description |
| ---- | ---- | ---------------- |
| workloads   | array | an array of workload information. Each workload information is the result of /containers docker remote API call. Please refer to [/containers docker API] (https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/list-containers) for details. |

**Example:**
```
curl -s http://localhost/workload |jq '.'
{
  "workloads": [
    {
      "Id": "36b0fdb6817f4e2b6e6e10e56b1bfb467444192c6e5ab34c50c8268cd0c0f172",
      "Image": "summit.hovitos.engineering/armhf/culex:volcano",
      "Command": "/bin/sh -c /start.sh",
      "Created": 1475259962,
      "Status": "Up 2 hours",
      "Ports": [
        {
          "PrivatePort": 1883,
          "Type": "tcp"
        },
        {
          "PrivatePort": 9001,
          "Type": "tcp"
        }
      ],
      "Names": [
        "/3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu-culex"
      ],
      "Labels": {
        "network.bluehorizon.colonus.agreement_id": "3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu",
        "network.bluehorizon.colonus.deployment_description_hash": "vpARQKldbtU8tJFKUPCdAZ8GY6M=",
        "network.bluehorizon.colonus.service_name": "culex",
        "network.bluehorizon.colonus.variation": ""
      },
      "NetworkSettings": {
        "Networks": {
          "3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu": {
            "MacAddress": "02:42:ac:14:00:04",
            "IPPrefixLen": 16,
            "IPAddress": "172.20.0.4",
            "Gateway": "172.20.0.1",
            "EndpointID": "bb96cf2ccb15322130611a797b646eda47cb536d2dd2c0e2384e190d40d10dcf"
          }
        }
      }
    },
    {
      "Id": "b06daaf1b1b8ad7f5eb3aa347ff77c532c66966322c62706bf545695f4230560",
      "Image": "summit.hovitos.engineering/armhf/location:v1.1",
      "Command": "/bin/sh -c /start.sh",
      "Created": 1475259958,
      "Status": "Up 2 hours",
      "Names": [
        "/3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu-location"
      ],
      "Labels": {
        "network.bluehorizon.colonus.agreement_id": "3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu",
        "network.bluehorizon.colonus.deployment_description_hash": "MT2zuzrwzWJEgi-2L0wNYCmuarA=",
        "network.bluehorizon.colonus.service_name": "location",
        "network.bluehorizon.colonus.variation": ""
      },
      "NetworkSettings": {
        "Networks": {
          "3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu": {
            "MacAddress": "02:42:ac:14:00:03",
            "IPPrefixLen": 16,
            "IPAddress": "172.20.0.3",
            "Gateway": "172.20.0.1",
            "EndpointID": "5b73f17448a9faef81c345453415e3bca34a3681a5b1eb68ba1d4d80d26876de"
          }
        }
      }
    }
  ]
}
```

### 6. Random Token

#### **API:** GET  /token/random
---

**Parameters:**
none

**Response:**

code: 
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| token   | string | a newly generated random token that you can use for device registration. |

**Example:**
```
curl -s http://localhost/token/random | jq  '.'
{
  "token": "MTKg9ZGceikVoCfAOOC6NpKbku1mH5NVd9f_3UxTSUoooPaIQzGQVLtEZUHsYuJy70RkidLAT-eXhcO-pdVeqA=="
}

```

