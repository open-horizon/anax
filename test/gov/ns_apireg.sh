#!/bin/bash

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

    echo -e "Pattern is set to $PATTERN"
    if [ "$PATTERN" == "" ]
    then

        # Configure the netspeed service variables, at an older version level just to be sure
        # that the runtime will still pick them up for the newer version that is installed in the exchange.
        # To test the services from different orgs with same url, we have setup 2 netspeed services.
        # IBM/netspeed depends on: IBM/nework, IBN/network2, IBM/cpu
        # e2edev@somecomp.com/netspeed depends on: e2edev@somecomp.com/network, e2edev@somecomp.com/network2, IBM/cpu e2edev@somecomp.com/cpu

        ### IBM/netspeed
        read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "aString",
        "var2": 5,
        "var3": 22.2
      }
    }
  ]
}
EOF
        echo -e "\n\n[D] IBM/netspeed service config payload: $snsconfig"
        echo "Registering IBM/netspeed service config on node"
        ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

        ### e2edev@somecomp.com/netspeed
        read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "2.2.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "node_String",
        "var2": 20,
        "var3": 23.2
      }
    }
  ]
}
EOF
        echo -e "\n\n[D] e2edev@somecomp.com/netspeed service config payload: $snsconfig"
        echo "Registering e2edev@somecomp.com/netspeed service config on node"
        ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

        ### IBM/network
        read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network",
  "versionRange": "1.0.0",
  "organization": "IBM",
  "attributes": []
}
EOF
        echo -e "\n\n[D] networkservice payload: $networkservice"
        echo "Registering IBM/network service"
        ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

        ### e2edev@somecomp.com/network
        read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network",
  "versionRange": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": []
}
EOF
        echo -e "\n\n[D] networkservice payload: $networkservice"
        echo "Registering e2edev@somecomp.com/network service"
        ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi


        ### IBM/network2
        read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network2",
  "versionRange": "1.0.0",
  "organization": "IBM",
  "attributes": []
}
EOF
        echo -e "\n\n[D] networkservice payload: $networkservice"
        echo "Registering IBM/network2 service"
        ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            if [ "${ERR:0:22}" != "Duplicate registration" ]; then
                echo -e "error occured: $ERR"
                exit 2
            fi
        fi

        ### e2edev@somecomp.com/network2
        read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network2",
  "versionRange": "1.0.0",
  "organization": "e2edev@somecomp.com",
  "attributes": []
}
EOF
        echo -e "\n\n[D] networkservice payload: $networkservice"
        echo "Registering e2edev@somecomp.com/network2 service"
        ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            if [ "${ERR:0:22}" != "Duplicate registration" ]; then
                echo -e "error occured: $ERR"
                exit 2
            fi
        fi

         ### IBM/cpu
        read -d '' slocservice <<EOF
{
    "url": "https://bluehorizon.network/service-cpu",
    "name": "cpu",
    "organization": "IBM",
    "versionRange": "1.0.0",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "cpu_var1": "ibmnodevar1"
            }
        }
    ]
}
EOF
        echo -e "\n\n[D] service based cpu service payload: $slocservice"
        echo "Registering service based IBM/cpu service"
        ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            if [ "${ERR:0:22}" != "Duplicate registration" ]; then 
                echo -e "error occured: $ERR"
                exit 2
            fi
        fi

         ### e2edev@somecomp.com/cpu
        read -d '' slocservice <<EOF
{
    "url": "https://bluehorizon.network/service-cpu",
    "name": "cpu",
    "organization": "e2edev@somecomp.com",
    "versionRange": "1.0.0",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "cpu_var1": "e2edevnodevar1"
             }
        }
    ]
}
EOF
        echo -e "\n\n[D] service based cpu service payload: $slocservice"
        echo "Registering service based e2edev@somecomp.com/cpu service"
        ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

    elif [ "$PATTERN" == "sns" ] || [ "$PATTERN" == "sall" ]
    then

        # Configure the netspeed service variables, at an older version level just to be sure
        # that the runtime will still pick them up for the newer version that is installed in the exchange.

        ### IBM/netspeed
        read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "2.2.0",
  "organization": "IBM",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "node_String",
        "var2": 20,
        "var3": 20.2
      }
    }
  ]
}
EOF
        echo -e "\n\n[D] IBM/netspeed service config payload: $snsconfig"
        echo "Registering IBM/netspeed service config on node"
        ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

        ### e2edev@somecomp.com/netspeed
        read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "versionRange": "2.2.0",
  "organization": "e2edev@somecomp.com",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "var1": "node_String",
        "var2": 21,
        "var3": 21.2
      }
    }
  ]
}
EOF
        echo -e "\n\n[D] e2edev@somecomp.com/netspeed service config payload: $snsconfig"
        echo "Registering e2edev@somecomp.com/netspeed service config on node"
        ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

         ### IBM/cpu
        read -d '' slocservice <<EOF
{
    "url": "https://bluehorizon.network/service-cpu",
    "name": "cpu",
    "organization": "IBM",
    "versionRange": "1.0.0",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "cpu_var1": "ibmnodevar1"
            }
        }
    ]
}
EOF
        echo -e "\n\n[D] service based cpu service payload: $slocservice"
        echo "Registering service based IBM/cpu service"
        ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            if [ "${ERR:0:22}" != "Duplicate registration" ]; then 
                echo -e "error occured: $ERR"
                exit 2
            fi
        fi
        
         ### e2edev@somecomp.com/cpu
        read -d '' slocservice <<EOF
{
    "url": "https://bluehorizon.network/service-cpu",
    "name": "cpu",
    "organization": "e2edev@somecomp.com",
    "versionRange": "1.0.0",
    "attributes": [
        {
            "type": "UserInputAttributes",
            "label": "User input variables",
            "publishable": false,
            "host_only": false,
            "mappings": {
                "cpu_var1": "e2edevnodevar1"
             }
        }
    ]
}
EOF
        echo -e "\n\n[D] service based cpu service payload: $slocservice"
        echo "Registering service based e2edev@somecomp.com/cpu service"
        ERR=$(echo "$slocservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

    fi
fi