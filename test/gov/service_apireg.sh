#!/bin/bash

# $1 - results
# $2 - 

TEST_DIFF_ORG=${TEST_DIFF_ORG:-1}

function results {
  if [ "$(echo "$1" | jq -r '.code')" != "ok" ]
  then
    echo -e "Error: $(echo "$1" | jq -r '.msg')"
    exit 2
  fi
}


echo -e "Registering services"
echo -e "PATTERN setting is $PATTERN"

EXCH_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"
IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"



export HZN_EXCHANGE_URL="http://${EXCH_APP_HOST:-172.17.0.1}:8080/v1"

# Register services via the hzn dev exchange commands
./hzn_dev_services.sh ${EXCH_URL} ${E2EDEV_ADMIN_AUTH}
if [ $? -ne 0 ]
then
    echo -e "hzn service and pattern registration with hzn dev failed."
    exit 1
fi

# Create the Helm package.
./helm_package.sh ${EXCH_URL} ${E2EDEV_ADMIN_AUTH}
if [ $? -ne 0 ]
then
    echo -e "helm package creation failed."
    exit 1
fi

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR
ls *.key &> /dev/null
if [ $? -eq 0 ]
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  hzn key create -l 4096 e2edev@somecomp.com e2edev@gmail.com -d .
  if [ $? -ne 0 ]
  then
    echo -e "hzn key create failed."
    exit 2
  fi
fi

echo -e "Copy public key into anax folder:"
cp $KEY_TEST_DIR/*public.pem /root/.colonus/. &> /dev/null

# test service amd64
read -d '' sdef <<EOF
{
  "label":"Test service",
  "description":"Test service",
  "public":false,
  "url":"https://bluehorizon.network/services/no-such-service",
  "version":"1.0.0",
  "arch":"amd64",
  "sharable":"multiple",
  "matchHardware":{},
  "userInput":[],
  "deployment":"",
  "deploymentSignature":""
}
EOF
echo -e "Register amd64 test service:"
RES=$(echo "$sdef" | curl -sLX POST -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')
results "$RES"

# test service arm64
read -d '' sdef <<EOF
{
  "label":"Test service",
  "description":"Test service",
  "public":false,
  "url":"https://bluehorizon.network/services/no-such-service",
  "version":"1.0.0",
  "arch":"arm64",
  "sharable":"multiple",
  "matchHardware":{},
  "userInput":[],
  "deployment":"",
  "deploymentSignature":""
}
EOF
echo -e "Register arm64 test service:"
RES=$(echo "$sdef" | curl -sLX POST -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')
results "$RES"

# test service arm
read -d '' sdef <<EOF
{
  "label":"Test service",
  "description":"Test service",
  "public":false,
  "url":"https://bluehorizon.network/services/no-such-service",
  "version":"1.0.0",
  "arch":"arm",
  "sharable":"multiple",
  "matchHardware":{},
  "userInput":[],
  "deployment":"",
  "deploymentSignature":""
}
EOF
echo -e "Register arm test service:"
RES=$(echo "$sdef" | curl -sLX POST -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')
results "$RES"

# Helm service
VERS="1.0.0"
echo -e "Register Helm service $VERS:"
hzn exchange service publish -I -u root/root:Horizon-Rul3s -o IBM -f /root/helm/hello/external/horizon/service.definition.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for Helm service."
    exit 2
fi

# cpu service - needed by the hzn dev tests and the location top level service as a 3rd level dependency.
VERS="1.2.2"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"https://bluehorizon.network/service-cpu",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"singleton",
  "matchHardware":{},
  "userInput":[
    {
      "name":"cpu_var1",
      "label":"",
      "type":"string"
    }
  ],
  "deployment":{
    "services":{
      "cpu":{
        "image":"openhorizon/example_ms_x86_cpu:1.2.2"
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register IBM/cpu service $VERS:"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/cpu."
    exit 2
fi

# cpu service - needed by the e2edev@somecomp.com/netspeed
VERS="1.0"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"https://bluehorizon.network/service-cpu",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"singleton",
  "matchHardware":{},
  "userInput":[
    {
      "name":"cpu_var1",
      "label":"",
      "type":"string"
    }
  ],
  "deployment":{
    "services":{
      "cpu":{
        "image":"openhorizon/amd64_cpu:1.2.2"
      }
    }
  },
  "deploymentSignature":""
}
EOF

echo -e "Register e2edev@somecomp.com/cpu service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/cpu."
    exit 2
fi


# A no-op network service used by the netspeed service as a dependency.
VERS="1.5.0"
read -d '' sdef <<EOF
{
  "label":"Network for x86_64",
  "description":"Network service",
  "public":true,
  "url":"https://bluehorizon.network/services/network",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"singleton",
  "matchHardware":{},
  "userInput":[],
  "deployment":"",
  "deploymentSignature":""
}
EOF
echo -e "Register IBM/network service $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/IBM/services" | jq -r '.')
results "$RES"

echo -e "Register e2edev@somecomp.com/network service $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')
results "$RES"


VERS="1.5.0"
read -d '' sdef <<EOF
{
  "label":"Network for x86_64",
  "description":"Network service",
  "public":true,
  "url":"https://bluehorizon.network/services/network2",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"singleton",
  "matchHardware":{},
  "userInput":[],
  "deployment":"",
  "deploymentSignature":""
}
EOF
echo -e "Register IBM/network service $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/IBM/services" | jq -r '.')
results "$RES"

echo -e "Register e2edev@somecomp.com/network service $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic root/root:Horizon-Rul3s" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')
results "$RES"


# GPS service 
VERS="2.0.3"
cat <<EOF >$KEY_TEST_DIR/svc_gps.json
{
  "label":"GPS for x86_64",
  "description":"GPS service",
  "public":true,
  "url":"https://bluehorizon.network/service-gps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"multiple",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[],
  "deployment": {
    "services":{
      "gps":{
        "image":"openhorizon/amd64_gps:2.0.3",
        "privileged":true,
        "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register GPS service $VERS"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH  -o IBM -f $KEY_TEST_DIR/svc_gps.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for GPS."
    exit 2
fi

VERS="2.0.4"
cat <<EOF >$KEY_TEST_DIR/svc_gps2.json
{
  "label":"GPS for x86_64",
  "description":"GPS service",
  "public":true,
  "url":"https://bluehorizon.network/service-gps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"multiple",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[],
  "deployment": {
    "services":{
      "gps":{
        "image":"openhorizon/amd64_gps:2.0.3",
        "privileged":true,
        "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register GPS service $VERS:"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_gps2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for GPS."
    exit 2
fi

# GPS service for the location service that has configurable user inputs,
# the sharable is single instead of singleton for backward compatibility.
VERS="2.0.3"
cat <<EOF >$KEY_TEST_DIR/svc_locgps.json
{
  "label":"GPS for Location for x86_64",
  "description":"GPS service for loc service",
  "public":false,
  "url":"https://bluehorizon.network/services/locgps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[
    {"name":"test","label":"","type":"string","defaultValue":null},
    {"name":"testdefault","label":"","type":"string","defaultValue":"default"}
  ],
  "deployment": {
    "services":{
      "gps":{
        "image":"openhorizon/amd64_gps:2.0.3",
        "privileged":true,
        "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
      }
    }
  },
  "deploymentSignature":""
}
EOF

# make the service public so that it can be used by devices in other orgs
if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    sed -i  's/"public":false/"public":true/g' $KEY_TEST_DIR/svc_locgps.json
fi
echo -e "Register GPS Loc service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_locgps.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for LocGPS."
    exit 2
fi

VERS="2.0.4"
cat <<EOF >$KEY_TEST_DIR/svc_locgps2.json
{
  "label":"GPS for Location for x86_64",
  "description":"GPS service for loc service",
  "public":false,
  "url":"https://bluehorizon.network/services/locgps",
  "version":"$VERS",
  "arch":"amd64",
  "sharable":"single",
  "requiredServices":[
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"IBM"},
    {"url":"https://bluehorizon.network/services/network2","version":"1.0.0","arch":"amd64","org":"IBM"}
  ],
  "matchHardware":{
    "usbDeviceIds":"1546:01a7",
    "devFiles":"/dev/ttyUSB*,/dev/ttyACM*"
  },
  "userInput":[
    {"name":"test","label":"","type":"string","defaultValue":null},
    {"name":"testdefault","label":"","type":"string","defaultValue":"default"}
  ],
  "deployment": {
    "services":{
      "gps":{
        "image":"openhorizon/amd64_gps:2.0.3",
        "privileged":true,
        "devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]
      }
    }
  },
  "deploymentSignature":""
}
EOF
# make the service public so that it can be used by devices in other orgs
if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    sed -i  's/"public":false/"public":true/g' $KEY_TEST_DIR/svc_locgps2.json
fi
echo -e "Register GPS Loc service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_locgps2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for LocGPS."
    exit 2
fi


# ============================= Top Level services here =============================

# The netspeed service:

# deployment configuration
# service definition
# register version 2.3.0 for execution purposes
VERS="2.3.0"
cat <<EOF >$KEY_TEST_DIR/svc_netspeed.json
{
  "label":"Netspeed for x86_64",
  "description":"Netspeed service",
  "sharable":"multiple",
  "public":true,
  "url":"https://bluehorizon.network/services/netspeed",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[
    {"url":"https://bluehorizon.network/services/network","version":"1.0.0","arch":"amd64","org":"IBM"},
    {"url":"https://bluehorizon.network/services/network2","version":"1.0.0","arch":"amd64","org":"IBM"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"IBM"}
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
  "deployment":{
    "services":{
      "netspeed5":{
        "image":"openhorizon/amd64_netspeed:2.5.0",
        "environment":["USE_NEW_STAGING_URL=false","DEPL_ENV=staging","SKIP_NUM_REPEAT_LOC_READINGS=0"]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register IBM/netspeed service $VERS:"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_netspeed.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/netspeed."
    exit 2
fi

cat <<EOF >$KEY_TEST_DIR/svc_netspeed.json
{
  "label":"Netspeed for x86_64",
  "description":"Netspeed service",
  "sharable":"multiple",
  "public":true,
  "url":"https://bluehorizon.network/services/netspeed",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[
    {"url":"https://bluehorizon.network/services/network","version":"1.0.0","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/services/network2","version":"1.0.0","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"IBM"}
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
  "deployment":{
    "services":{
      "netspeed5":{
        "image":"openhorizon/amd64_netspeed:2.5.0",
        "environment":["USE_NEW_STAGING_URL=false","DEPL_ENV=staging","SKIP_NUM_REPEAT_LOC_READINGS=0"]
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register e2edev@somecomp.com/netspeed service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_netspeed.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/netspeed."
    exit 2
fi

# The GPSTest service:
VERS="1.0.0"
cat <<EOF >$KEY_TEST_DIR/svc_gpstest.json
{
  "label":"GPSTest for x86_64",
  "description":"GPS Test service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/gpstest",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[
    {"url":"https://bluehorizon.network/service-gps","version":"1.0.0","arch":"amd64","org":"IBM"}
  ],
  "userInput":[],
  "deployment": {
    "services": {
      "gpstest": {
        "environment":["REPORTING_INTERVAL=1", "INTERVAL_SLEEP=5", "HEARTBEAT_TO_MQTT=true"],
        "image": "openhorizon/amd64_gps-test:latest"
      }
    }
  },
  "deploymentSignature":""
}
EOF
# make the service public so that it can be used by devices in other orgs
if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    sed -i  's/"public":false/"public":true/g' $KEY_TEST_DIR/svc_gpstest.json
fi

echo -e "Register GPSTest service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_gpstest.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for GPSTest."
    exit 2
fi


# Location definition
VERS="2.0.6"
cat <<EOF >$KEY_TEST_DIR/svc_location.json
{
  "label":"Location for x86_64",
  "description":"Location service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/location",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[
    {"url":"https://bluehorizon.network/services/locgps","version":"2.0.3","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"IBM"}
  ],
  "userInput":[],
  "deployment": {
    "services":{
      "location":{
        "environment":["USE_NEW_STAGING_URL=false","DEPL_ENV=staging"],
        "image":"openhorizon/amd64_location:2.0.6"
      }
    }
  },
  "deploymentSignature":""
}
EOF
# make the service public so that it can be used by devices in other orgs
if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    sed -i  's/"public":false/"public":true/g' $KEY_TEST_DIR/svc_location.json
fi
echo -e "Register service based location $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_location.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for Location."
    exit 2
fi

VERS="2.0.7"
cat <<EOF >$KEY_TEST_DIR/svc_location2.json
{
  "label":"Location for x86_64",
  "description":"Location service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/location",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[
    {"url":"https://bluehorizon.network/services/locgps","version":"2.0.4","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"IBM"}
  ],
  "userInput":[],
  "deployment": {
    "services":{
      "location":{
        "environment":["USE_NEW_STAGING_URL=false","DEPL_ENV=staging"],
        "image":"openhorizon/amd64_location:2.0.6"
      }
    }
  },
  "deploymentSignature":""
}
EOF
# make the service public so that it can be used by devices in other orgs
if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    sed -i  's/"public":false/"public":true/g' $KEY_TEST_DIR/svc_location2.json
fi
echo -e "Register service based location $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_location2.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for Location."
    exit 2
fi

# The weather service
VERS="1.5.0"
cat <<EOF >$KEY_TEST_DIR/svc_weather.json
{
  "label":"Weather for x86_64",
  "description":"PWS service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/weather",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[],
  "userInput":[
    {"name":"HZN_WUGNAME","label":"","type":"string"},
    {"name":"HZN_PWS_MODEL","label":"","type":"string"},
    {"name":"MTN_PWS_MODEL","label":"","type":"string"},
    {"name":"HZN_PWS_ST_TYPE","label":"","type":"string"},
    {"name":"MTN_PWS_ST_TYPE","label":"","type":"string"}
  ],
  "deployment": {
    "services": {
      "eaweather": {
        "environment":["DEPL_ENV=staging", "USE_NEW_STAGING_URL=false", "MOCK=true"],
        "image": "openhorizon/amd64_eaweather:1.8"
      }
    }
  },
  "deploymentSignature": ""
}
EOF
# make the service public so that it can be used by devices in other orgs
if [[ $TEST_DIFF_ORG -eq 1 ]]; then
    sed -i  's/"public":false/"public":true/g' $KEY_TEST_DIR/svc_weather.json
fi
echo -e "Register service based PWS $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_weather.json -k $KEY_TEST_DIR/*private.key
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for PWS."
    exit 2
fi

echo -e "Listing services:"
hzn exchange service list -o e2edev@somecomp.com
hzn exchange service list -o IBM



# ======================= Patterns that use top level services ======================
# sns pattern
VERS="2.3.0"
read -d '' pdef <<EOF
{
  "label": "Netspeed",
  "description": "a netspeed service based pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"https://bluehorizon.network/services/netspeed",
      "serviceOrgid":"IBM",
      "serviceArch":"amd64",
      "serviceVersions":[
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
echo -e "Register sns (service based netspeed) pattern $VERS:"
RES=$(echo "$pdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sns" | jq -r '.')
results "$RES"

# sgps test pattern
VERS="1.0.0"
read -d '' sdef <<EOF
{
  "label": "GPS Test",
  "description": "a GPS Test pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"https://bluehorizon.network/services/gpstest",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
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
echo -e "Register gps service pattern $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sgps" | jq -r '.')
results "$RES"

# shelm test pattern
VERS="1.0.0"
read -d '' sdef <<EOF
{
  "label": "Helm Test",
  "description": "a Helm Test pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"http://my.company.com/services/helm-service",
      "serviceOrgid":"IBM",
      "serviceArch":"amd64",
      "serviceVersions":[
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
echo -e "Register Helm service pattern $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/shelm" | jq -r '.')
results "$RES"

# susehello test pattern
VERS="1.0.0"
read -d '' sdef <<EOF
{
  "label": "UseHello",
  "description": "Multi-dependency Service pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"my.company.com.services.usehello2",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
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
echo -e "Register usehello service pattern $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/susehello" | jq -r '.')
results "$RES"

#
# sloc pattern
# This pattern tests a number of things:
# 1. That it is possible for an ag-service to depend on an ag-less (sharable=singleton) service.
# 2. That the higher version of 2.0.7 is chosen when the ag-service is executed.
# 3. That data verification, metering, and nodehealth work correctly.
#
# The verify_sloc.sh script verifies that this service is running correctly.
#

LOCVERS1="2.0.6"
LOCVERS2="2.0.7"
read -d '' sdef <<EOF
{
  "label": "Location Test",
  "description": "service based location pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"https://bluehorizon.network/services/location",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
        {
          "version":"$LOCVERS1",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{
            "priority_value": 3,
            "retries": 2,
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
            "retries": 2,
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
      "serviceUrl":"https://bluehorizon.network/services/locgps",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "agreementLess": true,
      "serviceVersions":[
        {
          "version":"2.0.4",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{},
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {}
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ]
}
EOF
echo -e "Register location service pattern $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sloc" | jq -r '.')
results "$RES"

# weather pattern
VERS="1.5.0"
read -d '' sdef <<EOF
{
  "label": "Weather",
  "description": "a weather pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"https://bluehorizon.network/services/weather",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
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
echo -e "Register weather service pattern $VERS:"
RES=$(echo "$sdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/spws" | jq -r '.')
results "$RES"

# the sall pattern
PWSVERS="1.5.0"
NSVERS="2.3.0"
LOCVERS1="2.0.6"
LOCVERS2="2.0.7"
GPSVERS="1.0.0"
UHSVERS="1.0.0"
read -d '' msdef <<EOF
{
  "label": "All",
  "description": "a pattern for all service based top level services",
  "public": true,
  "services": [
    {
      "serviceUrl":"https://bluehorizon.network/services/weather",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
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
      "serviceUrl":"https://bluehorizon.network/services/netspeed",
      "serviceOrgid":"IBM",
      "serviceArch":"amd64",
      "serviceVersions":[
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
            "retry_durations": 1800
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
      "serviceUrl":"https://bluehorizon.network/services/netspeed",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
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
            "retry_durations": 1800
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
      "serviceUrl":"https://bluehorizon.network/services/location",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
        {
          "version":"$LOCVERS1",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{
            "priority_value": 3,
            "retries": 2,
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
            "retries": 2,
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
      "serviceUrl":"https://bluehorizon.network/services/locgps",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "agreementLess": true,
      "serviceVersions":[
        {
          "version":"2.0.4",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{},
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {}
    },
    {
      "serviceUrl":"my.company.com.services.usehello2",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
        {
          "version":"1.0.0",
          "deployment_overrides":"",
          "deployment_overrides_signature":"",
          "priority":{},
          "upgradePolicy": {}
        }
      ],
      "dataVerification": {},
      "nodeHealth": {}
    },
    {
      "serviceUrl":"https://bluehorizon.network/services/gpstest",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
        {
          "version":"$GPSVERS",
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

if [[ $TEST_DIFF_ORG -eq 0 ]]; then
  msdef=$(echo $msdef |jq 'del(.services[] | select(.serviceUrl == "https://bluehorizon.network/services/netspeed") | select(.serviceOrgid == "e2edev@somecomp.com"))')
fi

echo -e "Register service based all pattern:"
RES=$(echo "$msdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sall" | jq -r '.')
results "$RES"


# ======================= Business Policies that use top level services ======================
read -d '' bpnsdef <<EOF
{
  "label": "business policy for netspeed",
  "description": "for netspeed",
  "service": {
    "name": "https://bluehorizon.network/services/netspeed",
    "org": "IBM",
    "arch": "amd64",
    "serviceVersions": [
      { 
        "version": "2.3.0",
        "priority":{
          "priority_value": 3,
          "retries": 1,
          "retry_durations": 1800,
          "verified_durations": 45
       }
      },
      { 
        "version": "2.3.0",
        "priority":{
          "priority_value": 2,
          "retries": 1,
          "retry_durations": 3600
        }
      }
    ]
  },
  "properties": [
      {
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "number",
          "value": "12"
      },
      {
          "name": "foo",
          "value": "bar"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy for netspeed:"
RES=$(echo "$bpnsdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_netspeed" | jq -r '.')
results "$RES"

read -d '' bpgpstestdef <<EOF
{
  "label": "business policy for gpstest",
  "description": "for gpstest",
  "service": {
    "name": "https://bluehorizon.network/services/gpstest",
    "org": "e2edev@somecomp.com",
    "arch": "amd64",
    "serviceVersions": [
      { 
        "version": "1.0.0"
      }
    ]
  },
  "properties": [
      {
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "number",
          "value": "12"
      },
      {
          "name": "foo",
          "value": "bar"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy for gpstest:"
RES=$(echo "$bpgpstestdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_gpstest" | jq -r '.')
results "$RES"

read -d '' bplocdef <<EOF
{
  "label": "business policy for location",
  "description": "for location",
  "service": {
    "name": "https://bluehorizon.network/services/location",
    "org": "e2edev@somecomp.com",
    "arch": "amd64",
    "serviceVersions": [
        {
          "version":"2.0.6",
          "priority":{
            "priority_value": 3,
            "retries": 2,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        },
        {
          "version":"2.0.7",
          "priority":{
            "priority_value": 2,
            "retries": 2,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        }
     ]
  },
  "properties": [
      {
          "name": "iame2edev2",
          "value": "true"
      },
      {
          "name": "number",
          "value": "12"
      },
      {
          "name": "foo",
          "value": "bar"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy for location:"
RES=$(echo "$bplocdef" | curl -sLX POST --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_location" | jq -r '.')
results "$RES"



# ======================= Service Policies that use top level services ======================
read -d '' nspoldef <<EOF
{
  "properties": [
      {
          "name": "iame2edev_service",
          "value": "true"
      },
      {
          "name": "service_var1",
          "value": "this is netspeed service"
      }
  ],
  "constraints": [
    "group == bluenode"
  ]
}
EOF
echo -e "Register service policy for netspeed:"
RES=$(echo "$nspoldef" | curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-netspeed_2.3.0_amd64/policy" | jq -r '.')
results "$RES"

read -d '' gpstestpoldef <<EOF
{
  "properties": [
      {
          "name": "iame2edev_service",
          "value": "true"
      },
      {
          "name": "service_var2",
          "value": "this is gpstest service"
      }
  ],
  "constraints": [
    "group == bluenode"
  ]
}
EOF
echo -e "Register service policy for gpstest:"
RES=$(echo "$gpstestpoldef" | curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-gpstest_1.0.0_amd64/policy" | jq -r '.')
results "$RES"

read -d '' locpoldef <<EOF
{
  "properties": [
      {
          "name": "iame2edev_service",
          "value": "true"
      },
      {
          "name": "service_var3",
          "value": "this is location service"
      }
  ],
  "constraints": [
      "group == bluenode"
  ]
}
EOF
echo -e "Register service policy for location:"
RES=$(echo "$locpoldef" | curl -sLX PUT --header 'Content-Type: application/json' --header 'Accept: application/json' -H "Authorization:Basic $E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-location_2.0.6_amd64/policy" | jq -r '.')
results "$RES"

unset HZN_EXCHANGE_URL

