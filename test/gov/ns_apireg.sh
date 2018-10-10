#!/bin/bash

echo -e "\nBC setting is $BC"

if [ "$BC" != "1" ]
then

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "" ]
then

# Configure the netspeed service variables, at an older version level just to be sure
# that the runtime will still pick them up for the newer version that is installed in the exchange.
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "version": "2.2.0",
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
        "var3": 10.2,
        "var4": ["abc", "123"],
        "var5": "override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeed service config payload: $snsconfig"

echo "Registering netspeed service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network",
  "version": "1.0.0",
  "organization": "IBM",
  "attributes": []
}
EOF

echo -e "\n\n[D] networkservice payload: $networkservice"

echo "Registering network service"

ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

read -d '' networkservice <<EOF
{
  "url": "https://bluehorizon.network/services/network2",
  "version": "1.0.0",
  "organization": "IBM",
  "attributes": []
}
EOF

echo -e "\n\n[D] networkservice payload: $networkservice"

echo "Registering network service"

ERR=$(echo "$networkservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  if [ "${ERR:0:22}" != "Duplicate registration" ]; then
    echo -e "error occured: $ERR"
    exit 2
  fi
fi

elif [ "$PATTERN" == "sns" ] || [ "$PATTERN" == "sall" ]
then

# Configure the netspeed service variables, at an older version level just to be sure
# that the runtime will still pick them up for the newer version that is installed in the exchange.
read -d '' snsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/netspeed",
  "version": "2.2.0",
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
        "var3": 10.2,
        "var4": ["abc", "123"],
        "var5": "override"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] netspeed service config payload: $snsconfig"

echo "Registering netspeed service config on node"

ERR=$(echo "$snsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi

fi