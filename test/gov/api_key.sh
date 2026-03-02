#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

results() {
  if [ "$(echo "$1" | jq -r '.code')" != "ok" ]
  then
    echo -e "Error: $(echo "$1" | jq -r '.msg')"
    exit 2
  fi
}

HZN_EXCHANGE_NODE_AUTH="testNode:Abcdefghijklmno1"
NODE_NAME="testNode"

# API_KEY_ORG is the organization used for all operations in this script.
# It defaults to ORG_ID if set, otherwise falls back to e2edev@somecomp.com.
API_KEY_ORG="${ORG_ID:-e2edev@somecomp.com}"

export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"

# Admin credentials used to generate the API key from the Exchange.
# E2EDEV_ADMIN_AUTH may be supplied as "user:password" or "org/user:password".
# E2EDEV_ADMIN_USER is the bare username (no org prefix) used in the Exchange URL path.
E2EDEV_ADMIN_AUTH="${E2EDEV_ADMIN_AUTH:-e2edevadmin:e2edevadminpw}"
_auth_userpass="${E2EDEV_ADMIN_AUTH##*/}"
E2EDEV_ADMIN_USER="${_auth_userpass%%:*}"
unset _auth_userpass

# Generate an API key for the admin user via the Exchange REST API.
# The Exchange returns {"id": "<key-id>", "key": "<secret>"}.
echo "Generating API key for ${E2EDEV_ADMIN_USER} in org ${API_KEY_ORG}..."
APIKEY_RESP=$(curl -sSL -X POST \
    --header 'Content-Type: application/json' \
    --header 'Accept: application/json' \
    -u "${API_KEY_ORG}/${E2EDEV_ADMIN_AUTH}" \
    -d '{"description":"api_key.sh test key"}' \
    "${EXCH_APP_HOST}/orgs/${API_KEY_ORG}/users/${E2EDEV_ADMIN_USER}/keys")
echo "API key response: ${APIKEY_RESP}"

APIKEY_ID=$(echo "${APIKEY_RESP}" | jq -r '.id // empty')
APIKEY_SECRET=$(echo "${APIKEY_RESP}" | jq -r '.key // empty')

if [ -z "${APIKEY_ID}" ] || [ -z "${APIKEY_SECRET}" ]; then
    echo "Error: failed to generate API key. Response: ${APIKEY_RESP}"
    exit 2
fi
echo "Generated API key id: ${APIKEY_ID}"

# Use the generated API key as the credential for all subsequent hzn commands.
# The Exchange accepts 'organization/apikey:<secret>' as a credential type.
# Prepending the org prevents the hzn CLI from double-prepending it.
MAIN_AUTH="${API_KEY_ORG}/apikey:${APIKEY_SECRET}"

# Register services via the hzn dev exchange commands
if ! ./gov/hzn_dev_services.sh "${HZN_EXCHANGE_URL}" "${MAIN_AUTH}" 1
then
    echo -e "hzn service and pattern registration with hzn dev failed."
    exit 1
fi

hzn exchange node remove -f -n "$HZN_EXCHANGE_NODE_AUTH" -o "$API_KEY_ORG" -u "$MAIN_AUTH" "$NODE_NAME"
hzn exchange service remove -u "$MAIN_AUTH" -o "$API_KEY_ORG" -f "$API_KEY_ORG/bluehorizon.network-service-cpu_1.0_${ARCH}"

if ! hzn exchange node create -n "$HZN_EXCHANGE_NODE_AUTH" -m "$NODE_NAME" -o "$API_KEY_ORG" -u "$MAIN_AUTH"
then
    echo -e "hzn exchange node create failed for $API_KEY_ORG."
    unset HZN_EXCHANGE_URL
    exit 2
fi

KEY_TEST_DIR="/tmp/keytest"
mkdir -p "${KEY_TEST_DIR}"

cd "$KEY_TEST_DIR" || { echo "Error: api_key.sh - ln 50 - Failure to change directories."; exit 1; }
if ls ./*.key > /dev/null 2>&1
then
    echo -e "Using existing key"
else
  echo -e "Generate new signing keys:"
  if ! hzn key create -l 4096 "$API_KEY_ORG" "$API_KEY_ORG@gmail.com" -d .
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

echo -e "Register $API_KEY_ORG/cpu service $VERS:"
if ! hzn exchange service publish -I -u "$MAIN_AUTH" -o "$API_KEY_ORG" -f $KEY_TEST_DIR/svc_cpu.json -k $KEY_TEST_DIR/*private.key -K $KEY_TEST_DIR/*public.pem
then
    echo -e "hzn exchange service publish failed for $API_KEY_ORG/cpu."
    exit 2
fi

echo "Display IBM Org Pattern List"
if ! hzn exchange pattern list -n "$HZN_EXCHANGE_NODE_AUTH" -o "$API_KEY_ORG" -u "$MAIN_AUTH" IBM/*
then
    echo -e "hzn exchange pattern list failed for IBM Org."
    exit 2
fi

echo "Display $API_KEY_ORG User List"
if ! hzn exchange user list -o "$API_KEY_ORG" -u "$MAIN_AUTH"
then
    echo -e "hzn exchange user list failed."
    exit 2
fi

echo -e "Delete $API_KEY_ORG/cpu service $VERS:"
if ! hzn exchange service remove -u "$MAIN_AUTH" -o "$API_KEY_ORG" -f "$API_KEY_ORG/bluehorizon.network-service-cpu_1.0_${ARCH}"
then
    echo -e "hzn exchange service publish failed for $API_KEY_ORG/cpu."
    exit 2
fi

echo -e "Delete node $HZN_EXCHANGE_NODE_AUTH"
if ! hzn exchange node remove -f -n "$HZN_EXCHANGE_NODE_AUTH" -o "$API_KEY_ORG" -u "$MAIN_AUTH" "$NODE_NAME"
then
    echo -e "hzn exchange node delete failed for $API_KEY_ORG."
    unset HZN_EXCHANGE_URL
    exit 2
fi

unset HZN_EXCHANGE_URL

# Clean up the generated API key from the Exchange.
echo "Deleting API key ${APIKEY_ID} for ${E2EDEV_ADMIN_USER} in org ${API_KEY_ORG}..."
curl -sSL -X DELETE \
    --header 'Accept: application/json' \
    -u "${API_KEY_ORG}/${E2EDEV_ADMIN_AUTH}" \
    "${EXCH_APP_HOST}/orgs/${API_KEY_ORG}/users/${E2EDEV_ADMIN_USER}/keys/${APIKEY_ID}"
echo "API key cleanup complete."
