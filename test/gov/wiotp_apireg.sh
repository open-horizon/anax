#!/bin/bash

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "e2egwtype" ]
then

  # Configure the  service variables
  read -d '' cpu2wiotconfig <<EOF
{
    "url": "https://internetofthings.ibmcloud.com/services/cpu2wiotp",
    "version": "1.2.2",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "WIOTP_GW_TOKEN": "$WIOTP_GW_TOKEN"
            }
        }
    ]
}
EOF

    echo -e "\n\n[D] e2egwtype service config payload: $cpu2wiotconfig"

    echo "Registering e2egwtype service config on node"

    ERR=$(echo "$cpu2wiotconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
        echo -e "error occured: $ERR"
        exit 2
    fi

 read -d '' coreiotconfig <<EOF
{
    "url": "https://internetofthings.ibmcloud.com/wiotp-edge/services/core-iot",
    "organization": "IBM",
    "version": "2.4.0",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "WIOTP_DEVICE_AUTH_TOKEN": "$WIOTP_GW_TOKEN",
                "WIOTP_DOMAIN": "$WIOTP_ORG_ID.messaging.internetofthings.ibmcloud.com"
            }
        }
    ]
}
EOF

    echo -e "\n\n[D] e2egwtype service config payload: $cpu2wiotconfig"

    echo "Registering e2egwtype service config on node"

    ERR=$(echo "$coreiotconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
     echo -e "error occured: $ERR"
        exit 2
    fi
elif [ "$PATTERN" == "e2egwtypenocore" ]
then

  read -d '' cpu2wiotconfig <<EOF
{
    "url": "https://internetofthings.ibmcloud.com/services/cpu2wiotp-no-core-iot",
    "version": "1.2.2",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "WIOTP_GW_TOKEN": "$WIOTP_GW_TOKEN"
            }
        }
    ]
}
EOF

    echo -e "\n\n[D] e2egwtype service config payload: $cpu2wiotconfig"

    echo "Registering e2egwtype service config on node"

    ERR=$(echo "$cpu2wiotconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
        echo -e "error occured: $ERR"
        exit 2
    fi

fi
