#!/bin/bash

PREFIX="All-in-one management hub deployment:"

# the environment variables set by the Makefile are:
#  ANAX_SOURCE

# copy the agbot, css and exchange config template files incase they are deleted
tempHorizonDir="/tmp/horizon"
mkdir -p ${tempHorizonDir}
cp -f ${ANAX_SOURCE}/test/docker/fs/etc/agbot/agbot-tmpl.json ${tempHorizonDir}
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed copy the agbot config template file to ${tempHorizonDir}."
  exit 1
fi
cp -f ${ANAX_SOURCE}/test/docker/fs/etc/edge-sync-service/css-tmpl.conf ${tempHorizonDir}
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed copy the css config template file to ${tempHorizonDir}."
  exit 1
fi
cp -f ${ANAX_SOURCE}/test/docker/fs/etc/exchange/exchange-tmpl.json ${tempHorizonDir}
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed copy the exchange config template file to ${tempHorizonDir}."
  exit 1
fi
export OH_DONT_DOWNLOAD='agbot-tmpl.json css-tmpl.conf exchange-tmpl.json'


cd /tmp
rm -f deploy-mgmt-hub.sh
wget https://raw.githubusercontent.com/open-horizon/devops/master/mgmt-hub/deploy-mgmt-hub.sh
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to download deploy-mgmt-hub.sh file."
  exit 1
fi
chmod +x /tmp/deploy-mgmt-hub.sh

# cleanup the management hub  
sudo -sE /tmp/deploy-mgmt-hub.sh -P -S
if [ $? -ne 0 ]; then
  echo -e "${PREFIX} Failed to cleanup."
  exit 1
fi

