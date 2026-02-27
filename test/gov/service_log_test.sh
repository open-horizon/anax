#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# This service log testing script requires the horizon node registration tests to be called before.

# Still need to add tests which check logging for services with multiple containers deployed.
PREFIX="hzn service log test:"

export HZN_ORG_ID=e2edev@somecomp.com
SERVICE_URL="$HZN_ORG_ID/https://bluehorizon.network/services/netspeed"
SERVICE_CONTAINER_NAME="netspeed"

echo ""
echo -e "${PREFIX} Starting tests on service $SERVICE_URL with one service container"

cmd="hzn service log $SERVICE_URL"
echo -e "$cmd"
if ! ret=$($cmd 2>&1); then
  echo -e "Error: hzn service log failed for $SERVICE_URL. $ret"
  exit 1
fi

cmd="hzn service log $SERVICE_URL -c $SERVICE_CONTAINER_NAME"
echo -e "$cmd"
if ! ret=$($cmd 2>&1); then
  echo -e "Error: hzn service log failed for $SERVICE_URL with container $SERVICE_CONTAINER_NAME. $ret"
  exit 1
fi

cmd="hzn service log ${SERVICE_URL} -c ${SERVICE_CONTAINER_NAME}_error"
echo -e "$cmd"
if ret=$($cmd 2>&1); then
  echo -e "Error: hzn service log should have failed for ${SERVICE_URL} with container ${SERVICE_CONTAINER_NAME}_error. $ret"
  exit 1
fi

cmd="hzn service log ${SERVICE_URL}_error"
echo -e "$cmd"
if ret=$($cmd 2>&1); then
  echo -e "Error: hzn service log should have failed for ${SERVICE_URL}_error. $ret"
  exit 1
fi

unset HZN_ORG_ID
echo -e "${PREFIX} Done"
