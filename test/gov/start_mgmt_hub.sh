#!/bin/bash

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
#  HUB_CONFIG

# exchange confiuration
# INFO or DEBUG
export EXCHANGE_LOG_LEVEL=INFO  

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

# vault configuration
VAULT_LOG_LEVEL=debug

# check if we need start the second agbot
if [ "$MULTIAGBOT" == "1" ]; then
    export START_SECOND_AGBOT=true
else
    export START_SECOND_AGBOT=false
fi

echo -e "${PREFIX} START_SECOND_AGBOT setting is ${START_SECOND_AGBOT}."

cd /tmp
rm -f deploy-mgmt-hub.sh
wget https://raw.githubusercontent.com/open-horizon/devops/master/mgmt-hub/deploy-mgmt-hub.sh
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to download deploy-mgmt-hub.sh file."
  exit 1
fi
chmod +x /tmp/deploy-mgmt-hub.sh

#check if config file is set
if [ -z "${HUB_CONFIG}" ]; then
  echo "hub config is null"
  sudo -sE /tmp/deploy-mgmt-hub.sh -A -E
else
  echo "hub config is not null"
  sudo -sE /tmp/deploy-mgmt-hub.sh -A -E -c ${HUB_CONFIG}
fi
# # run the management hub deployment script
# sudo -sE /tmp/deploy-mgmt-hub.sh -A -E
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed deploy."
  exit 1
fi

