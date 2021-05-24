#!/bin/bash

# bootstrap the vault

if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
	exit 0
fi

USERDEV_ADMIN="userdev/userdevadmin"
USERDEV_ADMIN_PW="userdevadminpw"
USER_ORG="userdev"
TEST_SECRET="secret"

#Starting vault tests and bootstrap in the dev environment.
if [ "$HZN_VAULT" == "true" ] && [ "$NOVAULT" != "1" ]
then
  echo -e "Checking vault reachability"
  /root/vault_test.sh
  if [ $? -ne 0 ]; then
    echo -e "Failed vault startup tests."
    exit 1
  fi

  echo -e "Bootstrapping vault"
  /root/vault_bootstrap.sh
  if [ $? -ne 0 ]; then
    echo -e "Failed vault bootstrap."
    exit 1
  fi

  # Login the userdevadmin user, as a test.
  echo -e "\nvault write auth/openhorizon/login id=${USERDEV_ADMIN} token=${USERDEV_ADMIN_PW}"
  vault write auth/openhorizon/login id=${USERDEV_ADMIN} token=${USERDEV_ADMIN_PW}
  
  # Write a sample secret to the userdev org
  echo -e "\nvault kv put openhorizon/${USER_ORG}/${TEST_SECRET} ${TEST_SECRET}=${TEST_SECRET}"
  vault kv put openhorizon/${USER_ORG}/${TEST_SECRET} ${TEST_SECRET}=${TEST_SECRET}

else
  echo -e "Vault reachability tests were skipped."
fi


