#!/bin/bash

# bootstrap the vault

if [ "${EXCH_APP_HOST}" != "http://exchange-api:8080/v1" ]; then
	exit 0
fi

USERDEV_ADMIN="userdev/userdevadmin"
USERDEV_ADMIN_PW="userdevadminpw"

#Starting vault tests and bootstrap in the dev environment.
if [ "$HZN_VAULT" == "true" ] && [ "$NOVAULT" != "1" ]
then
  echo -e "Checking hashicorp vault reachability"
  /root/vault_test.sh
  if [ $? -ne 0 ]; then
    echo -e "Failed hashicorp vault startup tests."
    exit 1
  fi

  echo -e "vault login $VAULT_TOKEN"
  vault login $VAULT_TOKEN

  echo -e "\nvault secrets enable -version=1 -path=openhorizon kv"
  vault secrets enable -version=1 -path=openhorizon kv

  echo -e "\npolicy write openhorizon-agbot-policy /root/vault/agbot.acl.policy.hcl"
  vault policy write openhorizon-agbot-policy /root/vault/agbot.acl.policy.hcl

  echo -e "\nsetup exchange auth plugin"

  SHASUM=$(shasum -a 256 "/root/vault/hznvaultauth" | cut -d " " -f1)
  vault write sys/plugins/catalog/openhorizon-exchange sha_256="$SHASUM" command="hznvaultauth"

  vault auth enable -path=/openhorizon -plugin-name=openhorizon-exchange plugin

  # Configure the plugin to point to the exchange.
  echo -e "\nvault write auth/openhorizon/config url=${EXCH_APP_HOST} token=${VAULT_TOKEN}"
  vault write auth/openhorizon/config url=${EXCH_APP_HOST} token=${VAULT_TOKEN}

  # log out the root user
  rm -f ~/.vault-token

  # Login the userdevadmin user, as a test.
  echo -e "\nvault write auth/openhorizon/login id=${USERDEV_ADMIN} token=${USERDEV_ADMIN_PW}"
  vault write auth/openhorizon/login id=${USERDEV_ADMIN} token=${USERDEV_ADMIN_PW}

  # TODO: implement a token helper so that we can write more tests.

  echo "Completed vault bootstrap"

else
  echo -e "Vault reachability tests and bootstrap were skipped."
fi


