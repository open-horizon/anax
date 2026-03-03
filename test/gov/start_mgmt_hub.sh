#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

PREFIX="All-in-one management hub deployment:"
# the environment variables set by the Makefile are:
#  EXCHANGE_ROOT_PW 
#  EXCHANGE_IMAGE_TAG
#  EXCHANGE_DATABASE 
#  EXCHANGE_HUB_ADMIN_PW
#  EXCHANGE_SYSTEM_ADMIN_PW
#  AGBOT_ID
#  AGBOT_TOKEN
#  AGBOT_IMAGE_TAG
#  CSS_IMAGE_TAG
#  CSS_INTERNAL_PORT
#  MONGO_IMAGE_TAG
#  POSTGRES_IMAGE_TAG 
#  POSTGRES_USER
#  TEST_VARS
#  ANAX_SOURCE

# exchange confiuration
# INFO or DEBUG
export EXCHANGE_LOG_LEVEL=${EXCHANGE_LOG_LEVEL:-INFO}

# agbot configuration
export AGBOT_AGREEMENT_TIMEOUT_S=300
export AGBOT_NEW_CONTRACT_INTERVAL_S=10
export AGBOT_PROCESS_GOVERNANCE_INTERVAL_S=10
export AGBOT_EXCHANGE_HEARTBEAT=10
export AGBOT_CHECK_UPDATED_POLICY_S=15
export AGBOT_AGREEMENT_BATCH_SIZE=1300
export AGBOT_RETRY_LOOK_BACK_WINDOW=900
export AGBOT_MMS_GARBAGE_COLLECTION_INTERVAL=20

# CSS configuration
export CSS_PERSISTENCE_PATH="/tmp/persist"
export CSS_LOG_LEVEL="TRACE"
export CSS_LOG_TRACE_DESTINATION="stdout,glog"
export CSS_LOG_ROOT_PATH="/tmp"
export CSS_TRACE_LEVEL="INFO"
export CSS_TRACE_ROOT_PATH="/tmp/trace"
export CSS_MONGO_AUTH_DB_NAME="admin"

# check if we need start the second agbot
if [ "$MULTIAGBOT" = "1" ]; then
    export START_SECOND_AGBOT=true
else
    export START_SECOND_AGBOT=false
fi

echo -e "${PREFIX} START_SECOND_AGBOT setting is ${START_SECOND_AGBOT}."

cd /tmp || { echo "Error: start_mgmt_hub.sh - ln 57 - Failure to change directories."; exit 1; }
rm -f deploy-mgmt-hub.sh
if ! wget https://raw.githubusercontent.com/open-horizon/devops/refs/heads/master/mgmt-hub/deploy-mgmt-hub.sh; then
  echo -e "${PREFIX} Failed to download deploy-mgmt-hub.sh file."
  exit 1
fi
chmod +x /tmp/deploy-mgmt-hub.sh

# run the management hub deployment script
if ! sudo -sE /tmp/deploy-mgmt-hub.sh -A -E; then
  echo -e "${PREFIX} Failed deploy."
  exit 1
fi

