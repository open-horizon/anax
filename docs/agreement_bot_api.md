---
copyright:
years: 2022 - 2023
lastupdated: "2023-02-05"
description: Agreement Bot APIs
title: "Agreement Bot APIs"

parent: Agent (anax)
nav_order: 4
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# {{site.data.keyword.horizon}} Agreement Bot APIs
{: #agbot-apis}

This document contains the {{site.data.keyword.horizon}} JSON APIs for the {{site.data.keyword.horizon}} system running an Agreement Bot. The output of the APIs is in JSON compact format. To get a better view, you can use JSONView extension in your web browser or use `jq` command from the command line interface. For example:

```bash
curl -s http://<ip>/agreement | jq '.'
```
{: codeblock}

## 1. {{site.data.keyword.horizon}} Agreement Bot Remote APIs

The following APIs can be run from a remote node. They are secure APIs, which means you need to run with HTTPS and with a CA certificate file that is provided by the Agreement Bot. You also need to provide your user name and password (or API key) from the Exchange for verification and authentication. For example:

```bash
curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/deploycompatible
```
{: codeblock}

## 1.1 Deployment Compatibility Check

### **API:** GET  /deploycheck/deploycompatible

---

This API does compatibility check for the given business policy (or a pattern), service definition, node policy and node user input. It does both policy compatibility check and user input compatibility check. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.

#### Parameters

query parameters:

| name | type | description |
| ---- | ---- | ---------------- |
| checkAll | boolean | return the compatibility check result for all the service versions referenced in the business policy or pattern. |
| long | boolean | show the input which was used to come up with the result. |
{: caption="Table 1. GET /deploymentcheck/deploycompatible JSON parameter fields" caption-side="top"}

body:

| name | type | description |
| ---- | ---- | ---------------- |
| node_id | string | the exchange id of the node. Mutually exclusive with node_policy and node_user_input. |
| node_arch | string | (optional) the architecture of the node. |
| node_policy | json | the node policy that will be put in the exchange. Mutually exclusive with node_id. Please refer to [node policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/node_policy_input.json){:target="_blank"}{: .externalLink} for the format. |
| node_user_input | json | the user input that will be put in the exchange for the services. Mutually exclusive with node_id. Please refer to [node user input sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/user_input.json){:target="_blank"}{: .externalLink} for the format. |
| business_policy_id | string | the exchange id of the business policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern.|
| business_policy | json | the defintion of the business policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern. Please refer to [business policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/business_policy.json){:target="_blank"}{: .externalLink} for the format. |
| pattern_id | string | the exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy. |
| pattern | json | the pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy. Please refer to [pattern sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/pattern.json){:target="_blank"}{: .externalLink} for the format. |
| service_policy | json | (optional) the service policy that will be put in the exchange for the top level service referenced in the business policy. If omitted, the service policy will be retrieved from the exchange. The service policy has the same format as the node policy. Please refer to [node policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/node_policy_input.json){:target="_blank"}{: .externalLink} for the format. |
| service | json array | (optional) an array of the top level services that will be put in the exchange. They are referenced in the business policy or pattern. If omitted, the services will be retrieved from the exchange. Please refer to [service sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/service.json){:target="_blank"}{: .externalLink} for the format. |
{: caption="Table 2. GET /deploymentcheck/deploycompatible JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| compatible | bool | the deployment resources are compatible or not. |
| reason | map | the key is the exchange id for a service and the value is the reason why this service is not compatible. It lists reasons for all the service versions referenced in the business policy (or pattern) if checkAll=1 is set in the url. |
| input | json | the input which is used to come up with the compatibility check result. It has the same structure as the paramter body above but with details filled by the code. For example, if a business policy id is given, the business policy will be retrieved from the exchange and set in the input field. The input is only shown when the API is called with long=1 in the url. |
{: caption="Table 3. GET /deploymentcheck/deploycompatible JSON response fields" caption-side="top"}

#### Example

```bash
read -d '' comp_input <<EOF
{
  "node_id":  "userdev/an12345,
  "business_policy_id": "userdev/bp_location"
}
EOF

echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/deploycompatible | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
  }
}


echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/deploycompatible?checkAll=1 | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.7_amd64": "Policy Incompatible: Compatibility Error: Properties do not satisfy node constraint."
  }
}


echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/deploycompatible?long=1 | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible"
  },
  "input": {
    "node_id": "userdev/an12345",
    "node_arch": "amd64",
    "node_policy": {
      "properties": [...],
      "constraints": [...]
    },
    "node_user_input": [
      {
        "serviceOrgid": "e2edev@somecomp.com",
        "serviceUrl": "https://bluehorizon.network/services/locgps",
        "serviceArch": "amd64",
        "serviceVersionRange": "2.0.3",
        "inputs": [...]
      }
     ],
    "business_policy": {
      "owner": "userdev/userdevadmin",
      "label": "business policy for location",
      ...
    },
    "service": [
      {
        "org": "e2edev@somecomp.com",
        "owner": "e2edev@somecomp.com/e2edevadmin",
        "url": "https://bluehorizon.network/services/location",
        ...
      }
    ]
  }
}
```
{: codeblock}

```bash
# three different ways of getting definitions of the resource:
bp_location=$(</user/me/input_files/compcheck/business_pol_location.json)
node_ui=`cat /user/me/input_files/compcheck/node_ui.json`
read -d '' node_pol <<EOF
{
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
    "NOLOC == false ",
    "openhorizon.service.version != 2.0.6"
  ]
}
EOF

read -d '' comp_input <<EOF
{
  "node_policy":      $node_pol,
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location
}
EOF

echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/deploycompatible | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
  }
}
```
{: codeblock}

### **API:** GET  /deploycheck/policycompatible

---

This API does the policy compatibility check for the given business policy, node policy and service policy. The business policy and the service policy will be merged to check against the node policy. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.

#### Parameters

query parameters:

| name | type | description |
| ---- | ---- | ---------------- |
| checkAll | boolean | return the compatibility check result for all the service versions referenced in the business policy. |
| long | boolean | show the input which was used to come up with the result. |
{: caption="Table 4. GET /deploymentcheck/policycompatible JSON parameter fields" caption-side="top"}

body:

| name | type | description |
| ---- | ---- | ---------------- |
| node_id | string | the exchange id of the node. Mutually exclusive with node_policy. |
| node_arch | string | (optional) the architecture of the node. |
| node_policy | json | the node policy that will be put in the exchange. Mutually exclusive with node_id. Please refer to [node policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/node_policy_input.json){:target="_blank"}{: .externalLink} for the format. |
| business_policy_id | string | the exchange id of the business policy. Mutually exclusive with business_policy. |
| business_policy | json | the defintion of the business policy that will be put in the exchange. Mutually exclusive with business_policy_id.  Please refer to [business policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/business_policy.json){:target="_blank"}{: .externalLink} for the format. |
| service_policy | json | (optional) the service policy that will be put in the exchange. They are for the top level service referenced in the business policy. If omitted, the service policy will be retrieved from the exchange. The service policy has the same format as the node policy. Please refer to [node policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/node_policy_input.json){:target="_blank"}{: .externalLink} for the format. |
{: caption="Table 5. GET /deploymentcheck/policycompatible JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| compatible | bool | the policies are compatible or not. |
| reason | map | the key is the exchange id for a service and the value is the reason why this service is not compatible. It lists reasons for all the service versions referenced in the business policy (or pattern) if checkAll=1 is set in the url. |
| input | json | the input which is used to come up with the compatibility check result. It has the same structure as the paramter body above but with details filled by the code. For example, if a business policy id is given, the business policy will be retrieved from the exchange and set in the input field. The input is only shown when the API is called with long=1 in the url. |
{: caption="Table 6. GET /deploymentcheck/policycompatible JSON response fields" caption-side="top"}

#### Example

```bash
read -d '' comp_input <<EOF
{
  "node_id":  "userdev/an12345,
  "business_policy_id": "userdev/bp_location"
}
EOF


echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/policycompatible?checkAll=1 | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.7_amd64": "Policy Incompatible: Compatibility Error: Properties do not satisfy node constraint."
  }
}
```
{: codeblock}

```bash
bp_location=$(</user/me/input_files/compcheck/business_pol_location.json)
node_pol=$(</user/me/input_files/compcheck/node_policy.json)
service_pol=$(</user/me/input_files/compcheck/service_policy.json)

read -d '' comp_input <<EOF
{
  "node_policy":      $node_pol,
  "business_policy":  $bp_location,
  "service_policy":   $service_pol
}
EOF

echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/policycompatible | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
  }
}
```
{: codeblock}

### **API:** GET  /deploycheck/userinputcompatible

---

This API does the user input compatibility check for the given business policy (or a pattern), service definition and node user input. The user input values in the business policy and the node will be merged to check against the service user input requirement defined in the service definition. If the result is compatible, it means that, when deployed, the node will form an agreement with the agbot and the service will be running on the node.

#### Parameters

query parameters:

| name | type | description |
| ---- | ---- | ---------------- |
| checkAll | boolean | return the compatibility check result for all the service versions referenced in the business policy or pattern. |
| long | boolean | show the input which was used to come up with the result. |
{: caption="Table 7. GET /deploymentcheck/userinputcompatible JSON parameter fields" caption-side="top"}

body:

| name | type | description |
| ---- | ---- | ---------------- |
| node_id | string | the exchange id of the node. Mutually exclusive with node_user_input.|
| node_arch | string | (optional) the architecture of the node. |
| node_user_input | json | the user input that will be put in the exchange for the services. Mutually exclusive with node_id. Please refer to [node user input sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/user_input.json){:target="_blank"}{: .externalLink} for the format. |
| business_policy_id | string | the exchange id of the business policy. Mutually exclusive with business_policy. Mutually exclusive with pattern_id and pattern. |
| business_policy | json | the defintion of the business policy that will be put in the exchange. Mutually exclusive with business_policy_id. Mutually exclusive with pattern_id and pattern. Please refer to [business policy sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/business_policy.json){:target="_blank"}{: .externalLink} for the format. |
| pattern_id | string | the exchange id of the pattern. Mutually exclusive with pattern. Mutually exclusive with business_policy_id and business_policy. |
| pattern | json | the pattern that will be put in the exchange. Mutually exclusive with pattern_id. Mutually exclusive with business_policy_id and business_policy. Please refer to [pattern sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/pattern.json){:target="_blank"}{: .externalLink} for the format. |
| service | json array | (optional) an array of the top level services that will be put in the exchange. They are referenced in the business policy or pattern. If omitted, the services will be retrieved from the exchange. Please refer to [service sample ](https://github.com/open-horizon/anax/blob/master/cli/samples/service.json){:target="_blank"}{: .externalLink} for the format. |
{: caption="Table 8. GET /deploymentcheck/userinputcompatible JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| compatible | bool | the user inputs are compatible or not. |
| reason | map | the key is the exchange id for a service and the value is the reason why this service is not compatible. It lists reasons for all the service versions referenced in the business policy (or pattern) if checkAll=1 is set in the url. |
| input | json | the input which is used to come up with the compatibility check result. It has the same structure as the paramter body above but with details filled by the code. For example, if a business policy id is given, the business policy will be retrieved from the exchange and set in the input field. The input is only shown when the API is called with long=1 in the url. |
{: caption="Table 9. GET /deploymentcheck/userinputcompatible JSON response fields" caption-side="top"}

#### Example

```bash
read -d '' comp_input <<EOF
{
  "node_id":  "userdev/an12345,
  "pattern_id": "userdev/pat_location"
}
EOF

echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/userinputcompatible?checkAll=1 | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.7_amd64": "UserInput Incompatible: Failed to verify user input for dependent service e2edev@somecomp.com/bluehorizon.network-services-locgps_2.0.4_amd64. Failed to validate the user input type for variable HZN_LAT. type string, expecting float."
  }
}
```
{: codeblock}

```bash
bp_location=`cat /user/me/input_files/compcheck/business_pol_location.json`
node_ui=`cat /user/me/input_files/compcheck/node_ui.json`

read -d '' comp_input <<EOF
{
  "node_user_input":  $node_ui,
  "business_policy":  $bp_location
}
EOF

echo "$comp_input" | curl -sLX GET -w %{http_code} --cacert <cert_file_name> -u myord/myusername:mypassword --data @- https://123.456.78.9:8083/deploycheck/userinputcompatible | jq '.'
{
  "compatible": true,
  "reason": {
    "e2edev@somecomp.com/bluehorizon.network-services-location_2.0.6_amd64": "Compatible",
  }
}
```
{: codeblock}

## 2. {{site.data.keyword.horizon}} Agreement Bot Local APIs

The following APIs should be run on same node where agbot is running.

## 2.1 Agreement

### **API:** GET  /agreement

---

Get all the active and archived agreements made on this agbot. The agreements that are being terminated but not yet archived are treated as archived in this API. Please note that the archived agreements get purged after a period of time which is defined by PurgeArchivedAgreementHours in the agbot configuration file. The purged agreements will not be shown by this API.

#### Parameters

none

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| agreements  | json | contains active and archived agreements |
| active | array | an array of current agreements. |
| archived | array | an array of terminated agreements. |
{: caption="Table 10. GET /agreement JSON response fields" caption-side="top"}

See the GET /agreement/{id} API for documentation of the fields in an agreement.

#### Example

```bash
curl -s http://localhost/agreement | jq '.'
{
  "agreements": {
    "active": [
      {
        "current_agreement_id": "79897cbcfd478b3dff8ec1fca48635b2b88456e1c6813e46b8b82c77ebc6247b",
        "device_id": "an12345",
        ...
        "archived": false,
        "terminated_reason": 0,
        "terminated_description": ""
      },
      {
        "current_agreement_id": "f1eec810bd82ebe20ca2b07631f9343f784c1fefb226b5e6d6ae28045356c115",
        "device_id": "an12345",
        ...
        "archived": false,
        "terminated_reason": 0,
        "terminated_description": ""
      }
    ],
    "archived": []
  }
}
```
{: codeblock}

### **API:** GET  /agreement/{id}

---

Get detailed information for an agreement.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the id of the agreement to be retrieved. |
{: caption="Table 11. GET /agreement/\{id\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success
* 404 -- the agreement does not exist.

body:

| name | type | description |
| ---- | ---- | ---------------- |
| current_agreement_id  | json | the agreement id for this agreement |
| device_id | json | the exchange id of the device in this agreement |
| agreement_protocol | json | the name of the agreement protocol used to make the agreement |
| agreement_inception_time | json | the time in seconds when the agbot started the agreement protocol |
| agreement_creation_time | json | the time in seconds when the agbot sent an agreement proposal to the device |
| agreement_finalized_time | json | the time in seconds when the agreement became safely visible on the blockchain |
| agreement_timeout | json | the time in seconds when the agreement was terminated by the agreement bot |
| proposal_signature | json | the stringified digital signature (using the device's private ethereum key) of the hash of the proposal for this agreement |
| proposal | json | the merged policy document that represents the proposal |
| proposal_hash | json | the hash of the proposal for this agreement |
| consumer_proposal_sig | json | the stringified digital signature (using the agbot's private ethereum key) of the hash of the proposal for this agreement |
| policy | json | the agbot policy that was used to create the proposal |
| policy_name | json | the name of the policy used to create the proposal |
| counter_party_address | json | the ethereum address of the device |
| disable_data_verification_checks | json | true if data verification (and metering) is turned off, otherwise false |
| data_verification_time | json | the time in seconds when the agbot last detected data being sent by the device |
| data_notification_sent | json | the time in seconds when the agbot last sent a data verification message to the device |
| metering_notification_sent | json | the time in seconds when the agbot last sent a metering notification message |
| metering_notification_msgs | json | the last two metering notification messages sent to the device, ordered newest to oldest |
| archived | json | false when the agreement is active, true when it is being terminated or has already terminated |
| terminated_reason | json | the termination reason code |
| terminated_description | json | the textual description of the terminated_reason code |
{: caption="Table 12. GET /agreement/\{id\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost/agreement/93bcddde28f43cf59761e948ebff45f0ad9e060e3081dcd76e9cc94235d73a90 | jq -r '.'
{
  "current_agreement_id": "93bcddde28f43cf59761e948ebff45f0ad9e060e3081dcd76e9cc94235d73a90",
  "device_id": "an12345",
  "agreement_protocol": "Citizen Scientist",
  "agreement_inception_time": 1494855357,
  "agreement_creation_time": 1494855357,
  "agreement_finalized_time": 1494855394,
  "agreement_timeout": 0,
  "proposal_signature": "...",
  "proposal": "...",
  "proposal_hash": "95c880e862f04a04cfe05bdd414218f3c1e379a805aa11bb52c26f3448faf881",
  "consumer_proposal_sig": "...",
  "policy": "...",
  "policy_name": "Sample policy",
  "counter_party_address": "0x7dbec5ed2ec187a56e6cae4e02a8531e9b1a77b3",
  "disable_data_verification_checks": false,
  "data_verification_time": 1494855503,
  "data_notification_sent": 1494855434,
  "metering_notification_sent": 1494855492,
  "metering_notification_msgs": [
    "...",
    "..."
  ],
  "archived": false,
  "terminated_reason": 0,
  "terminated_description": ""
}
```
{: codeblock}

### **API:** DELETE  /agreement/{id}

---

Delete an agreement. The agbot will start new agreement negotiation with the device after the agreement deletion.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the id of the agreement to be deleted. |
{: caption="Table 13. DELETE /agreement/\{id\} JSON parameter fields" caption-side="top"}

#### Response
code:

* 200 -- success
* 404 -- the agreement does not exist.

body:
none

#### Example

```bash
curl -X DELETE -s http://localhost/agreement/a70042dd17d2c18fa0c9f354bf1b560061d024895cadd2162a0768687ed55533
```
{: codeblock}

## 2.2 Policy

### **API:** GET  /policy

---

Get all the names of policies that this agbot hosts.

#### Parameters

none

#### Response
code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| {org} | json | the key is the organization name. The value is a list of the policy names for the organization that are hosted by this agbot. |
{: caption="Table 14. GET /policy JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8046/policy | jq
{
  "public": [
    "netspeed-docker-public_bluehorizon.network-workloads-location-docker-public_public_amd64",
    "netspeed-docker-public_bluehorizon.network-workloads-location_IBM_arm",
    "netspeed-docker-public_bluehorizon.network-workloads-netspeed-docker-public_public_amd64",
    "netspeed-docker-public_bluehorizon.network-workloads-netspeed-docker_IBM_arm",
    "netspeed-docker_bluehorizon.network-workloads-location_IBM_amd64",
    "netspeed-docker_bluehorizon.network-workloads-location_IBM_arm",
    "netspeed-docker_bluehorizon.network-workloads-netspeed-docker_IBM_amd64",
    "netspeed-docker_bluehorizon.network-workloads-netspeed-docker_IBM_arm"
  ],
  "test": [
    "location amd64",
    "location x86_64"
  ]
}
```
{: codeblock}

### **API:** GET  /policy/{org}

---

Get all name of the policies this agbot hosts for a organization.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| org | string | the name of the organization. |
{: caption="Table 15. GET /policy/\{org\} JSON parameter fields" caption-side="top"}

#### Response
code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| {org} | json | the key is the organization name. The value is a list of the policy names for the organization that are hosted by this agbot. |
{: caption="Table 16. GET /policy/\{org\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8046/policy/test | jq
{
  "test": [
    "location amd64",
    "location x86_64"
  ]
}
```
{: codeblock}

### **API:** GET  /policy/{org}/{name}

---

Get a specific policy.

#### Parameters

| name | type | description |
| ---- | ---- | ---------------- |
| org | string | the name of the organization. |
| name | string | the name of the policy. |
{: caption="Table 17. GET /policy/\{org\}/\{name\} JSON parameter fields" caption-side="top"}

#### Response

code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| header | json|  the header of the policy. It includes the name and the version of the policy. |
| patternId | string | the name of the pattern this policy is created for. |
| workloads | json | the workload name, version, priority and its deployment  information. |
| agreementProtocols | array | an array of agreement protocols. Each one includes the name of the agreement protocol. |
| properties | array | an array of name value pairs that the current party have. |
| dataVerification | json | contains information on how data gets verified. |
| nodeHealth | json | contains information on how to determine  the health of the node. |
{: caption="Table 18. GET /policy/\{org\}/\{name\} JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8046/policy/public/netspeed-docker_bluehorizon.network-workloads-location_IBM_arm | jq
{
  "header": {
    "name": "netspeed-docker_bluehorizon.network-workloads-location_IBM_arm",
    "version": "2.0"
  },
  "patternId": "public/netspeed-docker",
  "agreementProtocols": [
    {
      "name": "Basic",
      "protocolVersion": 1
    }
  ],
  "workloads": [
    {
      "priority": {
        "priority_value": 50,
        "retries": 1,
        "retry_durations": 3600,
        "verified_durations": 52
      },
      "workloadUrl": "https://bluehorizon.network/services/location",
      "organization": "IBM",
      "version": "2.0.6",
      "arch": "arm",
      "deployment_overrides": "{\"services\":{\"location\":{\"environment\":[\"USE_NEW_STAGING_URL=false\"]}}}",
      "deployment_overrides_signature": "CmOTNqB..."
    }
  ],
  "valueExchange": {},
  "dataVerification": {
    "enabled": true,
    "interval": 480,
    "check_rate": 15,
    "metering": {
      "tokens": 1,
      "per_time_unit": "min",
      "notification_interval": 30
    }
  },
  "proposalRejection": {},
  "nodeHealth": {
    "missing_heartbeat_interval": 90,
    "check_agreement_status": 60
  }
}
```
{: codeblock}

### **API:** POST  /policy/{policy name}/upgrade

---

Force a device to attempt a workload upgrade for the given device and given policy.

#### Parameters

| name | type | description |
| ---- | ---- | ----------- |
| policy name | string | the name of the policy or file name of the policy containing the workload to upgrade. |
{: caption="Table 19. POST /policy/\{policy name\}/upgrade JSON parameter fields" caption-side="top"}

body:

| name | type | description |
| ---- | ---- | ----------- |
| agreementId | string | the agreement id of an agreement between the given policy and the device to be upgraded. |
| org         | string | the organization in which the policy exists that you want to upgrade. |
| device      | string | the device id of the device to be upgraded. |
{: caption="Table 20. POST /policy/\{policy name\}/upgrade JSON parameter fields" caption-side="top"}

Note: At least one of agreementId or device MUST be specified. Organization is always required.

#### Response
code:

* 200 -- success

body:

none

#### Example

```bash
curl -s -X POST -H "Content-Type: application/json" -d '{"device":"12345678"}' http://localhost/policy/netspeed%20policy/upgrade
```
{: codeblock}

## 2.3 Workload Usage

### **API:** GET  /workloadusage

---

Get current workload usage information for the agreements whose agbot policies have more than one workload priorities.

#### Parameters
none

#### Response
code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | number | primary key of the usage record in the local database |
| device_id | string | the device id running a workload for the agbot |
| pending_upgrade_time | timestamp | the time (in seconds) when this workload was marked to be upgraded as a result of a policy change |
| policy | json | the full consumer (agbot) policy being used to manage a workload on the device |
| policy_name | string | the name of the consumer (agbot) policy with a workload on the device |
| priority | number | the workload priority currently being used by the device |
| retry_count | number | the current number of retries used to get the workload running |
| retry_durations | number | the number of seconds within which workload failures must occur in order for the workload rollback feature to cause a workload to rollback to a lower priority workload |
| first_try_time | timestamp | the time (number of seconds) when the workload was first attempted |
| latest_retry_time | timestamp | the time (number of seconds) when the workload was most recently attempted before declaring the workload to be stable |
| disable_retry | boolean | if true, workload retries have been turned off because a stable workload priority was found |
| verified_durations | number | the number of seconds of successful data verification before disabling workload rollback retries |
| current_agreement_id | string | the agreement id which forms the agreement between the consumer (agbot) and the device |
{: caption="Table 21. GET /workloadusage JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost/workloadusage | jq '.'
[
  {
    "record_id": 1,
    "device_id": "an12345",
    "pending_upgrade_time": 0,
    "policy": "...",
    "policy_name": "netspeed policy",
    "priority": 2,
    "retry_count": 0,
    "retry_durations": 1800,
    "current_agreement_id": "9a0a76bbbb06a6d35e66992b0e6dade8f1ecab992f9c93dbcc7f076a20583790",
    "first_try_time": 1495649010,
    "latest_retry_time": 0,
    "disable_retry": true,
    "verified_durations": 45
  }
]
```
{: codeblock}

## 2.4 Status

### **API:** GET  /status

---

Get the connectivity, configuration, and blockchain status on the agbot. The output includes the status of all blockchain containers, agent configuration and the agbot host's connectivity.

#### Parameters
none

#### Response
code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| configuration| json| the configuration data. |
| configuration.exchange_api | string | the url for the exchange being used by the {{site.data.keyword.horizon}} agent. |
| configuration.exchange_version | string | the current version of the exchange being used. |
| configuration.preferred_exchange_version | string | the preferred version for the exchange in order to use all the {{site.data.keyword.horizon}} functions. |
| configuration.required_minimum_exchange_version | string | the required minimum version for the exchange. |
| configuration.architecture | string | the hardware architecture of the node as returned from the Go language API runtime.GOARCH. |
| connectivity | json | whether or not the node has network connectivity with some remote sites. |
{: caption="Table 22. GET /status JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8046/status | jq '.'
{
  "configuration": {
    "exchange_api": "https://exchange.staging.bluehorizon.network/api/v1/",
    "exchange_version": "1.55.0",
    "required_minimum_exchange_version": "1.49.0",
    "preferred_exchange_version": "1.55.0",
    "architecture": "amd64",
    "horizon_version": "2.17.2"
  },
  "liveHealth": {
    "lastDBHeartbeat": 1609137731
  }
}
```
{: codeblock}

### **API:** GET  /status/workers

---

Get the current {{site.data.keyword.horizon}} agent worker status and the status transition logs.

#### Parameters
none

#### Response
code:

* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| workers | json | the current status of each worker and its subworkers. |
| worker_status_log | string array | the history of the worker status changes. |
{: caption="Table 23. GET /status/workers JSON response fields" caption-side="top"}

#### Example

```bash
curl -s http://localhost:8046/status/workers | jq
{
  "workers": {
    "AgBot": {
      "name": "AgBot",
      "status": "initialized",
      "subworker_status": {
        "AgBotGovernAgreements": "started",
        "AgBotGovernArchivedAgreements": "started",
        "AgBotGovernBlockchain": "started",
        "AgBotPolicyGenerator": "started",
        "AgBotPolicyWatcher": "started",
        "AgbotHeartBeat": "started"
      }
    },
    "BasicProtocolHandler": {
      "name": "BasicProtocolHandler",
      "status": "initialized",
      "subworker_status": {
        "129b5b8c-11da-45d4-98b4-fc8b191ae38a": "started",
        "1dcb639c-45ee-43bc-bd39-6104eac7a03d": "started",
        "3843f354-531e-44af-9f24-9d79737ca401": "started",
      }
    }
  },
  "worker_status_log": [
    "2018-05-02 19:25:11 Worker AgBot: started.",
    "2018-05-02 19:25:13 Worker BasicProtocolHandler: initialized.",
    "2018-05-02 19:25:13 Worker BasicProtocolHandler: subworker 3843f354-531e-44af-9f24-9d79737ca401 started.",
    "2018-05-02 19:25:13 Worker BasicProtocolHandler: subworker 1dcb639c-45ee-43bc-bd39-6104eac7a03d started.",
    "2018-05-02 19:25:13 Worker BasicProtocolHandler: subworker 129b5b8c-11da-45d4-98b4-fc8b191ae38a started.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgbotHeartBeat added.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotGovernAgreements added.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotGovernArchivedAgreements added.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotPolicyWatcher added.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotPolicyGenerator added.",
    "2018-05-02 19:25:13 Worker AgBot: initialized.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgbotHeartBeat started.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotGovernAgreements started.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotGovernArchivedAgreements started.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotPolicyWatcher started.",
    "2018-05-02 19:25:13 Worker AgBot: subworker AgBotPolicyGenerator started."
  ]
}
```
{: codeblock}
