#!/bin/bash

# The patterns, workloads and microservices registered in this script are used to test:
#  1. self supplied public signing key in the exchange
#  2. use repo digest (something@sha256:xxxx) to specify an image for workload and miroservice
#  3. non-public pattern. the pattern is in a different org than the agbot.
#  4. workload sharing same microservice in mutiple mode.

echo -e "Registering microservice, workload and pattern (with user defined signing keys) with hzn exchange command"

export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
KEY_TEST_DIR="/tmp/keytest1"
mkdir -p $KEY_TEST_DIR
rm -f $KEY_TEST_DIR/*
cd $KEY_TEST_DIR

echo -e "Generate signing keys for microservice in $KEY_TEST_DIR:"
hzn key create e2edev e2edev@gmail.com
if [ $? -ne 0 ]
then
    echo -e "hzn key create failed."
    exit 1
fi

echo -e "Creating microservice with user defined signing keys:"
cat <<EOF >$KEY_TEST_DIR/ms.json
{
  "label": "GPS key test for amd64",
  "description": "blah blah",
  "public": true,
  "specRef": "https://bluehorizon.network/microservices/gpskeytest",
  "version": "2.0.3",
  "arch": "amd64",
  "sharable": "multiple",
  "downloadUrl": "not used yet",
  "matchHardware": {
    "usbDeviceIds": "1546:01a7",
    "devFiles": "/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput": [
  ],
  "workloads": [
    {
      "deployment": {
        "services": {
          "gps": {
            "image": "openhorizon/amd64_gps:2.0.3",
            "privileged": false,
            "devices": [
              "/dev/loop1:/dev/loop1:rw"
            ]
          }
        }
      },
      "deployment_signature": "",
      "torrent": ""
    }
  ]
}
EOF
hzn exchange microservice publish -I -u ibmadmin:ibmadminpw -o IBM -f $KEY_TEST_DIR/ms.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for key test."
    exit 1
fi

rm $KEY_TEST_DIR/*

echo -e "Generate signing keys for workload in $KEY_TEST_DIR:"
hzn key create e2edev e2edev@gmail.com
if [ $? -ne 0 ]
then
    echo -e "hzn key create failed."
    exit 1
fi

echo -e "Creating workload locationkeytest with user defined signing keys and specifying the image as the repo digest:"
cat <<EOF >$KEY_TEST_DIR/wl.json
{
  "label": "Location key test for amd64",
  "description": "blah blah",
  "public": true,
  "workloadUrl": "https://bluehorizon.network/workloads/locationkeytest",
  "version": "2.0.6",
  "arch": "amd64",
  "downloadUrl": "not used yet",
  "apiSpec": [
    {
      "specRef": "https://bluehorizon.network/microservices/gpskeytest",
      "org": "IBM",
      "version": "2.0.3",
      "arch": "amd64"
    }
  ],
  "userInput": [
  ],
  "workloads": [
    {
      "deployment": {
        "services": {
          "location": {
            "image": "openhorizon/amd64_location@sha256:6701bcbe8d0d505da490692e0dfd320f6ac8be6b292fe8679f7e8752352ee6ae",
            "environment": ["BAR=foo"]
          }
        }
      },
      "deployment_signature": "",
      "torrent": ""
     }
  ]
}

EOF
hzn exchange workload publish -u ibmadmin:ibmadminpw -o IBM -f $KEY_TEST_DIR/wl.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for locationkeytest."
    exit 1
fi

echo -e "Creating workload locationkeytest2 that shares the same microservice with locationkeytest:"
cat <<EOF >$KEY_TEST_DIR/wl2.json
{
  "label": "Location key test for amd64",
  "description": "blah blah",
  "public": true,
  "workloadUrl": "https://bluehorizon.network/workloads/locationkeytest2",
  "version": "2.0.6",
  "arch": "amd64",
  "downloadUrl": "not used yet",
  "apiSpec": [
    {
      "specRef": "https://bluehorizon.network/microservices/gpskeytest",
      "org": "IBM",
      "version": "2.0.3",
      "arch": "amd64"
    }
  ],
  "userInput": [
  ],
  "workloads": [
    {
      "deployment": {
        "services": {
          "location": {
            "image": "openhorizon/amd64_location@sha256:6701bcbe8d0d505da490692e0dfd320f6ac8be6b292fe8679f7e8752352ee6ae",
            "environment": ["BAR=foo"]
          }
        }
      },
      "deployment_signature": "",
      "torrent": ""
     }
  ]
}

EOF
hzn exchange workload publish -u ibmadmin:ibmadminpw -o IBM -f $KEY_TEST_DIR/wl2.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for locationkeytest2."
    exit 1
fi


echo -e "Creating workload networkkeytest sharing the same microservice with workload netspeed:"
cat <<EOF >$KEY_TEST_DIR/wl3.json
{
  "label":"Netspeedkeytest for workload config tests x86_64",
  "description":"Netspeed key test workload",
  "public":true,
  "workloadUrl":"https://bluehorizon.network/workloads/netspeedkeytest",
  "version":"2.3",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[{"specRef":"https://bluehorizon.network/microservices/network","version":"1.0.0","arch":"amd64","org":"IBM"}],
  "userInput":[],
  "workloads": [
    {
      "deployment": {
        "services": {
          "netspeed5":{
            "image": "openhorizon/amd64_netspeed:2.5.0",
            "environment":[
              "USE_NEW_STAGING_URL=false",
              "DEPL_ENV=staging",
              "SKIP_NUM_REPEAT_LOC_READINGS=0"
            ]
          }
        }
      },
      "deployment_signature": "",
      "torrent": ""
     }
  ]
}
EOF

hzn exchange workload publish -I -u ibmadmin:ibmadminpw -o IBM -f $KEY_TEST_DIR/wl3.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for netspeedkeytest."
    exit 1
fi

rm $KEY_TEST_DIR/*


echo -e "Generate signing keys for workload in $KEY_TEST_DIR:"
hzn key create e2edev e2edev@gmail.com
if [ $? -ne 0 ]
then
    echo -e "hzn key create failed."
    exit 1
fi

echo -e "Creating pattern with user defined signing keys:"
cat <<EOF >$KEY_TEST_DIR/ns-keytest.json
{
  "label": "netspeed keytest for amd64",
  "description": "Horizon deployment pattern that runs the netspeed workload",
  "public": false,
  "workloads": [
    {
      "workloadUrl": "https://bluehorizon.network/workloads/locationkeytest",
      "workloadOrgid": "IBM",
      "workloadArch": "amd64",
      "workloadVersions": [
        {
          "version": "2.0.6",
          "deployment_overrides": {
		        "services": {
      			   "location": {
				        "environment":[
                    "USE_NEW_STAGING_URL=false",
                    "DEPL_ENV=staging",
                    "SKIP_NUM_REPEAT_LOC_READINGS=0",
                    "E2EDEV_OVERRIDE=1"
                    ]
			         }
		        }
	         },
          "deployment_overrides_signature": "",
          "priority": {},
          "upgradePolicy": {}
        }
      ],
      "nodeHealth": {
        "missing_heartbeat_interval": 600,
        "check_agreement_status": 120
      }
    },
    {
      "workloadUrl": "https://bluehorizon.network/workloads/locationkeytest2",
      "workloadOrgid": "IBM",
      "workloadArch": "amd64",
      "workloadVersions": [
        {
          "version": "2.0.6",
          "deployment_overrides": {
            "services": {
               "location": {
                "environment":[
                    "USE_NEW_STAGING_URL=false",
                    "DEPL_ENV=staging",
                    "SKIP_NUM_REPEAT_LOC_READINGS=0",
                    "E2EDEV_OVERRIDE=1"]
               }
            }
           },
          "deployment_overrides_signature": "",
          "priority": {},
          "upgradePolicy": {}
        }
      ],
      "nodeHealth": {
        "missing_heartbeat_interval": 600,
        "check_agreement_status": 120
      }
    },
    {
      "workloadUrl": "https://bluehorizon.network/workloads/netspeed",
      "workloadOrgid": "IBM",
      "workloadArch": "amd64",
      "workloadVersions": [
        {
          "version": "2.2.0",
          "deployment_overrides": {
		      "services":{
			     "netspeed5": {
				      "environment":[
                "USE_NEW_STAGING_URL=false",
                "DEPL_ENV=staging",
                "SKIP_NUM_REPEAT_LOC_READINGS=0",
                "E2EDEV_OVERRIDE=1"]
			         }
		        }
	        },
	        "deployment_overrides_signature": "",
          "priority": {},
          "upgradePolicy": {}
        }
      ],
      "nodeHealth": {
        "missing_heartbeat_interval": 600,
        "check_agreement_status": 120
      }
    },
    {
      "workloadUrl": "https://bluehorizon.network/workloads/netspeedkeytest",
      "workloadOrgid": "IBM",
      "workloadArch": "amd64",
      "workloadVersions": [
        {
          "version": "2.3",
          "deployment_overrides": {
          "services":{
            "netspeed5": {
              "environment":[
                "USE_NEW_STAGING_URL=false",
                "DEPL_ENV=staging",
                "SKIP_NUM_REPEAT_LOC_READINGS=0",
                "E2EDEV_OVERRIDE=1"]
              }
            }
          },
          "deployment_overrides_signature": "",
          "priority": {},
          "upgradePolicy": {}
        }
      ],
      "nodeHealth": {
        "missing_heartbeat_interval": 600,
        "check_agreement_status": 120
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    },
    {
      "name": "Citizen Scientist"
    }
  ]
}
EOF
hzn exchange pattern publish -u e2edevadmin:e2edevadminpw -o e2edev -f $KEY_TEST_DIR/ns-keytest.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange pattern for publish failed for key test."
    exit 1
fi

unset HZN_EXCHANGE_URL

echo -e "Success registering microservice, workload and pattern (with user defined signing keys) with hzn exchange command"
