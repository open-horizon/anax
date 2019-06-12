#!/bin/bash

# $1 is the org you want to use

export HZN_ORG_ID=${1}
export HZN_EXCHANGE_USER_AUTH=${1}/${1}admin:${1}adminpw
export HZN_SSL_SKIP_VERIFY=1
export HZN_FSS_CSSURL=https://css-api:9443
