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
#  MONGO_IMAGE_TAG
#  POSTGRES_IMAGE_TAG 
#  POSTGRES_USER
#  TEST_VARS
#  ANAX_SOURCE

# copy the agbot, css and exchange config template files
tempHorizonDir="/tmp/horizon-all-in-1"
mkdir -p ${tempHorizonDir}
cp -f ${ANAX_SOURCE}/test/docker/fs/etc/agbot/agbot-tmpl.json ${tempHorizonDir}
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to copy the agbot config template file to ${tempHorizonDir}."
  exit 1
fi
cp -f ${ANAX_SOURCE}/test/docker/fs/etc/edge-sync-service/css-tmpl.conf ${tempHorizonDir}
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to copy the css config template file to ${tempHorizonDir}."
  exit 1
fi
cp -f ${ANAX_SOURCE}/test/docker/fs/etc/exchange/exchange-tmpl.json ${tempHorizonDir}
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to copy the exchange config template file to ${tempHorizonDir}."
  exit 1
fi
export OH_DONT_DOWNLOAD='agbot-tmpl.json css-tmpl.conf exchange-tmpl.json'

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

# run the management hub deployment script
sudo -sE /tmp/deploy-mgmt-hub.sh -A -E
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed deploy."
  exit 1
fi

