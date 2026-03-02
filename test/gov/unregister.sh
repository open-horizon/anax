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

# The purpose of this test is to verify that the DELETE /node API works correctly in a full
# runtime context. Some parts of this test simulate the fact that anax is configured to auto-restart
# when it terminates.

EXCH_URL="${EXCH_APP_HOST}"
export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
debug "unregister: ANAX_API=${ANAX_API} EXCH_URL=${EXCH_URL} DEVICE_ID=${DEVICE_ID} DEVICE_ORG=${DEVICE_ORG}"

echo "Unregister node, non-blocking"
if ! hzn unregister -f; then
  echo -e "Error unregistering the node."
  exit 2
fi

if [ "${CERT_LOC}" -eq 1 ]; then
  CERT_VAR="--cacert /certs/css.crt"
else
  CERT_VAR=(--silent)
fi

# Start polling for unconfig completion. Unconfig could take several minutes if we are running this test with a blockchain
# configuration.
echo -e "Polling anax API for completion of device unconfigure."
COUNT=1
while :
do
  debug "unregister: GET ${ANAX_API}/node (poll iteration ${COUNT})"
  if ! GET=$(curl_debug GET "${ANAX_API}/node" -L); then
    # curl exit code 7 = connection refused (anax is down)
    debug "unregister: anax is down (curl failed), unregister complete"
    break
  else
    debug "unregister: anax still up"
    echo -e "Is anax still up: $GET"

    # Since anax is still up, verify that a POST to /node will return the correct error.
    pat=$PATTERN
    if [[ "$PATTERN" != "" ]]; then
      pat="e2edev@somecomp.com/$PATTERN"
    fi
    newhzndevice=$(cat <<EOF
{
  "id": "$DEVICE_ID",
  "token": "$TOKEN",
  "name": "$DEVICE_NAME",
  "organization": "$DEVICE_ORG",
  "pattern": "$pat"
}
EOF
)
    debug "unregister: POST ${ANAX_API}/node (expect 'Node is restarting' error)"
    HDS=$(curl_debug POST "${ANAX_API}/node" -H "Content-Type: application/json" --data "$newhzndevice")

    rc=$?
    debug "unregister: POST ${ANAX_API}/node rc=${rc}"

    # We only want to look at the response if it's a json document. Everything else we can ignore because anax could have terminated
    # between the GET call above and this POST call.
    if [ $rc -eq 0 ] && [ "$HDS" != "null" ] && [ "${HDS:0:1}" = "{" ]
    then
      ERR=$(echo "$HDS" | jq -r '.error')
      debug "unregister: POST /node error field=${ERR}"
      if [ "${ERR:0:19}" != "Node is restarting," ]
      then
        echo -e "node object has the wrong state: $HDS"
        exit 2
      fi
    elif [ $rc -eq 7 ]; then
      echo -e "Anax is down."
      break
    fi
  fi

  # This is the loop/timeout control. Exit the test in error after 4 mins without an anax termination.
  if [ "$COUNT" = "48" ]
  then
    echo -e "Error, anax is taking too long to terminate."
    exit 2
  fi
  sleep 5
  COUNT=$((COUNT+1))
done

# Following the API call, the node's entry in the exchange should have some changes in it. The messaging key should be empty,
# and the list of registered microservices should be empty.
echo -e "Checking node status in the exchange."
NST=$(curl_debug GET "${EXCH_URL}/orgs/e2edev@somecomp.com/nodes/${DEVICE_ID:-an12345}" "${CERT_VAR[@]}" --header 'Accept: application/json' -u "e2edev@somecomp.com/e2edevadmin:e2edevadminpw" | jq -r '.')
PK=$(echo "$NST" | jq -r '.publicKey')
debug "unregister: publicKey=${PK} (expected null)"
if [ "$PK" != "null" ]
then
  echo -e "publicKey should be empty: $PK"
  exit 2
fi

RM=$(echo "$NST" | jq -r '.registeredServices[0]')
debug "unregister: registeredServices[0]=${RM} (expected null)"
if [ "$RM" != "null" ]
then
  echo -e "registeredServices should be empty: $RM"
  exit 2
fi
