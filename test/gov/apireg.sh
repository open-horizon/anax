#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# debug() - Print a debug message to stderr when DEBUG=1 or RUNNER_DEBUG=1.
debug() {
    if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
        echo "[DEBUG] [${BASH_SOURCE[0]##*/}:${BASH_LINENO[0]}] $*" >&2
    fi
}

# curl_debug() - Run curl, log method/URL/HTTP-code/body to stderr, return body on stdout.
# Usage: result=$(curl_debug METHOD URL [extra curl args...])
curl_debug() {
    local method="$1" url="$2"
    shift 2
    local out http_code body
    out=$(curl -sS -w "\n%{http_code}" -X "${method}" "$@" "${url}")
    http_code=$(echo "${out}" | tail -1)
    body=$(echo "${out}" | head -n -1)
    if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
        echo "[DEBUG] [${BASH_SOURCE[0]##*/}:${BASH_LINENO[0]}] ${method} ${url} -> HTTP ${http_code} body=${body:0:300}" >&2
    fi
    echo "${body}"
}

echo -e "\nBC setting is $BC"
echo -e "\nPATTERN setting is $PATTERN\n"
debug "apireg: ANAX_API=${ANAX_API} DEVICE_ID=${DEVICE_ID} DEVICE_ORG=${DEVICE_ORG} TOKEN=${TOKEN:0:4}***"

if [ "$HA" = "1" ]; then
    if [ "$PATTERN" = "sgps" ] || [ "$PATTERN" = "sloc" ] || [ "$PATTERN" = "sall" ] || [ "$PATTERN" = "susehello" ] || [ "$PATTERN" = "shelm" ]; then
        echo -e "Pattern $PATTERN is not supported with HA tests, only sns and spws are supported."
        exit 2
    fi
fi

echo "Calling node API"

pat=$PATTERN
if [[ "$PATTERN" != "" ]]; then
    pat="e2edev@somecomp.com/$PATTERN"
fi

read -dr '' newhzndevice <<EOF
{
  "id": "$DEVICE_ID",
  "token": "$TOKEN",
  "name": "$DEVICE_NAME",
  "organization": "$DEVICE_ORG",
  "pattern": "$pat"
}
EOF

while :
do
  echo "Updating horizon with id and token"
  debug "apireg: POST ${ANAX_API}/node"

  ERR=$(echo "$newhzndevice" | curl_debug POST "${ANAX_API}/node" -H "Content-Type: application/json" --data @- | jq -r '.error')
  debug "apireg: POST ${ANAX_API}/node -> error=${ERR}"
  echo -e "the error $ERR"
  if [ "$ERR" = "null" ]
  then
    break
  fi

  if [ "${ERR:0:19}" = "Node is restarting," ]
  then
    debug "apireg: node restarting, retrying in 5s"
    sleep 5
  else
    echo -e "error occured: $ERR"
    exit 2
  fi

done

echo "Waiting for node to sync up with exchange"
sleep 30

# ================================================================
# Set a node policy indicating the testing purpose of the node.

constraint2=""
if [ "$NONS" = "1" ]; then 
    constraint2="NONS==true"
else
    constraint2="NONS==false"
fi
if [ "$NOGPS" = "1" ]; then 
    constraint2="$constraint2 || NOGPS == true"
else
    constraint2="$constraint2 || NOGPS == false"
fi
if [ "$NOLOC" = "1" ]; then 
    constraint2="$constraint2 || NOLOC == true"
else
    constraint2="$constraint2 || NOLOC == false"
fi
if [ "$NOPWS" = "1" ]; then 
    constraint2="$constraint2 || NOPWS == true"
else
    constraint2="$constraint2 || NOPWS == false"
fi
if [ "$NOHELLO" = "1" ]; then 
    constraint2="$constraint2 || NOHELLO == true"
else
    constraint2="$constraint2 || NOHELLO == false"
fi
if [ "$NOK8S" = "1" ]; then 
    constraint2="$constraint2 || NOK8S == true"
else
    constraint2="$constraint2 || NOK8S == false"
fi

constraint3=""
if [ "$NOAGENTAUTO" = "1" ]; then 
    constraint3="NOAGENTAUTO==true"
else
    constraint3="NOAGENTAUTO==false"
fi

read -dr '' newhznpolicy <<EOF
{
  "deployment": {
    "properties": [
      {
        "name":"purpose","value":"network-testing"
      },
      {
        "name":"group","value":"bluenode"
      }
    ],
    "constraints": [
      "iame2edev == true",
      "$constraint2"
    ]
  },
  "management": {
    "properties": [
      {
        "name":"purpose","value":"nmp-testing"
      },
      {
        "name":"group","value":"bluenode"
      }
    ],
    "constraints": [
      "iame2edev == true",
      "$constraint3"
    ]
  }
}
EOF

echo "Adding policy to the node using node/policy API"
debug "apireg: PUT ${ANAX_API}/node/policy constraint2=${constraint2} constraint3=${constraint3}"
RES=$(echo "$newhznpolicy" | curl_debug PUT "${ANAX_API}/node/policy" -w "%{http_code}" -H "Content-Type: application/json" --data @-)
debug "apireg: PUT ${ANAX_API}/node/policy -> RES=${RES}"

if [ "$RES" = "" ]
then
  echo -e "$newhznpolicy \nresulted in empty response"
  exit 2
fi

ERR=$(echo "$RES" | jq -r '.' | tail -1)
if [ "$ERR" != "201" ]
then
    echo -e "$newhznpolicy \nresulted in incorrect response: $RES"

    echo -e "Wait for 30 seconds and try again"
    sleep 30
    debug "apireg: PUT ${ANAX_API}/node/policy (retry)"
    RES=$(echo "$newhznpolicy" | curl_debug PUT "${ANAX_API}/node/policy" -H "Content-Type: application/json" --data @-)
    debug "apireg: PUT ${ANAX_API}/node/policy (retry) -> RES=${RES}"
    ERR=$(echo "$RES" | jq -r '.' | tail -1)
    if [ "$ERR" != "201" ]
    then
        echo -e "$newhznpolicy \nsecond try resulted in incorrect response: $RES"
        exit 2
    else
        echo -e "found expected response in second try: $RES"
    fi
else
  echo -e "found expected response: $RES"
fi

# =======================================================================
# Setup some services/workloads
echo -e "\nNo netspeed setting is $NONS"
if [ "$NONS" != "1" ]
then
  if ! ./ns_apireg.sh; then
    exit 2
  fi
fi

echo -e "\nNo location setting is $NOLOC"
if [ "$NOLOC" != "1" ]
then
    if ! ./loc2_apireg.sh; then
        exit 2
    fi
fi

echo -e "\nNo gpstest setting is $NOGPS"
if [ "$NOGPS" != "1" ]
then
    if ! ./gpstest_apireg.sh; then
        exit 2
    fi
fi

echo -e "\nNo pws setting is $NOPWS"
if [ "$NOPWS" != "1" ]
then
  if ! ./pws_apireg.sh; then
    exit 2
  fi
fi

if ! ./hello_apireg.sh; then
  exit 2
fi

echo -e "\nCompleting node registration"
if ! ./cs_apireg.sh; then
  echo -e "Error setting up to run workloads"
  exit 2
else
  echo -e "Workload setup SUCCESSFUL"
fi

echo -e "\n\n[D] all registered attributes:\n"
curl -sS -H "Content-Type: application/json" "$ANAX_API/attribute" | jq -r '.'
