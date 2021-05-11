#!/bin/bash

# bootstrap the vault using a token for an admin user that has the authority to perform these steps

echo -e "vault login $VAULT_TOKEN"
vault login ${VAULT_TOKEN}

echo -e "\nvault secrets enable -version=1 -path=openhorizon kv"
vault secrets enable -version=1 -path=openhorizon kv

echo -e "\nsetup exchange auth plugin"

mkdir -p /tmp/vault
docker cp ${DOCKER_VAULT_CNAME}:/vault/plugins/hznvaultauth /tmp/vault/.

SHASUM=$(shasum -a 256 "/tmp/vault/hznvaultauth" | cut -d " " -f1)
vault write sys/plugins/catalog/openhorizon-exchange sha_256="$SHASUM" command="hznvaultauth"

vault auth enable -path=/openhorizon -plugin-name=openhorizon-exchange plugin

# Configure the plugin to point to the exchange.
echo -e "\nvault write auth/openhorizon/config url=${EXCH_APP_HOST} token=${VAULT_TOKEN}"
vault write auth/openhorizon/config url=${EXCH_APP_HOST} token=${VAULT_TOKEN}

# log out the root user
rm -f ~/.vault-token

echo "Completed vault bootstrap"