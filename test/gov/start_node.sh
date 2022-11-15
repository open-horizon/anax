#!/bin/bash

# Please set the following env variable before calling this script.
# For example:
# export USER=anax1
# export PASS=anax1pw
# export DEVICE_ID="an12345"
# export DEVICE_ORG="e2edev@somecomp.com"
# export DEVICE_NAME="anaxdev1"
# export ANAX_API="http://localhost:${HZN_AGENT_PORT}"
# export EXCH="${EXCH_APP_HOST}"
# export TOKEN="Abcdefghijklmno1"
# This env var can be changed to whatever pattern you want to run.
# export PATTERN="sall"

# create an HA group in the exchange with 2 nodes: an12345 and an54321
function create_HA_group {
    export HZN_EXCHANGE_URL="${EXCH_APP_HOST}"
    E2EDEV_ADMIN_AUTH="e2edev@somecomp.com/e2edevadmin:e2edevadminpw"
    USERDEV_ADMIN_AUTH="userdev/userdevadmin:userdevadminpw"

    if [ $DEVICE_ORG == "userdev" ]; then
        auth=${USERDEV_ADMIN_AUTH}
    else
        auth=${E2EDEV_ADMIN_AUTH}
    fi
    read -d '' hagroup <<EOF
{
    "description": "HA group testing",
    "members": [
      "an12345",
      "an54321"
    ]
}
EOF
    echo "$hagroup" | hzn exchange hagroup add -f- group1 -o $DEVICE_ORG -u $auth

}


nohup ./start_anax_loop.sh 1 &>/dev/null &

sleep 5

# Make sure org is null
DST=$(curl -sSL $ANAX_API/node | jq -r '.')
THEORG=$(echo "$DST" | jq -r '.organization')
if [ "$THEORG" != "null" ]
then
  echo -e "organization should be empty: $THEORG"
  exit 2
fi

# Setup anax itself through APIs.
if [ "$HA" == "1" ]
then
    # create an HA group
    create_HA_group

    ./apireg.sh

    if [ $? -ne 0 ]
    then
        echo "HA registration failed"
        TESTFAIL="1"
        exit 2
    else
        echo "Anax1 ready to run workloads."
        sleep 5

        echo "Starting Anax2."
        export DEVICE_ID="an54321"
        export DEVICE_NAME="anaxdev2"
        export HZN_AGENT_PORT=8511
        export ANAX_API="http://localhost:${HZN_AGENT_PORT}"

        # start anax2
        nohup ./start_anax_loop.sh 2 &>/dev/null &

        sleep 5

        ./apireg.sh
        if [ $? -ne 0 ]
        then
            TESTFAIL="1"
            exit 2
        fi
    fi

else
    ./apireg.sh
    if [ $? -ne 0 ]
    then
        TESTFAIL="1"
        exit 2
    fi
fi
