#!/bin/bash

echo -e "Pattern is set to $PATTERN"
if [ "$PATTERN" == "" ]
then

read -d '' pwsservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/microservices/weathersim",
  "sensor_name": "weathersim",
  "sensor_version": "1.0.0",
  "attributes": [
    {
      "type": "ComputeAttributes",
      "label": "Compute Resources",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "ram": 256,
        "cpus": 1
      }
    },
    {
      "type": "MeteringAttributes",
      "label": "Metering Policy",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "tokens": 2,
        "perTimeUnit": "hour",
        "notificationInterval": 3600
      }
    },
    {
      "type": "UserInputAttributes",
      "label": "app",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "HZN_WUGNAME": "e2edev mocked pws",
        "HZN_PWS_MODEL": "LaCrosse WS2317",
        "MTN_PWS_MODEL": "LaCrosse WS2317",
        "HZN_PWS_ST_TYPE": "WS23xx",
        "MTN_PWS_ST_TYPE": "WS23xx"
      }
    },
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] pwsservice payload: $pwsservice"

echo "Registering pws service"

echo "$pwsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config"

# using a pattern, no policy. Only service patterns are supported.
elif [ "$PATTERN" = "spws" ]; then

read -d '' pwsservice <<EOF
{
  "url": "https://bluehorizon.network/services/weather",
  "version": "1.5.0",
  "organization": "e2edev@somecomp.com",
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
    },
    {
      "type": "HAAttributes",
      "label": "HA Partner",
      "publishable": true,
      "host_only": false,
      "mappings": {
        "partnerID": ["$PARTNERID"]
      }
    }
  ]
}
EOF

echo -e "\n\n[D] pwsservice payload: $pwsservice"

echo "Registering pws service"

ERR=$(echo "$pwsservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/service/config" | jq -r '.error')
if [ "$ERR" != "null" ]; then
  echo -e "error occured: $ERR"
  exit 2
fi

else
  echo "Non weather pattern, nothing to do."
fi