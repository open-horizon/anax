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

debug "del_loop: ANAX_API=${ANAX_API} NOLOOP=${NOLOOP}"

for (( ; ; ))
do
   debug "del_loop: GET ${ANAX_API}/agreement (fetching active agreement ids)"
   _ags_json=$(curl_debug GET "${ANAX_API}/agreement")
   AGID1=$(echo "${_ags_json}" | jq -r '.agreements.active[0].current_agreement_id')
   AGID2=$(echo "${_ags_json}" | jq -r '.agreements.active[1].current_agreement_id')
   AGID3=$(echo "${_ags_json}" | jq -r '.agreements.active[2].current_agreement_id')
   AGID4=$(echo "${_ags_json}" | jq -r '.agreements.active[3].current_agreement_id')
   debug "del_loop: active agreement ids: AGID1=${AGID1} AGID2=${AGID2} AGID3=${AGID3} AGID4=${AGID4}"

   echo "Device deleting agreements"
   echo "Deleting $AGID1"
   curl_debug DELETE "${ANAX_API}/agreement/${AGID1}" > /dev/null

   echo "Deleting $AGID2"
   curl_debug DELETE "${ANAX_API}/agreement/${AGID2}" > /dev/null

   echo "Deleting $AGID3"
   curl_debug DELETE "${ANAX_API}/agreement/${AGID3}" > /dev/null

   echo "Deleting $AGID4"
   curl_debug DELETE "${ANAX_API}/agreement/${AGID4}" > /dev/null

   if [ "$NOLOOP" = "1" ]; then
     exit 0
   else
      echo -e "Sleeping now\n"
      sleep 600
   fi
done
