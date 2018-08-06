#!/bin/bash

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"

# $1 - results
# $2 - 
function results {
  if [ "$(echo "$1" | jq -r '.code')" != "ok" ]
  then
    echo -e "Error: $(echo "$1" | jq -r '.msg')"
    exit 2
  fi
}


EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
E2EDEV_ADMIN_AUTH="e2edev/e2edevadmin:e2edevadminpw"
export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR
ls *.key &> /dev/null
if [ $? -eq 0 ]
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  hzn key create -l 4096 e2edev e2edev@gmail.com
  if [ $? -ne 0 ]
  then
    echo -e "hzn key create failed."
    exit 2
  fi
fi

echo -e "Copy public key into anax folder:"
cp $KEY_TEST_DIR/*public.pem /root/.colonus/. &> /dev/null


echo -e "Registering microservices and workloads"
echo -e "PATTERN setting is $PATTERN"

# cpu microservice - needed by the hzn dev tests.
VERS="1.2.2"
cat <<EOF >$KEY_TEST_DIR/ms_cpu.json
{
  "label":"CPU microservice",
  "description":"CPU microservice",
  "public":true,
  "specRef":"https://internetofthings.ibmcloud.com/microservices/cpu",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "downloadUrl":"",
  "matchHardware":{},
  "userInput":[],
  "workloads":[
    {
      "deployment": {
        "services": {
          "cpu": {
            "image": "openhorizon/example_ms_x86_cpu:1.2.2"
          }
        }
      },
      "deployment_signature":"",
      "torrent": ""
    }
  ]
}
EOF
echo -e "Register cpu microservice $VERS:"
hzn exchange microservice publish -I -u root/root:Horizon-Rul3s -o IBM -f $KEY_TEST_DIR/ms_cpu.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for cpu."
    exit 2
fi

# Register a microservice and workload via the hzn dev commands
${SCRIPT_DIR}/hzn_reg.sh
if [ $? -ne 0 ]
then
    echo -e "hzn dev microservice and workload registration failure."
    exit 1
fi

export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# test microservice
read -d '' msdef <<EOF
{
  "label":"Test microservice",
  "description":"Test microservice",
  "public":false,
  "specRef":"https://bluehorizon.network/microservices/no-such-service",
  "version":"1.0.0",
  "arch":"amd64",
  "sharable":"exclusive",
  "downloadUrl":"",
  "matchHardware":{},
  "userInput":[],
  "workloads":[]
}
EOF
echo -e "Register test microservice:"
RES=$(echo "$msdef" | curl -sLX POST -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev/microservices" | jq -r '.')
results "$RES"


# GPS microservices
VERS="2.0.3"
cat <<EOF >$KEY_TEST_DIR/ms_gps.json
{
  "label":"GPS for x86_64",
  "description":"GPS microservice",
  "public":false,
  "specRef":"https://bluehorizon.network/microservices/gps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "downloadUrl":"",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[],
  "workloads":[{
    "deployment":{
      "services":{
        "gps":{
          "image":"openhorizon/amd64_gps:2.0.3",
          "privileged":true,
          "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
        }
      }
    },
    "deployment_signature":"",
    "torrent":""
  }]
}
EOF
echo -e "Register GPS microservice $VERS:"
hzn exchange microservice publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/ms_gps.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for gps."
    exit 2
fi


# GPS microservices
VERS="2.0.4"
cat <<EOF >$KEY_TEST_DIR/ms_gps2.json
{
  "label":"GPS for x86_64",
  "description":"GPS microservice",
  "public":false,
  "specRef":"https://bluehorizon.network/microservices/gps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "downloadUrl":"",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[],
  "workloads":[{
    "deployment":{
      "services":{
        "gps":{
          "image":"openhorizon/amd64_gps:2.0.3",
          "privileged":true,
          "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
        }
      }
    },
    "deployment_signature":"",
    "torrent":""
  }]
}
EOF
echo -e "Register GPS microservice $VERS:"
hzn exchange microservice publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/ms_gps2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for gps."
    exit 2
fi


# GPS microservice for the location workload (because we dont have shared microservices yet)
VERS="2.0.3"
cat <<EOF >$KEY_TEST_DIR/ms_locgps.json
{
  "label":"GPS for Location for x86_64",
  "description":"GPS microservice for loc workload",
  "public":true,
  "specRef":"https://bluehorizon.network/microservices/locgps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "downloadUrl":"",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[
    {"name":"test","label":"","type":"string","defaultValue":null},
    {"name":"testdefault","label":"","type":"string","defaultValue":"default"}
  ],
  "workloads":[{
    "deployment":{
      "services":{
        "gps":{
          "image":"openhorizon/amd64_gps:2.0.3",
          "privileged":true,
          "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
        }
      }
    },
    "deployment_signature":"",
    "torrent":""
  }]
}
EOF
echo -e "Register LOCGPS Loc microservice $VERS:"
hzn exchange microservice publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/ms_locgps.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for gps."
    exit 2
fi

VERS="2.0.4"
cat <<EOF >$KEY_TEST_DIR/ms_locgps2.json
{
  "label":"GPS for Location for x86_64",
  "description":"GPS microservice for loc workload",
  "public": false,
  "specRef":"https://bluehorizon.network/microservices/locgps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "downloadUrl":"",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[
    {"name":"test","label":"","type":"string","defaultValue":null},
    {"name":"testdefault","label":"","type":"string","defaultValue":"default"}
  ],
  "workloads":[{
    "deployment":{
      "services":{
        "gps":{
          "image":"openhorizon/amd64_gps:2.0.3",
          "privileged":true,
          "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
        }
      }
    },
    "deployment_signature":"",
    "torrent":""
  }]
}
EOF
echo -e "Register GPS Loc microservice $VERS:"
hzn exchange microservice publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/ms_locgps2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for gps."
    exit 2
fi



# network microservices - in the IBM org
VERS="1.5.0"
read -d '' msdef <<EOF
{
  "label":"Network for x86_64",
  "description":"Network microservice",
  "public":true,
  "specRef":"https://bluehorizon.network/microservices/network",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"multiple",
  "downloadUrl":"",
  "matchHardware":{},
  "userInput":[],
  "workloads":[]
}
EOF
echo -e "Register network microservice $VERS:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/IBM/microservices" | jq -r '.')
results "$RES"

VERS="1.5.0"
read -d '' msdef <<EOF
{
  "label":"Network2 for x86_64",
  "description":"Network2 microservice",
  "public":true,
  "specRef":"https://bluehorizon.network/microservices/network2",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"multiple",
  "downloadUrl":"",
  "matchHardware":{},
  "userInput":[],
  "workloads":[]
}
EOF
echo -e "Register network2 microservice $VERS:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/IBM/microservices" | jq -r '.')
results "$RES"

# weather sim microservice
VERS="1.0.0"
cat <<EOF >$KEY_TEST_DIR/ms_weathersim.json
{
  "label":"Weather sim for x86_64",
  "description":"Weather sim microservice",
  "public":false,
  "specRef":"https://bluehorizon.network/microservices/weathersim",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"exclusive",
  "downloadUrl":"",
  "matchHardware":{},
  "userInput":[],
  "workloads":[]
}
EOF
echo -e "Register weather sim microservice $VERS:"
hzn exchange microservice publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/ms_weathersim.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange microservice publish failed for weather sim."
    exit 2
fi

# Location workload
VERS="2.0.6"
cat <<EOF >$KEY_TEST_DIR/wl_location.json
{
  "label":"Location for x86_64",
  "description":"Location workload",
  "public":false,
  "workloadUrl":"https://bluehorizon.network/workloads/location",
  "version":"$VERS",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[
    {"specRef":"https://bluehorizon.network/microservices/locgps","version":"2.0.3","arch":"amd64","org":"e2edev"}
  ],
  "userInput":[],
  "workloads":[
    {
      "deployment":{
        "services":{
          "location":{
            "environment":["USE_NEW_STAGING_URL=false", "DEPL_ENV=staging"],
            "image": "openhorizon/amd64_location:2.0.6"
          }
        }
      },
      "deployment_signature":"",
      "torrent":""
    }
  ]
}
EOF
echo -e "Register location workload $VERS:"
hzn exchange workload publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/wl_location.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for location."
    exit 2
fi

VERS="2.0.7"
cat <<EOF >$KEY_TEST_DIR/wl_location2.json
{
  "label":"Location for x86_64",
  "description":"Location workload",
  "public":false,
  "workloadUrl":"https://bluehorizon.network/workloads/location",
  "version":"$VERS",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[
    {"specRef":"https://bluehorizon.network/microservices/locgps","version":"2.0.4","arch":"amd64","org":"e2edev"}
  ],
  "userInput":[],
  "workloads":[
    {
      "deployment":{
        "services":{
          "location":{
            "environment":["USE_NEW_STAGING_URL=false", "DEPL_ENV=staging"],
            "image": "openhorizon/amd64_location:2.0.6"
          }
        }
      },
      "deployment_signature":"",
      "torrent":""
    }
  ]
}
EOF
echo -e "Register location workload $VERS:"
hzn exchange workload publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/wl_location2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for location."
    exit 2
fi

# Netspeed workload

# register version 2.2.0 for workload config testing purposes
VERS="2.2.0"
cat <<EOF >$KEY_TEST_DIR/wl_netspeed.json
{
  "label":"Netspeed for workload config tests x86_64",
  "description":"Netspeed workload",
  "public":true,
  "workloadUrl":"https://bluehorizon.network/workloads/netspeed",
  "version":"$VERS",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[{"specRef":"https://bluehorizon.network/microservices/network","version":"1.0.0","arch":"amd64","org":"IBM"}],
  "userInput":[
    {
      "name":"var1",
      "label":"",
      "type":"string"
    },
    {
      "name":"var2",
      "label":"",
      "type":"int"
    },
    {
      "name":"var3",
      "label":"",
      "type":"float"
    },
    {
      "name":"var4",
      "label":"",
      "type":"list of strings"
    },
    {
      "name":"var5",
      "label":"",
      "type":"string",
      "defaultValue":"default"
    },
    {
      "name":"var6",
      "label":"",
      "type":"string",
      "defaultValue":"default"
    }
  ],
  "workloads":[
    {
      "deployment":{
        "services":{
          "netspeed5":{
            "image":"openhorizon/amd64_netspeed:2.5.0",
            "environment":["USE_NEW_STAGING_URL=false","DEPL_ENV=staging","SKIP_NUM_REPEAT_LOC_READINGS=0"]
          }
        }
      },
      "deployment_signature":"",
      "torrent":""
    }
  ]
}
EOF
echo -e "Register netspeed workload $VERS:"
hzn exchange workload publish -I -u root/root:Horizon-Rul3s -o IBM -f $KEY_TEST_DIR/wl_netspeed.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for network."
    exit 2
fi

# register version 2.3.0 for workload execution purposes
VERS="2.3.0"
cat <<EOF >$KEY_TEST_DIR/wl_netspeed2.json
{
  "label":"Netspeed for x86_64",
  "description":"Netspeed workload",
  "public":true,
  "workloadUrl":"https://bluehorizon.network/workloads/netspeed",
  "version":"$VERS",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[
    {"specRef":"https://bluehorizon.network/microservices/network","version":"1.0.0","arch":"amd64","org":"IBM"},
    {"specRef":"https://bluehorizon.network/microservices/network2","version":"1.0.0","arch":"amd64","org":"IBM"}
  ],
  "userInput":[
    {
      "name":"var1",
      "label":"",
      "type":"string"
    },
    {
      "name":"var2",
      "label":"",
      "type":"int"
    },
    {
      "name":"var3",
      "label":"",
      "type":"float"
    },
    {
      "name":"var4",
      "label":"",
      "type":"list of strings"
    },
    {
      "name":"var5",
      "label":"",
      "type":"string",
      "defaultValue":"default"
    },
    {
      "name":"var6",
      "label":"",
      "type":"string",
      "defaultValue":"default"
    }
  ],
  "workloads":[
    {
      "deployment": {
        "services":{
          "netspeed5":{
            "image":"openhorizon/amd64_netspeed:2.5.0",
            "environment":["USE_NEW_STAGING_URL=false","DEPL_ENV=staging","SKIP_NUM_REPEAT_LOC_READINGS=0"]
          }
        }
      },
      "deployment_signature":"",
      "torrent":""
    }
  ]
}
EOF
echo -e "Register netspeed workload $VERS:"
hzn exchange workload publish -I -u root/root:Horizon-Rul3s -o IBM -f $KEY_TEST_DIR/wl_netspeed2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for network."
    exit 2
fi

# GPSTest workload
VERS="1.0.0"
cat <<EOF >$KEY_TEST_DIR/wl_gpstest.json
{
  "label":"GPSTest for x86_64",
  "description":"GPS Test workload",
  "public":false,
  "workloadUrl":"https://bluehorizon.network/workloads/gpstest",
  "version":"$VERS",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[
    {"specRef":"https://bluehorizon.network/microservices/gps","version":"2.0.3","arch":"amd64","org":"e2edev"}
  ],
  "userInput":[],
  "workloads":[
    {
      "deployment": {
        "services": {
          "gpstest": {
            "environment":["REPORTING_INTERVAL=1", "INTERVAL_SLEEP=5", "HEARTBEAT_TO_MQTT=true"],
            "image": "openhorizon/amd64_gps-test:latest"
          }
        }
      },
      "deployment_signature":"",
      "torrent":""
    }
  ]
}
EOF
echo -e "Register GPSTest workload $VERS:"
hzn exchange workload publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/wl_gpstest.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for GPSTest."
    exit 2
fi

# PWS workload
VERS="1.5.0"
cat <<EOF >$KEY_TEST_DIR/wl_weather.json
{
  "label":"Weather for x86_64",
  "description":"PWS workload",
  "public":false,
  "workloadUrl":"https://bluehorizon.network/workloads/weather",
  "version":"$VERS",
  "arch":"amd64",
  "downloadUrl":"",
  "apiSpec":[
    {"specRef":"https://bluehorizon.network/microservices/weathersim","version":"1.0.0","arch":"amd64","org":"e2edev"}
  ],
  "userInput":[
    {"name":"HZN_WUGNAME","label":"","type":"string"},
    {"name":"HZN_PWS_MODEL","label":"","type":"string"},
    {"name":"MTN_PWS_MODEL","label":"","type":"string"},
    {"name":"HZN_PWS_ST_TYPE","label":"","type":"string"},
    {"name":"MTN_PWS_ST_TYPE","label":"","type":"string"}
  ],
  "workloads":[
    {
      "deployment": {
        "services": {
          "eaweather": {
            "environment":["DEPL_ENV=staging", "USE_NEW_STAGING_URL=false", "MOCK=true"],
            "image":"openhorizon/amd64_eaweather:1.8"
          }
        }
      },
      "deployment_signature":"",
      "torrent":""
    }
  ]
}
EOF
echo -e "Register PWS workload $VERS:"
hzn exchange workload publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev -f $KEY_TEST_DIR/wl_weather.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange workload publish failed for PWS."
    exit 2
fi



# Create Patterns
# Patterns are only created when explicitly testing them. All pattern are registered. The PATTERN env var conditions which pattern the agbot
# is configured to serve.

# weather pattern
VERS="1.5.0"
read -d '' msdef <<EOF
{
  "label": "Weather",
  "description": "a weather pattern",
  "public": true,
  "workloads": [
    {
      "workloadUrl":"https://bluehorizon.network/workloads/weather",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$VERS",
          "deployment_overrides": "",
          "deployment_overrides_signature": "",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        },
        {
          "version":"$VERS",
          "deployment_overrides": "",
          "deployment_overrides_signature": "",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 180,
        "check_agreement_status": 45
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ]
}
EOF
echo -e "Register pws pattern $VERS:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev/patterns/pws" | jq -r '.')
results "$RES"

# ns pattern
VERS="2.3.0"
read -d '' msdef <<EOF
{
  "label": "Netspeed",
  "description": "a netspeed pattern",
  "public": true,
  "workloads": [
    {
      "workloadUrl":"https://bluehorizon.network/workloads/netspeed",
      "workloadOrgid":"IBM",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$VERS",
          "deployment_overrides":"{\\\"services\\\":{\\\"netspeed5\\\":{\\\"environment\\\":[\\\"E2EDEV_OVERRIDE=1\\\"]}}}",
          "deployment_overrides_signature":"CoQb1Tw204vbMP0H1Faw7Sp9lHHSiIzvhlX9SEejx2kRY+x6uj7PB4fvJUBoYlWJJOkecQKDD9zdLm6hD32b+f9zMWaBdRF5Ab4pHU5gcDPpuPGnYup1ZreSe4eqPnThkGgfYIW5zcQd/vbxO9tx31EM8lJ5NrhcJ5rwhwbIPDh7Hstxi84IetNAygE1gPaTGQaJzzqATFYINwWkxjJXjihdEVuo5IvINJusHtIs7C6BIVy9+CExUXXxem1I/bzvwzY1wpKuubHxq1CddIKr+BaHAsErHIHvJQVc3JoDPgXPjVE8ew1QKjLCkC86wRbANN6rhCB2Q6+HhyaRfv8oJDz8XoLBcYw6bKerGMCxEBTuyUu0n9mTSCzEZZaaLdmxTzaLN47Svm8Gj18tT5CjvYkeSgDpISwRR0aME8YSHO6OtRKhLFGvZDzR4hu6kzyfp7aiYHRzVDrfcKhch/c0AuAEb6qEQ8nCHnSFJwEXP/3L3qKy8y8OT+42vumXTYOp7IadZ+UnFxLNJip9qnEsXFS8+WlT3PwaNMKFdg+zsJfUz5V+OXaotZKfe9PABn4+656PfngIi+N7q/unnrNSzc/BN8Dgy1FSHqVQ0UfRWST31pStJi2kS46UreIBgG9T6D/WgwnvATN3BaZkveiwDpUXRNv5nGzcWqnIerWWfL8=",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 1800,
            "verified_durations": 45
          },
          "upgradePolicy": {}
        },
        {
          "version":"$VERS",
          "deployment_overrides":"{\\\"services\\\":{\\\"netspeed5\\\":{\\\"environment\\\":[\\\"E2EDEV_OVERRIDE=1\\\"]}}}",
          "deployment_overrides_signature":"CoQb1Tw204vbMP0H1Faw7Sp9lHHSiIzvhlX9SEejx2kRY+x6uj7PB4fvJUBoYlWJJOkecQKDD9zdLm6hD32b+f9zMWaBdRF5Ab4pHU5gcDPpuPGnYup1ZreSe4eqPnThkGgfYIW5zcQd/vbxO9tx31EM8lJ5NrhcJ5rwhwbIPDh7Hstxi84IetNAygE1gPaTGQaJzzqATFYINwWkxjJXjihdEVuo5IvINJusHtIs7C6BIVy9+CExUXXxem1I/bzvwzY1wpKuubHxq1CddIKr+BaHAsErHIHvJQVc3JoDPgXPjVE8ew1QKjLCkC86wRbANN6rhCB2Q6+HhyaRfv8oJDz8XoLBcYw6bKerGMCxEBTuyUu0n9mTSCzEZZaaLdmxTzaLN47Svm8Gj18tT5CjvYkeSgDpISwRR0aME8YSHO6OtRKhLFGvZDzR4hu6kzyfp7aiYHRzVDrfcKhch/c0AuAEb6qEQ8nCHnSFJwEXP/3L3qKy8y8OT+42vumXTYOp7IadZ+UnFxLNJip9qnEsXFS8+WlT3PwaNMKFdg+zsJfUz5V+OXaotZKfe9PABn4+656PfngIi+N7q/unnrNSzc/BN8Dgy1FSHqVQ0UfRWST31pStJi2kS46UreIBgG9T6D/WgwnvATN3BaZkveiwDpUXRNv5nGzcWqnIerWWfL8=",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600
          },
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 120,
        "check_agreement_status": 30
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ]
}
EOF
echo -e "Register ns pattern $VERS:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev/patterns/ns" | jq -r '.')
results "$RES"

# loc pattern
VERS="2.0.6"
VERS2="2.0.7"
read -d '' msdef <<EOF
{
  "label": "Location",
  "description": "a location pattern",
  "public": true,
  "workloads": [
    {
      "workloadUrl":"https://bluehorizon.network/workloads/location",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$VERS",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        },
        {
          "version":"$VERS2",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 240,
        "check_agreement_status": 60
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ]
}
EOF
echo -e "Register loc pattern $VERS:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev/patterns/loc" | jq -r '.')
results "$RES"

# gps test pattern
VERS="1.0.0"
read -d '' msdef <<EOF
{
  "label": "GPS Test",
  "description": "a GPS Test pattern",
  "public": true,
  "workloads": [
    {
      "workloadUrl":"https://bluehorizon.network/workloads/gpstest",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$VERS",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{},
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 90,
        "check_agreement_status": 60
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ]
}
EOF
echo -e "Register gps pattern $VERS:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev/patterns/gps" | jq -r '.')
results "$RES"

# the all pattern
PWSVERS="1.5.0"
NSVERS="2.3.0"
LOCVERS1="2.0.6"
LOCVERS2="2.0.7"
GPSVERS="1.0.0"
UHSVERS="1.0.0"
read -d '' msdef <<EOF
{
  "label": "All",
  "description": "a pattern for all workloads",
  "public": true,
  "workloads": [
    {
      "workloadUrl":"https://bluehorizon.network/workloads/weather",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$PWSVERS",
          "deployment_overrides": "",
          "deployment_overrides_signature": "",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        },
        {
          "version":"$PWSVERS",
          "deployment_overrides": "",
          "deployment_overrides_signature": "",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 180,
        "check_agreement_status": 45
      }
    },
    {
      "workloadUrl":"https://bluehorizon.network/workloads/netspeed",
      "workloadOrgid":"IBM",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$NSVERS",
          "deployment_overrides":"{\\\"services\\\":{\\\"netspeed5\\\":{\\\"environment\\\":[\\\"E2EDEV_OVERRIDE=1\\\"]}}}",
          "deployment_overrides_signature":"CoQb1Tw204vbMP0H1Faw7Sp9lHHSiIzvhlX9SEejx2kRY+x6uj7PB4fvJUBoYlWJJOkecQKDD9zdLm6hD32b+f9zMWaBdRF5Ab4pHU5gcDPpuPGnYup1ZreSe4eqPnThkGgfYIW5zcQd/vbxO9tx31EM8lJ5NrhcJ5rwhwbIPDh7Hstxi84IetNAygE1gPaTGQaJzzqATFYINwWkxjJXjihdEVuo5IvINJusHtIs7C6BIVy9+CExUXXxem1I/bzvwzY1wpKuubHxq1CddIKr+BaHAsErHIHvJQVc3JoDPgXPjVE8ew1QKjLCkC86wRbANN6rhCB2Q6+HhyaRfv8oJDz8XoLBcYw6bKerGMCxEBTuyUu0n9mTSCzEZZaaLdmxTzaLN47Svm8Gj18tT5CjvYkeSgDpISwRR0aME8YSHO6OtRKhLFGvZDzR4hu6kzyfp7aiYHRzVDrfcKhch/c0AuAEb6qEQ8nCHnSFJwEXP/3L3qKy8y8OT+42vumXTYOp7IadZ+UnFxLNJip9qnEsXFS8+WlT3PwaNMKFdg+zsJfUz5V+OXaotZKfe9PABn4+656PfngIi+N7q/unnrNSzc/BN8Dgy1FSHqVQ0UfRWST31pStJi2kS46UreIBgG9T6D/WgwnvATN3BaZkveiwDpUXRNv5nGzcWqnIerWWfL8=",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 1800,
            "verified_durations": 45
          },
          "upgradePolicy": {}
        },
        {
          "version":"$NSVERS",
          "deployment_overrides":"{\\\"services\\\":{\\\"netspeed5\\\":{\\\"environment\\\":[\\\"E2EDEV_OVERRIDE=1\\\"]}}}",
          "deployment_overrides_signature":"CoQb1Tw204vbMP0H1Faw7Sp9lHHSiIzvhlX9SEejx2kRY+x6uj7PB4fvJUBoYlWJJOkecQKDD9zdLm6hD32b+f9zMWaBdRF5Ab4pHU5gcDPpuPGnYup1ZreSe4eqPnThkGgfYIW5zcQd/vbxO9tx31EM8lJ5NrhcJ5rwhwbIPDh7Hstxi84IetNAygE1gPaTGQaJzzqATFYINwWkxjJXjihdEVuo5IvINJusHtIs7C6BIVy9+CExUXXxem1I/bzvwzY1wpKuubHxq1CddIKr+BaHAsErHIHvJQVc3JoDPgXPjVE8ew1QKjLCkC86wRbANN6rhCB2Q6+HhyaRfv8oJDz8XoLBcYw6bKerGMCxEBTuyUu0n9mTSCzEZZaaLdmxTzaLN47Svm8Gj18tT5CjvYkeSgDpISwRR0aME8YSHO6OtRKhLFGvZDzR4hu6kzyfp7aiYHRzVDrfcKhch/c0AuAEb6qEQ8nCHnSFJwEXP/3L3qKy8y8OT+42vumXTYOp7IadZ+UnFxLNJip9qnEsXFS8+WlT3PwaNMKFdg+zsJfUz5V+OXaotZKfe9PABn4+656PfngIi+N7q/unnrNSzc/BN8Dgy1FSHqVQ0UfRWST31pStJi2kS46UreIBgG9T6D/WgwnvATN3BaZkveiwDpUXRNv5nGzcWqnIerWWfL8=",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600
          },
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 120,
        "check_agreement_status": 30
      }
    },
    {
      "workloadUrl":"https://bluehorizon.network/workloads/location",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$LOCVERS1",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        },
        {
          "version":"$LOCVERS2",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 240,
        "check_agreement_status": 60
      }
    },
    {
      "workloadUrl":"https://bluehorizon.network/workloads/gpstest",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$GPSVERS",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{},
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {}
    },
    {
      "workloadUrl":"http://bluehorizon.network/workloads/usehello",
      "workloadOrgid":"e2edev",
      "workloadArch":"amd64",
      "workloadVersions":[
        {
          "version":"$UHSVERS",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{},
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {
        "missing_heartbeat_interval": 90,
        "check_agreement_status": 60
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ]
}
EOF
echo -e "Register all pattern:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev/patterns/all" | jq -r '.')
results "$RES"

unset HZN_EXCHANGE_URL
