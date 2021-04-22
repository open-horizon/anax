#!/bin/bash

# Basic tests to check api reachability of hashicorp vault running as a docker instance.

# Check vault status using API calls
RES=$(curl --write-out "%{http_code}" --silent -o /dev/null ${VAULT_ADDR}/v1/sys/seal-status)

if [ "$RES" != "200" ]
then
  echo -e "Error: Vault is unreachable at $VAULT_ADDR"
  exit 1
fi

# Check success on aunthentication to vault server
RES=$(curl --write-out "%{http_code}" --silent -o /dev/null -H "X-Vault-Token: $VAULT_TOKEN" -X GET ${VAULT_ADDR}/v1/auth/token/lookup-self)
if [ "$RES" != "200" ]
then
  echo -e "Error: Cannot authenticate to vault at $VAULT_ADDR with vault token $VAULT_TOKEN"
  exit 1
fi

# Check response on secret creation in vault
RES=$(curl --write-out "%{http_code}" --silent -o /dev/null -H "X-Vault-Token: $VAULT_TOKEN" -X POST --data '{ "data": {"password": "my-long-password"} }' ${VAULT_ADDR}/v1/secret/data/creds)
if [ "$RES" != "200" ]
then
  echo -e "Error: Cannot create secret in vault $VAULT_ADDR at secret/ with vault token $VAULT_TOKEN"
  exit 1
fi

# Check vault cli, vault reachability has been checked earlier
echo -e "Checking if the vault cli commands function properly"
ret=$(vault status)
if [ $? -ne 0 ]; then
  echo -e "Error: vault cli not configured.\n $ret"
  exit 1
fi

echo -e "Vault tests passed."
