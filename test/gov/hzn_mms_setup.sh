#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# $1 is the org you want to use

export HZN_ORG_ID=${1}
export HZN_EXCHANGE_USER_AUTH=${1}/${1}admin:${1}adminpw
export HZN_SSL_SKIP_VERIFY=1
export HZN_FSS_CSSURL=${CSS_URL}
