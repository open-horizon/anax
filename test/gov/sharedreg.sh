#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

EMAIL="foo@goo.com"

  echo "Calling node API"

if curl -sS -H "Content-Type: application/json" "$ANAX_API/node" | jq -er '. | .account.id' > /dev/null; then
  read -dr '' updatehzntoken <<EOF
{
  "account": {
    "id": "$USER"
  },
  "token": "$TOKEN"
}
EOF

  echo -e "\n[D] hzntoken payload: $updatehzntoken"

  echo "Setting device id and token into horizon API"

  echo "$updatehzntoken" | curl -sS -X PATCH -H "Content-Type: application/json" --data @- "$ANAX_API/node"

else

  read -dr '' newhzndevice <<EOF
{
  "account": {
    "id": "$USER",
    "email": "$EMAIL"
  },
  "id": "$DEVICE_ID",
  "name": "$DEVICE_NAME",
  "token": "$TOKEN"
}
EOF

  echo -e "\n[D] hzndevice payload: $newhzndevice"

  echo "Updating horizon with out device id and token"

  echo "$newhzndevice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/node"
fi

read -dr '' gpstestservice <<EOF
{
  "sensor_url": "https://bluehorizon.network/documentation/gpstest-device-api",
  "sensor_name": "gpstest",
  "attributes": [
    {
      "id": "free form",
      "short_type": "mapped",
      "label": "Extra",
      "publishable": true,
      "mappings": {
        "foo": "goo",
        "new": "zoo"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] gpstestservice payload: $gpstestservice"

echo "Registering gpstest service"

echo "$gpstestservice" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config"

read -dr '' location2service <<EOF
{
  "sensor_url": "https://bluehorizon.network/documentation/location2-device-api",
  "sensor_name": "location2",
  "attributes": [
    {
      "id": "free form",
      "short_type": "mapped",
      "label": "Extra",
      "publishable": true,
      "mappings": {
        "foo": "goo",
        "new": "zoo"
      }
    }
  ]
}
EOF

echo -e "\n\n[D] location2service payload: $location2service"

echo "Registering location2 service"

echo "$location2service" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/microservice/config"

echo -e "\n\n[D] all registered attributes:\n"
curl -sS -H "Content-Type: application/json" "$ANAX_API/attribute" | jq -r '.'
