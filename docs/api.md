---
copyright: Contributors to the Open Horizon project
years: 2022 - 2025
title: Horizon APIs
description: Horizon APIs
lastupdated: 2025-05-03
nav_order: 5
parent: Agent (anax)
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# {{site.data.keyword.horizon}} APIs
{: #agent-apis}

This document contains the {{site.data.keyword.horizon}} REST APIs for the {{site.data.keyword.horizon}} agent running on an edge node. The output of the APIs is in JSON compact format. To get a better view, you can use the JSONView extension in your web browser or use the `jq` command from the command line interface. For example:

```bash
curl -s http://<ip>/status | jq '.'
```
{: codeblock}

## 1. {{site.data.keyword.horizon}} Agent

### **API:** GET /status

---

Get the connectivity, and configuration, status on the node. The output includes the status of the agent configuration and the node's connectivity.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| configuration||json| the configuration data. |
| |exchange_api| string | the url for the exchange being used by the {{site.data.keyword.horizon}} agent. |
| |exchange_version | string | the current version of the exchange being used. |
| |required_minimum_exchange_version | string | the required minimum version for the exchange. |
| |preferred_exchange_version | string | the preferred version for the exchange in order to use all the {{site.data.keyword.horizon}} functions. |
| |mms_api| string | the url for the model management system. |
| |architecture | string | the hardware architecture of the node as returned from the Go language API runtime.GOARCH. |
| |horizon_version | string | The current version of the horiozn running on this node. |
| connectivity || json | whether or not the node has network connectivity with some remote sites. |
{: caption="Table 1. GET /status JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/status | jq '.'
{
  "configuration": {
    "exchange_api": "http://exchange-api:8080/v1/",
    "exchange_version": "2.15.1",
    "required_minimum_exchange_version": "2.15.1",
    "preferred_exchange_version": "2.15.1",
    "mms_api": "https://css-api:9443",
    "architecture": "amd64",
    "horizon_version": "2.24.5"
  },
  "liveHealth": null
}
```
{: codeblock}

### **API:** GET /status/workers

---

Get the current {{site.data.keyword.horizon}} agent worker status and the status trasition logs.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| workers | | json | the current status of each worker and its subworkers. |
| | name | string | the name of the worker. |
| | status | string | the status of the worker. The valid values are: added, started, initialized, initialization failed, terminating, terminated. |
| | subworker_status | json | the name and the status of the subworkers that are created by this worker. |
| worker_status_log | | string array |  the history of the worker status changes. |
{: caption="Table 2. GET /status/workers JSON response fields" caption-side="top"}

#### Example

```bash
curl -s  http://localhost:8510/status/workers | jq
{
  "workers": {
    "AgBot": {
      "name": "AgBot",
      "status": "terminated",
      "subworker_status": {}
    },
    "Agreement": {
      "name": "Agreement",
      "status": "initialized",
      "subworker_status": {}
    },
    "Container": {
      "name": "Container",
      "status": "initialized",
      "subworker_status": {}
    },
    "ExchangeChanges": {
      "name": "ExchangeChanges",
      "status": "initialized",
      "subworker_status": {}
    },
    "ExchangeMessages": {
      "name": "ExchangeMessages",
      "status": "initialized",
      "subworker_status": {}
    },
    "Governance": {
      "name": "Governance",
      "status": "initialized",
      "subworker_status": {
        "ContainerGovernor": "started",
        "MicroserviceGovernor": "started",
        "SurfaceExchErrors": "started"
      }
    },
    "ImageFetch": {
      "name": "ImageFetch",
      "status": "initialized",
      "subworker_status": {}
    },
    "Kube": {
      "name": "Kube",
      "status": "initialized",
      "subworker_status": {}
    },
    "Resource": {
      "name": "Resource",
      "status": "initialized",
      "subworker_status": {}
    }
  },
  "worker_status_log": [
    "2020-03-27 19:06:07 Worker AgBot: started.",
    "2020-03-27 19:06:07 Worker AgBot: initialization failed.",
    "2020-03-27 19:06:07 Worker Agreement: started.",
    "2020-03-27 19:06:07 Worker Governance: started.",
    "2020-03-27 19:06:07 Worker ExchangeMessages: started.",
    "2020-03-27 19:06:07 Worker ExchangeMessages: initialized.",
    "2020-03-27 19:06:07 Worker Container: started.",
    "2020-03-27 19:06:07 Worker AgBot: terminated.",
    "2020-03-27 19:06:07 Worker ImageFetch: started.",
    "2020-03-27 19:06:07 Worker ImageFetch: initialized.",
    "2020-03-27 19:06:07 Worker Kube: started.",
    "2020-03-27 19:06:07 Worker Kube: initialized.",
    "2020-03-27 19:06:07 Worker Resource: started.",
    "2020-03-27 19:06:07 Worker Resource: initialized.",
    "2020-03-27 19:06:07 Worker ExchangeChanges: started.",
    "2020-03-27 19:06:07 Worker ExchangeChanges: initialized.",
    "2020-03-27 19:06:07 Worker Container: initialized.",
    "2020-03-27 19:06:12 Worker Agreement: initialized.",
    "2020-03-27 19:19:32 Worker Governance: subworker SurfaceExchErrors added.",
    "2020-03-27 19:19:32 Worker Governance: subworker ContainerGovernor added.",
    "2020-03-27 19:19:32 Worker Governance: subworker MicroserviceGovernor added.",
    "2020-03-27 19:19:32 Worker Governance: subworker SurfaceExchErrors started.",
    "2020-03-27 19:19:32 Worker Governance: subworker ContainerGovernor started.",
    "2020-03-27 19:19:32 Worker Governance: subworker MicroserviceGovernor started.",
    "2020-03-27 19:19:32 Worker Governance: initialized."
  ]
}
```
{: codeblock}

## 2. Node

### **API:** GET /node

---

Get the {{site.data.keyword.horizon}} platform configuration of the {{site.data.keyword.horizon}} agent. The configuration includes the agent's exchange id, organization, configuration state, and whether or not the agent is using a pattern configuration.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the agent's unique exchange id. |
| organization | string | the agent's organization. |
| pattern | string | the pattern that will be deployed on the node. |
| name | string | the user readable name for the agent. |
| nodeType | string | the node type. Valid values are 'device' and 'cluster'. |
| token_valid | bool| whether the agent's exchange token is valid or not. |
| token_last_valid_time | uint64 | the time stamp when the agent's token was last valid. |
| ha_group | string | the name of the HA group that node is in. |
| configstate | json | the current configuration state of the agent. It contains the state and the last_update_time. The valid values for the state are "configuring", "configured", "unconfiguring", and "unconfigured". |
{: caption="Table 3. GET /node JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/node | jq '.'
{
  "id": "myvs1",
  "organization": "mycompany",
  "pattern": "netspeed-amd64",
  "name": "mydevice",
  "nodeType": "device",
  "token_last_valid_time": 1508174346,
  "token_valid": true,
  "ha_group": "mygroup",
  "configstate": {
    "state": "configured",
    "last_update_time": 1508174348
  }
}
```
{: codeblock}

### **API:** POST /node

---

Configure the {{site.data.keyword.horizon}} agent. This API assumes that the agent's node has already been registered in the exchange. The configstate of the agent is changed to "configuring" after calling this API.

#### Parameters

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the agent's unique exchange id. |
| token | string | the agent's authentication token for the exchange. |
| organization | string | the agent's organization. |
| pattern | string | the pattern that will be deployed on the node. |
| name | string | the user readable name for the agent. |
{: caption="Table 4. POST /node JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '{
      "id": "mydevice",
      "organization": "mycompany",
      "pattern": "pat3",
      "name": "mydevice",
      "token": "dfjskjdsfkj"
    }'  http://localhost:8510/node
```
{: codeblock}

### **API:** PATCH /node

---

Update the agent's exchange token. This API can only be called when configstate is "configuring".

#### Parameters

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the agent's unique exchange id. |
| token | string | the agent's authentication token for the exchange. |
{: caption="Table 5. PATCH /node JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X PATCH -H 'Content-Type: application/json'  -d '{
      "id": "mydevice",
      "token": "kj123idifdfjsklj"
    }'  http://localhost:8510/node

```
{: codeblock}

### **API:** DELETE /node

---

Unconfigure the agent so that it can be re-configured. All agreements are cancelled, and workloads are stopped. The API could take minutes to send a response if invoked with block=true. This API can only be called when configstate is "configured" or "configuring". After calling this API, configstate will be changed to "unconfiguring" while the agent quiesces, and then it will become "unconfigured".

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| block | bool | If true (the default), the API blocks until the agent is quiesced. If false, the caller will get control back quickly while the quiesce happens in the background. While this is occurring, the caller should invoke GET /node until they receive an HTTP status 404. |
| removeNode | bool | If true, the node’s entry in the exchange is also deleted, instead of just being cleared. The default is false. |
| deepClean | bool | If true, all the history of the previous registration will be removed. The default is false. |
{: caption="Table 6. DELETE /node JSON parameter fields" caption-side="top"}

#### Response

code:

* 204 -- success

body:

none

#### Example

```bash
curl -s -w "%{http_code}" -X DELETE "http://localhost:8510/node?block=true&removeNode=false"
```
{: codeblock}

### **API:** GET /node/configstate

---

Get the current configuration state of the agent.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| state   | string | Current configuration state of the agent. Valid values are "configuring", "configured", "unconfiguring", and "unconfigured". |
| last_update_time | uint64 | timestamp when the state was last updated. |
{: caption="Table 7. GET /node/configstate JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/node/configstate | jq '.'
{
  "state": "configured",
  "last_update_time": 1510174292
}
```
{: codeblock}

### **API:** PUT /node/configstate

---

Change the configuration state of the agent. The valid values for the state are "configuring" and "configured". The "unconfigured" state is not settable through this API. The agent starts in the "configuring" state. You can change the state to "configured" after you have set the agent's pattern through the /node API, and have configured all the service user input variables through the /service/config API. The agent will advertise itself as available for services once it enters the "configured" state.

#### Parameters

body:

| name | type | description |
| ---- | ---- | ---------------- |
| state  | string | the agent configuration state. The valid values are "configuring" and "configured". |
{: caption="Table 8. PUT /node/configstate JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

body:

none

#### Example

```bash
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json'  -d '{
       "state": "configured"
    }'  http://localhost:8510/node/configstate
```
{: codeblock}

## 3. Attributes

### **API:** GET /attribute

---

Get all the attributes for the edge node and registered services.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attributes | array | an array of all the attributes for all the services. The fields of an attribute are defined in the following. |
{: caption="Table 9. GET /attribute JSON response fields" caption-side="top"}

attribute

| name | type| description |
| ---- | ---- | ---------------- |
| id | string| the id of the attribute. |
| label | string | the user readable name of the attribute |
| type| string | the attribute type. Supported attribute types are: MeteringAttributes, AgreementProtocolAttributes, UserInputAttributes, HTTPSBasicAuthAttributes, and DockerRegistryAuthAttributes. |
| publishable| bool | whether the attribute can be made public or not. |
| host_only | bool | whether or not the attribute will be passed to the service containers. |
| service_specs | array of json | an array of service organization and url. It applies to all services if it is empty. It is only required for the following attributes:  MeteringAttributes, AgreementProtocolAttributes, UserInputAttributes. |
| mappings | map | a list of key value pairs. |
{: caption="Table 10. GET /attribute JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/attribute | jq '.'
{
  "attributes": [
    {
      "id": "f85de917-ecb1-4d65-9310-66b0d9d2642f",
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "service_specs": [
        {
          "url": "https://bluehorizon.network/services/weather",
          "organization": "e2edev"
        }
      ],
      "mappings": {
        "HZN_PWS_MODEL": "LaCrosse WS2317",
        "HZN_PWS_ST_TYPE": "WS23xx",
        "HZN_WUGNAME": "e2edev mocked pws",
        "MTN_PWS_MODEL": "LaCrosse WS2317",
        "MTN_PWS_ST_TYPE": "WS23xx"
      }
    }
  ]
}
```
{: codeblock}

### **API:** POST /attribute

---

Register an attribute for a service. If the service_specs is omitted, the attribute applies to all the services.

#### Parameters

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to [Attribute Definitions](./attributes.md) for a description of all attributes. |
{: caption="Table 11. POST /attribute JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

body:

none

#### Example

```bash
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json' -d '{
  "type": "DockerRegistryAuthAttributes",
  "label": "Docker auth",
  "publishable": false,
  "host_only": true,
  "mappings": {
    "auths": [
      {
        "registry": "mydockerrepo", username": "user1", "token": "myDockerhubPassword"
      },
      {
        "registry": "registry.ng.bluemix.net", "username": "token", "token": "myDockerToken"
      }
    ]
  }
}'  http://localhost:8510/attribute
```
{: codeblock}

### **API:** GET /attribute/{id}

---

Get the attribute with the given id

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type| description |
| ---- | ---- | ---------------- |
| id | string| the id of the attribute. |
| label | string | the user readable name of the attribute |
| type| string | the attribute type. Supported attribute types are: MeteringAttributes, AgreementProtocolAttributes, UserInputAttributes, HTTPSBasicAuthAttributes, and DockerRegistryAuthAttributes. |
| publishable| bool | whether the attribute can be made public or not. |
| host_only | bool | whether or not the attribute will be passed to the service containers. |
| service_specs | array of json | an array of service organization and url. It applies to all services if it is empty. It is only required for the following attributes:  MeteringAttributes, AgreementProtocolAttributes, UserInputAttributes. |
| mappings | map | a list of key value pairs. |
{: caption="Table 12. GET /attribute/\{id\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s -w "%{http_code}" http://localhost:8510/attribute/0d5762bf-67a6-49ff-8ff9-c0fd32a8699f | jq '.'
{
  "attributes": [
    {
      "id": "a784b89c-e6f7-46dc-bf39-f2ac812a70b4",
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "service_specs": [
        {
          "url": "https://bluehorizon.network/services/netspeed",
          "organization": "myorg"
        }
      ],
      "mappings": {
        "var1": "aString",
        "var2": 5,
        "var3": 10.2,
        "var4": [
          "abc",
          "123"
        ],
        "var5": "override"
      }
    }
  ]
}
200
```
{: codeblock}

### **API:** PUT, PATCH /attribute/{id}

---

Modify an attribute for a service. If the service_specs is omitted, the attribute applies to all the services.

#### Parameters

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute/{id} api for the fields of an attribute. |
{: caption="Table 13. PUT /attribute/\{id\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute/{id} api for the fields of an attribute. |
{: caption="Table 14. POST /attribute/\{id\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s -w "%{http_code}" -X PUT -d '{
      "id": "0d5762bf-67a6-49ff-8ff9-c0fd32a8699f",
      "type": "UserInputAttributes",
      "service_specs": [
        {
          "url": "https://bluehorizon.network/services/netspeed",
          "organization": "IBM"
        }
      ],
      "label": "app",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "foo": "bar"
      }
    }' http://localhost:8510/attribute/0d5762bf-67a6-49ff-8ff9-c0fd32a8699f | jq '.'
```
{: codeblock}

### **API:** DELETE /attribute/{id}

---

Modify an attribute for a service. If the service_specs is omitted, the attribute applies to all the services.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| attribute | json | Please refer to the response body for the GET /attribute/{id} api for the fields of an attribute. |
{: caption="Table 15. DELETE /attribute/\{id\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s -w "%{http_code}" -X DELETE http://localhost:8510/attribute/0d5762bf-67a6-49ff-8ff9-c0fd32a8699f | jq '.'
```
{: codeblock}

## 4. Service

### **API:** GET  /service

---

Get the definition, the instance information and the configuration information for all the services.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| config | | array of json | all the services and the associated configuration attributes. For the pattern case, these are the top level services and their dependent services the pattern references to. For the non-pattern case, thses are the registered dependent services and any top level services that use has configured. |
| definitions | | json | the definition of all the related services. |
| | active | array of json | an array of service definitions that are actively in use. Please refer to the following table for the fields of a service definition object. |
| | archived | array of json | an array of service definitions that are archived. Please refer to the following table for the fields of a service definition object. |
| instances | | json | the instances of all the running services. It contains the information about the running service containers. |
| | active | array of json | an array of service instances that are active. Please refer to the following table for the fields of a service instance object. |
| | archived | array of json | an array of service instances that are archived. Please refer to the following table for the fields of a service instance object. |
{: caption="Table 16. GET /service JSON response fields" caption-side="top"}

service configuration:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| sensor_url | | string | the url of the service. |
| sensor_org | | string | the organization of the service. |
| sensor_version | | string | the version of the service. |
| auto_upgrade | | boolean | if the service should be automatically upgraded when a new version becomes available. |
| active_upgrade | | boolean | if the {{site.data.keyword.horizon}} agent should actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade. |
| attributes  | | array of json | an array of attributes that are associated with the service. |
| | meta | json | the meta data for an attribute. It includes id, type, lable etc. |
| | {key1} | string | key value pairs to be used to configure the service. |
| | {key2} | string | key value pairs to be used to configure the service. |
{: caption="Table 17. GET /service configuration JSON response fields" caption-side="top"}

service definition:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| record_id | | string | the record id in the db. |
| owner | | string | the owner of the service. |
| label | | string | the user readable name of the service. |
| description | | string | the discription of the service. |
| specRef | | string | the url of the service. |
| organization | | string | the organization of the service. |
| version | | string | the version of the service. |
| arch | | string | the architecture of the node the service can run on. |
| sharable | | string | how the service containers are shared if there are mutiple services reference this service. The valid values are: singleton, mutiple and exlusice. "singleton" means that all the services can share a single instance of this service. "mutiple" means that each service should have its own instance of this service. "exclusive" means that there can only be one service that references this service on at ay time. |
| userInput | | json | defines variables that can be configured by the user.. |
| | name | json | the name of the variable. |
| | label | json | the user readable name of the variable. |
| | type | json | the data type of the variable. |
| | defaultValue | json | the default value of the variable. If it is set, then the user does not have to configure this variable. |
| public | | boolean | whether the service can be refenced outside the organization. |
| requiredServices | | array of json | an array of services that this service depends on. |
| | url | string | the url of the dependent service. |
| | org | string | the organization of the dependent service.  |
| | version | string | the version of the dependent service. |
| | arch | string | of architecture of the dependent service. |
| deployment | | string | how the service is deployed. It defines the containers, images and configurations for this service. |
| deployment_signature | | string | the signature that can be used to verify the "deployment" string with a public key. |
| lastUpdated | | string | date where the service is last update on the exchange. |
| archived | | boolean | if the service definition is archived. |
| name | | string | the name of the service. |
| requested_arch | | string | the architecture from user input or from a service that refrences this service. It can be a synonym of the node architecture. |
| auto_upgrade | | boolean | if the service should be automatically upgraded when a new version becomes available. |
| active_upgrade | | boolean | if the {{site.data.keyword.horizon}} agent should actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade. |
| upgrade_start_time | | uint64 | the time when the service upgrade is started. |
| upgrade_ms_unregistered_time | | uint64 |  the time when the service is unregistered during the upgrade process. |
| upgrade_agreements_cleared_time | | uint64 | the time when all the associated agreements are cleared during the upgrade process. |
| upgrade_execution_start_time | | uint64 | the time when the new service is started running during the upgrade process. |
| upgrade_ms_reregistered_time | | uint64 | the time when the service is reregistered during the upgrade process. |
| upgrade_failed_time | | uint64 | the time when the service upgrade failed. |
| upgrade_failure_reason | | uint64 | the reason code for the service upgrade failure. |
| upgrade_failure_description | | sting | the description for the service upgrade failure. |
| upgrade_new_ms_id | | string | the record_id of the new service that this service is upgrading to. |
| metadata_hash | | string | the hash for the service defined in the exchange. |
{: caption="Table 18. GET /service definition JSON response fields" caption-side="top"}

service instance:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| ref_url | | string | the url of the service. |
| organization | | string | the organization of the service. |
| version | | string | the version of the service. |
| arch | | string | the architecture of the node the service can run on. |
| instance_id | | string | an unique id for this service instance. |
| archived | | boolean | if the service instance is archived. |
| instance_creation_time | | uint64 | the time when the service instance is created. |
| execution_start_time | | uint64 | the time when the service containers are started. |
| execution_failure_code | | uint64 | the reason code for the service instance failure. |
| execution_failure_desc | | sting | the description for the service instance failure. |
| cleanup_start_time | | uint64 | the time when the service instance is being cleaned. |
| associated_agreements | | array of string | agreements that use this service instance. |
| microservicedef_id | | string | record_id for the definiton of the service that this instance is for. |
| service_instance_path | | array of array | the parent path of how to get to this service instance. Since there may be multiple services that depend on this service, there may be multiple paths. |
| | url | string | the url of the parent or grandparent service. |
| | org | string | the organization of the parent or grandparent service.  |
| | version | string | the version of the parent or grandparent service. |
| agreement_less | | boolean | if this service is a agreement-less service (as defined in the pattern). |
| max_retries | | uint | maximum retries allowed. |
| max_retry_duration | | uint | the number of seconds in which the specified number of retries must occur in order for next retry cycle. |
| current_retry_count | | uint | the current retry count. |
| retry_start_time | | uint64 | the time when the service retry is started. |
| containers | | json | the info for the running docker containers for this service. |
{: caption="Table 19. GET /service instance JSON response fields" caption-side="top"}

#### Example

```bash
curl http://localhost:8510/service | jq '.config'
[
  {
    "sensor_url": "https://bluehorizon.network/services/netspeed",
    "sensor_org": "e2edev",
    "sensor_version": "2.3.0",
    "auto_upgrade": true,
    "active_upgrade": false,
    "attributes": [
      {
        "meta": {
          "id": "49cde8e2-a448-4f5e-98a1-c033deca5c53",
          "type": "UserInputAttributes",
          "label": "User input variables",
          "host_only": false,
          "publishable": false
        },
        "service_specs": [
          {
            "url": "https://bluehorizon.network/services/netspeed",
            "organization": "e2edev"
          }
        ],
        "mappings": {
          "var1": "aString",
          "var2": 5,
          "var3": 10.2,
          "var4": [
            "abc",
            "123"
          ],
          "var5": "override"
        }
      }
    ]
  },
  ...
]
```
{: codeblock}

```bash
curl http://localhost:8510/service | jq '.definitions.active'
[
  {
    "record_id": "1",
    "owner": "IBM/ibmadmin",
    "label": "Netspeed for x86_64",
    "description": "Netspeed service",
    "specRef": "https://bluehorizon.network/services/netspeed",
    "organization": "e2edev",
    "version": "2.3.0",
    "arch": "amd64",
    "sharable": "multiple",
    "downloadUrl": "",
    "matchHardware": {},
    "userInput": [
      {
        "name": "var1",
        "label": "",
        "type": "string",
        "defaultValue": ""
      },
      {
        "name": "var2",
        "label": "",
        "type": "int",
        "defaultValue": ""
      }
    ],
    "workloads": null,
    "public": true,
    "requiredServices": [
      {
        "url": "https://bluehorizon.network/services/network",
        "org": "myorg",
        "version": "1.0.0",
        "arch": "amd64"
      }
    ],
    "deployment": "{\"services\":{\"netspeed5\":{\"environment\":[\"MY_SETTINGS=0\"],\"image\":\"openhorizon/amd64_netspeed:2.5.0\"}}}",
    "deployment_signature": "vqwgYA/b",
    "lastUpdated": "2019-02-13T21:56:02.228Z[UTC]",
    "archived": false,
    "name": "netspeed",
    "requested_arch": "amd64",
    "upgrade_version_range": "[0.0.0,INFINITY)",
    "auto_upgrade": true,
    "active_upgrade": false,
    "upgrade_start_time": 0,
    "upgrade_ms_unregistered_time": 0,
    "upgrade_agreements_cleared_time": 0,
    "upgrade_execution_start_time": 0,
    "upgrade_ms_reregistered_time": 0,
    "upgrade_failed_time": 0,
    "upgrade_failure_reason": 0,
    "upgrade_failure_description": "",
    "upgrade_new_ms_id": "",
    "metadata_hash": "q8Lxbb/poHcq/+aDoFdAtF1PwCYXxfPWDjCEjm49Dc8="
  },
  ...
]
```
{: codeblock}

```bash
curl http://localhost:8510/service | jq '.instances.active'
[
  {
    "ref_url": "https://bluehorizon.network/services/location",
    "organization": "e2edev",
    "version": "2.0.6",
    "arch": "amd64",
    "instance_id": "535369111ae8d5d7c6dced904c0457f13c30b9ec6ed024fb53be649e4729c814",
    "archived": false,
    "instance_creation_time": 1550265488,
    "execution_start_time": 1550265501,
    "execution_failure_code": 0,
    "execution_failure_desc": "",
    "cleanup_start_time": 0,
    "associated_agreements": [
      "535369111ae8d5d7c6dced904c0457f13c30b9ec6ed024fb53be649e4729c814"
    ],
    "microservicedef_id": "2",
    "service_instance_path": [
      [
        {
          "url": "https://bluehorizon.network/services/location",
          "org": "e2edev",
          "version": "2.0.6"
        }
      ]
    ],
    "agreement_less": false,
    "max_retries": 0,
    "max_retry_duration": 0,
    "current_retry_count": 0,
    "retry_start_time": 0,
    "containers": [
      {
        "Id": "f9bca37e87e6128530902432b8cbb66dcd63e955b059e07ebc9b26f0266b9e63",
        "Image": "openhorizon/amd64_location:2.0.6",
        "Command": "/bin/sh -c /start.sh",
        "Created": 1550265500,
        "State": "running",
        "Status": "Up 2 days",
        "Names": [
          "/535369111ae8d5d7c6dced904c0457f13c30b9ec6ed024fb53be649e4729c814-location"
        ],
        "Labels": {
          "openhorizon.anax.agreement_id": "535369111ae8d5d7c6dced904c0457f13c30b9ec6ed024fb53be649e4729c814",
          "openhorizon.anax.deployment_description_hash": "tYE0SSJMmIjEH6wffMlFBP-qD_0=",
          "openhorizon.anax.service_name": "location",
          "openhorizon.anax.variation": ""
        },
        "NetworkSettings": {
          "Networks": {
            "535369111ae8d5d7c6dced904c0457f13c30b9ec6ed024fb53be649e4729c814": {
              "MacAddress": "02:42:c0:a8:70:02",
              "IPPrefixLen": 20,
              "IPAddress": "192.168.112.2",
              "Gateway": "192.168.112.1",
              "EndpointID": "c367b1de79cc87569c5b1b40c9c4a856f6f21dfea7e7b6766d58c08a829d6051",
              "NetworkID": "9a74f202ed8292a05ac06f7cc94186f69ab8455271a1dfc912c9bf8bb7859d59"
            },
             "e2edev_bluehorizon.network-services-locgps_2.0.4_22249248-9e92-470a-828d-da1875d5462c": {
              "MacAddress": "02:42:c0:a8:60:03",
              "IPPrefixLen": 20,
              "IPAddress": "192.168.96.3",
              "Gateway": "192.168.96.1",
              "EndpointID": "807817c5175a186f6e8de30ecd9291951b0538a19e389e37f9808d248f8b1a84",
              "NetworkID": "762d3afabd1cd20e25dc7bc15396ca07b8fd36dd6e2c25d10eed9e1a1ce9a8a9"
            }
          }
        },
        "Mounts": [
          {
            "Source": "/tmp/service_storage/535369111ae8d5d7c6dced904c0457f13c30b9ec6ed024fb53be649e4729c814",
            "Destination": "/service_config",
            "Mode": "rw",
            "RW": true
          }
        ]
      }
    ]
  },
  ...
]
```
{: codeblock}

### **API:** GET  /service/config

---

Get the service configuration for all services.

#### Parameters

none

#### Response

code:

* 200 -- success

body:
  Please refer to the service configuration table of `GET /service` api for the field definitions.

#### Example

```bash
curl http://localhost:8510/service/config | jq
"config": [
  {
    "sensor_url": "https://bluehorizon.network/services/netspeed",
    "sensor_org": "e2edev",
    "sensor_version": "2.3.0",
    "auto_upgrade": true,
    "active_upgrade": false,
    "attributes": [
      {
        "meta": {
          "id": "49cde8e2-a448-4f5e-98a1-c033deca5c53",
          "type": "UserInputAttributes",
          "label": "User input variables",
          "host_only": false,
          "publishable": false
        },
        "service_specs": [
          {
            "url": "https://bluehorizon.network/services/netspeed",
            "organization": "e2edev"
          }
        ],
        "mappings": {
          "var1": "aString",
          "var2": 5,
          "var3": 10.2,
          "var4": [
            "abc",
            "123"
          ],
          "var5": "override"
        }
      }
    ]
  },
  ...
]
```
{: codeblock}

### **API:** POST /service/config

---

Configure attributes for a service.

#### Parameters

body:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| url | | string | the url of the service to be configured. |
| organization | | string | the organization of the service. |
| name | | string | (optional) the name of the service. |
| arch | | string | architecture of the service to be configured, could be a synonym. The default is the current node architecture. |
| versionRange | | string | the version range of the service that the configuration applies to. The versionRange is in OSGI version format. The default is [0.0.0,INFINITY) |
| auto_upgrade | | boolean | whether the service should be automatically upgraded or not when a new version becomes available. The default is true. |
| active_upgrade | | boolean | whether the {{site.data.keyword.horizon}} agent should actively terminate agreements or not when new versions become available (active) or wait for all the associated agreements terminated before making upgrade. The default is false. |
| attributes  | | array of json | an array of attributes that will be applied to the the service. |
| | type | string | the type of the attribute. Most commonly used type is UserInputAttributes. |
| | label | string | a short description for this configuration. |
| | publishable| bool | whether the attribute can be made public or not. |
| | host_only | bool | whether or not the attribute will be passed to the service containers. |
| | mappings | json | a list of name and value pairs of configuration data for the service. |
{: caption="Table 20. POST /service/config JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

#### Example

```bash
read -d '' nsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "2.2.0",
  "organization": "e2edev",
  "publishable": false,
  "host_only": false,
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "bString",
        "var2": 10,
        "var3": 10.22,
        "var4": ["abcd", "1234"],
        "var5": "override2"
      }
    }
  ]
}
EOF

echo "$nsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- http://localhost:8510/service/config
```
{: codeblock}

### **API:** GET  /service/configstate

---

Get the service configuration state for all services.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| configstates | | array of json | an array of service configuration state. |
| | url | string | the url for the service. |
| | org | string | the organization for the service. |
| | configstate | string | the current configuration state for the service. The valid values are "active" and "suspended". |
{: caption="Table 21. GET /service/configstate JSON response fields" caption-side="top"}

#### Example

```bash
curl http://localhost:8510/service/configstate | jq
{
  "configstates": [
    {
      "url": "https://bluehorizon.network/services/netspeed",
      "org": "e2edev",
      "configState": "active"
    },
    {
      "url": "https://bluehorizon.network/service-cpu",
      "org": "e2edev",
      "configState": "suspended"
    },
   ...
  ]
}
```
{: codeblock}

### **API:** POST /service/configstate

---

Configure attributes for a service.

#### Parameters

body:

| name | type | description |
| ---- | ----| ---------------- |
| url | string | the url of the service to be configured. If it is an empty string and the org is also an empty string, the new configuration state will apply to all the services. If it is an empty string and the org is not an empty string, the new configuration state will apply to all the services within the organization. |
| org | string | the organization of the service to be configured. |
| configstate | string | the new configuration state for the service. |
{: caption="Table 22. POST /service/configstate JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

#### Example

```bash
curl -sS -X POST -H "Content-Type: application/json" --data '{"url": "myservice", "org": "myorg", "configstate": "suspended"}' http://localhost:8510/service/configstate
```
{: codeblock}

### **API:** GET  /service/policy

---

Get the current list of policies for each registered service.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| {policy name} | | json | the name of a policy generated for a service. |
| | header | json|  the header of the policy. It includes the name and the version of the policy. |
| | apiSpec | array | an array of api specifications. Each one includes a URL pointing to the definition of the API spec, the version of the API spec in OSGI version format, the organization that implements the API spec, whether or not exclusive access to this API spec is required and the hardware architecture of the API spec implementation. |
| | properties | array | an array of name value pairs that the current party have. |
| | agreementProtocols | array | an array of agreement protocols. Each one includes the name of the agreement protocol. |
{: caption="Table 23. GET /service/policy JSON response fields" caption-side="top"}

Note: The policy also contains other fields that are unused and therefore not documented.

#### Example

```bash
curl http://localhost:8510/service/policy | jq '.'
{
  "Policy for IBM_netspeed": {
    "header": {
      "name": "Policy for e2edev_netspeed",
      "version": "2.0"
    },
    "apiSpec": [
      {
        "specRef": "https://bluehorizon.network/services/netspeed",
        "organization": "e2edev",
        "version": "2.3.0",
        "exclusiveAccess": true,
        "arch": "amd64"
      }
    ],
    "valueExchange": {},
    "dataVerification": {
      "metering": {}
    },
    "proposalRejection": {},
    "properties": [
      {
        "name": "cpus",
        "value": "1"
      },
      {
        "name": "ram",
        "value": "1024"
      }
    ],
    "nodeHealth": {}
  },
  ...
]
```
{: codeblock}

## 5. Agreement

### **API:** GET  /agreement

---

Get all the active and archived agreements ever made by the agent. The agreements that are being terminated but not yet archived are treated as archived in this api.

#### Parameters

none

#### Response

code:

* 200 -- success

body:
| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| agreements | | json | all established agreements, active and archived. |
| | active  | array of json | an array of all the active agreements. Please refer to the following talbe for the fileds of an agreement element. |
| |archived | array of json | an array of all the archived agreements. Please refer to the following talbe for the fileds of an agreement element.|

| name | subfield | type | description |
| ---- | ---- |----| ---------------- |
| name | | string | the name of the policies used to make the agreement. |
| dependent_services || array of json | the organizations and urls of the services that the agreement workload depend on. |
| | url | string | the url for a service. |
| | organization | string | the organization for a service. |
| archived | | bool | if the agreement is archived or not. |
| current_agreement_id | | string | the id of the agreement. |
| consumer_id | | string | the id of the agbot that proposed the agreement. |
| counterparty_address | | string | the name of the agbot that proposed the agreement. |
| agreement_creation_time | | uint64 | the time when the agent received the agreement proposal from the agbot. The negotiation process starts. |
| agreement_accepted_time | | uint64 | the time when the agbot and the agent have come to agreement on the terms. Workload downloading starts. |
| agreement_finalized_time | | uint64 | the time when the agbot and the agent have finalized the agreement. Workloads are running and data is verified by the agbot. |
| agreement_execution_start_time | | uint64 | the time when the agent starts running the workloads. |
| agreement_data_received_time | | uint64 | the time when the agbot has verified that data was received from the workload. |
| agreement_terminated_time| | uint64 | the time when the agreement is terminated. |
| agreement_force_terminated_time| | uint64 | the time when the agreement is forced to be terminated by the {{site.data.keyword.horizon}} agent initialization process. |
| terminated_reason| | uint64 | the reason code for the agreement termination. |
| terminated_description | | string | the description of the agreement termination. |
| agreement_protocol_terminated_time | | uint64 | the time when the agreement protocol terminated. |
| workload_terminated_time | | uint64 | the time when the workload for an agreement terminated. |
| proposal | | string | the proposal currently in effect. |
| proposal_sig | | string | the proposal signature. |
| agreement_protocol | | string | the name of the agreement protocol being used. |
| protocol_version | | int | the version of the agreement protocol being used. |
| current_deployment | | json | contains the deployment configuration for the workload. The key is the name of the workload and the value is the result of the [/containers/\<id\> docker remote API call](https://docs.docker.com/engine/reference/api/docker_remote_api_v1.24/#/inspect-a-container) for the workload container. Please refer to the link for details. |
| extended_deployment | | json | contains the deployment configuration for the cluster node. It contains the image and the operator for deploying a Kubernetes application. |
| metering_notification | | json | the most recent metering notification received. It includes the amount, metering start time, data missed time, consumer address, consumer signature etc. |
| workload_to_run | | json | the service to run for this agreement. |
| | url | json | the url of the service. |
| | org | json | the organization of the service. |
| | version | json | the version of the service. |
| | arch | json | the architecture of the edge node the service can run on. |
{: caption="Table 24. GET /agreement JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/agreement | jq '.'
{
  "agreements": {
    "active": [
      {
        "name": "Policy for netspeed merged with netspeed arm",
        "dependent_services": [
         {
            "url": "https://bluehorizon.network/services/locgps",
            "organization": "e2edev"
          }
        ],
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
        "extended_deployment": null,
        "proposal"...",
        "proposal_sig": "174f67d343fcb0241e9bd8c01ef93f9cb17e1eb434f0234af6e5a3afcd227d93229bb97d26a2b9fae706aa3e5f2521a8dc48503405d682f319cc8925dcc3c34c01",
        "agreement_protocol": "Basic",
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
    "workload_to_run": {
      "url": "https://bluehorizon.network/services/netspeed",
      "org": "e2edev",
      "version": "2.0.6",
      "arch": "armhf"
    }
    "archived": []
  }
}
```
{: codeblock}

### **API:** DELETE  /agreement/{id}

---

Delete an agreement. The agbot will start a new agreement negotiation with the agent after the agreement is deleted.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the id of the agreement to be deleted. |
{: caption="Table 25. DELETE /agreement/\{id\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success
* 404 -- the agreement does not exist.

body:

none

#### Example

```bash
curl -X DELETE -s http://localhost:8510/agreement/a70042dd17d2c18fa0c9f354bf1b560061d024895cadd2162a0768687ed55533
```
{: codeblock}

## 6. Trusted Certs for Service Image Verification

### **API:** GET  /trust[?verbose=true]

---

Get the user stored x509 certificates for service container image verification.

#### Parameters

| name | type | description |
| -----| ---- | ---------------- |
| (query) verbose | string | (optional) parameter expands output type to include more detail about trusted certificates. Note, bare RSA PSS public keys (if trusted) are not included in detail output. |
{: caption="Table 26. POST /service/config JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| pem  | json | an array of x509 certs or public keys (if the 'verbose' query param is not supplied) that are trusted by the agent. A cert can be trusted using the PUT method in an HTTP request to the trust/ path). |
{: caption="Table 27. GET /trust JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/trust | jq  '.'
{
  "pem": [
    "Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem",
    "LULZ-1e0572c9f28c5e9a0dafa14741665c3cfd80b580-public.pem"
  ]
}
```
{: codeblock}

Verbose output:

```bash
curl -s 'http://localhost:8510/trust?verbose=true' | jq  '.'
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
{: codeblock}

Retrieve RSA PSS public key from a particular enclosing x509 certificate suitable for shell redirection to a .pem file:

```bash
curl -s 'http://localhost:8510/trust?verbose=true' | jq -r '.pem[] | select(.serial_number == "1e:05:72:c9:f2:8c:5e:9a:0d:af:a1:47:41:66:5c:3c:fd:80:b5:80")'
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
{: codeblock}

### **API:** GET  /trust/{filename}

---

Get the content of a trusted x509 cert from a file that has been previously stored by the agent.

#### Parameters

| name | type | description |
| -----| ---- | ---------------- |
| filename | string | the name of the x509 cert file to retrieve. |
{: caption="Table 28. GET /trust/\{filename\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

The contents of the requested file.

#### Example

```bash
curl -s http://localhost:8510/trust/Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem > Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem
```
{: codeblock}

### **API:** PUT  /trust/{filename}

---

Trust an x509 cert; used in service container image verification.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| filename | string | the name of the x509 cert file to upload. |
{: caption="Table 29. PUT /trust/\{filename\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

none

#### Example

```bash
curl -T ~/.rsapsstool/keypairs/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem http://localhost:8510/trust/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem
```
{: codeblock}

### **API:** DELETE  /trust/{filename}

---

Delete an x509 cert from the agent; this is a revocation of trust for a particular certificate and enclosing RSA PSS key.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| filename | string | the name of the x509 cert file to remove. |
{: caption="Table 30. DELETE /trust/\{filename\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 204 -- success

body:

none

#### Example

```bash
curl -s -X DELETE http://localhost:8510/trust/SomeOrg-6458f6e1efcbe13d5c567bd7c815ecfd0ea5459f-public.pem
```
{: codeblock}

## 7. Event Log

### **API:** GET  /eventlog

---

Get event logs for the {{site.data.keyword.horizon}} agent for the current registration. It supports selection strings. The selections can be made against the attributes.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| record_id   | string | the record id in the data base. |
| timestamp | uint64 | the time when the event log was saved. |
| severity | string | the severity for this event. It can be 'info', 'warning' or 'error'. |
| message | string | a human consumable the event message.  |
| event_code | string| an event code that can be used by programs. |
| source_type | string | the source for the event. It can be 'agreement', 'service', 'exchange', 'node' etc. |
| event_source | json | a structure that holds the event source object. |
{: caption="Table 31. GET /eventlog JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/eventlog | jq '.'
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
    "message": "Workload service containers for e2edev/https://bluehorizon.network/services/netspeed are up and running.",
    "event_code": "container_running",
    "source_type": "agreement",
    "event_source": {
      "agreement_id": "0a94bb0e85e2d98050cd89b5fc98ac3270462170ea836b1a91a2d2a01613c4f8",
       "workload_to_run": {
        "url": "https://bluehorizon.network/services/netspeed",
        "org": "e2edev",
        "version": "1.0",
        "arch": "amd64"
      },
      "dependent_services": [
        {
          "url": https://bluehorizon.network/services/gps",
          "organization": "e2edev"
      ],
      "consumer_id": "IBM/ag12345",
      "agreement_protocol": "Basic"
    }
  }
  ....
]
```
{: codeblock}

```bash
curl -s http://localhost:8510/eventlog?source_type=node&message=~Complete | jq '.'
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
{: codeblock}

### **API:** GET  /eventlog/all

---

Get all the event logs including the previous regstrations for the {{site.data.keyword.horizon}} agent. It supports selection strings. The selections can be made against the attributes.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| record_id  | string | the record id in the data base. |
| timestamp | uint64 | the time when the event log was saved. |
| severity | string | the severity for this event. It can be 'info', 'warning' or 'error'. |
| message | string | a human consumable the event message. |
| event_code | string| an event code that can be used by programs. |
| source_type | string | the source for the event. It can be 'agreement', 'service', 'exchange', 'node' etc. |
| event_source | json | a structure that holds the event source object. |
{: caption="Table 32. GET /eventlog/all JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/eventlog/all | jq '.'
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
{: codeblock}

## 8. Node User Input

### **API:** GET  /node/userinput

---

Get the node's user input for the service configurations. The user input on the local node is alway in sync with the user input for the node on the exchange.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| serviceOrgid  | string | the organization of the service. |
| serviceUrl | string | the url of the service. |
| serviceArch | string | the architecture of the service. |
| serviceVersionRange | string | the version range of the service that the configuration applies to. The serviceVersionRange is in OSGI version format. The default is [0.0.0,INFINITY). |
| inputs | json| an array of name and value pairs where the name is the variable name and the value is the variable value for service configuration. |
{: caption="Table 33. GET /node/userinput JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/node/userinput | jq
[
  {
    "serviceOrgid": "userdev",
    "serviceUrl": "mytest1",
    "serviceArch": "amd64",
    "serviceVersionRange": "[0.0.1,INFINITY)",
    "inputs": [
      {
        "name": "var1",
        "value": "aString"
      },
      {
        "name": "var2",
        "value": 22.2
      }
    ]
  },
  {
    "serviceOrgid": "userdev",
    "serviceUrl": "mytest2",
    "serviceArch": "amd64",
    "serviceVersionRange": "[2.0.1,INFINITY)",
    "inputs": [
      {
        "name": "city_name",
        "value": "New York"
      }
    ]
  }
]
```
{: codeblock}

### **API:** POST  /node/userinput

---

Set the node's user input for the service configuration. The node on the exchange will be updated too with the new user input.

#### Parameters

body:

The body is an array of the following:

| name | type | description |
| ---- | ---- | ---------------- |
| serviceOrgid | string | the organization of the service. |
| serviceUrl | string | the url of the service. |
| serviceArch | string | the architecture of the service. |
| serviceVersionRange | string | the version range of the service that the configuration applies to. The serviceVersionRange is in OSGI version format. The default is [0.0.0,INFINITY). |
| inputs | json | an array of name and value pairs where the name is the variable name and the value is the variable value for service configuration. |
{: caption="Table 34. POST /node/userinput JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '[
  {
    "serviceOrgid": "userdev",
    "serviceUrl": "mytest",
    "serviceArch": "amd64",
    "serviceVersionRange": "[0.0.1,INFINITY)",
    "inputs": [
      {
        "name": "city_name",
        "value": "New York"
      }
    ]
  }
]'  http://localhost:8510/node/userinput | jq '.'
```
{: codeblock}

### **API:** PATCH  /node/userinput

---

Patch the node's user input for the service configuration. The node on the exchange will be updated too with the new user input.

#### Parameters

body:

The body is an array of the following:

| name | type | description |
| ---- | ---- | ---------------- |
| serviceOrgid | string | the organization of the service. |
| serviceUrl | string | the url of the service. |
| serviceArch | string | the architecture of the service. |
| serviceVersionRange | string | the version range of the service that the configuration applies to. The serviceVersionRange is in OSGI version format. The default is [0.0.0,INFINITY). |
| inputs | json | an array of name and value pairs where the name is the variable name and the value is the variable value for service configuration. |
{: caption="Table 35. PUT /node/userinput JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X PATCH -H 'Content-Type: application/json'  -d '[
  {
    "serviceOrgid": "userdev",
    "serviceUrl": "mytest2",
    "serviceArch": "amd64",
    "serviceVersionRange": "[2.0.1,INFINITY)",
    "inputs": [
      {
        "name": "my_var",
        "value": "aString"
      }
    ]
  }
]'  http://localhost:8510/node/userinput | jq '.'
```
{: codeblock}

### **API:** DELETE  /node/usrinput

---

Delete the node's user input for service configuration. The exchange copy of the node's user input will also be deleted.

#### Parameters

none

#### Response

code:

* 204 -- success

body:

none

#### Example

```bash
curl -s -w "%{http_code}" -X DELETE "http://localhost:8510/node/userinput" | jq '.'
204
```
{: codeblock}

## 9. Node Policy

### **API:** GET  /node/policy

---

Get the node policy. The local node policy is alway in sync with the node policy on the exchange.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| properties | array | an array of the name-value pairs to describe the policy properties. |
| constraints | string | an array of constraint expressions of the form \<property name\> \<operator\> \<property value\>, separated by boolean operators AND (&&) or OR (\|\|). |
{: caption="Table 36. GET /node/policy JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/node/policy | jq '.'
{
  "properties": [
    {
      "name": "purpose",
      "value": "network-testing"
    },
    {
      "name": "group",
      "value": "bluenode"
    },
    {
      "name": "openhorizon.cpu",
      "value": 2
    },
    {
      "name": "openhorizon.arch",
      "value": "amd64"
    },
    {
      "name": "openhorizon.memory",
      "value": 3946
    },
    {
      "name": "openhorizon.hardwareId",
      "value": "abcdefg"
    },
    {
      "name": "openhorizon.allowPrivileged",
      "value": false
    }
  ],
  "constraints": [
    "iame2edev == true"
    "prop1 == value && prop2 == 2"
  ]
}
```
{: codeblock}

### **API:** POST  /node/policy

---

Set the node policy. The node on the exchange will be updated too with the new policy. Properties openhorizon.cpu, openhorizon.arch, openhorizon.memory, openhorizon.hardwareId are buit-in properties which cannot be changed. When openhorizon.allowPrivileged is set to true the service container is allowed to run in the 'privileged' mode if it chooses to. The default value for openhorizon.allowPrivileged is false.

#### Parameters

body:

The body is an array of the following:

| name | type | description |
| ---- | ---- | ---------------- |
| properties | array | an array of the name-value pairs to describe the policy properties. |
| constraints | string | an array of constraint expressions of the form \<property name\> \<operator\> \<property value\>, separated by boolean operators AND (&&) or OR (\|\|). |
{: caption="Table 37. POST /node/policy JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X POST -H 'Content-Type: application/json'  -d '{
  "properties": [
    {
      "name": "purpose",
      "value": "network-testing"
    },
    {
      "name": "group",
      "value": "bluenode"
    }
  ],
  "constraints": [
    "iame2edev == true",
    "MyVar==2"
  ]
}'  http://localhost:8510/node/policy  | jq '.'
```
{: codeblock}

### **API:** PATCH  /node/policy

---

Patch the properties or the constraints for the node policy. The node on the exchange will be updated too with the new patch. Properties openhorizon.cpu, openhorizon.arch, openhorizon.memory, penhorizon.hardwareId are buit-in properties which cannot be changed. When openhorizon.allowPrivileged is set to true the service container is allowed to run in the 'privileged' mode if it chooses to. The default value for openhorizon.allowPrivileged is false.

#### Parameters

body:

The body is an array of the following:

| name | type | description |
| ---- | ---- | ---------------- |
| properties | array | an array of the name-value pairs to describe the policy properties. |
| constraints | string | an array of constraint expressions of the form \<property name\> \<operator\> \<property value\>, separated by boolean operators AND (&&) or OR (\|\|). |
{: caption="Table 38. PATCH /node/policy JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X PATCH -H 'Content-Type: application/json'  -d '{
  "constraints": [
    "iame2edev == false",
    "MyVar==3"
  ]
}'  http://localhost:8510/node/policy | jq '.'
```
{: codeblock}

### **API:** DELETE  /node/policy

---

Delete the node policy. The exchange copy of the node policy will also be deleted.

#### Parameters

none

#### Response

code:

* 204 -- success

body:

none

#### Example

```bash
curl -s -w "%{http_code}" -X DELETE "http://localhost:8510/node/policy" | jq '.'
204
```
{: codeblock}

## 10. Node Management

### **API:** GET  /nodemanagement/nextjob

---

Get the status object of the next scheduled node management job. A node management job has a status object once its node management policy is picked up by the node management worker and matched to a node. If there are multiple NMP's running on the node, the earliest scheduled NMP is returned. A guide to what each status value means can be found here: [node_management_status.md](./node_management_status.md)

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| type | string | the type of job to query. Currently, the only type of job is "agentUpgrade" for agent auto upgrade jobs. If this filter is omitted, all statuses will be queried regardless of type. |
| ready | boolean | if true, only statuses that are in the "downloaded" state (upgrade packages have been downloaded to the node) will be queried. If false, only statuses that are in the "waiting" state (upgrade packages have **not** been downloaded to the node) will be queried. If this filter is omitted, all statuses will be queried regardless of state. |
{: caption="Table 39. GET /nodemanagement/nextjob JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

**agentUpgradePolicyStatus**:

* The following fields describe the status of an agent auto upgrade job as defined by the NMP created by a user. This is the structure that stays synchronized with the Exchange during the upgrade process.

| name | subfield | type | description |
| ---- | ---- | ---- | ---------------- |
| scheduledTime | | string | a RFC3339 timestamp designating when the upgrade job should start. |
| startTime | | string | a RFC3339 timestamp designating when the upgrade job actually started. This field is populated when the agent auto upgrade cronjob picks up this job and changes the status to "initiated". |
| endTime | | string | a RFC3339 timestamp designating when the upgrade job completed successfully. This field is populated when the agent auto upgrade cronjob changes the status to "successful". |
| upgradedVersions | | json | a json structure that defines the versions being upgraded/downgraded to. |
| | softwareVersion | string | the version that the agent software packages are to be upgraded/downgraded to. |
| | certVersion | string | the version of the certificate file to be upgraded/downgraded to. |
| | configVersion | string | the version of the configuration file to be upgraded/downgraded to. |
| status | | string | a string message that lists the current state of the upgrade job. |
| errorMessage | | string | a string message containing any possible error messages that occur during the job. |
| workingDirectory | | string | the directory that the upgrade job will be reading and writing files to. |
{: caption="Table 40. GET /nodemanagement/nextjob JSON response fields" caption-side="top"}

**agentUpgradeInternal**:

* The following fields describe the internal status object of an agent auto upgrade job used by the node management worker. This structure is an extension of the agentUpgradePolicyStatus structure that provides extra information to the node management worker needed to perform the upgrade.

| name | subfield | type | description |
| ---- | ---- | ---- | ---------------- |
| allowDowngrade | | boolean | a Boolean value that designates if a downgrade to a previous version is allowed to occur. |
| manifest | | string | a string value that corresponds to an upgrade manifest in the Exchange. |
| scheduledUnixTime | | string | a RFC3339 timestamp designating when the upgrade job should start in the local unix time. |
| latestMap | | json | a json map that describes which upgrade types are to track and stay up-to-date with the latest available version. |
| | softwareLatest | boolean | a Boolean value that designates if the agent software packages should stay up-to-date with the latest available version. |
| | configLatest | boolean | a Boolean value that designates if the configuration file should stay up-to-date with the latest available version. |
| | certLatest | boolean | a Boolean value that designates if the certificate should stay up-to-date with the latest available version. |
{: caption="Table 41. GET /nodemanagement/nextjob JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/nodemanagement/nextjob?type=agentUpgrade&ready=true | jq '.'
{
  "e2edev/sample-nmp": {
    "agentUpgradePolicyStatus": {
      "scheduledTime": "2022-05-24T12:00:00Z",
      "startTime": "2022-05-24T12:00:01-07:00",
      "endTime": "2022-05-24T12:00:02-07:00",
      "upgradedVersions": {
        "softwareVersion": "2.30.0",
        "certVersion": "",
        "configVersion": ""
      },
      "status": "successful",
      "workingDirectory": "/var/horizon/nmp"
    },
    "agentUpgradeInternal": {
      "allowDowngrade": false,
      "manifest": "sample_manifest",
      "scheduledUnixTime": "2022-05-24T12:00:00-07:00",
      "latestMap": {
        "softwareLatest": true,
        "configLatest": false,
        "certLatest": false
      }
    }
  }
}
```
{: codeblock}

### **API:** GET  /nodemanagement/status

---

Get a map of all status objects that apply to the node. Each status object corresponds to a node management policy that has been matched to the node and picked up by the node management worker. A guide to what each status value means can be found here: [node_management_status.md](node_management_status.md)

#### Parameters

none

#### Response

code:

* 200 -- success

body:

**agentUpgradePolicyStatus**:

* The following fields describe the status of an agent auto upgrade job as defined by the NMP created by a user. This is the structure that stays synchronized with the Exchange during the upgrade process.

| name | subfield | type | description |
| ---- | ---- | ---- | ---------------- |
| scheduledTime | | string | a RFC3339 timestamp designating when the upgrade job should start. |
| startTime | | string | a RFC3339 timestamp designating when the upgrade job actually started. This field is populated when the agent auto upgrade cronjob picks up this job and changes the status to "initiated". |
| endTime | | string | a RFC3339 timestamp designating when the upgrade job completed successfully. This field is populated when the agent auto upgrade cronjob changes the status to "successful". |
| upgradedVersions | | json | a json structure that defines the versions being upgraded/downgraded to. |
| | softwareVersion | string | the version that the agent software packages are to be upgraded/downgraded to. |
| | certVersion | string | the version of the certificate file to be upgraded/downgraded to. |
| | configVersion | string | the version of the configuration file to be upgraded/downgraded to. |
| status | | string | a string message that lists the current state of the upgrade job. |
| errorMessage | | string | a string message containing any possible error messages that occur during the job. |
| workingDirectory | | string | the directory that the upgrade job will be reading and writing files to. |
{: caption="Table 42. GET /nodemanagement/status JSON response fields" caption-side="top"}

**agentUpgradeInternal**:

* The following fields describe the internal status object of an agent auto upgrade job used by the node management worker. This structure is an extension of the agentUpgradePolicyStatus structure that provides extra information to the node management worker needed to perform the upgrade.

| name | subfield | type | description |
| ---- | ---- | ---- | ---------------- |
| allowDowngrade | | boolean | a Boolean value that designates if a downgrade to a previous version is allowed to occur. |
| manifest | | string | a string value that corresponds to an upgrade manifest in the Exchange. |
| scheduledUnixTime | | string | a RFC3339 timestamp designating when the upgrade job should start in the local unix time. |
| latestMap | | json | a json map that describes which upgrade types are to track and stay up-to-date with the latest available version. |
| | softwareLatest | boolean | a Boolean value that designates if the agent software packages should stay up-to-date with the latest available version. |
| | configLatest | boolean | a Boolean value that designates if the configuration file should stay up-to-date with the latest available version. |
| | certLatest | boolean | a Boolean value that designates if the certificate should stay up-to-date with the latest available version. |
{: caption="Table 43. GET /nodemanagement/status JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/nodemanagement/status | jq '.'
{
  "e2edev/sample-nmp": {
    "agentUpgradePolicyStatus": {
      "scheduledTime": "2022-05-24T11:00:00Z",
      "startTime": "2022-05-24T11:00:01-07:00",
      "endTime": "2022-05-24T11:00:02-07:00",
      "upgradedVersions": {
        "softwareVersion": "2.30.0",
        "certVersion": "1.0.0",
        "configVersion": "1.0.0"
      },
      "status": "successful",
      "workingDirectory": "/var/horizon/nmp"
    },
    "agentUpgradeInternal": {
      "allowDowngrade": true,
      "manifest": "sample_manifest",
      "scheduledUnixTime": "2022-05-24T11:00:00-07:00",
      "latestMap": {
        "softwareLatest": true,
        "configLatest": false,
        "certLatest": false
      }
    }
  },
  "e2edev/sample-nmp-2": {
    "agentUpgradePolicyStatus": {
      "scheduledTime": "2022-05-24T12:00:00Z",
      "startTime": "2022-05-24T12:00:01-07:00",
      "upgradedVersions": {
        "softwareVersion": "2.30.0",
        "certVersion": "1.0.0",
        "configVersion": "1.0.0"
      },
      "status": "failed",
      "errorMessage": "sample error message",
      "workingDirectory": "/var/horizon/nmp"
    },
    "agentUpgradeInternal": {
      "allowDowngrade": false,
      "manifest": "sample_manifest_2",
      "scheduledUnixTime": "2022-05-24T12:00:00-07:00",
      "latestMap": {
        "softwareLatest": true,
        "configLatest": true,
        "certLatest": true
      }
    }
  }
}
```
{: codeblock}

### **API:** GET  /nodemanagement/status/{nmpName}

---

Get the status objects that corresponds to the given node management policy name. The org that the NMP and node belong to can be optionally prepended (i.e. `/nodemanagement/status/nmp-name` and `/nodemanagement/status/org/nmp-name` refer to the same object, so long as the node is part of the given org) A guide to what each status value means can be found here: [node_management_status.md](node_management_status.md)

#### Parameters

none

#### Response

code:

* 200 -- success

body:

**agentUpgradePolicyStatus**:

* The following fields describe the status of an agent auto upgrade job as defined by the NMP created by a user. This is the structure that stays synchronized with the Exchange during the upgrade process.

| name | subfield | type | description |
| ---- | ---- | ---- | ---------------- |
| scheduledTime | | string | a RFC3339 timestamp designating when the upgrade job should start. |
| startTime | | string | a RFC3339 timestamp designating when the upgrade job actually started. This field is populated when the agent auto upgrade cronjob picks up this job and changes the status to "initiated". |
| endTime | | string | a RFC3339 timestamp designating when the upgrade job completed successfully. This field is populated when the agent auto upgrade cronjob changes the status to "successful". |
| upgradedVersions | | json | a json structure that defines the versions being upgraded/downgraded to. |
| | softwareVersion | string | the version that the agent software packages are to be upgraded/downgraded to. |
| | certVersion | string | the version of the certificate file to be upgraded/downgraded to. |
| | configVersion | string | the version of the configuration file to be upgraded/downgraded to. |
| status | | string | a string message that lists the current state of the upgrade job. |
| errorMessage | | string | a string message containing any possible error messages that occur during the job. |
| workingDirectory | | string | the directory that the upgrade job will be reading and writing files to. |
{: caption="Table 44. GET /nodemanagement/status/\{nmpname\} JSON response fields" caption-side="top"}

**agentUpgradeInternal**:

* The following fields describe the internal status object of an agent auto upgrade job used by the node management worker. This structure is an extension of the agentUpgradePolicyStatus structure that provides extra information to the node management worker needed to perform the upgrade.

| name | subfield | type | description |
| ---- | ---- | ---- | ---------------- |
| allowDowngrade | | boolean | a Boolean value that designates if a downgrade to a previous version is allowed to occur. |
| manifest | | string | a string value that corresponds to an upgrade manifest in the Exchange. |
| scheduledUnixTime | | string | a RFC3339 timestamp designating when the upgrade job should start in the local unix time. |
| latestMap | | json | a json map that describes which upgrade types are to track and stay up-to-date with the latest available version. |
| | softwareLatest | boolean | a Boolean value that designates if the agent software packages should stay up-to-date with the latest available version. |
| | configLatest | boolean | a Boolean value that designates if the configuration file should stay up-to-date with the latest available version. |
| | certLatest | boolean | a Boolean value that designates if the certificate should stay up-to-date with the latest available version. |
{: caption="Table 45. GET /nodemanagement/status/\{nmpname\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8510/nodemanagement/status/sample-nmp | jq '.'
{
  "e2edev/sample-nmp": {
    "agentUpgradePolicyStatus": {
      "scheduledTime": "2022-05-24T11:00:00Z",
      "startTime": "2022-05-24T11:00:01-07:00",
      "endTime": "2022-05-24T11:00:02-07:00",
      "upgradedVersions": {
        "softwareVersion": "2.30.0",
        "certVersion": "1.0.0",
        "configVersion": "1.0.0"
      },
      "status": "successful",
      "workingDirectory": "/var/horizon/nmp"
    },
    "agentUpgradeInternal": {
      "allowDowngrade": true,
      "manifest": "sample_manifest",
      "scheduledUnixTime": "2022-05-24T11:00:00-07:00",
      "latestMap": {
        "softwareLatest": true,
        "configLatest": false,
        "certLatest": false
      }
    }
  }
}
```
{: codeblock}

### **API:** PUT  /nodemanagement/status/{nmpName}

---

Update the status object that corresponds to the given node management policy name. The org that the NMP and node belong to can be optionally prepended (i.e. `/nodemanagement/status/nmp-name` and `/nodemanagement/status/org/nmp-name` refer to the same object, so long as the node is part of the given org) A guide to what each status value means can be found here: [node_management_status.md](node_management_status.md)

Currently, the only supported update to the status object is the agentUpgradePolicyStatus structure.

#### Parameters

body:

**agentUpgradePolicyStatus**:

* The following fields describe the status of an agent auto upgrade job as defined by the NMP created by a user. This is the structure that stays synchronized with the Exchange during the upgrade process.

| name  | type | description |
| ----  | ---- | ---------------- |
| startTime | string | a RFC3339 timestamp designating when the upgrade job actually started. This field can only be set if it has not been previously set and the status field is also changed to "initiated". |
| endTime | string | a RFC3339 timestamp designating when the upgrade job actually started. This field can only be updated if it has not been previously set and the status field is also changed to "successful". |
| status | string | a string message that lists the current state of the upgrade job. |
| errorMessage | string | a string message containing any possible error messages that occur during the job. This field can only be updated if the status field is also changed. |
{: caption="Table 46. PUT /nodemanagement/status/\{nmpname\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json' -d '{
  "agentUpgradePolicyStatus": {
    "startTime": "2022-05-24T12:00:01-07:00",
    "status": "initiated"
  }
}'  http://localhost:8510/nodemanagement/status/sample-nmp
```
{: codeblock}

```bash
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json' -d '{
  "agentUpgradePolicyStatus": {
    "endTime": "2022-05-24T12:00:02-07:00",
    "status": "successful"
  }
}'  http://localhost:8510/nodemanagement/status/sample-nmp
```
{: codeblock}

```bash
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json' -d '{
  "agentUpgradePolicyStatus": {
    "status": "failed",
    "errorMessage": "Sample error message",
  }
}'  http://localhost:8510/nodemanagement/status/sample-nmp
```
{: codeblock}

### **API:** PUT  /nodemanagement/reset

---

Reset all of the NMP status objects that are stored on the node to the "waiting" state.

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json' http://localhost:8510/nodemanagement/reset
```
{: codeblock}

### **API:** PUT  /nodemanagement/reset/{nmpName}

---

Reset all of the NMP status objects that are stored on the node to the "waiting" state.

Reset the status object that corresponds to the given node management policy name to the "waiting" state. The org that the NMP and node belong to can be optionally prepended (i.e. `/nodemanagement/status/nmp-name` and `/nodemanagement/status/org/nmp-name` refer to the same object, so long as the node is part of the given org)

#### Response

code:

* 201 -- success

#### Example

```bash
curl -s -w "%{http_code}" -X PUT -H 'Content-Type: application/json' http://localhost:8510/nodemanagement/reset/sample-nmp
```
{: codeblock}
