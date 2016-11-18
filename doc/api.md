
## Horizon APIs

### *N.B.* This is not applicable for v2. Updates coming soon.

This document contains the Horizon JSON APIs for the horizon system running on an IOT device. The output of the APIs is in JSON compact format. To get a better view, you can use JSONView extension in your web browser or use `jq` command from the command line interface. For example:

```
curl -s http://<ip>/info | jq -r '.'
```

### 1. Information about the horizon system

**API:**
GET  /info

**Parameters:**
none

**Response:**

| name | type | description |
| ---- | ---- | ---------------- |
| geth   | json | the information about the ethereum client. |
| geth.net_peer_count | int64 |  the number of peers that the ethereum client is contacting with |
| geth.eth_syncing | boolean |  whether the ethereum client is syncing with the blockchain or not. A false value meanse syncing.  |
| geth.eth_block_number | int64 | the latest block number that the client has imported. |
| geth.eth_accounts | array | an array of ethereum account numbers for this device. |
| firmware | json | the information about the horizon version on the device.  |
| connectivity | json | device has connections with some sites or not.  |


**Example:**
```
curl -s http://localhost/info |jq -r '.'
{
  "geth": {
    "net_peer_count": 5,
    "eth_syncing": false,
    "eth_block_number": 1217669,
    "eth_accounts": [
      "0x1359ba9833cac6cbf01f6b28ffc7dcb0eae37801"
    ]
  },
  "firmware": {
    "definition": "/root/.colonus/firmware/colonus-1468536653",
    "flash_version": "1474556166"
  },
  "colonus": {},
  "connectivity": {
    "firmware.bluehorizon.network": true,
    "images.bluehorizon.network": true
  }
}
```

:zap: The name attribute name `firmware` needs to be changed. The false `syncing` value means syncing is misleading. `colonus` needs to be changed to horizon and the information about the snap needs to get there.


### 2. Information about the contracts

**API:**
GET  /contract

**Parameters:**
none

**Response:**

| name | type | description |
| ---- | ---- | ---------------- |
| contracts   | array | an array of contract information. The attributes of each contract are defined in the following rows. |
| contract_address | string |  the contract address on the blockchain. |
| name | string |  the name of the contract.  |
| archived | boolean | not used.   |
| current_agreement_id | string | the id of agreement between the device and the data consumer that is currently in effect. |
| previous_agreements | array | an array of the previous agreement ids that are canceled.  |
| configure_nonce | string | a unique identifier that is used for contract negotiation. It is set when the negotiation starts and unset when the device starts downloading the workloads. |
| agreement_creation_time | int64 | the time when the agreement proposal is received from the data consumer and the negotiation process starts. |
| agreement_execution_start_time | int64 | the time when the device starts the workloads. |
| agreement_accepted_time | int64 | the time when the agreement is finalized and the device will start receiving payment tokens. |
| private_environment_additions | json | the private environment variables that will be passed into the workload containers. |
| environment_additions | json | the public environment variables that will be passed into the workload containers. They will be written into the contract agreement. |
| current_deployment | json | contains the information of the workloads. The key is the name of the workload and the value is the result of [/containers/<id> docker remote API call](https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/inspect-a-container) for the  workload container. Please refer to the link for details. |

**Example:**
```
curl -s http://localhost/contract |jq -r '.'
{
  "contracts": [
    {
      "contract_address": "0xdc8a53fcb5c598878d01a5520310d3c9597b8de6",
      "name": "Netspeed Contract",
      "archived": false,
      "current_agreement_id": "oKyubbPHZLKb1W5FcO9klNO3GnHP8caC",
      "previous_agreements": [
        "7rDdpKAKisEHDhVngMRQgV7JXgwPNrKP"
      ],
      "configure_nonce": "",
      "agreement_accepted_time": 1475782514,
      "private_environment_additions": {
        "lat": "41.5861",
        "lon": "-73.883"
      },
      "environment_additions": {
        "MTN_AGREEMENTID": "oKyubbPHZLKb1W5FcO9klNO3GnHP8caC",
        "MTN_ARCH": "arm",
        "MTN_CONFIGURE_NONCE": "FES_guwFaCw8-e_WMWB3eUGqjY0-DZ7k9W6BJhoxl6JYSK7xMEFLW7rcEG8S2VxbxpOtGgqjWMi2A_7HVwbHiA==",
        "MTN_CONTRACT": "0xdc8a53fcb5c598878d01a5520310d3c9597b8de6",
        "MTN_CPUS": "4",
        "MTN_HOURLY_COST_BACON": "180",
        "MTN_IS_BANDWIDTH_TEST_ENABLED": "true",
        "MTN_IS_LOC_ENABLED": "true",
        "MTN_LAT": "41.5861",
        "MTN_LON": "-73.883",
        "MTN_NAME": "Netspeed Contract",
        "MTN_RAM": "128",
        "MTN_TARGET_SERVER": "closest"
      },
      "agreement_creation_time": 1475782392,
      "agreement_execution_start_time": 1475782487,
      "current_deployment": {
        "netspeed5": {
          "config": {
            "Env": [
              "MTN_LON=-73.883",
              "MTN_NAME=Netspeed Contract",
              "MTN_ARCH=arm",
              "MTN_CPUS=4",
              "MTN_HOURLY_COST_BACON=180",
              "MTN_IS_BANDWIDTH_TEST_ENABLED=true",
              "MTN_IS_LOC_ENABLED=true",
              "MTN_LAT=41.5861",
              "MTN_AGREEMENTID=oKyubbPHZLKb1W5FcO9klNO3GnHP8caC",
              "MTN_CONFIGURE_NONCE=FES_guwFaCw8-e_WMWB3eUGqjY0-DZ7k9W6BJhoxl6JYSK7xMEFLW7rcEG8S2VxbxpOtGgqjWMi2A_7HVwbHiA==",
              "MTN_CONTRACT=0xdc8a53fcb5c598878d01a5520310d3c9597b8de6",
              "MTN_RAM=128",
              "MTN_TARGET_SERVER=closest",
              "DEPL_ENV=prod"
            ],
            "Cmd": null,
            "Image": "summit.hovitos.engineering/armhf/netspeed5:v1.8",
            "Volumes": {
              "/var/snap/bluehorizon/common/workload_ro/oKyubbPHZLKb1W5FcO9klNO3GnHP8caC": {}
            },
            "Entrypoint": null,
            "Labels": {
              "network.bluehorizon.colonus.agreement_id": "oKyubbPHZLKb1W5FcO9klNO3GnHP8caC",
              "network.bluehorizon.colonus.deployment_description_hash": "voofD9Cqn1WpmmbtCTgm6ExUf6A=",
              "network.bluehorizon.colonus.service_name": "netspeed5",
              "network.bluehorizon.colonus.variation": ""
            }
          },
          "host_config": {
            "Binds": [
              "/var/snap/bluehorizon/common/workload_ro/oKyubbPHZLKb1W5FcO9klNO3GnHP8caC:/workload_config:ro"
            ],
            "RestartPolicy": {
              "Name": "always"
            },
            "LogConfig": {
              "Type": "syslog",
              "Config": {
                "tag": "workload-okyubbphzlkb1w5fco9klno3gnhp8cac_netspeed5"
              }
            },
            "Memory": 134217728
          }
        },
        "pvp": {
          "config": {
            "Env": [
              "MTN_AGREEMENTID=oKyubbPHZLKb1W5FcO9klNO3GnHP8caC",
              "MTN_CONFIGURE_NONCE=FES_guwFaCw8-e_WMWB3eUGqjY0-DZ7k9W6BJhoxl6JYSK7xMEFLW7rcEG8S2VxbxpOtGgqjWMi2A_7HVwbHiA==",
              "MTN_CONTRACT=0xdc8a53fcb5c598878d01a5520310d3c9597b8de6",
              "MTN_RAM=128",
              "MTN_TARGET_SERVER=closest",
              "MTN_NAME=Netspeed Contract",
              "MTN_ARCH=arm",
              "MTN_CPUS=4",
              "MTN_HOURLY_COST_BACON=180",
              "MTN_IS_BANDWIDTH_TEST_ENABLED=true",
              "MTN_IS_LOC_ENABLED=true",
              "MTN_LAT=41.5861",
              "MTN_LON=-73.883",
              "DEPL_ENV=prod"
            ],
            "Cmd": null,
            "Image": "summit.hovitos.engineering/armhf/pvp:v1.0",
            "Volumes": {
              "/var/snap/bluehorizon/common/workload_ro/oKyubbPHZLKb1W5FcO9klNO3GnHP8caC": {}
            },
            "Entrypoint": null,
            "Labels": {
              "network.bluehorizon.colonus.agreement_id": "oKyubbPHZLKb1W5FcO9klNO3GnHP8caC",
              "network.bluehorizon.colonus.deployment_description_hash": "Gb1iMbfBno3_N2gYtmgpZ1IefFI=",
              "network.bluehorizon.colonus.service_name": "pvp",
              "network.bluehorizon.colonus.variation": ""
            }
          },
          "host_config": {
            "Binds": [
              "/var/snap/bluehorizon/common/workload_ro/oKyubbPHZLKb1W5FcO9klNO3GnHP8caC:/workload_config:ro"
            ],
            "RestartPolicy": {
              "Name": "always"
            },
            "LogConfig": {
              "Type": "syslog",
              "Config": {
                "tag": "workload-okyubbphzlkb1w5fco9klno3gnhp8cac_pvp"
              }
            },
            "Memory": 134217728
          }
        }
      }
    }
  ]
}

```

### 3. Information about the workload

**API:**
GET  /workload

**Parameters:**
none

**Response:**

| name | type | description |
| ---- | ---- | ---------------- |
| workloads   | array | an array of workload information. Each workload information is the result of /containers docker remote API call. Please refer to [/containers docker API] (https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/list-containers) for details. |

**Example:**
```
curl -s http://localhost/workload |jq -r '.'
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

### 4. Information about micropayments for contracts

**API:**
GET  /micropayment

**Parameters:**
none

**Response:**

| name | type | description |
| ---- | ---- | ---------------- |
| micropayments | array | an array of micropayment information. Each array element is the micropayment information for a contract. The attributes of the micropayment information are defined in the following rows. |
| payer_address | string | the payer's account number. |
| contract_address | string | the contract address on the blockchain. |
| agreement_id | string | the id of the contract agreement. |
| payments | json | the history of the micropayments. The key is the time when tokens were paid and the value is the total number of tokens paid so far. |

**Example:**
```
curl -s http://localhost/micropayment |jq -r '.'
{
  "micropayments": [
    {
      "payer_address": "0x1635440027b0bf13b389669f39c533a31624d336",
      "contract_address": "0x3c69e74ec297b0a38772060934d550781724124e",
      "agreement_id": "3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu",
      "payments": {
        "1475260291": 11,
        "1475260624": 22,
        "1475260980": 34,
        "1475261300": 44,
        "1475261616": 55,
        "1475261930": 65,
        "1475262252": 76,
      }
    },
    {
      "payer_address": "0x1635440027b0bf13b389669f39c533a31624d336",
      "contract_address": "0x94734fb6bcd14214ba79650c5c6227caaafcf17f",
      "agreement_id": "ZVMlQ6RvjSfAtWuHmbpEOYRCNXtPHzGN",
      "payments": {
        "1475260196": 15,
        "1475260532": 32,
        "1475260984": 55,
        "1475261304": 71,
        "1475261619": 86,
        "1475261933": 102,
        "1475262254": 118,
      }
    }
  ]
}
```

### 5. Latest micropayment information for a contract
**API:**
GET  /agreement/<agreentment_id>/latestmicropayment

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| agreentment_id | string | the id of a contract agreement. |


**Response:**

| name | type | description |
| ---- | ---- | ---------------- |
| agreement_id | string | the id of the contract agreement that the micropayment is for. |
| payment_time | int64 | the time when the last micropayment was made. |
| payment_value | uint64 | total number of tokens paid so far. |


**Example:**
```
curl -s http://localhost/agreement/3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu/latestmicropayment |jq -r '.'
{
  "agreement_id": "3AoI5I7K8mGtrR2DdTTQqhfXHnryebqu",
  "payment_time": 1475268257,
  "payment_value": 276
}
```

