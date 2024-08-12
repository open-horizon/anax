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

# check if the hub is all-in-1 management hub or not
if [[ ${EXCH_APP_HOST} == *"://exchange-api:"* ]]; then
  export REMOTE_HUB=0
else
  export REMOTE_HUB=1
fi

echo -e "Registering services"
echo -e "PATTERN setting is $PATTERN"

EXCH_URL="${EXCH_APP_HOST}"
IBM_ADMIN_AUTH="IBM/ibmadmin:ibmadminpw"
E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

export ARCH=${ARCH}
export CPU_IMAGE_NAME="${DOCKER_CPU_INAME}"
export CPU_IMAGE_TAG="${DOCKER_CPU_TAG}"

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

# test service ppc64le
read -d '' sdef <<EOF
{
  "label":"Test service",
  "description":"Test service",
  "public":false,
  "url":"https://bluehorizon.network/services/no-such-service",
  "version":"1.0.0",
  "arch":"ppc64le",
  "sharable":"multiple",
  "matchHardware":{},
  "userInput":[],
  "deployment":"",
  "deploymentSignature":""
}
EOF
echo -e "Register ppc64le test service:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR -H "Content-Type: application/json" -H "Accept: application/json" -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services" | jq -r '.')

# Helm service
# VERS="1.0.0"
# echo -e "Register Helm service $VERS:"
# hzn exchange service publish -I -u root/root:${EXCH_ROOTPW} -o IBM -f /root/helm/hello/external/horizon/service.definition.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
# if [ $? -ne 0 ]
# then
#     echo -e "hzn exchange service publish failed for Helm service."
#     exit 2
# fi

if [ "${NOVAULT}" != "1" ]; then
  CPU_FILE_IBM="/root/service_defs/IBM/service-cpu_1.2.2_secrets.json"
  CPU_FILE_E2EDEV="/root/service_defs/e2edev@somecomp.com/service-cpu_1.0_secrets.json"

else
  CPU_FILE_IBM="/root/service_defs/IBM/service-cpu_1.2.2.json"
  CPU_FILE_E2EDEV="/root/service_defs/e2edev@somecomp.com/service-cpu_1.0.json"

fi

# IBM cpu service - needed by the hzn dev tests, netspeed, and the location top level service as a 3rd level dependency.
export VERS="1.2.2"

cat ${CPU_FILE_IBM} | envsubst > $KEY_TEST_DIR/svc_cpu.json

echo -e "Register IBM/cpu service $VERS:"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/cpu."
    exit 2
fi

# e2edev@somecomp.com cpu service - needed by the e2edev@somecomp.com/netspeed
export VERS="1.0"

cat ${CPU_FILE_E2EDEV} | envsubst > $KEY_TEST_DIR/svc_cpu.json

echo -e "Register e2edev@somecomp.com/cpu service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/cpu."
    exit 2
fi

# A no-op network service used by the netspeed service as a dependency.
VERS="1.5.0"
read -d '' sdef <<EOF
{
  "label":"Network for ${ARCH}",
  "description":"Network service",
  "public":true,
  "url":"https://bluehorizon.network/services/network",
  "version":"$VERS",
  "arch":"${ARCH}",
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
  "label":"Network for ${ARCH}",
  "description":"Network service",
  "public":true,
  "url":"https://bluehorizon.network/services/network2",
  "version":"$VERS",
  "arch":"${ARCH}",
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
  "label":"GPS for ${ARCH}",
  "description":"GPS service",
  "public":true,
  "url":"https://bluehorizon.network/service-gps",
  "version":"$VERS",
  "arch":"${ARCH}",
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
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register GPS service $VERS"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH  -o IBM -f $KEY_TEST_DIR/svc_gps.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for GPS."
    exit 2
fi

VERS="2.0.4"
cat <<EOF >$KEY_TEST_DIR/svc_gps2.json
{
  "label":"GPS for ${ARCH}",
  "description":"GPS service",
  "public":true,
  "url":"https://bluehorizon.network/service-gps",
  "version":"$VERS",
  "arch":"${ARCH}",
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
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
      }
    }
  },
  "deploymentSignature":""
}
EOF
echo -e "Register GPS service $VERS:"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_gps2.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
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
  "label":"GPS for Location for ${ARCH}",
  "description":"GPS service for loc service",
  "public":false,
  "url":"https://bluehorizon.network/services/locgps",
  "version":"$VERS",
  "arch":"${ARCH}",
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
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
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
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_locgps.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for LocGPS."
    exit 2
fi

VERS="2.0.4"
cat <<EOF >$KEY_TEST_DIR/svc_locgps2.json
{
  "label":"GPS for Location for ${ARCH}",
  "description":"GPS service for loc service",
  "public":false,
  "url":"https://bluehorizon.network/services/locgps",
  "version":"$VERS",
  "arch":"${ARCH}",
  "sharable":"single",
  "requiredServices":[
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"${ARCH}","org":"IBM"},
    {"url":"https://bluehorizon.network/services/network2","version":"1.0.0","arch":"${ARCH}","org":"IBM"}
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
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
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
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_locgps2.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
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

if [ "${NOVAULT}" != "1" ]; then
  NS_FILE_IBM="/root/service_defs/IBM/netspeed_2.3.0_secrets.json"
  NS_FILE_E2EDEV="/root/service_defs/e2edev@somecomp.com/netspeed_2.3.0_secrets.json"

else
  NS_FILE_IBM="/root/service_defs/IBM/netspeed_2.3.0.json"
  NS_FILE_E2EDEV="/root/service_defs/e2edev@somecomp.com/netspeed_2.3.0.json"

fi

export VERS="2.3.0"

cat ${NS_FILE_IBM} | envsubst > $KEY_TEST_DIR/svc_netspeed.json

echo -e "Register IBM/netspeed service $VERS:"
hzn exchange service publish -I -u $IBM_ADMIN_AUTH -o IBM -f $KEY_TEST_DIR/svc_netspeed.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for IBM/netspeed."
    exit 2
fi

cat ${NS_FILE_E2EDEV} | envsubst > $KEY_TEST_DIR/svc_netspeed.json

echo -e "Register e2edev@somecomp.com/netspeed service $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_netspeed.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for e2edev@somecomp.com/netspeed."
    exit 2
fi

# The GPSTest service:
VERS="1.0.0"
cat <<EOF >$KEY_TEST_DIR/svc_gpstest.json
{
  "label":"GPSTest for ~${ARCH}",
  "description":"GPS Test service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/gpstest",
  "version":"$VERS",
  "arch":"${ARCH}",
  "requiredServices":[
    {"url":"https://bluehorizon.network/service-gps","version":"1.0.0","arch":"${ARCH}","org":"IBM"}
  ],
  "userInput":[],
  "deployment":{
    "services":{
      "gpstest":{
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
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
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_gpstest.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for GPSTest."
    exit 2
fi


# Location definition
VERS="2.0.6"
cat <<EOF >$KEY_TEST_DIR/svc_location.json
{
  "label":"Location for ${ARCH}",
  "description":"Location service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/location",
  "version":"$VERS",
  "arch":"${ARCH}",
  "requiredServices":[
    {"url":"https://bluehorizon.network/services/locgps","versionRange":"2.0.3","arch":"${ARCH}","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","versionRange":"1.0.0","arch":"${ARCH}","org":"IBM"}
  ],
  "userInput":[],
  "deployment":{
    "services":{
      "location":{
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
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
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_location.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for Location."
    exit 2
fi

VERS="2.0.7"
cat <<EOF >$KEY_TEST_DIR/svc_location2.json
{
  "label":"Location for ${ARCH}",
  "description":"Location service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/location",
  "version":"$VERS",
  "arch":"${ARCH}",
  "requiredServices":[
    {"url":"https://bluehorizon.network/services/locgps","version":"2.0.4","arch":"${ARCH}","org":"e2edev@somecomp.com"},
    {"url":"https://bluehorizon.network/service-cpu","version":"1.0.0","arch":"${ARCH}","org":"IBM"}
  ],
  "userInput":[],
  "deployment":{
    "services":{
      "location2":{
        "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
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
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_location2.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for Location."
    exit 2
fi

# The weather service
VERS="1.5.0"
cat <<EOF >$KEY_TEST_DIR/svc_weather.json
{
  "label":"Weather for ${ARCH}",
  "description":"PWS service",
  "public":false,
  "sharable":"multiple",
  "url":"https://bluehorizon.network/services/weather",
  "version":"$VERS",
  "arch":"${ARCH}",
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
      "image":"${CPU_IMAGE_NAME}:${CPU_IMAGE_TAG}"
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
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_weather.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for PWS."
    exit 2
fi

VERS="1.0.0"
cat <<EOF >$KEY_TEST_DIR/svc_k8s1.json
{
  "label":"Cluster service test for ${ARCH}",
  "description":"Cluster Service Test service1",
  "public":true,
  "sharable":"multiple",
  "url":"k8s-service1",
  "version":"$VERS",
  "arch":"${ARCH}",
  "requiredServices":[
  ],
  "userInput": [
      {
        "name": "var1",
        "label": "",
        "type": "string"
       },
      {
        "name": "var2",
        "label": "",
        "type": "int"
      },
      {
        "name": "var3",
        "label": "",
        "type": "float"
      }
  ],
  "clusterDeployment": {
    "operatorYamlArchive": "/root/input_files/k8s_deploy/topservice-operator/topservice-operator.tar.gz"
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

cat <<EOF >$KEY_TEST_DIR/svc_k8s_embedded_ns.json
{
  "label":"Cluster service test for ${ARCH}",
  "description":"Cluster Service with Embedded ns",
  "public":true,
  "sharable":"multiple",
  "url":"k8s-service-embedded-ns",
  "version":"$VERS",
  "arch":"${ARCH}",
  "requiredServices":[
  ],
  "userInput": [
      {
        "name": "var1",
        "label": "",
        "type": "string"
       },
      {
        "name": "var2",
        "label": "",
        "type": "int"
      },
      {
        "name": "var3",
        "label": "",
        "type": "float"
      }
  ],
  "clusterDeployment": {
    "operatorYamlArchive": "/root/input_files/k8s_deploy/topservice-operator-with-embedded-ns/topservice-operator-with-embedded-ns.tar.gz"
   },
  "clusterDeploymentSignature": ""
}
EOF

echo -e "Register k8s-service-embedded-ns $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_k8s_embedded_ns.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for k8s-service-embedded-ns."
    exit 2
fi

cat <<EOF >$KEY_TEST_DIR/svc_k8s_secret.json
{
  "label":"Cluster service test vault secret for amd64",
  "description":"Cluster Service Test k8s-hello-secret",
  "public":true,
  "sharable":"multiple",
  "url":"k8s-hello-secret",
  "version":"$VERS",
  "arch":"${ARCH}",
  "requiredServices":[
  ],
  "userInput": [
      {
        "name": "var1",
        "label": "",
        "type": "string"
       },
      {
        "name": "var2",
        "label": "",
        "type": "int"
      },
      {
        "name": "var3",
        "label": "",
        "type": "float"
      }
  ],
  "clusterDeployment": {
    "operatorYamlArchive": "/root/input_files/k8s_deploy/k8s-secret-operator/k8s-secret-operator.tar.gz",
    "secrets": {
          "secret1": {"description": "Secret 1 for cluster hello-secret."}
        }
   },
  "clusterDeploymentSignature": ""
}
EOF

echo -e "Register k8s-hello-secret $VERS:"
hzn exchange service publish -I -u $E2EDEV_ADMIN_AUTH -o e2edev@somecomp.com -f $KEY_TEST_DIR/svc_k8s_secret.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
if [ $? -ne 0 ]
then
    echo -e "hzn exchange service publish failed for k8s-hello-secret."
    exit 2
fi


echo -e "Listing services:"
hzn exchange service list -o e2edev@somecomp.com
hzn exchange service list -o IBM



# ======================= Patterns that use top level services ======================
# sns pattern
if [ ${REMOTE_HUB} -eq 0 ]; then
  export MHI=120
  export CAS=30
else
  export MHI=600
  export CAS=600
fi

export VERS="2.3.0"

if [ "${NOVAULT}" != "1" ]; then
  NS_PATTERN="/root/patterns/e2edev@somecomp.com/netspeed_secrets.json"

else
  NS_PATTERN="/root/patterns/e2edev@somecomp.com/netspeed.json"

fi

export VERS="2.3.0"

cat ${NS_PATTERN} | envsubst > $KEY_TEST_DIR/pattern_netspeed.json

echo -e "Register sns (service based netspeed) pattern $VERS:"

RES=$(cat $KEY_TEST_DIR/pattern_netspeed.json | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sns" | jq -r '.')

results "$RES"

# sgps test pattern
if [ ${REMOTE_HUB} -eq 0 ]; then
  MHI=90
  CAS=60
else
  MHI=600
  CAS=600
fi

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
      "serviceArch":"${ARCH}",
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
# if [ ${REMOTE_HUB} -eq 0 ]; then
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
#       "serviceArch":"${ARCH}",
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

if [ "${NOHZNDEV}" == "1" ] && [ "${NOHELLO}" == "1" ] && [ "${TEST_PATTERNS}" != "sall" ] && [ "${TEST_PATTERNS}" != "susehello" ]; then
    echo -e "Skipping use hello pattern creation"
else

# susehello test pattern
if [ ${REMOTE_HUB} -eq 0 ]; then
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
      "serviceArch":"${ARCH}",
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

fi

#
# sloc pattern
# This pattern tests a number of things:
# 1. That it is possible for an ag-service to depend on an ag-less (sharable=singleton) service.
# 2. That the higher version of 2.0.7 is chosen when the ag-service is executed.
# 3. That data verification, metering, and nodehealth work correctly.
#
# The verify_sloc.sh script verifies that this service is running correctly.
#
if [ ${REMOTE_HUB} -eq 0 ]; then
  export MHI=240
  export CAS=60
else
  export MHI=600
  export CAS=600
fi
export LOCVERS1="2.0.6"
export LOCVERS2="2.0.7"

if [ "${NOVAULT}" != "1" ]; then
  SLOC_PATTERN="/root/patterns/e2edev@somecomp.com/sloc_secrets.json"
else
  SLOC_PATTERN="/root/patterns/e2edev@somecomp.com/sloc.json"
fi

cat $SLOC_PATTERN | envsubst > $KEY_TEST_DIR/pattern_sloc.json

sdef=$(cat $KEY_TEST_DIR/pattern_sloc.json)

echo -e "Register location service pattern $VERS:"

RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sloc" | jq -r '.')

results "$RES"

# weather pattern
if [ ${REMOTE_HUB} -eq 0 ]; then
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
      "serviceArch":"${ARCH}",
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
if [ ${REMOTE_HUB} -eq 0 ]; then
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
      "serviceArch":"${ARCH}",
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
echo -e "Register k8s service pattern $K8SVERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sk8s" | jq -r '.')

results "$RES"

# k8s pattern with cluster namespace
K8SVERS="1.0.0"
read -d '' sdef <<EOF
{
  "label": "K8s",
  "description": "a k8s pattern",
  "public": true,
  "clusterNamespace": "agent-namespace",
  "services": [
    {
      "serviceUrl":"k8s-service1",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"${ARCH}",
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
echo -e "Register k8s service pattern $K8SVERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sk8s-with-cluster-ns" | jq -r '.')

results "$RES"

# k8s pattern with service has embedded ns
K8SVERS="1.0.0"
read -d '' sdef <<EOF
{
  "label": "K8s",
  "description": "k8s pattern with service that has embedded ns",
  "public": true,
  "services": [
    {
      "serviceUrl":"k8s-service-embedded-ns",
      "serviceOrgid":"e2edev@somecomp.com",
      "serviceArch":"${ARCH}",
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
echo -e "Register k8s service with embedded namesapce pattern $K8SVERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sk8s-with-embedded-ns" | jq -r '.')

results "$RES"

if [ "${NOVAULT}" != "1" ]; then
  K8S_SECRET_PATTERN="/root/patterns/e2edev@somecomp.com/sk8s_secrets.json"
  cat $K8S_SECRET_PATTERN | envsubst > $KEY_TEST_DIR/pattern_k8s_secret.json

  sdef=$(cat $KEY_TEST_DIR/pattern_k8s_secret.json)

  echo -e "Register k8s service with secret pattern $K8SVERS:"

  RES=$(echo "$sdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sk8s-with-secrets" | jq -r '.')

  results "$RES"



fi


# the sall pattern

if [ "${NOHZNDEV}" == "1" ] && [ "${NOHELLO}" == "1" ] && [ "${TEST_PATTERNS}" != "sall" ]; then
    echo -e "Skipping sall pattern creation"
else

export PWSVERS="1.5.0"
export NSVERS="2.3.0"
export LOCVERS1="2.0.6"
export LOCVERS2="2.0.7"
export GPSVERS="1.0.0"
export UHSVERS="1.0.0"
export K8SVERS="1.0.0"
if [ ${REMOTE_HUB} -eq 0 ]; then
  export MHI_240=240
  export MHI_180=180
  export MHI_120=120
  export MHI_90=120
  export CAS_60=60
  export CAS_45=45
  export CAS_30=30
else
  export MHI_240=600
  export MHI_180=600
  export MHI_120=600
  export MHI_90=600
  export CAS_60=600
  export CAS_45=600
  export CAS_30=600
fi

if [ "${NOVAULT}" != "1" ]; then
  SALL_PATTERN="/root/patterns/e2edev@somecomp.com/sall_secrets.json"

else
  SALL_PATTERN="/root/patterns/e2edev@somecomp.com/sall.json"

fi

cat $SALL_PATTERN | envsubst > $KEY_TEST_DIR/pattern_sall.json

msdef=$(cat $KEY_TEST_DIR/pattern_sall.json)

if [[ $TEST_DIFF_ORG -eq 0 ]]; then
  msdef=$(echo $msdef |jq 'del(.services[] | select(.serviceUrl == "https://bluehorizon.network/services/netspeed") | select(.serviceOrgid == "e2edev@somecomp.com"))')
fi

echo -e "Register service based all pattern:"

  RES=$(echo $msdef | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/patterns/sall" | jq -r '.')

results "$RES"

fi

# ======================= Business Policies that use top level services ======================

# netspeed policy
if [ "${NOVAULT}" != "1" ]; then
  NS_DP="/root/deployment_policies/userdev/netspeed_secrets.json"

else
  NS_DP="/root/deployment_policies/userdev/netspeed.json"

fi

cat ${NS_DP} | envsubst > $KEY_TEST_DIR/policy_netspeed.json

echo -e "Register business policy for netspeed:"

RES=$(cat $KEY_TEST_DIR/policy_netspeed.json | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_netspeed" | jq -r '.')

results "$RES"

# location policy 
if [ "${NOVAULT}" != "1" ]; then
  NS_DP="/root/deployment_policies/userdev/location_secrets.json"

else
  NS_DP="/root/deployment_policies/userdev/location.json"
fi

cat ${NS_DP} | envsubst > $KEY_TEST_DIR/policy_location.json

echo -e "Register business policy for netspeed:"

RES=$(cat $KEY_TEST_DIR/policy_location.json | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_location" | jq -r '.')

results "$RES"

# gpstest policy
read -d '' bpgpstestdef <<EOF
{
  "label": "business policy for gpstest",
  "description": "for gpstest",
  "service": {
    "name": "https://bluehorizon.network/services/gpstest",
    "org": "e2edev@somecomp.com",
    "arch": "${ARCH}",
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

read -d '' bppwsdef <<EOF
{
  "label": "business policy for personal weather station",
  "description": "for pws",
  "service": {
    "name": "https://bluehorizon.network/services/weather",
    "org": "e2edev@somecomp.com",
    "arch": "${ARCH}",
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

if [ "${NOHZNDEV}" == "1" ] && [ "${NOHELLO}" == "1" ] && [ "${TEST_PATTERNS}" != "susehello" ]; then
    echo -e "Skipping usehello policy creation"
else

read -d '' bphellodef <<EOF
{
  "label": "business policy for usehello",
  "description": "for usehello",
  "service": {
    "name": "my.company.com.services.usehello2",
    "org": "e2edev@somecomp.com",
    "arch": "${ARCH}",
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

fi

read -d '' bpk8ssvc1def <<EOF
{
  "label": "business policy for k8s-service1",
  "description": "for gpstest",
  "service": {
    "name": "k8s-service1",
    "org": "e2edev@somecomp.com",
    "arch": "${ARCH}",
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
      },
      {
          "name": "has.service.embedded.ns",
          "value": false
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy bp_k8s for k8s-service1:"

  RES=$(echo "$bpk8ssvc1def" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_k8s" | jq -r '.')

results "$RES"

read -d '' bpk8swithembeddednsdef <<EOF
{
  "label": "business policy for k8s-service-embedded-ns",
  "description": "for k8s embedded namespace test",
  "service": {
    "name": "k8s-service-embedded-ns",
    "org": "e2edev@somecomp.com",
    "arch": "${ARCH}",
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
      },
      {
          "name": "service.embedded.ns",
          "value": "operator-embedded-ns"
      }
  ],
  "constraints": [
    "purpose == network-testing"
  ]
}
EOF
echo -e "Register business policy bp_k8s_embedded_ns for k8s-service-embedded-ns:"

  RES=$(echo "$bpk8swithembeddednsdef" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_k8s_embedded_ns" | jq -r '.')

results "$RES"

if [ "${NOVAULT}" != "1" ]; then
read -d '' bpk8ssvc1def <<EOF
{
  "label": "business policy for k8s-hello-secret",
  "description": "Deployment Policy for k8s-hello-secret using a secret",
  "service": {
    "name": "k8s-hello-secret",
    "org": "e2edev@somecomp.com",
    "arch": "${ARCH}",
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
      },
      {
          "name": "policy.purpose",
          "value": "k8s-service-secret-testing"
      }
  ],
  "constraints": [
    "purpose == k8s-service-secret-testing"
  ],
  "secretBinding": [
    {
      "serviceUrl": "k8s-hello-secret",
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceArch": "amd64",
      "serviceVersionRange": "[0.0.0,INFINITY)",
      "secrets": [
        {"secret1": "k8s-hello-secret"}
      ]
    }
  ]
}
EOF
echo -e "Register business policy bp_k8s_secret for k8s-hello-secret:"

  RES=$(echo "$bpk8ssvc1def" | curl -sLX POST $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$USERDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/userdev/business/policies/bp_k8s_secret" | jq -r '.')

results "$RES"
fi


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

  RES=$(echo "$nspoldef" | curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-netspeed_2.3.0_${ARCH}/policy" | jq -r '.')

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

  RES=$(echo "$gpstestpoldef" | curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-gpstest_1.0.0_${ARCH}/policy" | jq -r '.')

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

  RES=$(echo "$locpoldef" | curl -sLX PUT $CERT_VAR --header 'Content-Type: application/json' --header 'Accept: application/json' -u "$E2EDEV_ADMIN_AUTH" --data @- "${EXCH_URL}/orgs/e2edev@somecomp.com/services/bluehorizon.network-services-location_2.0.6_${ARCH}/policy" | jq -r '.')

results "$RES"

unset HZN_EXCHANGE_URL
