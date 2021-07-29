#!/bin/bash

# bootstrap the vault

if [ "${EXCH_APP_HOST}" != "http://exchange-api:8081/v1" ]; then
	exit 0
fi

USER_ORG="userdev"
TEST_SECRET="secret"

#Starting vault tests and bootstrap in the dev environment.
if [ "$NOVAULT" != "1" ]
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
  
  # Write a sample secret to the userdev org
  echo -e "\nvault kv put openhorizon/${USER_ORG}/${TEST_SECRET} ${TEST_SECRET}=${TEST_SECRET}"
  vault kv put openhorizon/${USER_ORG}/${TEST_SECRET} ${TEST_SECRET}=${TEST_SECRET}
  if [ $? -ne 0 ]; then
    echo -e "Failed put to kv store, vault bootstrap failed"
    exit 1
  fi

  # Delete it
  echo -e "\nvault kv metadata delete openhorizon/${USER_ORG}/${TEST_SECRET}"
  vault kv metadata delete openhorizon/${USER_ORG}/${TEST_SECRET}
  if [ $? -ne 0 ]; then
    echo -e "Failed delete temp secret from kv store, vault bootstrap failed"
    exit 1
  fi

else
  echo -e "Vault reachability tests were skipped."
fi


