#!/bin/bash

function results {
  if [ "$(echo "$1" | jq -r '.code')" != "ok" ]
  then
    echo -e "Error: $(echo "$1" | jq -r '.msg')"
    exit 2
  fi
}

if [ "${CERT_LOC}" -eq 1 ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=(--silent)
fi

MAIN_AUTH="apikey:${API_KEY}"

HZN_EXCHANGE_NODE_AUTH="testNode:Abcdefghijklmno1"
NODE_NAME="testNode"

if [ "${ORG_ID}" = "" ]; then
  ORG_ID=${AGBOT_NAME//"-agbot"/}
fi

export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

# Register services via the hzn dev exchange commands
if ! ./gov/hzn_dev_services.sh "${HZN_EXCHANGE_URL}" "${MAIN_AUTH}" 1
then
    echo -e "hzn service and pattern registration with hzn dev failed."
    exit 1
fi

hzn exchange node remove -f -n "$HZN_EXCHANGE_NODE_AUTH" -o "$ORG_ID" -u "$MAIN_AUTH" "$NODE_NAME"
hzn exchange service remove -u "$MAIN_AUTH" -o "$ORG_ID" -f "$ORG_ID/bluehorizon.network-service-cpu_1.0_${ARCH}"

if ! hzn exchange node create -n "$HZN_EXCHANGE_NODE_AUTH" -m "$NODE_NAME" -o "$ORG_ID" -u "$MAIN_AUTH"
then
    echo -e "hzn exchange node create failed for $ORG_ID."
    unset HZN_EXCHANGE_URL
    exit 2
fi

KEY_TEST_DIR="/tmp/keytest"
mkdir -p $KEY_TEST_DIR

cd $KEY_TEST_DIR || { echo "Error: api_key.sh - ln 50 - Failure to change directories."; error 1; }
if ! ls "*".key &> /dev/null
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  if ! hzn key create -l 4096 "$ORG_ID" "$ORG_ID@gmail.com" -d .
  then
    echo -e "hzn key create failed."
    exit 2
  fi
fi

# cpu service
VERS="1.0"
cat <<EOF >$KEY_TEST_DIR/svc_cpu.json
{
  "label":"CPU service",
  "description":"CPU service",
  "public":true,
  "url":"https://bluehorizon.network/service-cpu",
  "version":"$VERS",
  "arch":"${ARCH}",
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
        "image":"openhorizon/${ARCH}_cpu:1.2.2"
      }
    }
  },
  "deploymentSignature":""
}
EOF

echo -e "Register $ORG_ID/cpu service $VERS:"
if ! hzn exchange service publish -I -u "$MAIN_AUTH" -o "$ORG_ID" -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
then
    echo -e "hzn exchange service publish failed for $ORG_ID/cpu."
    exit 2
fi

echo "Display IBM Org Pattern List"
if ! hzn exchange pattern list -n "$HZN_EXCHANGE_NODE_AUTH" -o "$ORG_ID" -u "$MAIN_AUTH" IBM/*
then
    echo -e "hzn exchange pattern list failed for IBM Org."
    exit 2
fi

echo "Display $ORG_ID User List"
if ! hzn exchange user list -o "$ORG_ID" -u "$MAIN_AUTH"
then
    echo -e "hzn exchange user list failed."
    exit 2
fi

echo -e "Delete $ORG_ID/cpu service $VERS:"
if ! hzn exchange service remove -u "$MAIN_AUTH" -o "$ORG_ID" -f "$ORG_ID/bluehorizon.network-service-cpu_1.0_${ARCH}"
then
    echo -e "hzn exchange service publish failed for $ORG_ID/cpu."
    exit 2
fi

echo -e "Delete node $HZN_EXCHANGE_NODE_AUTH"
if ! hzn exchange node remove -f -n "$HZN_EXCHANGE_NODE_AUTH" -o "$ORG_ID" -u "$MAIN_AUTH" "$NODE_NAME"
then
    echo -e "hzn exchange node delete failed for $ORG_ID."
    unset HZN_EXCHANGE_URL
    exit 2
fi

unset HZN_EXCHANGE_URL
