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

if [ ${CERT_LOC} -eq "1" ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=""
fi


echo -e "Registering services"
echo -e "PATTERN setting is $PATTERN"

EXCH_URL="${EXCH_APP_HOST}"
IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"



export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

# Register services via the hzn dev exchange commands
./hzn_dev_services.sh ${EXCH_URL} ${E2EDEV_ADMIN_AUTH} 0
if [ $? -ne 0 ]
then
    echo -e "hzn service and pattern registration with hzn dev failed."
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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR -H "Content-Type: application/json" -H "Accept: application/json" -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')
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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR -H "Content-Type: application/json" -H "Accept: application/json" -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')

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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR -H "Content-Type: application/json" -H "Accept: application/json" -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')

results "$RES"

# Helm service
# VERS="1.0.0"
# echo -e "Register Helm service $VERS:"
# hzn exchange service publish -I -u root/root:${EXCH_ROOTPW} -o IBM -f /root/helm/hello/external/horizon/service.definition.json -k $KEY_TEST_DIR/*private.key
# if [ $? -ne 0 ]
# then
#     echo -e "hzn exchange service publish failed for Helm service."
#     exit 2
# fi

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
        "image":"openhorizon/example_ms_x86_cpu:1.2.2",
        "binds":["/tmp:/hosttmp"]
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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" --data @- "${EXCH_URL}/orgs/IBM/services" | jq -r '.')

results "$RES"

echo -e "Register e2edev@somecomp.com/network service $VERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')

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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" --data @- "${EXCH_URL}/orgs/IBM/services" | jq -r '.')

results "$RES"

echo -e "Register e2edev@somecomp.com/network service $VERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "root/root:${EXCH_ROOTPW}" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')

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
  "userInput":[
    {"name":"HZN_LAT","label":"","type":"string","defaultValue":"41.921766"},
    {"name":"HZN_LON","label":"","type":"string","defaultValue":"-73.894224"},
    {"name":"HZN_LOCATION_ACCURACY_KM","label":"","type":"string","defaultValue":"0.5"},
    {"name":"HZN_USE_GPS","label":"","type":"string","defaultValue":"false"}
  ],
  "deployment":{
    "services":{
      "gps":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
  "userInput":[
    {"name":"HZN_LAT","label":"","type":"float"},
    {"name":"HZN_LON","label":"","type":"float"},
    {"name":"HZN_LOCATION_ACCURACY_KM","label":"","type":"float"},
    {"name":"HZN_USE_GPS","label":"","type":"bool"}
  ],
  "deployment":{
    "services":{
      "gps2":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
    {"name":"test","label":"","type":"","defaultValue":null},
    {"name":"testdefault","label":"","type":"string","defaultValue":"default"},
    {"name":"HZN_LAT","label":"","type":"float"},
    {"name":"HZN_LON","label":"","type":"float"},
    {"name":"HZN_LOCATION_ACCURACY_KM","label":"","type":"float"},
    {"name":"HZN_USE_GPS","label":"","type":"bool"}
  ],
  "deployment":{
    "services":{
      "locgps":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
    {"name":"test","label":"","type":"","defaultValue":null},
    {"name":"testdefault","label":"","type":"string","defaultValue":"default"},
    {"name":"HZN_LAT","label":"","type":"float"},
    {"name":"HZN_LON","label":"","type":"float"},
    {"name":"HZN_LOCATION_ACCURACY_KM","label":"","type":"float"},
    {"name":"HZN_USE_GPS","label":"","type":"bool"}
  ],
  "deployment":{
    "services":{
      "locgps2":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
    {"url":"https://bluehorizon.network/services/network","versionRange":"1.0.0","arch":"amd64","org":"IBM"},
    {"url":"https://bluehorizon.network/services/network2","versionRange":"1.0.0","arch":"amd64","org":"IBM"},
    {"url":"https://bluehorizon.network/service-cpu","versionRange":"1.0.0","arch":"amd64","org":"IBM"}
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
      "netspeed":{
        "image":"openhorizon/amd64_cpu:1.2.2"
      }
    }
  },
  "deploymentSignature":"",
  "clusterDeployment": {
    "operatorYamlArchive": "/root/input_files/k8s_deploy/k8s_deployment_files_1.tar.gz"
  },
  "clusterDeploymentSignature": ""
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
    {"url":"https://bluehorizon.network/services/network","versionRange":"1.0.0","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/services/network2","version":"1.0.0","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","versionRange":"1.0.0","arch":"amd64","org":"IBM"}
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
      "netspeed":{
        "image":"openhorizon/amd64_cpu:1.2.2"
      }
    }
  },
  "deploymentSignature":"",
  "clusterDeployment": {
    "operatorYamlArchive": "/root/input_files/k8s_deploy/k8s_deployment_files_1.tar.gz"
  },
  "clusterDeploymentSignature": ""
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
  "deployment":{
    "services":{
      "gpstest":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
    {"url":"https://bluehorizon.network/services/locgps","versionRange":"2.0.3","arch":"amd64","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","versionRange":"1.0.0","arch":"amd64","org":"IBM"}
  ],
  "userInput":[],
  "deployment":{
    "services":{
      "location":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
  "deployment":{
    "services":{
      "location2":{
        "image":"openhorizon/amd64_cpu:1.2.2"
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
    {"name":"MTN_PWS_ST_TYPE","label":"","type":"string"},
    {"name":"HZN_LAT","label":"","type":"float"},
    {"name":"HZN_LON","label":"","type":"float"},
    {"name":"HZN_LOCATION_ACCURACY_KM","label":"","type":"float"},
    {"name":"HZN_USE_GPS","label":"","type":"bool"}
],
"deployment":{
  "services":{
    "weather":{
      "image":"openhorizon/amd64_cpu:1.2.2"
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

VERS="1.0.0"
cat <<EOF >$KEY_TEST_DIR/svc_k8s1.json
{
  "label":"Cluster service test for x86_64",
  "description":"Cluster Service Test service1",
  "public":true,
  "sharable":"multiple",
  "url":"k8s-service1",
  "version":"$VERS",
  "arch":"amd64",
  "requiredServices":[
  ],
  "clusterDeployment": {
    "operatorYamlArchive": "/root/input_files/k8s_deploy/k8s_deployment_files_1.tar.gz"
   },
  "clusterDeploymentSignature": ""
}
EOF

echo -e "Register k8s-service1 $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_k8s1.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for k8s-service1."
    exit 2
fi


echo -e "Listing services:"
hzn exchange service list -o e2edev@somecomp.com
hzn exchange service list -o IBM



# ======================= Patterns that use top level services ======================
# sns pattern
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI=120
  CAS=30
else
  MHI=600
  CAS=600
fi

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
        "missing_heartbeat_interval": $MHI,
        "check_agreement_status": $CAS
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ],
  "userInput": [
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/services/netspeed",
      "serviceArch": "",
      "serviceVersionRange": "2.2.0",
      "inputs": [
        {
          "name": "var1",
          "value": "bString"
        },
        {
          "name": "var2",
          "value": 10
        },
        {
          "name": "var3",
          "value": 10.22
        },
        {
          "name": "var4",
          "value": ["abcd", "1234"]
        },
        {
          "name": "var5",
          "value": "override2"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "ibm_var1"
        }
      ]
    }
  ]
}
EOF
echo -e "Register sns (service based netspeed) pattern $VERS:"

  RES=$(echo "$pdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sns" | jq -r '.')

results "$RES"

if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI=90
  CAS=60
else
  MHI=600
  CAS=600
fi


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
        "missing_heartbeat_interval": $MHI,
        "check_agreement_status": $CAS
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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sgps" | jq -r '.')

results "$RES"

# shelm test pattern
# if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
#   MHI=90
#   CAS=60
# else
#   MHI=600
#   CAS=600
# fi

# VERS="1.0.0"
# read -d '' sdef <<EOF
# {
#   "label": "Helm Test",
#   "description": "a Helm Test pattern",
#   "public": true,
#   "services": [
#     {
#       "serviceUrl":"http://my.company.com/services/helm-service",
#       "serviceOrgid":"IBM",
#       "serviceArch":"amd64",
#       "serviceVersions":[
#         {
#           "version":"$VERS",
#           "deployment_overrides":"",
#           "deployment_overrides_signature":"",
#           "priority":{},
#           "upgradePolicy": {}
#         }
#       ],
#       "dataVerification": {},
#       "nodeHealth": {
#         "missing_heartbeat_interval": $MHI,
#         "check_agreement_status": $CAS
#       }
#     }
#   ],
#   "agreementProtocols": [
#     {
#       "name": "Basic"
#     }
#   ]
# }
# EOF
# echo -e "Register Helm service pattern $VERS:"

#   RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/shelm" | jq -r '.')

# results "$RES"

# susehello test pattern
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI=90
  CAS=60
else
  MHI=600
  CAS=600
fi

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
        "missing_heartbeat_interval": $MHI,
        "check_agreement_status": $CAS
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

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/susehello" | jq -r '.')

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
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI=240
  CAS=60
else
  MHI=600
  CAS=600
fi
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
        "missing_heartbeat_interval": $MHI,
        "check_agreement_status": $CAS
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
  ],
  "userInput": [
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/locgps",
      "serviceArch": "",
      "serviceVersionRange": "2.0.3",
      "inputs": [
        {
          "name": "test",
          "value": "testValue"
        },
        {
          "name": "extra",
          "value": "extraValue"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "ibmvar1"
        }
      ]
    }
  ]
}
EOF
echo -e "Register location service pattern $VERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sloc" | jq -r '.')

results "$RES"

# weather pattern
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI=90
  CAS=60
else
  MHI=600
  CAS=600
fi
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
        "missing_heartbeat_interval": $MHI,
        "check_agreement_status": $CAS
      }
    }
  ],
  "agreementProtocols": [
    {
      "name": "Basic"
    }
  ],
  "userInput": [
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/weather",
      "serviceArch": "",
      "serviceVersionRange": "1.5.0",
      "inputs": [
        {
          "name": "HZN_WUGNAME",
          "value": "e2edev mocked pws"
        },
        {
          "name": "HZN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "MTN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "HZN_PWS_ST_TYPE",
          "value": "WS23xx"
        },
        {
          "name": "MTN_PWS_ST_TYPE",
          "value": "WS23xx"
        }
      ]
    }
  ]
}
EOF
echo -e "Register weather service pattern $VERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/spws" | jq -r '.')

results "$RES"

# k8s pattern
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI=90
  CAS=60
else
  MHI=600
  CAS=600
fi
K8SVERS="1.0.0"
read -d '' sdef <<EOF
{
  "label": "K8s",
  "description": "a k8s pattern",
  "public": true,
  "services": [
    {
      "serviceUrl":"k8s-service1",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
        {
          "version":"$K8SVERS",
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
echo -e "Register k8s service pattern $VERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sk8s" | jq -r '.')

results "$RES"


# the sall pattern
PWSVERS="1.5.0"
NSVERS="2.3.0"
LOCVERS1="2.0.6"
LOCVERS2="2.0.7"
GPSVERS="1.0.0"
UHSVERS="1.0.0"
K8SVERS="1.0.0"
if [ "${EXCH_APP_HOST}" = "http://exchange-api:8080/v1" ]; then
  MHI_240=240
  MHI_180=180
  MHI_120=120
  MHI_90=90
  CAS_60=60
  CAS_45=45
  CAS_30=30
else
  MHI_240=600
  MHI_180=600
  MHI_120=600
  MHI_90=600
  CAS_60=600
  CAS_45=600
  CAS_30=600
fi
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
        "missing_heartbeat_interval": $MHI_180,
        "check_agreement_status": $CAS_45
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
        "missing_heartbeat_interval": $MHI_120,
        "check_agreement_status": $CAS_30
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
        "missing_heartbeat_interval": $MHI_120,
        "check_agreement_status": $CAS_30
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
        "missing_heartbeat_interval": $MHI_240,
        "check_agreement_status": $CAS_60
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
        "missing_heartbeat_interval": $MHI_90,
        "check_agreement_status": $CAS_60
      }
    },
    {
      "serviceUrl":"k8s-service1",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"amd64",
      "serviceVersions":[
        {
          "version":"$K8SVERS",
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
  ],
  "userInput": [
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/netspeed",
      "serviceArch": "",
      "serviceVersionRange": "2.2.0",
      "inputs": [
        {
          "name": "var3",
          "value": 10.22
        },
        {
          "name": "var4",
          "value": ["abcd", "1234"]
        },
        {
          "name": "var5",
          "value": "override2"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/services/netspeed",
      "serviceArch": "",
      "serviceVersionRange": "2.2.0",
      "inputs": [
        {
          "name": "var3",
          "value": 11.22
        },
        {
          "name": "var4",
          "value": ["abc", "123"]
        },
        {
          "name": "var5",
          "value": "override1"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "e2edev_var1"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "ibm_var1"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "e2edev_var1"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/locgps",
      "serviceArch": "",
      "serviceVersionRange": "2.0.3",
      "inputs": [
        {
          "name": "test",
          "value": "testValue"
        },
        {
          "name": "extra",
          "value": "extraValue"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "ibmvar1"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/weather",
      "serviceArch": "",
      "serviceVersionRange": "1.5.0",
      "inputs": [
        {
          "name": "HZN_WUGNAME",
          "value": "e2edev mocked pws"
        },
        {
          "name": "HZN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "MTN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "HZN_PWS_ST_TYPE",
          "value": "WS23xx"
        },
        {
          "name": "MTN_PWS_ST_TYPE",
          "value": "WS23xx"
        }
      ]
    }
  ]
}
EOF

if [[ $TEST_DIFF_ORG -eq 0 ]]; then
  msdef=$(echo $msdef |jq 'del(.services[] | select(.serviceUrl == "https://bluehorizon.network/services/netspeed") | select(.serviceOrgid == "e2edev@somecomp.com"))')
fi

echo -e "Register service based all pattern:"

  RES=$(echo "$msdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sall" | jq -r '.')

results "$RES"


# ======================= Business Policies that use top level services ======================
read -d '' bpnsdef <<EOF
{
  "label": "business policy for netspeed",
  "description": "for netspeed",
  "service": {
    "name": "https://bluehorizon.network/services/netspeed",
    "org": "e2edev@somecomp.com",
    "arch": "*",
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
    ],
    "nodeHealth": {
      "missing_heartbeat_interval": 0,
      "check_agreement_status": 0
    }
  },
  "properties": [
      {
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "NONS",
          "value": false
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
    "purpose==network-testing"
  ],
  "userInput": [
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/netspeed",
      "serviceArch": "",
      "serviceVersionRange": "2.2.0",
      "inputs": [
        {
          "name": "var1",
          "value": "bp_string"
        },
        {
          "name": "var2",
          "value": 10
        },
        {
          "name": "var3",
          "value": 10.22
        },
        {
          "name": "var4",
          "value": ["bp_abcd", "bp_1234"]
        },
        {
          "name": "var5",
          "value": "bp_override2"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "bp_ibm_var1"
        }
      ]
    },
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "bp_e2edev_var1"
        }
      ]
    }
  ]
}
EOF
echo -e "Register business policy for netspeed:"

  RES=$(echo "$bpnsdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_netspeed" | jq -r '.')

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
          "name": "NOGPS",
          "value": false
      },
      {
          "name": "number",
          "value": 24
      },
      {
          "name": "gpsvar",
          "value": "gpsval"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy for gpstest:"

  RES=$(echo "$bpgpstestdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_gpstest" | jq -r '.')

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
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "NOLOC",
          "value": false
      },
      {
          "name": "number",
          "value": "36"
      },
      {
          "name": "locvar",
          "value": "location value"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ],
  "userInput": [
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/locgps",
      "serviceArch": "",
      "serviceVersionRange": "2.0.3",
      "inputs": [
        {
          "name": "test",
          "value": "testValue"
        },
        {
          "name": "extra",
          "value": "extraValue"
        }
      ]
    },
    {
      "serviceOrgid": "IBM",
      "serviceUrl": "https://bluehorizon.network/service-cpu",
      "serviceArch": "",
      "serviceVersionRange": "1.0.0",
      "inputs": [
        {
          "name": "cpu_var1",
          "value": "ibmvar1"
        }
      ]
    }
  ]
}
EOF
echo -e "Register business policy for location:"

  RES=$(echo "$bplocdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_location" | jq -r '.')

results "$RES"

read -d '' bppwsdef <<EOF
{
  "label": "business policy for personal weather station",
  "description": "for pws",
  "service": {
    "name": "https://bluehorizon.network/services/weather",
    "org": "e2edev@somecomp.com",
    "arch": "amd64",
    "serviceVersions": [
        {
          "version":"1.5.0",
          "priority":{
            "priority_value": 3,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        },
        {
          "version":"1.5.0",
          "priority":{
            "priority_value": 2,
            "retries": 1,
            "retry_durations": 3600,
            "verified_durations": 52
          },
          "upgradePolicy": {}
        }
     ]
  },
  "properties": [
      {
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "NOPWS",
          "value": false
      },
      {
          "name": "number",
          "value": 48
      },
      {
          "name": "pwsvar",
          "value": "pws value"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ],
  "userInput": [
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/weather",
      "serviceArch": "",
      "serviceVersionRange": "1.5.0",
      "inputs": [
        {
          "name": "HZN_WUGNAME",
          "value": "e2edev mocked pws"
        },
        {
          "name": "HZN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "MTN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "HZN_PWS_ST_TYPE",
          "value": "WS23xx"
        },
        {
          "name": "MTN_PWS_ST_TYPE",
          "value": "WS23xx"
        }
      ]
    }
  ]
}
EOF
echo -e "Register business policy for pws:"

  RES=$(echo "$bppwsdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_pws" | jq -r '.')

results "$RES"

read -d '' bphellodef <<EOF
{
  "label": "business policy for usehello",
  "description": "for usehello",
  "service": {
    "name": "my.company.com.services.usehello2",
    "org": "e2edev@somecomp.com",
    "arch": "amd64",
    "serviceVersions": [
        {
          "version":"1.0.0",
          "priority":{},
          "upgradePolicy": {}
        }
     ]
  },
  "properties": [
      {
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "NOHELLO",
          "value": false
      },
      {
          "name": "number",
          "value": 60
      },
      {
          "name": "hellovar",
          "value": "hello value"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy for usehelllo:"

  RES=$(echo "$bphellodef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_usehello" | jq -r '.')

results "$RES"

read -d '' bpk8ssvc1def <<EOF
{
  "label": "business policy for k8s-service1",
  "description": "for gpstest",
  "service": {
    "name": "k8s-service1",
    "org": "e2edev@somecomp.com",
    "arch": "amd64",
    "serviceVersions": [
        {
          "version":"$K8SVERS",
          "priority":{},
          "upgradePolicy": {}
        }
     ]
  },
  "properties": [
      {
          "name": "iame2edev",
          "value": "true"
      },
      {
          "name": "NOK8S",
          "value": false
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy for gpstest:"

  RES=$(echo "$bpk8ssvc1def" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_k8s" | jq -r '.')

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

  RES=$(echo "$nspoldef" | curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-netspeed_2.3.0_amd64/policy" | jq -r '.')

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

  RES=$(echo "$gpstestpoldef" | curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-gpstest_1.0.0_amd64/policy" | jq -r '.')

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

  RES=$(echo "$locpoldef" | curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-location_2.0.6_amd64/policy" | jq -r '.')

results "$RES"

unset HZN_EXCHANGE_URL
