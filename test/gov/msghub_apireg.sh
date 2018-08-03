#!/bin/bash

MH_SAMPLES_PATH="/root/examples/edge/msghub"
export HZN_ORG_ID=IBM
export MYDOMAIN="bluehorizon.network"

export CPU2MSGHUB_NAME=cpu2msghub   # the name of the service, used in the docker image path and in the service url
export CPU2MSGHUB_VERSION=1.2.2   # the service version, and also used as the tag for the docker image. Must be in OSGI version format.

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "cpu2msghub" ]; then

    echo "Registering cpu2msghub service config on node"

    read -d '' cpu2msghubconfig <<EOF
{
    "url": "https://$MYDOMAIN/service-$CPU2MSGHUB_NAME",
    "version": "$CPU2MSGHUB_VERSION",
    "organization": "IBM",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "MOCK": false,
                "MSGHUB_API_KEY": "$MSGHUB_API_KEY",
                "PUBLISH": true,
                "SAMPLE_INTERVAL": 2,
                "SAMPLE_SIZE": 5,
                "VERBOSE": "1"
            }
        }
    ]
}
EOF

    echo -e "\n\n[D] cpu2msghub service config payload: $cpu2msghubconfig"


    echo "Registering cpu2msghub service config on node"

    ERR=$(echo "$cpu2msghubconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
        echo -e "error occured: $ERR"
        exit 2
    fi

fi
