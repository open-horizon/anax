#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

PREFIX="All-in-one management hub deployment:"

# the environment variables set by the Makefile are:
#  ANAX_SOURCE

cd /tmp || exit
rm -f deploy-mgmt-hub.sh
if ! wget https://raw.githubusercontent.com/open-horizon/devops/master/mgmt-hub/deploy-mgmt-hub.sh; then
  echo -e "${PREFIX} Failed to download deploy-mgmt-hub.sh file."
  exit 1
fi
chmod +x /tmp/deploy-mgmt-hub.sh

# cleanup the management hub
if ! sudo -sE /tmp/deploy-mgmt-hub.sh -P -S; then
  echo -e "${PREFIX} Failed to cleanup."
  exit 1
fi

