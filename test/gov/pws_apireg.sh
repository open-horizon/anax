#!/bin/bash

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "pws" ] || [ "$PATTERN" == "all" ]; then

# and then configure the workload variables
read -d '' splitpwsworkload <<EOF
{
  "workload_url": "https://bluehorizon.network/workloads/weather",
  "workload_version": "1.0.0",
  "organization": "e2edev",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "HZN_WUGNAME": "e2edev mocked pws",
        "HZN_PWS_MODEL": "LaCrosse WS2317",
        "MTN_PWS_MODEL": "LaCrosse WS2317",
        "HZN_PWS_ST_TYPE": "WS23xx",
        "MTN_PWS_ST_TYPE": "WS23xx"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] split pws workload payload: $splitpwsworkload"

echo "Registering split pws workload"

RES=$(echo "$splitpwsworkload" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/workload/config" | jq -r '.')
if [ "$RES" == "" ]; then
  echo -e "error occured: $RES"
  exit 2
fi

elif [ "$PATTERN" == "spws" ] || [ "$PATTERN" == "sall" ]
then

# Configure the weather service variables
read -d '' spwsconfig <<EOF
{
  "url": "https://bluehorizon.network/services/weather",
  "version": "1.5.0",
  "organization": "e2edev",
  "attributes": [
    {
      "type": "UserInputAttributes",
      "label": "User input variables",
      "publishable": false,
      "host_only": false,
      "mappings": {
        "HZN_WUGNAME": "e2edev mocked pws",
        "HZN_PWS_MODEL": "LaCrosse WS2317",
        "MTN_PWS_MODEL": "LaCrosse WS2317",
        "HZN_PWS_ST_TYPE": "WS23xx",
        "MTN_PWS_ST_TYPE": "WS23xx"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] weather service config payload: $spwsconfig"

echo "Registering weather service config on node"

ERR=$(echo "$spwsconfig" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

fi