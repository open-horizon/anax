#!/bin/bash

echo -e "Pattern is set to $PATTERN"

if [ "$PATTERN" == "spws" ] || [ "$PATTERN" == "sall" ] || [ "$PATTERN" == "" ]
then
  read -d '' nodeui <<EOF
[
    {
      "serviceOrgid": "e2edev@somecomp.com",
      "serviceUrl": "https://bluehorizon.network/services/weather",
      "serviceArch": "amd64",
      "serviceVersionRange": "1.5.0",
      "inputs": [
        {
          "name": "HZN_LAT",
          "value": 41.921766
        },
        {
          "name": "HZN_LON",
          "value": -73.894224
        },
        {
          "name": "HZN_LOCATION_ACCURACY_KM",
          "value": 0.5
        },
        {
          "name": "HZN_USE_GPS",
          "value": false
        },
        {
          "name": "HZN_WUGNAME",
          "value": "e2edev mocked pws"
        },
        {
          "name": "HZN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "MTN_PWS_MODEL",
          "value": "LaCrosse WS2317"
        },
        {
          "name": "HZN_PWS_ST_TYPE",
          "value": "WS23xx"
        },
        {
          "name": "MTN_PWS_ST_TYPE",
          "value": "WS23xx"
        }
      ]
    }
]

EOF
  echo "Adding service configuration for weather with /node/userinput api..."
  RES=$(echo "$nodeui" | curl -sS -X PATCH -w "%{http_code}" -H "Content-Type: application/json" --data @- "$ANAX_API/node/userinput")
  if [ "$RES" == "" ]
  then
    echo -e "$newhznpolicy \nresulted in empty response"
    exit 2
  fi

  ERR=$(echo $RES | jq -r '.' | tail -1)
  if [ "$ERR" != "201" ]
  then
    echo -e "$nodeui \nresulted in incorrect response: $RES"
  else
    echo -e "found expected response: $RES"
  fi

fi
