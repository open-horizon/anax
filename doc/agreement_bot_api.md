## Horizon APIs

This document contains the Horizon JSON APIs for the horizon system running an Agreement Bot. The output of the APIs is in JSON compact format. To get a better view, you can use JSONView extension in your web browser or use `jq` command from the command line interface. For example:

```
curl -s http://<ip>/agreement | jq '.'
```

### 1. Agreement

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
| active | array | an array of current agreements. | 
| archived | array | an array of terminated agreements. | 

See the GET /agreement/{id} API for documentation of the fields in an agreement.

Note: The agreements that are being terminated but not yet archived are treated as archived in this API.

**Example:**
```
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

#### **API:** GET  /agreement/{id}
---

**Parameters:**

| name | type | description |
| ---- | ---- | ---------------- |
| id   | string | the id of the agreement to be retrieved. |

**Response:**
code:
* 200 -- success
* 404 -- the agreement does not exist.

body: 

| name | type | description |
| ---- | ---- | ---------------- |
| current_agreement_id  | json | the agreement id for this agreement |
| device_id | json | the exchange id of the device in this agreement |
| ha_partners | json | a list of device ids which are HA partners of the device in this agreement |
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
| metering_notification_msgs | json | the last 2 metering notification messages sent to the device, ordered newest to oldest |
| archived | json | false when the agreement is active, true when it is being terminated or has already terminated |
| terminated_reason | json | the termination reason code |
| terminated_description | json | the textual description of the terminated_reason code |

**Example:**
```
curl -s http://localhost/agreement/93bcddde28f43cf59761e948ebff45f0ad9e060e3081dcd76e9cc94235d73a90 | jq -r '.'
{
  "current_agreement_id": "93bcddde28f43cf59761e948ebff45f0ad9e060e3081dcd76e9cc94235d73a90",
  "device_id": "an12345",
  "ha_partners": null,
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

### 2. Policy

#### **API:** POST  /policy/<policy name>/upgrade
---

This API is used to force a device to attempt a workload upgrade.

**Parameters:**
| name | type | description |
| ---- | ---- | ----------- |
| policy name | string | the name of the policy or file name of the policy containing the workload to upgrade. |

body:
| name | type | description |
| ---- | ---- | ----------- |
| agreementId | string | the agreement id of an agreement between the given policy and the device to be upgraded. |
| device      | string | the device id of the device to be upgraded.

Note: At least one of the body parameters MUST be specified.

**Response:**
code:
* 200 -- success

body:

none

**Example:**
```
curl -s -X POST -H "Content-Type: application/json" -d '{"device":"12345678"}' http://localhost/policy/netspeed%20policy/upgrade
```

### 3. Workload Usage

#### **API:** GET  /workloadusage
---

**Parameters:**
none

**Response:**
code:
* 200 -- success

body:

| name | type | description |
| ---- | ---- | ---------------- |
| id   | number | primary key of the usage record in the local database |
| device_id | string | the device id running a workload for the agbot |
| ha_partners | array | a list of device ids that are HA partners with this device |
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

**Example:**
```
curl -s http://localhost/workloadusage | jq '.'
[
  {
    "record_id": 1,
    "device_id": "an12345",
    "ha_partners": null,
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