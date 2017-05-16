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