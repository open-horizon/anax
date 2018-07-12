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


# mqttbroker microservice
MQMD='{\\\"services\\\":{\\\"edge-mqttbroker\\\":{\\\"image\\\":\\\"wiotp-connect/edge/amd64/edge-mqttbroker:1.0.7\\\",\\\"binds\\\":[\\\"/etc/wiotp-edge:/etc/wiotp-edge\\\",\\\"/var/wiotp-edge:/var/wiotp-edge\\\"],\\\"privileged\\\":true},\\\"edge-connector\\\":{\\\"image\\\":\\\"wiotp-connect/edge/amd64/edge-connector:1.2.1\\\",\\\"binds\\\":[\\\"/etc/wiotp-edge:/etc/wiotp-edge\\\",\\\"/var/wiotp-edge:/var/wiotp-edge\\\"],\\\"privileged\\\":true,\\\"specific_ports\\\":[{\\\"HostPort\\\":\\\"8883\\\",\\\"HostIP\\\":\\\"0.0.0.0\\\"},{\\\"HostPort\\\":\\\"1883\\\",\\\"HostIP\\\":\\\"0.0.0.0\\\"}]},\\\"edge-im\\\":{\\\"image\\\":\\\"wiotp-infomgmt/edge/amd64/edge-im:1.0.3\\\",\\\"binds\\\":[\\\"/etc/wiotp-edge:/etc/wiotp-edge\\\",\\\"/var/wiotp-edge:/var/wiotp-edge\\\"],\\\"privileged\\\":true}}}'
MQMDS='L7VR+Z6Dc+TU6DmtRa3naeJI6sfjRj7lS9uRrH2BTm8ePeSnrBkh7KDGtfbYJntQ1niqkDDvT4RyZgxEuLPFyjF1z7O59QMWdlNFXXaOyC2YERTwyOzQJFaKPMeNyz/YS91xkzt75FAb73sl6YhIo27bzKUzXkjGK6gZVz4UsnrKxR63zXxVu7L0G1ChKH2XhRl4DPT4ehQBXSAUHKDsmpUk7PrtxLNt0O/kAUl9t1WpCrrO6OAP8II+pO+/E/q0jA6JhrknMkJAjaztzFKPNGVhRJTAd2JyLZW9PtCfzeDaEIs93lpvuATpc9jgzrVjddHcPRY5iw4nt33GFqaFxyfMuX7wmaE+NQdJSQ7NWpWJuK32r6EL3W5ne3kWV0cRcnzIBcYp+Udd/puAS5zgmNuMGO2q0mnTf+LFqfVjnrJfcRC+tM2ZoXBLRIcM78RTMEa4S3tJdNKRV7LORfa/y0BLFCTBsU0G1/jkMOJwK8UrDArhPyVOi3fQfKEMvzH9KgIoSj3KkpukFXmwD23YW8hlSe3DK61a2BF5wqF0oX8R4WLPE++1RyeZRjWzsPBhoxM0o1t+TCC5JWvdEoJZKJoW+9csxjdcJosoyHRRXsqtB4oWuyGsBItOejrK/AaZDaZRGmvYQQxOFjMNy7VcOG+s+TsUXP1gEnwuUrl9ers='
MQMT='{\\\"url\\\":\\\"https://us.internetofthings.ibmcloud.com/api/v0002/horizon-image/common/939a394cdb48c3bbe468a9770cf3c6571d934490.json\\\",\\\"signature\\\":\\\"AQO6MfqvBY7dVgTi2//UXU/sRwuiy7Un0eeWHTHqhAyfV2f2bTdzNTUBj9aesvvMOiux7RwlbBB01s2PLhB/1PVUPsBnqwhMblOfZZqcX4d32aBYgah/Ac30W0seb+6Vrh5F3O0TKJMHufRhRSv7DI96HV5MNWZPrd+7QkXJAC21Hpj07wwBxQ0+AgcfO6Lra/sR3k2ZV6f/P/k+7WsrpH4eUnXlwu913qH3AyaOecdsIQ9ERCRz6mBMxQflO8TipTsFJddpWyHaFjfc6n8uHPfKYzUBpbYuG2kUjPX1GUeJMqV1q3caR4YResuUtBlOriekaBUmSreI4a3v28eZ9SWH/i2ldOh/iiRraFxQAdJ6HTC1whEbJHou98k2CmbPkEbyd/Mu1lrbt62Dj/BCh1G7T8dnXtllnZ+UteRDzJVF1VN8gaXF05R8XFuvaFBhtI5ZHJnyAc4uXh4tk4A6D264yPa2IqrNVh4gR/f6ygC1CfbhwmRYeEIPheWrs7MnbpJom183nOnk8rrx1LjYqbl7k80L3KAQlH3XIxAzVPi0aCl8MdbAGV983od57JuCLcyo6Zck1RFgBgf+GEoqk6ygnlee/gKVMBmuMPlnQkSsn5FN+RpJWiVeS6vM/HM/q4gwY0x87pyX/0f9DPV2I/FW11trrIeOwUq9wUqyMKU=\\\"}'
VERS="1.0.11"
read -d '' msdef <<EOF
{
  "label": "Edge Core IoT Microservice",
  "description": "Images for Edge Core IoT Microservice",
  "public":true,
  "specRef":"https://internetofthings.ibmcloud.com/wiotp-edge/microservices/edge-core-iot-microservice",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"exclusive",
  "downloadUrl":"",
  "matchHardware":{},
  "userInput": [
      {
        "name": "WIOTP_LOCAL_BROKER_PORT",
        "label": "MQTT Broker connection port",
        "type": "string",
        "defaultValue": ""
      },
      {
        "name": "WIOTP_CLIENT_ID",
        "label": "Watson IoT Platform client ID in the format g:orgid:gatewaytype:gatewayid",
        "type": "string",
        "defaultValue": ""
      },
      {
        "name": "WIOTP_DEVICE_AUTH_TOKEN",
        "label": "Watson IoT Platform auth token for this gateway ID.",
        "type": "string",
        "defaultValue": ""
      },
      {
        "name": "WIOTP_DOMAIN",
        "label": "Watson IoT Platform domain: \u003corgID\u003e.messaging.internetofthings.ibmcloud.com",
        "type": "string",
        "defaultValue": ""
      }
  ],
  "workloads":[{
    "deployment":"$MQMD",
    "deployment_signature":"$MQMDS",
    "torrent":"$MQMT"
  }]
}
EOF
echo -e "Register MQTT broker microservice $VERS"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/IBM/microservices" | jq -r '.')
results "$RES"

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
