#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# debug() - Print a debug message to stderr when DEBUG=1 or RUNNER_DEBUG=1.
# Usage: debug "message"
# Messages are prefixed with [DEBUG] and the calling script name for easy filtering.
debug() {
    if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
        echo "[DEBUG] [${BASH_SOURCE[1]##*/}:${BASH_LINENO[0]}] $*" >&2
    fi
}

# curl_debug() - Run curl and log the HTTP method, URL, response code, and truncated body.
# Usage: result=$(curl_debug METHOD URL [extra curl args...])
# The function logs to stderr when DEBUG=1 and returns the response body (stdout).
# The HTTP status code is logged but not returned; callers should use -w "%{http_code}" directly
# when they need to act on the status code.
curl_debug() {
    local method="$1"
    local url="$2"
    shift 2
    local out http_code body
    out=$(curl -sS -w "\n%{http_code}" -X "${method}" "$@" "${url}")
    http_code=$(echo "${out}" | tail -1)
    body=$(echo "${out}" | head -n -1)
    if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
        echo "[DEBUG] [${BASH_SOURCE[1]##*/}:${BASH_LINENO[0]}] ${method} ${url} -> HTTP ${http_code} body=${body:0:300}" >&2
    fi
    echo "${body}"
}

# checks the http code from a http call with "-w %{http_code}"
# $1 -- the expected code
# $2 -- the http call output
check_api_result() {

  rc="${2: -3}"
  output="${2::-3}"

  debug "check_api_result: expected=$1 got=$rc output=${output:0:200}"

  # check http code
  if [ "$rc" != "$1" ]
  then
    echo -e "Error: $(echo "$output" | jq -r '.')\n"
    exit 2
  fi

  #statements
  echo -e "Result expected."
}

# $1 - Service url to wait for
# $2 - Service org to wait for
# $3 - Service version to wait for (optional)
# $4 - Service with error (bool, optional, default to false)
# please export ANAX_API, MAX_ITERATION(default 25)
WaitForService() {
  # shellcheck disable=SC2034  # current_svc_version is read by callers after WaitForService returns
  current_svc_version=""
  TIMEOUT=0

  # set default 
  if [ -z "$MAX_ITERATION" ]; then
    MAX_ITERATION=25
  fi

  echo "ANAX_API=$ANAX_API, MAX_ITERATION=$MAX_ITERATION" 

  while [[ $TIMEOUT -le $MAX_ITERATION ]]
  do
    if [ "${3}" = "" ]; then
        echo -e "Waiting for service $2/$1 with any version."
        if ! svc_inst=$(curl -s "$ANAX_API/service" | jq -r ".instances.active[] | select (.ref_url == \"$1\") | select (.organization == \"$2\")"); then
            echo -e "Failed to get $1 service instance. ${svc_inst}"
            exit 2
        fi
    else
        echo -e "Waiting for service $2/$1 with version $3."
        if ! svc_inst=$(curl -s "$ANAX_API/service" | jq -r ".instances.active[] | select (.ref_url == \"$1\") | select (.organization == \"$2\") | select (.version == \"$3\")"); then
            echo -e "Failed to get $1 service instance. ${svc_inst}"
            exit 2
        fi
    fi

    echo "svc_inst=$svc_inst"
    if [ "$4" = "true" ] && [ "$svc_inst" != "" ]; then
        echo -e "Found service $2/$1 with version $3. Checking for err service: $4"
	break
    elif [ "$svc_inst" != "" ]; then
        svc_start_time=$(echo "$svc_inst" |jq -r '.execution_start_time')
    fi

    if [ "$svc_inst" == "" ] || [ "$svc_start_time" = "0" ]; then
        sleep 5s
        ((TIMEOUT++))
    else
        echo -e "Found service $2/$1 with version $3."
        break
    fi

    if [[ $TIMEOUT = $(("$MAX_ITERATION" + 1)) ]]; then echo -e "Timeout waiting for service $1 to start"; exit 2; fi
  done
}
