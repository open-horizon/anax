#!/bin/bash

PREFIX="All-in-one management hub deployment:"

# the environment variables set by the Makefile are:
#  ANAX_SOURCE

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

